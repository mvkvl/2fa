package service

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mvkvl/2fa/internal/core"
)

const serviceTestSecret = "JBSWY3DPEHPK3PXP"

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	return New(path, "test"), path
}

func TestServiceLifecycle(t *testing.T) {
	svc, _ := newTestService(t)
	status := svc.Status()
	if status.Exists {
		t.Fatal("vault should not exist yet")
	}
	if !status.Locked {
		t.Fatal("missing vault should be reported as locked")
	}
	if status.Path == "" {
		t.Fatal("status path is empty")
	}

	status, err := svc.Create(core.ProviderPassphrase, "pass")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !status.Exists || status.Locked || status.Provider != core.ProviderPassphrase {
		t.Fatalf("unexpected status after create: %+v", status)
	}

	view, err := svc.SaveAccount(AccountInput{
		Name:        "github",
		Description: "GitHub account",
		Favorite:    true,
		NewSecret:   serviceTestSecret,
	})
	if err != nil {
		t.Fatalf("SaveAccount new: %v", err)
	}
	if view.Name != "github" || view.Description != "GitHub account" || !view.Favorite {
		t.Fatalf("unexpected view: %+v", view)
	}
	if view.CreatedAt.IsZero() || view.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not filled: %+v", view)
	}

	if _, err := svc.SaveAccount(AccountInput{Name: "mail", NewSecret: serviceTestSecret}); err != nil {
		t.Fatalf("SaveAccount mail: %v", err)
	}

	accounts, err := svc.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 2 || accounts[0].Name != "github" || accounts[1].Name != "mail" {
		t.Fatalf("unexpected accounts: %+v", accounts)
	}

	result, err := svc.CopyCode("github")
	if err != nil {
		t.Fatalf("CopyCode: %v", err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(result.Code) {
		t.Fatalf("bad code %q", result.Code)
	}
	if result.Remaining < 1 || result.Remaining > 30 {
		t.Fatalf("bad remaining %d", result.Remaining)
	}

	edited, err := svc.SaveAccount(AccountInput{
		OriginalName: "github",
		Name:         "GitHub",
		Description:  "Personal",
		Favorite:     true,
	})
	if err != nil {
		t.Fatalf("SaveAccount edit: %v", err)
	}
	if edited.Name != "GitHub" || edited.Description != "Personal" {
		t.Fatalf("unexpected edited view: %+v", edited)
	}
	if _, err := svc.GetAccount("github"); err == nil {
		t.Fatal("old name should be gone after rename")
	}
	if _, err := svc.CopyCode("GitHub"); err != nil {
		t.Fatalf("CopyCode after rename: %v", err)
	}

	if _, err := svc.SetFavorite("mail", true); err != nil {
		t.Fatalf("SetFavorite: %v", err)
	}
	view, err = svc.GetAccount("mail")
	if err != nil || !view.Favorite {
		t.Fatalf("favorite not saved: %+v, %v", view, err)
	}

	status = svc.Lock()
	if !status.Locked {
		t.Fatal("vault should be locked")
	}
	if _, err := svc.ListAccounts(); err == nil {
		t.Fatal("ListAccounts should fail while locked")
	}
	if _, err := svc.Unlock("wrong"); err == nil {
		t.Fatal("Unlock with wrong password should fail")
	}
	status, err = svc.Unlock("pass")
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if status.Locked {
		t.Fatal("vault should be unlocked")
	}
	if _, err := svc.GetAccount("GitHub"); err != nil {
		t.Fatalf("account missing after unlock: %v", err)
	}
}

func TestServiceExportImport(t *testing.T) {
	svc, path := newTestService(t)
	if _, err := svc.Create(core.ProviderPassphrase, "pass"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAccount(AccountInput{
		Name:        "github",
		Description: "GitHub account",
		Favorite:    true,
		NewSecret:   serviceTestSecret,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAccount(AccountInput{
		Name:      "mail",
		NewSecret: serviceTestSecret,
	}); err != nil {
		t.Fatal(err)
	}

	exportPath := filepath.Join(filepath.Dir(path), "backup.2fa")
	if err := svc.Export(exportPath, "exportpass"); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if err := svc.DeleteAccount("github"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	preview, err := svc.PreviewImport(exportPath, "exportpass")
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if preview.Total != 2 || len(preview.New) != 1 || preview.New[0] != "github" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if len(preview.Conflicts) != 1 || preview.Conflicts[0] != "mail" {
		t.Fatalf("unexpected conflicts: %+v", preview.Conflicts)
	}

	stats, err := svc.Import(exportPath, "exportpass", false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if stats.Added != 1 || stats.Skipped != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, err := svc.GetAccount("github"); err != nil {
		t.Fatalf("github was not imported: %v", err)
	}

	if _, err := svc.PreviewImport(exportPath, "wrong"); err == nil {
		t.Fatal("PreviewImport with wrong password should fail")
	}
}

func TestServiceRejectsPlaintext(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Create(core.ProviderPlaintext, ""); err == nil {
		t.Fatal("Create plaintext should be rejected")
	}
}

func TestServiceProviders(t *testing.T) {
	svc, _ := newTestService(t)
	providers := svc.Providers()
	if len(providers) != 2 {
		t.Fatalf("got %d providers", len(providers))
	}
	foundPassphrase := false
	for _, provider := range providers {
		if provider.Name == core.ProviderPassphrase {
			foundPassphrase = provider.Available
		}
		if provider.Name == core.ProviderPlaintext {
			t.Fatal("plaintext must not be offered in GUI")
		}
	}
	if !foundPassphrase {
		t.Fatal("passphrase provider should always be available")
	}
}
