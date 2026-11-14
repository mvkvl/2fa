// Package service exposes vault operations to the Wails frontend.
// It never returns account secrets and computes TOTP codes only on demand.
package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mvkvl/2fa/internal/core"
)

type Status struct {
	Exists           bool   `json:"exists"`
	Path             string `json:"path"`
	Provider         string `json:"provider"`
	Locked           bool   `json:"locked"`
	KeyringAvailable bool   `json:"keyringAvailable"`
	AccountCount     int    `json:"accountCount"`
	Plaintext        bool   `json:"plaintext"`
	Error            string `json:"error,omitempty"`
}

type AccountView struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Favorite    bool      `json:"favorite"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AccountInput struct {
	OriginalName string `json:"originalName"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Favorite     bool   `json:"favorite"`
	NewSecret    string `json:"newSecret"`
}

type CopyResult struct {
	Code      string `json:"code"`
	Remaining int    `json:"remaining"`
}

type ProviderInfo struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Available   bool   `json:"available"`
	Recommended bool   `json:"recommended"`
}

type ImportPreview struct {
	Accounts  []AccountView `json:"accounts"`
	New       []string      `json:"new"`
	Conflicts []string      `json:"conflicts"`
	Total     int           `json:"total"`
}

type ImportStats = core.ImportStats

// Service is bound to the frontend as a Wails service.
type Service struct {
	mu      sync.Mutex
	path    string
	pathErr error
	vault   *core.Vault
	version string
}

func New(path, version string) *Service {
	s := &Service{version: version}
	if path == "" {
		path, s.pathErr = core.DefaultVaultPath()
	}
	s.path = path
	return s
}

func (s *Service) Version() string { return s.version }

func (s *Service) VaultPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.path
}

func (s *Service) SetVaultPath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vault != nil {
		return errors.New("сначала заблокируйте vault")
	}
	if path == "" {
		p, err := core.DefaultVaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	s.path = path
	s.pathErr = nil
	return nil
}

func (s *Service) statusLocked() Status {
	status := Status{
		Path:             s.path,
		Locked:           true,
		KeyringAvailable: core.KeyringAvailable(),
	}
	if s.pathErr != nil {
		status.Error = s.pathErr.Error()
		return status
	}
	if s.vault != nil {
		status.Exists = true
		status.Provider = s.vault.Header.Provider
		status.Locked = false
		status.Plaintext = s.vault.Header.Provider == core.ProviderPlaintext
		if s.vault.Payload != nil {
			status.AccountCount = len(s.vault.Payload.Accounts)
		}
		return status
	}
	header, err := core.ReadVaultHeader(s.path)
	if err == nil {
		status.Exists = true
		status.Provider = header.Provider
		status.Plaintext = header.Provider == core.ProviderPlaintext
		return status
	}
	if !errors.Is(err, core.ErrVaultNotFound) {
		status.Exists = true
		status.Error = err.Error()
	}
	return status
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) Providers() []ProviderInfo {
	keyring := core.KeyringAvailable()
	return []ProviderInfo{
		{Name: core.ProviderKeyring, Label: "Keyring", Available: keyring, Recommended: keyring},
		{Name: core.ProviderPassphrase, Label: "Пароль", Available: true, Recommended: !keyring},
	}
}

func (s *Service) Create(provider, passphrase string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.statusLocked()
	if s.pathErr != nil {
		return status, s.pathErr
	}
	if s.vault != nil {
		return status, errors.New("vault уже открыт")
	}
	if provider == "" {
		provider = core.ProviderKeyring
	}
	if provider == core.ProviderPlaintext {
		return status, errors.New("режим plaintext не поддерживается в GUI")
	}
	resolved, err := core.ResolveProvider(provider)
	if err != nil {
		return status, err
	}
	if resolved == core.ProviderPassphrase && passphrase == "" {
		return status, errors.New("пароль не может быть пустым")
	}
	vault, err := core.CreateVault(s.path, provider, core.ProviderOptions{
		Passphrase:    passphrase,
		PassphraseSet: passphrase != "",
	})
	if err != nil {
		return status, err
	}
	s.vault = vault
	return s.statusLocked(), nil
}

func (s *Service) Unlock(passphrase string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.statusLocked()
	if s.pathErr != nil {
		return status, s.pathErr
	}
	if s.vault != nil {
		return status, nil
	}
	header, err := core.ReadVaultHeader(s.path)
	if err != nil {
		return status, err
	}
	if header.Provider == core.ProviderPassphrase && passphrase == "" {
		return status, errors.New("введите пароль vault")
	}
	vault, err := core.OpenVault(s.path, header.Provider, core.ProviderOptions{
		Passphrase:    passphrase,
		PassphraseSet: passphrase != "",
	})
	if err != nil {
		return status, err
	}
	s.vault = vault
	return s.statusLocked(), nil
}

func (s *Service) Lock() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vault != nil {
		s.vault.Close()
		s.vault = nil
	}
	return s.statusLocked()
}

func (s *Service) requireVault() (*core.Vault, error) {
	if s.vault == nil || s.vault.Payload == nil {
		return nil, errors.New("vault заблокирован")
	}
	return s.vault, nil
}

func accountView(info core.AccountInfo) AccountView {
	return AccountView{
		Name:        info.Name,
		Description: info.Description,
		Favorite:    info.Favorite,
		CreatedAt:   info.CreatedAt,
		UpdatedAt:   info.UpdatedAt,
	}
}

func (s *Service) ListAccounts() ([]AccountView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return nil, err
	}
	infos := vault.Payload.List()
	accounts := make([]AccountView, 0, len(infos))
	for _, info := range infos {
		accounts = append(accounts, accountView(info))
	}
	return accounts, nil
}

