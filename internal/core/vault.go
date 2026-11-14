package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const vaultFormatVersion = 1

var (
	ErrVaultNotFound = errors.New("хранилище не инициализировано")
	ErrVaultExists   = errors.New("хранилище уже инициализировано")
)

type KDFParams struct {
	Name    string `json:"name"`
	Salt    string `json:"salt"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"key_len"`
}

type VaultHeader struct {
	Version    int        `json:"version"`
	VaultID    string     `json:"vault_id"`
	Provider   string     `json:"provider"`
	Cipher     string     `json:"cipher"`
	Nonce      string     `json:"nonce,omitempty"`
	KDF        *KDFParams `json:"kdf,omitempty"`
	Ciphertext string     `json:"ciphertext"`
}

type Vault struct {
	Path     string
	Header   VaultHeader
	Payload  *Payload
	key      []byte
	provider KeyProvider
	created  bool
}

func defaultVaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить каталог конфигурации: %w", err)
	}
	return filepath.Join(dir, "2fa", "accounts.json"), nil
}

func defaultVaultPathString() string {
	path, err := defaultVaultPath()
	if err != nil {
		return "~/.config/2fa/accounts.json"
	}
	return path
}

func randomBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("не удалось получить случайные данные: %w", err)
	}
	return buf, nil
}

func newVaultID() (string, error) {
	buf, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type vaultAAD struct {
	Version  int        `json:"version"`
	VaultID  string     `json:"vault_id"`
	Provider string     `json:"provider"`
	Cipher   string     `json:"cipher"`
	KDF      *KDFParams `json:"kdf,omitempty"`
}

func (h *VaultHeader) aad() ([]byte, error) {
	return json.Marshal(vaultAAD{
		Version:  h.Version,
		VaultID:  h.VaultID,
		Provider: h.Provider,
		Cipher:   h.Cipher,
		KDF:      h.KDF,
	})
}

func encryptPayload(header *VaultHeader, payload *Payload, key []byte) error {
	plain, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("не удалось сериализовать хранилище: %w", err)
	}
	if header.Cipher == "none" {
		header.Nonce = ""
		header.Ciphertext = base64.StdEncoding.EncodeToString(plain)
		return nil
	}
	if header.Cipher != "aes-256-gcm" {
		return fmt.Errorf("неподдерживаемый шифр %q", header.Cipher)
	}
	if len(key) != 32 {
		return fmt.Errorf("неверная длина ключа шифрования: %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("не удалось инициализировать шифр: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("не удалось инициализировать GCM: %w", err)
	}
	nonce, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return err
	}
	aad, err := header.aad()
	if err != nil {
		return err
	}
	header.Nonce = base64.StdEncoding.EncodeToString(nonce)
	header.Ciphertext = base64.StdEncoding.EncodeToString(gcm.Seal(nil, nonce, plain, aad))
	return nil
}

func decryptPayload(header *VaultHeader, key []byte) (*Payload, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(header.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("повреждённое поле ciphertext: %w", err)
	}
	if header.Cipher == "none" {
		var payload Payload
		if err := json.Unmarshal(ciphertext, &payload); err != nil {
			return nil, fmt.Errorf("повреждённые данные хранилища: %w", err)
		}
		payload.ensure()
		return &payload, nil
	}
	if header.Cipher != "aes-256-gcm" {
		return nil, fmt.Errorf("неподдерживаемый шифр %q", header.Cipher)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("неверная длина ключа шифрования: %d", len(key))
	}
	nonce, err := base64.StdEncoding.DecodeString(header.Nonce)
	if err != nil {
		return nil, fmt.Errorf("повреждённое поле nonce: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать шифр: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать GCM: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("неверная длина nonce")
	}
	aad, err := header.aad()
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("не удалось расшифровать хранилище (неверный пароль или повреждённый файл): %w", err)
	}
	var payload Payload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, fmt.Errorf("повреждённые данные хранилища: %w", err)
	}
	payload.ensure()
	return &payload, nil
}

func validateVaultHeader(header *VaultHeader) error {
	switch header.Provider {
	case providerPlaintext:
		if header.Cipher != "none" {
			return fmt.Errorf("несовместимые провайдер и шифр vault: %q/%q", header.Provider, header.Cipher)
		}
	case providerKeyring, providerPassphrase:
		if header.Cipher != "aes-256-gcm" {
			return fmt.Errorf("несовместимые провайдер и шифр vault: %q/%q", header.Provider, header.Cipher)
		}
	default:
		return fmt.Errorf("неподдерживаемый провайдер ключа vault: %q", header.Provider)
	}
	return nil
}

