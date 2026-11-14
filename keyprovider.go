package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
	"golang.org/x/term"
)

const (
	providerAuto       = "auto"
	providerKeyring    = "keyring"
	providerPassphrase = "passphrase"
	providerPlaintext  = "plaintext"

	keyringService = "2fa"
	keyringProbe   = "probe:availability"
)

// KeyProvider создаёт, открывает и удаляет ключ шифрования vault.
type KeyProvider interface {
	Name() string
	Create(vaultID string) (key []byte, kdf *KDFParams, err error)
	Open(header *VaultHeader) ([]byte, error)
	Cleanup(vaultID string) error
}

type ProviderOptions struct {
	Stdin           io.Reader
	Stderr          io.Writer
	StdinIsTTY      bool
	PassphraseStdin bool
}

type keyringStore interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (osKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (osKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

var keyringBackend keyringStore = osKeyring{}

func keyringAvailable() bool {
	_, err := keyringBackend.Get(keyringService, keyringProbe)
	return err == nil || errors.Is(err, keyring.ErrNotFound)
}

func newKeyProvider(name string, opts ProviderOptions) (KeyProvider, error) {
	switch name {
	case providerKeyring:
		return keyringProvider{}, nil
	case providerPassphrase:
		return passphraseProvider{opts: opts}, nil
	case providerPlaintext:
		return plaintextProvider{}, nil
	default:
		return nil, fmt.Errorf("неизвестный провайдер ключа %q", name)
	}
}

func resolveProvider(requested string) (string, error) {
	switch requested {
	case "", providerAuto:
		if keyringAvailable() {
			return providerKeyring, nil
		}
		return providerPassphrase, nil
	case providerKeyring:
		if !keyringAvailable() {
			return "", errors.New("keyring недоступен; используйте --key-provider passphrase")
		}
		return providerKeyring, nil
	case providerPassphrase, providerPlaintext:
		return requested, nil
	default:
		return "", fmt.Errorf("неизвестный провайдер ключа %q (допустимо: auto, keyring, passphrase, plaintext)", requested)
	}
}

func deriveKey(passphrase string, kdf *KDFParams) ([]byte, error) {
	if kdf == nil || kdf.Name != "argon2id" {
		return nil, errors.New("в vault отсутствуют корректные параметры KDF")
	}
	if kdf.KeyLen != 32 {
		return nil, fmt.Errorf("неподдерживаемая длина ключа KDF: %d", kdf.KeyLen)
	}
	if kdf.Time < 1 || kdf.Time > 10 {
		return nil, fmt.Errorf("недопустимое число итераций KDF: %d", kdf.Time)
	}
	if kdf.Threads < 1 || kdf.Threads > 16 {
		return nil, fmt.Errorf("недопустимое число потоков KDF: %d", kdf.Threads)
	}
	if kdf.Memory < 8*1024 || kdf.Memory > 1024*1024 {
		return nil, fmt.Errorf("недопустимый объём памяти KDF: %d КиБ", kdf.Memory)
	}
	salt, err := base64.StdEncoding.DecodeString(kdf.Salt)
	if err != nil {
		return nil, fmt.Errorf("повреждённая соль KDF: %w", err)
	}
	if len(salt) < 16 || len(salt) > 64 {
		return nil, fmt.Errorf("недопустимая длина соли KDF: %d байт", len(salt))
	}
	return argon2.IDKey([]byte(passphrase), salt, kdf.Time, kdf.Memory, kdf.Threads, kdf.KeyLen), nil
}

// readLine читает одну строку, не заглядывая дальше разделителя.
// Это важно, когда из одного stdin последовательно читаются секрет и пароль:
// bufio.Reader мог бы забрать вторую строку в свой буфер и потерять её.
func readLine(r io.Reader) (string, error) {
	var line strings.Builder
	var buf [1]byte
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			if buf[0] == '\n' {
				return strings.TrimSuffix(line.String(), "\r"), nil
			}
			line.WriteByte(buf[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return strings.TrimSuffix(line.String(), "\r"), nil
			}
			return "", err
		}
	}
}

func readHiddenOrLine(opts ProviderOptions, prompt string, confirm bool, forceLine bool) (string, error) {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if forceLine {
		return readLine(opts.Stdin)
	}
	if opts.StdinIsTTY {
		if f, ok := opts.Stdin.(*os.File); ok {
			return readPasswordFromTerminal(f, opts.Stderr, prompt, confirm)
		}
	}
	return readLine(opts.Stdin)
}

func readPasswordFromTerminal(f *os.File, w io.Writer, prompt string, confirm bool) (string, error) {
	if _, err := fmt.Fprint(w, prompt); err != nil {
		return "", err
	}
	first, err := term.ReadPassword(int(f.Fd()))
	if _, werr := fmt.Fprintln(w); werr != nil && err == nil {
		err = werr
	}
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать скрытый ввод: %w", err)
	}
	value := string(first)
	if !confirm {
		return value, nil
	}
	if _, err := fmt.Fprint(w, "Повторите ввод: "); err != nil {
		return "", err
	}
	second, err := term.ReadPassword(int(f.Fd()))
	if _, werr := fmt.Fprintln(w); werr != nil && err == nil {
		err = werr
	}
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать скрытый ввод: %w", err)
	}
	if value != string(second) {
		return "", errors.New("значения не совпадают")
	}
	return value, nil
}

