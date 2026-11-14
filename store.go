package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Account struct {
	Secret      string    `json:"secret"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Payload struct {
	Accounts map[string]Account `json:"accounts"`
}

func newPayload() *Payload {
	return &Payload{Accounts: map[string]Account{}}
}

func (p *Payload) ensure() {
	if p.Accounts == nil {
		p.Accounts = map[string]Account{}
	}
}

func validateAccountName(name string) error {
	if name == "" {
		return errors.New("имя аккаунта не может быть пустым")
	}
	if strings.HasPrefix(name, "-") {
		return errors.New("имя аккаунта не может начинаться с '-'")
	}
	if utf8.RuneCountInString(name) > 128 {
		return errors.New("имя аккаунта слишком длинное (максимум 128 символов)")
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return errors.New("имя аккаунта не может содержать пробелы")
		}
		if unicode.IsControl(r) {
			return errors.New("имя аккаунта содержит недопустимые символы")
		}
	}
	return nil
}

func (p *Payload) Add(name, secret string, description *string, force bool) (bool, error) {
	if err := validateAccountName(name); err != nil {
		return false, err
	}
	normalized, err := normalizeAndValidateSecret(secret)
	if err != nil {
		return false, fmt.Errorf("некорректный секрет: %w", err)
	}
	p.ensure()
	old, exists := p.Accounts[name]
	if exists && !force {
		return false, fmt.Errorf("аккаунт %q уже существует (используйте --force для перезаписи)", name)
	}
	createdAt := time.Now().UTC()
	if exists {
		createdAt = old.CreatedAt
	}
	account := Account{Secret: normalized, CreatedAt: createdAt}
	if description != nil {
		account.Description = *description
	} else if exists {
		account.Description = old.Description
	}
	p.Accounts[name] = account
	return exists, nil
}

func (p *Payload) Update(name string, secret, description *string) error {
	p.ensure()
	account, ok := p.Accounts[name]
	if !ok {
		return fmt.Errorf("аккаунт %q не найден", name)
	}
	if secret != nil {
		normalized, err := normalizeAndValidateSecret(*secret)
		if err != nil {
			return fmt.Errorf("некорректный секрет: %w", err)
		}
		account.Secret = normalized
	}
	if description != nil {
		account.Description = *description
	}
	p.Accounts[name] = account
	return nil
}

func (p *Payload) Rename(oldName, newName string) error {
	if err := validateAccountName(newName); err != nil {
		return err
	}
	p.ensure()
	if oldName == newName {
		return errors.New("новое имя совпадает со старым")
	}
	account, ok := p.Accounts[oldName]
	if !ok {
		return fmt.Errorf("аккаунт %q не найден", oldName)
	}
	if _, exists := p.Accounts[newName]; exists {
		return fmt.Errorf("аккаунт %q уже существует", newName)
	}
	delete(p.Accounts, oldName)
	p.Accounts[newName] = account
	return nil
}

func (p *Payload) Remove(name string) error {
	p.ensure()
	if _, ok := p.Accounts[name]; !ok {
		return fmt.Errorf("аккаунт %q не найден", name)
	}
	delete(p.Accounts, name)
	return nil
}

func (p *Payload) Get(name string) (Account, bool) {
	p.ensure()
	account, ok := p.Accounts[name]
	return account, ok
}

type AccountInfo struct {
	Name string
	Account
}

func (p *Payload) List() []AccountInfo {
	p.ensure()
	infos := make([]AccountInfo, 0, len(p.Accounts))
	for name, account := range p.Accounts {
		infos = append(infos, AccountInfo{Name: name, Account: account})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}