func (s *Service) Favorites() ([]AccountView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return nil, err
	}
	infos := vault.Payload.Favorites()
	accounts := make([]AccountView, 0, len(infos))
	for _, info := range infos {
		accounts = append(accounts, accountView(info))
	}
	return accounts, nil
}

func (s *Service) GetAccount(name string) (AccountView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return AccountView{}, err
	}
	account, ok := vault.Payload.Get(name)
	if !ok {
		return AccountView{}, fmt.Errorf("аккаунт %q не найден", name)
	}
	return accountView(core.AccountInfo{Name: name, Account: account}), nil
}

func (s *Service) SaveAccount(input AccountInput) (AccountView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return AccountView{}, err
	}
	name := strings.TrimSpace(input.Name)
	if err := core.ValidateAccountName(name); err != nil {
		return AccountView{}, err
	}
	description := input.Description

	oldName := strings.TrimSpace(input.OriginalName)
	if oldName == "" {
		if input.NewSecret == "" {
			return AccountView{}, errors.New("секрет не может быть пустым")
		}
		if _, err := vault.Payload.Add(name, input.NewSecret, &description, false); err != nil {
			return AccountView{}, err
		}
		if input.Favorite {
			if err := vault.Payload.SetFavorite(name, true); err != nil {
				return AccountView{}, err
			}
		}
	} else {
		if _, ok := vault.Payload.Get(oldName); !ok {
			return AccountView{}, fmt.Errorf("аккаунт %q не найден", oldName)
		}
		var secret *string
		if input.NewSecret != "" {
			normalized, err := core.NormalizeAndValidateSecret(input.NewSecret)
			if err != nil {
				return AccountView{}, fmt.Errorf("некорректный секрет: %w", err)
			}
			secret = &normalized
		}
		if oldName != name {
			if err := vault.Payload.Rename(oldName, name); err != nil {
				return AccountView{}, err
			}
		}
		if err := vault.Payload.Update(name, secret, &description); err != nil {
			return AccountView{}, err
		}
		if err := vault.Payload.SetFavorite(name, input.Favorite); err != nil {
			return AccountView{}, err
		}
	}
	if err := vault.Save(); err != nil {
		return AccountView{}, err
	}
	account, ok := vault.Payload.Get(name)
	if !ok {
		return AccountView{}, fmt.Errorf("аккаунт %q не найден после сохранения", name)
	}
	return accountView(core.AccountInfo{Name: name, Account: account}), nil
}

func (s *Service) DeleteAccount(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return err
	}
	if err := vault.Payload.Remove(name); err != nil {
		return err
	}
	return vault.Save()
}

func (s *Service) SetFavorite(name string, favorite bool) (AccountView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return AccountView{}, err
	}
	if err := vault.Payload.SetFavorite(name, favorite); err != nil {
		return AccountView{}, err
	}
	if err := vault.Save(); err != nil {
		return AccountView{}, err
	}
	account, _ := vault.Payload.Get(name)
	return accountView(core.AccountInfo{Name: name, Account: account}), nil
}

func (s *Service) CopyCode(name string) (CopyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return CopyResult{}, err
	}
	account, ok := vault.Payload.Get(name)
	if !ok {
		return CopyResult{}, fmt.Errorf("аккаунт %q не найден", name)
	}
	now := time.Now()
	code, err := core.GenerateTOTPAt(account.Secret, now)
	if err != nil {
		return CopyResult{}, err
	}
	remaining := 30 - int(now.Unix()%30)
	if remaining <= 0 {
		remaining = 30
	}
	return CopyResult{Code: code, Remaining: remaining}, nil
}

func (s *Service) Rekey(target, passphrase string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.statusLocked()
	vault, err := s.requireVault()
	if err != nil {
		return status, err
	}
	if target == "" {
		return status, errors.New("не указан целевой провайдер")
	}
	if target == core.ProviderPlaintext {
		return status, errors.New("режим plaintext не поддерживается в GUI")
	}
	if target == core.ProviderKeyring && !core.KeyringAvailable() {
		return status, errors.New("keyring недоступен")
	}
	if target == core.ProviderPassphrase && passphrase == "" {
		return status, errors.New("пароль не может быть пустым")
	}
	if err := core.RekeyVault(vault, target, core.ProviderOptions{
		Passphrase:    passphrase,
		PassphraseSet: passphrase != "",
	}); err != nil {
		return status, err
	}
	return s.statusLocked(), nil
}

func (s *Service) Export(path, passphrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return err
	}
	return core.WriteExport(path, vault.Payload, passphrase)
}

func (s *Service) PreviewImport(path, passphrase string) (ImportPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return ImportPreview{}, err
	}
	payload, err := core.ReadExport(path, passphrase)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := ImportPreview{Total: len(payload.Accounts)}
	for _, info := range payload.List() {
		preview.Accounts = append(preview.Accounts, accountView(info))
		if _, exists := vault.Payload.Accounts[info.Name]; exists {
			preview.Conflicts = append(preview.Conflicts, info.Name)
		} else {
			preview.New = append(preview.New, info.Name)
		}
	}
	return preview, nil
}

func (s *Service) Import(path, passphrase string, overwrite bool) (core.ImportStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vault, err := s.requireVault()
	if err != nil {
		return core.ImportStats{}, err
	}
	payload, err := core.ReadExport(path, passphrase)
	if err != nil {
		return core.ImportStats{}, err
	}
	stats, err := vault.Payload.Import(payload, overwrite)
	if err != nil {
		return stats, err
	}
	if err := vault.Save(); err != nil {
		return stats, err
	}
	return stats, nil
}
