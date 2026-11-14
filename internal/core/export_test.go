package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func exportTestPayload() *Payload {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &Payload{Accounts: map[string]Account{
		"github": {
			Secret:      "JBSWY3DPEHPK3PXP",
			Description: "GitHub account",
			Favorite:    true,
			CreatedAt:   created,
			UpdatedAt:   created,
		},
		"mail": {
			Secret:      "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
			Description: "Почта",
			CreatedAt:   created,
			UpdatedAt:   created,
		},
	}}
}

func TestExportRoundTrip(t *testing.T) {
	payload := exportTestPayload()
	data, err := EncryptExport(payload, "correct horse")
	if err != nil {
		t.Fatalf("EncryptExport: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty export")
	}
	decrypted, err := DecryptExport(data, "correct horse")
	if err != nil {
		t.Fatalf("DecryptExport: %v", err)
	}
	if len(decrypted.Accounts) != 2 {
		t.Fatalf("got %d accounts", len(decrypted.Accounts))
	}
	github := decrypted.Accounts["github"]
	if github.Secret != "JBSWY3DPEHPK3PXP" || github.Description != "GitHub account" || !github.Favorite {
		t.Fatalf("github mismatch: %+v", github)
	}
	if !github.CreatedAt.Equal(payload.Accounts["github"].CreatedAt) {
		t.Fatalf("created_at mismatch: %v", github.CreatedAt)
	}
	if !github.UpdatedAt.Equal(payload.Accounts["github"].UpdatedAt) {
		t.Fatalf("updated_at mismatch: %v", github.UpdatedAt)
	}
}

func TestDecryptExportWrongPassword(t *testing.T) {
	data, err := EncryptExport(exportTestPayload(), "right")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptExport(data, "wrong"); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestDecryptExportTamper(t *testing.T) {
	data, err := EncryptExport(exportTestPayload(), "password")
	if err != nil {
		t.Fatal(err)
	}
	var envelope ExportEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Ciphertext = flipBase64FirstChar(envelope.Ciphertext)
	tampered, err := json.Marshal(&envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptExport(tampered, "password"); err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecryptExportRejectsWrongFormat(t *testing.T) {
	data, err := EncryptExport(exportTestPayload(), "password")
	if err != nil {
		t.Fatal(err)
	}
	var envelope ExportEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Format = "other"
	tampered, err := json.Marshal(&envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptExport(tampered, "password"); err == nil {
		t.Fatal("expected error for wrong format")
	}
}

func TestWriteReadExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.2fa")
	if err := WriteExport(path, exportTestPayload(), "password"); err != nil {
		t.Fatalf("WriteExport: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	payload, err := ReadExport(path, "password")
	if err != nil {
		t.Fatalf("ReadExport: %v", err)
	}
	if len(payload.Accounts) != 2 {
		t.Fatalf("got %d accounts", len(payload.Accounts))
	}
}

func TestPayloadImport(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	base := &Payload{Accounts: map[string]Account{
		"github": {Secret: "JBSWY3DPEHPK3PXP", Description: "old", CreatedAt: created, UpdatedAt: created},
		"mail":   {Secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", CreatedAt: created, UpdatedAt: created},
	}}
	incoming := &Payload{Accounts: map[string]Account{
		"github": {Secret: "MFRGGZDFMZTWQ2LK", Description: "new", Favorite: true},
		"bank":   {Secret: "JBSWY3DPEHPK3PXP", Description: "Bank"},
	}}
	stats, err := base.Import(incoming, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if stats.Added != 1 || stats.Skipped != 1 || stats.Updated != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if got := base.Accounts["github"].Description; got != "old" {
		t.Fatalf("github was overwritten without overwrite=true: %q", got)
	}
	if _, ok := base.Accounts["bank"]; !ok {
		t.Fatal("bank was not added")
	}

	stats, err = base.Import(incoming, true)
	if err != nil {
		t.Fatalf("Import overwrite: %v", err)
	}
	if stats.Updated != 2 || stats.Skipped != 0 {
		t.Fatalf("unexpected overwrite stats: %+v", stats)
	}
	github := base.Accounts["github"]
	if github.Description != "new" || !github.Favorite {
		t.Fatalf("github was not updated: %+v", github)
	}
	if github.CreatedAt.IsZero() || github.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not filled: %+v", github)
	}
}

func TestPayloadImportRejectsInvalidAccount(t *testing.T) {
	base := newPayload()
	incoming := &Payload{Accounts: map[string]Account{
		"bad name": {Secret: "JBSWY3DPEHPK3PXP"},
	}}
	if _, err := base.Import(incoming, false); err == nil {
		t.Fatal("expected error for invalid name")
	}
	incoming = &Payload{Accounts: map[string]Account{
		"good": {Secret: "not-base32!"},
	}}
	if _, err := base.Import(incoming, false); err == nil || !strings.Contains(err.Error(), "good") {
		t.Fatalf("expected named secret error, got %v", err)
	}
}

func flipBase64FirstChar(value string) string {
	if value == "" {
		return "A"
	}
	if value[0] == 'A' {
		return "B" + value[1:]
	}
	return "A" + value[1:]
}
