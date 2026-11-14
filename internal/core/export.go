package core

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	ExportFormat        = "2fa"
	exportFormatVersion = 1
	exportCipher        = "aes-256-gcm"
)

// ExportEnvelope is the password-encrypted .2fa interchange format.
// It is deliberately independent from the live vault provider so the file
// can be moved between machines even when the vault uses an OS keyring.
type ExportEnvelope struct {
	Format     string     `json:"format"`
	Version    int        `json:"version"`
	KDF        *KDFParams `json:"kdf"`
	Cipher     string     `json:"cipher"`
	Nonce      string     `json:"nonce"`
	Ciphertext string     `json:"ciphertext"`
}

type exportAAD struct {
	Format  string     `json:"format"`
	Version int        `json:"version"`
	KDF     *KDFParams `json:"kdf"`
	Cipher  string     `json:"cipher"`
}

// ImportStats describes how many records were applied/skipped by Payload.Import.
type ImportStats struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

func exportAADBytes(envelope *ExportEnvelope) ([]byte, error) {
	return json.Marshal(exportAAD{
		Format:  envelope.Format,
		Version: envelope.Version,
		KDF:     envelope.KDF,
		Cipher:  envelope.Cipher,
	})
}

// EncryptExport serializes payload into the password-protected .2fa format.
func EncryptExport(payload *Payload, password string) ([]byte, error) {
	if payload == nil {
		return nil, errors.New("нет данных для экспорта")
	}
	if password == "" {
		return nil, errors.New("пароль экспорта не может быть пустым")
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("не удалось сериализовать экспорт: %w", err)
	}
	salt, err := randomBytes(16)
	if err != nil {
		return nil, err
	}
	kdf := &KDFParams{
		Name:    "argon2id",
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  32,
	}
	key, err := deriveKey(password, kdf)
	if err != nil {
		return nil, err
	}
	envelope := &ExportEnvelope{
		Format:  ExportFormat,
		Version: exportFormatVersion,
		KDF:     kdf,
		Cipher:  exportCipher,
	}
	aad, err := exportAADBytes(envelope)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать шифр экспорта: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать GCM экспорта: %w", err)
	}
	nonce, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	envelope.Nonce = base64.StdEncoding.EncodeToString(nonce)
	envelope.Ciphertext = base64.StdEncoding.EncodeToString(gcm.Seal(nil, nonce, plain, aad))
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("не удалось сериализовать контейнер экспорта: %w", err)
	}
	return append(data, '\n'), nil
}

// DecryptExport reads and validates the password-protected .2fa format.
func DecryptExport(data []byte, password string) (*Payload, error) {
	if password == "" {
		return nil, errors.New("пароль экспорта не может быть пустым")
	}
	var envelope ExportEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("повреждённый файл экспорта: %w", err)
	}
	if envelope.Format != ExportFormat {
		return nil, fmt.Errorf("неподдерживаемый формат экспорта %q", envelope.Format)
	}
	if envelope.Version != exportFormatVersion {
		return nil, fmt.Errorf("неподдерживаемая версия экспорта: %d", envelope.Version)
	}
	if envelope.Cipher != exportCipher {
		return nil, fmt.Errorf("неподдерживаемый шифр экспорта %q", envelope.Cipher)
	}
	if envelope.KDF == nil {
		return nil, errors.New("в файле экспорта отсутствуют параметры KDF")
	}
	key, err := deriveKey(password, envelope.KDF)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, fmt.Errorf("повреждённое поле nonce экспорта: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("повреждённое поле ciphertext экспорта: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать шифр экспорта: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать GCM экспорта: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("неверная длина nonce экспорта")
	}
	aad, err := exportAADBytes(&envelope)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("не удалось расшифровать экспорт (неверный пароль или повреждённый файл)")
	}
	var payload Payload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, fmt.Errorf("повреждённые данные экспорта: %w", err)
	}
	payload.ensure()
	if err := payload.NormalizeAndValidate(); err != nil {
		return nil, err
	}
	return &payload, nil
}

// WriteExport encrypts payload and atomically writes it to path with mode 0600.
func WriteExport(path string, payload *Payload, password string) error {
	if path == "" {
		return errors.New("не указан путь для экспорта")
	}
	data, err := EncryptExport(payload, password)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("не удалось создать каталог экспорта: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".2fa-export-*.tmp")
	if err != nil {
		return fmt.Errorf("не удалось создать временный файл экспорта: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("не удалось записать экспорт: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		if removeErr := os.Remove(path); removeErr == nil {
			err = os.Rename(tmpName, path)
		}
		if err != nil {
			return fmt.Errorf("не удалось заменить файл экспорта: %w", err)
		}
	}
	return os.Chmod(path, 0600)
}

// ReadExport decrypts a .2fa file.
func ReadExport(path, password string) (*Payload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл экспорта: %w", err)
	}
	return DecryptExport(data, password)
}

// NormalizeAndValidate checks all account names/secrets and fills timestamps.
func (p *Payload) NormalizeAndValidate() error {
	p.ensure()
	now := time.Now().UTC()
	for name, account := range p.Accounts {
		if err := validateAccountName(name); err != nil {
			return fmt.Errorf("аккаунт %q: %w", name, err)
		}
		normalized, err := normalizeAndValidateSecret(account.Secret)
		if err != nil {
			return fmt.Errorf("аккаунт %q: %w", name, err)
		}
		account.Secret = normalized
		if account.CreatedAt.IsZero() {
			account.CreatedAt = now
		}
		if account.UpdatedAt.IsZero() {
			account.UpdatedAt = account.CreatedAt
		}
		p.Accounts[name] = account
	}
	return nil
}

// Import merges accounts from another payload. Existing records are skipped
// unless overwrite is true. The source payload is not modified.
func (p *Payload) Import(other *Payload, overwrite bool) (ImportStats, error) {
	var stats ImportStats
	if other == nil {
		return stats, errors.New("нет данных для импорта")
	}
	p.ensure()
	now := time.Now().UTC()
	for name, account := range other.Accounts {
		if err := validateAccountName(name); err != nil {
			return stats, fmt.Errorf("аккаунт %q: %w", name, err)
		}
		normalized, err := normalizeAndValidateSecret(account.Secret)
		if err != nil {
			return stats, fmt.Errorf("аккаунт %q: %w", name, err)
		}
		account.Secret = normalized
		if account.CreatedAt.IsZero() {
			account.CreatedAt = now
		}
		if account.UpdatedAt.IsZero() {
			account.UpdatedAt = account.CreatedAt
		}
		if _, exists := p.Accounts[name]; exists {
			if !overwrite {
				stats.Skipped++
				continue
			}
			stats.Updated++
		} else {
			stats.Added++
		}
		p.Accounts[name] = account
	}
	return stats, nil
}
