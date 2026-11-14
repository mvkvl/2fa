package core

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func passphraseOpts(passphrase string) ProviderOptions {
	return ProviderOptions{
		Stdin:      strings.NewReader(passphrase + "\n"),
		Stderr:     io.Discard,
		StdinIsTTY: false,
	}
}

func TestVaultPassphraseRoundTrip(t *testing.T) {
	t.Setenv("2FA_PASSPHRASE", "")
	path := filepath.Join(t.TempDir(), "accounts.json")

	vault, err := createVault(path, providerPassphrase, passphraseOpts("secret"))
	if err != nil {
		t.Fatalf("createVault: %v", err)
	}
	desc := "GitHub"
	if _, err := vault.Payload.Add("github", testSecret, &desc, false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := vault.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file mode = %o, want 0600", perm)
	}

	opened, err := openVault(path, providerAuto, passphraseOpts("secret"))
	if err != nil {
		t.Fatalf("openVault: %v", err)
	}
	account, ok := opened.Payload.Get("github")
	if !ok {
		t.Fatal("account not found after decrypt")
	}
	if account.Secret != testSecret || account.Description != "GitHub" {
		t.Errorf("account = %+v, want secret %q and description GitHub", account, testSecret)
	}

	if _, err := openVault(path, providerAuto, passphraseOpts("wrong")); err == nil {
		t.Fatal("openVault with wrong passphrase succeeded")
	}
}

