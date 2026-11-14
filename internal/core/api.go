package core

import "time"

const (
	ProviderAuto       = providerAuto
	ProviderKeyring    = providerKeyring
	ProviderPassphrase = providerPassphrase
	ProviderPlaintext  = providerPlaintext
)

// DefaultVaultPath returns the default location of the encrypted vault.
func DefaultVaultPath() (string, error) {
	return defaultVaultPath()
}

func DefaultVaultPathString() string {
	return defaultVaultPathString()
}

// OpenVault opens and decrypts an existing vault.
func OpenVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	return openVault(path, requestedProvider, opts)
}

// CreateVault creates a new encrypted vault.
func CreateVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	return createVault(path, requestedProvider, opts)
}

// OpenOrCreateVault opens an existing vault or creates a new one.
func OpenOrCreateVault(path, requestedProvider string, opts ProviderOptions) (*Vault, error) {
	return openOrCreateVault(path, requestedProvider, opts)
}

// RekeyVault re-encrypts the vault with a new key provider.
func RekeyVault(vault *Vault, target string, opts ProviderOptions) error {
	return rekeyVault(vault, target, opts)
}

func ResolveProvider(requested string) (string, error) {
	return resolveProvider(requested)
}

func KeyringAvailable() bool {
	return keyringAvailable()
}

func NewPayload() *Payload {
	return newPayload()
}

func NormalizeAndValidateSecret(secret string) (string, error) {
	return normalizeAndValidateSecret(secret)
}

func DecodeSecret(secret string) ([]byte, error) {
	return decodeSecret(secret)
}

func GenerateTOTP(secret string) (string, error) {
	return generateTOTP(secret)
}

func GenerateTOTPAt(secret string, t time.Time) (string, error) {
	return generateTOTPAt(secret, t)
}

func ValidateAccountName(name string) error {
	return validateAccountName(name)
}

func ReadSecretValue(opts ProviderOptions, prompt string, confirm bool, forceLine bool) (string, error) {
	return readSecretValue(opts, prompt, confirm, forceLine)
}