func readPassphrase(opts ProviderOptions, prompt string, confirm bool) (string, error) {
	if value := os.Getenv("2FA_PASSPHRASE"); value != "" {
		return value, nil
	}
	value, err := readHiddenOrLine(opts, prompt, confirm, opts.PassphraseStdin)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.New("пароль не может быть пустым")
	}
	return value, nil
}

func readSecretValue(opts ProviderOptions, prompt string, confirm bool, forceLine bool) (string, error) {
	return readHiddenOrLine(opts, prompt, confirm, forceLine)
}

type keyringProvider struct{}

func (keyringProvider) Name() string { return providerKeyring }

func keyringUser(vaultID string) string {
	return "vault:" + vaultID
}

func (keyringProvider) Create(vaultID string) ([]byte, *KDFParams, error) {
	key, err := randomBytes(32)
	if err != nil {
		return nil, nil, err
	}
	if err := keyringBackend.Set(keyringService, keyringUser(vaultID), base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, nil, fmt.Errorf("не удалось сохранить ключ в keyring: %w", err)
	}
	return key, nil, nil
}

func (keyringProvider) Open(header *VaultHeader) ([]byte, error) {
	value, err := keyringBackend.Get(keyringService, keyringUser(header.VaultID))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, errors.New("ключ vault не найден в keyring (возможно, файл перенесён с другой машины)")
		}
		return nil, fmt.Errorf("не удалось получить ключ из keyring: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("повреждённый ключ в keyring: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("неверная длина ключа в keyring: %d", len(key))
	}
	return key, nil
}

func (keyringProvider) Cleanup(vaultID string) error {
	err := keyringBackend.Delete(keyringService, keyringUser(vaultID))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

type passphraseProvider struct {
	opts ProviderOptions
}

func (passphraseProvider) Name() string { return providerPassphrase }

func (p passphraseProvider) Create(string) ([]byte, *KDFParams, error) {
	passphrase, err := readPassphrase(p.opts, "Новый пароль vault: ", true)
	if err != nil {
		return nil, nil, err
	}
	salt, err := randomBytes(16)
	if err != nil {
		return nil, nil, err
	}
	kdf := &KDFParams{
		Name:    "argon2id",
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  32,
	}
	key, err := deriveKey(passphrase, kdf)
	if err != nil {
		return nil, nil, err
	}
	return key, kdf, nil
}

func (p passphraseProvider) Open(header *VaultHeader) ([]byte, error) {
	passphrase, err := readPassphrase(p.opts, "Пароль vault: ", false)
	if err != nil {
		return nil, err
	}
	return deriveKey(passphrase, header.KDF)
}

func (passphraseProvider) Cleanup(string) error { return nil }

type plaintextProvider struct{}

func (plaintextProvider) Name() string { return providerPlaintext }

func (plaintextProvider) Create(string) ([]byte, *KDFParams, error) {
	return nil, nil, nil
}

func (plaintextProvider) Open(*VaultHeader) ([]byte, error) { return nil, nil }

func (plaintextProvider) Cleanup(string) error { return nil }