func writeVaultFile(path string, header VaultHeader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("не удалось создать каталог хранилища: %w", err)
	}
	data, err := json.MarshalIndent(header, "", "  ")
	if err != nil {
		return fmt.Errorf("не удалось сериализовать vault: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("не удалось создать временный файл: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("не удалось записать vault: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		// Windows не заменяет существующий файл при rename.
		if removeErr := os.Remove(path); removeErr == nil {
			err = os.Rename(tmpName, path)
		}
		if err != nil {
			return fmt.Errorf("не удалось заменить файл хранилища: %w", err)
		}
	}
	return os.Chmod(path, 0600)
}

func (v *Vault) Save() error {
	if err := encryptPayload(&v.Header, v.Payload, v.key); err != nil {
		return err
	}
	return writeVaultFile(v.Path, v.Header)
}

func ReadVaultHeader(path string) (*VaultHeader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrVaultNotFound, path)
		}
		return nil, fmt.Errorf("не удалось прочитать хранилище: %w", err)
	}
	var header VaultHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("повреждённый файл хранилища: %w", err)
	}
	if header.Version != vaultFormatVersion {
		return nil, fmt.Errorf("неподдерживаемая версия формата vault: %d", header.Version)
	}
	if header.VaultID == "" {
		return nil, errors.New("в заголовке vault отсутствует vault_id")
	}
	if err := validateVaultHeader(&header); err != nil {
		return nil, err
	}
	return &header, nil
}

func openVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	headerPtr, err := ReadVaultHeader(path)
	if err != nil {
		return nil, err
	}
	header := *headerPtr
	if requestedProvider != "" && requestedProvider != providerAuto && requestedProvider != header.Provider {
		return nil, fmt.Errorf("vault использует провайдер %q, а указан %q; для смены провайдера используйте 2fa rekey", header.Provider, requestedProvider)
	}
	provider, err := newKeyProvider(header.Provider, opts)
	if err != nil {
		return nil, err
	}
	key, err := provider.Open(&header)
	if err != nil {
		return nil, err
	}
	payload, err := decryptPayload(&header, key)
	if err != nil {
		return nil, err
	}
	return &Vault{
		Path:     path,
		Header:   header,
		Payload:  payload,
		key:      key,
		provider: provider,
	}, nil
}

func (v *Vault) WasCreated() bool {
	return v.created
}

// Close drops the decrypted payload and overwrites the in-memory key.
func (v *Vault) Close() {
	for i := range v.key {
		v.key[i] = 0
	}
	v.key = nil
	v.Payload = nil
	v.provider = nil
}

func createVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrVaultExists, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("не удалось проверить хранилище: %w", err)
	}
	resolved, err := resolveProvider(requestedProvider)
	if err != nil {
		return nil, err
	}
	vaultID, err := newVaultID()
	if err != nil {
		return nil, err
	}
	provider, err := newKeyProvider(resolved, opts)
	if err != nil {
		return nil, err
	}
	key, kdf, err := provider.Create(vaultID)
	if err != nil {
		return nil, err
	}
	cipherName := "aes-256-gcm"
	if resolved == providerPlaintext {
		cipherName = "none"
	}
	header := VaultHeader{
		Version:  vaultFormatVersion,
		VaultID:  vaultID,
		Provider: resolved,
		Cipher:   cipherName,
		KDF:      kdf,
	}
	payload := newPayload()
	if err := encryptPayload(&header, payload, key); err != nil {
		_ = provider.Cleanup(vaultID)
		return nil, err
	}
	if err := writeVaultFile(path, header); err != nil {
		_ = provider.Cleanup(vaultID)
		return nil, err
	}
	return &Vault{
		Path:     path,
		Header:   header,
		Payload:  payload,
		key:      key,
		provider: provider,
		created:  true,
	}, nil
}

func openOrCreateVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	vault, err := openVault(path, requestedProvider, opts)
	if err == nil {
		return vault, nil
	}
	if !errors.Is(err, ErrVaultNotFound) {
		return nil, err
	}
	return createVault(path, requestedProvider, opts)
}

func rekeyVault(vault *Vault, target string, opts ProviderOptions) error {
	if target == vault.Header.Provider {
		return fmt.Errorf("vault уже использует провайдер %q", target)
	}
	resolved, err := resolveProvider(target)
	if err != nil {
		return err
	}
	if resolved == vault.Header.Provider {
		return fmt.Errorf("vault уже использует провайдер %q", resolved)
	}
	newProvider, err := newKeyProvider(resolved, opts)
	if err != nil {
		return err
	}
	newKey, kdf, err := newProvider.Create(vault.Header.VaultID)
	if err != nil {
		return err
	}
	fail := func(err error) error {
		_ = newProvider.Cleanup(vault.Header.VaultID)
		return err
	}
	header := vault.Header
	header.Provider = resolved
	header.KDF = kdf
	if resolved == providerPlaintext {
		header.Cipher = "none"
	} else {
		header.Cipher = "aes-256-gcm"
	}
	header.Nonce = ""
	header.Ciphertext = ""
	if err := encryptPayload(&header, vault.Payload, newKey); err != nil {
		return fail(err)
	}
	if err := writeVaultFile(vault.Path, header); err != nil {
		return fail(err)
	}
	if err := vault.provider.Cleanup(vault.Header.VaultID); err != nil && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "Предупреждение: не удалось удалить старый ключ: %v\n", err)
	}
	vault.Header = header
	vault.key = newKey
	vault.provider = newProvider
	return nil
}
