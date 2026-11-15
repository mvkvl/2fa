package core

import (
	"path/filepath"
	"testing"
)

func TestVaultReloadPicksUpExternalChanges(t *testing.T) {
	t.Setenv("2FA_PASSPHRASE", "")
	path := filepath.Join(t.TempDir(), "accounts.json")

	vault, err := createVault(path, providerPassphrase, passphraseOpts("secret"))
	if err != nil {
		t.Fatalf("createVault: %v", err)
	}
	if _, err := vault.Payload.Add("local", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := vault.Save(); err != nil {
		t.Fatal(err)
	}

	external, err := openVault(path, providerAuto, passphraseOpts("secret"))
	if err != nil {
		t.Fatalf("openVault external: %v", err)
	}
	if _, err := external.Payload.Add("external", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := external.Save(); err != nil {
		t.Fatal(err)
	}

	if err := Reload(vault); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	for _, name := range []string{"local", "external"} {
		if _, ok := vault.Payload.Get(name); !ok {
			t.Errorf("account %q missing after reload", name)
		}
	}

	if err := rekeyVault(external, providerPlaintext, ProviderOptions{}); err != nil {
		t.Fatalf("external rekey: %v", err)
	}
	if err := Reload(vault); err == nil {
		t.Fatal("Reload accepted a vault with a different provider")
	}
	if vault.Header.Provider != providerPassphrase {
		t.Errorf("provider changed after failed reload: %q", vault.Header.Provider)
	}
}