func TestVaultTamperDetection(t *testing.T) {
	t.Setenv("2FA_PASSPHRASE", "")
	path := filepath.Join(t.TempDir(), "accounts.json")
	vault, err := createVault(path, providerPassphrase, passphraseOpts("secret"))
	if err != nil {
		t.Fatalf("createVault: %v", err)
	}
	if _, err := vault.Payload.Add("github", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := vault.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var header VaultHeader
	if err := json.Unmarshal(data, &header); err != nil {
		t.Fatal(err)
	}

	t.Run("header", func(t *testing.T) {
		tampered := header
		tampered.VaultID = "00000000000000000000000000000000"
		if err := writeVaultFile(path, tampered); err != nil {
			t.Fatal(err)
		}
		if _, err := openVault(path, providerAuto, passphraseOpts("secret")); err == nil {
			t.Fatal("openVault accepted tampered header")
		}
	})

	t.Run("ciphertext", func(t *testing.T) {
		tampered := header
		raw, err := base64.StdEncoding.DecodeString(tampered.Ciphertext)
		if err != nil {
			t.Fatal(err)
		}
		raw[0] ^= 0xff
		tampered.Ciphertext = base64.StdEncoding.EncodeToString(raw)
		if err := writeVaultFile(path, tampered); err != nil {
			t.Fatal(err)
		}
		if _, err := openVault(path, providerAuto, passphraseOpts("secret")); err == nil {
			t.Fatal("openVault accepted tampered ciphertext")
		}
	})

	t.Run("downgrade to plaintext", func(t *testing.T) {
		tampered := header
		raw, err := json.Marshal(newPayload())
		if err != nil {
			t.Fatal(err)
		}
		tampered.Cipher = "none"
		tampered.Nonce = ""
		tampered.Ciphertext = base64.StdEncoding.EncodeToString(raw)
		if err := writeVaultFile(path, tampered); err != nil {
			t.Fatal(err)
		}
		if _, err := openVault(path, providerAuto, passphraseOpts("secret")); err == nil {
			t.Fatal("openVault accepted cipher downgrade")
		}
	})
}

func TestVaultPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	vault, err := createVault(path, providerPlaintext, ProviderOptions{})
	if err != nil {
		t.Fatalf("createVault: %v", err)
	}
	if _, err := vault.Payload.Add("github", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := vault.Save(); err != nil {
		t.Fatal(err)
	}
	opened, err := openVault(path, providerAuto, ProviderOptions{})
	if err != nil {
		t.Fatalf("openVault: %v", err)
	}
	if opened.Header.Provider != providerPlaintext {
		t.Errorf("provider = %q, want plaintext", opened.Header.Provider)
	}
	if opened.Header.Cipher != "none" {
		t.Errorf("cipher = %q, want none", opened.Header.Cipher)
	}
	if account, ok := opened.Payload.Get("github"); !ok || account.Secret != testSecret {
		t.Errorf("account = %+v, ok = %v", account, ok)
	}
}

type fakeKeyring struct {
	data map[string]string
	err  error
}

func fakeKeyringKey(service, user string) string {
	return service + "\x00" + user
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	value, ok := f.data[fakeKeyringKey(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (f *fakeKeyring) Set(service, user, password string) error {
	if f.err != nil {
		return f.err
	}
	f.data[fakeKeyringKey(service, user)] = password
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if f.err != nil {
		return f.err
	}
	key := fakeKeyringKey(service, user)
	if _, ok := f.data[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.data, key)
	return nil
}

func withFakeKeyring(t *testing.T, fake *fakeKeyring) {
	t.Helper()
	old := keyringBackend
	keyringBackend = fake
	t.Cleanup(func() { keyringBackend = old })
}

func TestVaultKeyringAndRekey(t *testing.T) {
	t.Setenv("2FA_PASSPHRASE", "")
	fake := &fakeKeyring{data: map[string]string{}}
	withFakeKeyring(t, fake)
	path := filepath.Join(t.TempDir(), "accounts.json")

	vault, err := createVault(path, providerKeyring, ProviderOptions{})
	if err != nil {
		t.Fatalf("createVault: %v", err)
	}
	if _, err := vault.Payload.Add("github", testSecret, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := vault.Save(); err != nil {
		t.Fatal(err)
	}
	opened, err := openVault(path, providerAuto, ProviderOptions{})
	if err != nil {
		t.Fatalf("openVault: %v", err)
	}
	if account, ok := opened.Payload.Get("github"); !ok || account.Secret != testSecret {
		t.Errorf("account = %+v, ok = %v", account, ok)
	}

	vaultID := vault.Header.VaultID
	key := fakeKeyringKey(keyringService, keyringUser(vaultID))
	if _, ok := fake.data[key]; !ok {
		t.Fatal("keyring entry missing after create")
	}

	if err := rekeyVault(opened, providerPassphrase, passphraseOpts("pass")); err != nil {
		t.Fatalf("rekey to passphrase: %v", err)
	}
	if opened.Header.Provider != providerPassphrase {
		t.Errorf("provider = %q, want passphrase", opened.Header.Provider)
	}
	if opened.Header.KDF == nil {
		t.Error("KDF params missing after rekey to passphrase")
	}
	if _, ok := fake.data[key]; ok {
		t.Error("old keyring key was not removed")
	}

	opened2, err := openVault(path, providerAuto, passphraseOpts("pass"))
	if err != nil {
		t.Fatalf("openVault after rekey: %v", err)
	}
	if account, ok := opened2.Payload.Get("github"); !ok || account.Secret != testSecret {
		t.Errorf("account after rekey = %+v, ok = %v", account, ok)
	}

	if err := rekeyVault(opened2, providerKeyring, ProviderOptions{}); err != nil {
		t.Fatalf("rekey to keyring: %v", err)
	}
	if opened2.Header.Provider != providerKeyring {
		t.Errorf("provider = %q, want keyring", opened2.Header.Provider)
	}
	if _, err := openVault(path, providerAuto, ProviderOptions{}); err != nil {
		t.Fatalf("openVault after rekey back: %v", err)
	}
}

func TestResolveProvider(t *testing.T) {
	t.Run("auto with keyring", func(t *testing.T) {
		withFakeKeyring(t, &fakeKeyring{data: map[string]string{}})
		got, err := resolveProvider(providerAuto)
		if err != nil {
			t.Fatal(err)
		}
		if got != providerKeyring {
			t.Errorf("resolveProvider(auto) = %q, want keyring", got)
		}
	})

	t.Run("auto without keyring", func(t *testing.T) {
		withFakeKeyring(t, &fakeKeyring{data: map[string]string{}, err: errors.New("dbus unavailable")})
		got, err := resolveProvider(providerAuto)
		if err != nil {
			t.Fatal(err)
		}
		if got != providerPassphrase {
			t.Errorf("resolveProvider(auto) = %q, want passphrase", got)
		}
	})

	t.Run("explicit unavailable keyring", func(t *testing.T) {
		withFakeKeyring(t, &fakeKeyring{data: map[string]string{}, err: errors.New("dbus unavailable")})
		if _, err := resolveProvider(providerKeyring); err == nil {
			t.Fatal("expected error for unavailable keyring")
		}
	})

	t.Run("explicit plaintext", func(t *testing.T) {
		got, err := resolveProvider(providerPlaintext)
		if err != nil {
			t.Fatal(err)
		}
		if got != providerPlaintext {
			t.Errorf("resolveProvider(plaintext) = %q", got)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if _, err := resolveProvider("nope"); err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}
