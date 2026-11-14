package core

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "JBSWY3DPEHPK3PXP"

func strPtr(value string) *string { return &value }

func TestPayloadLifecycle(t *testing.T) {
	payload := newPayload()
	desc := "GitHub"
	overwritten, err := payload.Add("github", testSecret, &desc, false)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if overwritten {
		t.Fatal("new account reported as overwritten")
	}

	account, ok := payload.Get("github")
	if !ok {
		t.Fatal("account not found")
	}
	if account.Secret != testSecret {
		t.Errorf("secret = %q, want %q", account.Secret, testSecret)
	}
	if account.Description != "GitHub" {
		t.Errorf("description = %q, want GitHub", account.Description)
	}
	if account.CreatedAt.IsZero() {
		t.Error("created_at is zero")
	}

	createdAt := account.CreatedAt
	time.Sleep(time.Millisecond)
	overwritten, err = payload.Add("github", testSecret, nil, true)
	if err != nil {
		t.Fatalf("Add force: %v", err)
	}
	if !overwritten {
		t.Fatal("existing account reported as new")
	}
	account, _ = payload.Get("github")
	if account.Description != "GitHub" {
		t.Errorf("force without desc cleared description: %q", account.Description)
	}
	if !account.CreatedAt.Equal(createdAt) {
		t.Errorf("created_at changed: %v != %v", account.CreatedAt, createdAt)
	}

	if err := payload.Update("github", nil, strPtr("Work")); err != nil {
		t.Fatalf("Update: %v", err)
	}
	account, _ = payload.Get("github")
	if account.Description != "Work" {
		t.Errorf("description = %q, want Work", account.Description)
	}

	if err := payload.Rename("github", "work"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, ok := payload.Get("github"); ok {
		t.Error("old name still exists")
	}
	if _, ok := payload.Get("work"); !ok {
		t.Error("new name missing")
	}

	if err := payload.Remove("work"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := payload.Get("work"); ok {
		t.Error("account still exists after removal")
	}
}

func TestPayloadListSorted(t *testing.T) {
	payload := newPayload()
	for _, name := range []string{"zeta", "alpha", "beta"} {
		if _, err := payload.Add(name, testSecret, nil, false); err != nil {
			t.Fatalf("Add %s: %v", name, err)
		}
	}
	infos := payload.List()
	if len(infos) != 3 {
		t.Fatalf("len(List) = %d, want 3", len(infos))
	}
	for i, want := range []string{"alpha", "beta", "zeta"} {
		if infos[i].Name != want {
			t.Errorf("List[%d] = %q, want %q", i, infos[i].Name, want)
		}
	}
}

func TestPayloadErrors(t *testing.T) {
	payload := newPayload()
	if _, err := payload.Add("dup", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Add("dup", testSecret, nil, false); err == nil {
		t.Fatal("expected duplicate error")
	}
	if _, err := payload.Add("bad", "not-base32!", nil, false); err == nil {
		t.Fatal("expected invalid secret error")
	}
	if err := payload.Update("missing", nil, strPtr("x")); err == nil {
		t.Fatal("expected update missing error")
	}
	if err := payload.Rename("dup", "dup"); err == nil {
		t.Fatal("expected rename same error")
	}
	if err := payload.Rename("missing", "new"); err == nil {
		t.Fatal("expected rename missing error")
	}
	if _, err := payload.Add("other", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := payload.Rename("dup", "other"); err == nil {
		t.Fatal("expected rename conflict error")
	}
	if err := payload.Remove("missing"); err == nil {
		t.Fatal("expected remove missing error")
	}
}

func TestValidateAccountName(t *testing.T) {
	valid := []string{"github", "work-mail", "почта", "a_b.c", "x"}
	for _, name := range valid {
		if err := validateAccountName(name); err != nil {
			t.Errorf("validateAccountName(%q) = %v", name, err)
		}
	}
	invalid := []string{"", "-x", "a b", "a\tb", "a\nb", strings.Repeat("x", 129)}
	for _, name := range invalid {
		if err := validateAccountName(name); err == nil {
			t.Errorf("validateAccountName(%q) = nil, want error", name)
		}
	}
}
