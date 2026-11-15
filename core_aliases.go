package main

import "github.com/mvkvl/2fa/internal/core"

// The CLI predates the internal/core package. These aliases keep the command
// layer small while the GUI imports the same core directly.
type (
	Account         = core.Account
	AccountInfo     = core.AccountInfo
	Payload         = core.Payload
	Vault           = core.Vault
	VaultHeader     = core.VaultHeader
	KDFParams       = core.KDFParams
	ProviderOptions = core.ProviderOptions
	KeyProvider     = core.KeyProvider
)

const (
	providerAuto       = core.ProviderAuto
	providerKeyring    = core.ProviderKeyring
	providerPassphrase = core.ProviderPassphrase
	providerPlaintext  = core.ProviderPlaintext
)

var (
	ErrVaultNotFound = core.ErrVaultNotFound
	ErrVaultExists   = core.ErrVaultExists
)

func defaultVaultPath() (string, error) {
	return core.DefaultVaultPath()
}

func defaultVaultPathString() string {
	return core.DefaultVaultPathString()
}

func openVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	return core.OpenVault(path, requestedProvider, opts)
}

func createVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	return core.CreateVault(path, requestedProvider, opts)
}

func openOrCreateVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	return core.OpenOrCreateVault(path, requestedProvider, opts)
}

func rekeyVault(vault *Vault, target string, opts ProviderOptions) error {
	return core.RekeyVault(vault, target, opts)
}

func withVaultLock(path string, exclusive bool, fn func() error) error {
	return core.WithVaultLock(path, exclusive, fn)
}

func newPayload() *Payload {
	return core.NewPayload()
}

func normalizeAndValidateSecret(secret string) (string, error) {
	return core.NormalizeAndValidateSecret(secret)
}

func decodeSecret(secret string) ([]byte, error) {
	return core.DecodeSecret(secret)
}

func generateTOTP(secret string) (string, error) {
	return core.GenerateTOTP(secret)
}

func validateAccountName(name string) error {
	return core.ValidateAccountName(name)
}

func readSecretValue(opts ProviderOptions, prompt string, confirm bool, forceLine bool) (string, error) {
	return core.ReadSecretValue(opts, prompt, confirm, forceLine)
}
