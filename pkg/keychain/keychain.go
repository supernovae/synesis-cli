// Package keychain provides OS-level secure storage for API keys
// using platform-specific keychain implementations.
package keychain

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Keychain provides secure storage for secrets
type Keychain struct {
	serviceName string
	accountName string
}

// New creates a new Keychain instance
func New(serviceName, accountName string) *Keychain {
	return &Keychain{
		serviceName: serviceName,
		accountName: accountName,
	}
}

// Get retrieves a secret from the keychain
func (k *Keychain) Get() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return k.getDarwin()
	case "windows":
		return k.getWindows()
	case "linux":
		return k.getLinux()
	default:
		return "", errors.New("keychain not supported on this platform")
	}
}

// Set stores a secret in the keychain
func (k *Keychain) Set(secret string) error {
	switch runtime.GOOS {
	case "darwin":
		return k.setDarwin(secret)
	case "windows":
		return k.setWindows(secret)
	case "linux":
		return k.setLinux(secret)
	default:
		return errors.New("keychain not supported on this platform")
	}
}

// Delete removes a secret from the keychain
func (k *Keychain) Delete() error {
	switch runtime.GOOS {
	case "darwin":
		return k.deleteDarwin()
	case "windows":
		return k.deleteWindows()
	case "linux":
		return k.deleteLinux()
	default:
		return errors.New("keychain not supported on this platform")
	}
}

// Exists checks if a secret exists in the keychain
func (k *Keychain) Exists() (bool, error) {
	_, err := k.Get()
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Error variables
var (
	ErrNotFound      = errors.New("secret not found in keychain")
	ErrNotSupported  = errors.New("keychain not supported on this platform")
	ErrCommandFailed = errors.New("keychain command failed")
)

// --- Darwin (macOS) Implementation ---

func (k *Keychain) getDarwin() (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", k.serviceName, "-w")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 42 {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return string(output), nil
}

func (k *Keychain) setDarwin(secret string) error {
	cmd := exec.Command("security", "add-generic-password", "-s", k.serviceName, "-a", k.accountName, "-w", secret)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return nil
}

func (k *Keychain) deleteDarwin() error {
	cmd := exec.Command("security", "delete-generic-password", "-s", k.serviceName, "-a", k.accountName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return nil
}

// --- Windows Implementation ---

func (k *Keychain) getWindows() (string, error) {
	cmd := exec.Command("cmdkey", "/list", k.serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", ErrNotFound
	}

	// Use PowerShell to retrieve the password
	psCmd := fmt.Sprintf(`(Get-Credential -User "%s" -Password (ConvertTo-SecureString -AsPlainText -Force)).Password`, k.accountName)
	cmd = exec.Command("powershell", "-Command", psCmd)
	output, err = cmd.Output()
	if err != nil {
		return "", ErrNotFound
	}
	return string(output), nil
}

func (k *Keychain) setWindows(secret string) error {
	// Use cmdkey directly with argument arrays to avoid shell injection
	cmd := exec.Command("cmdkey", "/add:"+k.serviceName, "/user:"+k.accountName, "/pass:"+secret)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return nil
}

func (k *Keychain) deleteWindows() error {
	cmd := exec.Command("cmdkey", "/delete", k.serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return nil
}

// --- Linux Implementation ---

func (k *Keychain) getLinux() (string, error) {
	// Try secret-tool first (libsecret)
	cmd := exec.Command("secret-tool", "lookup", "service", k.serviceName, "user", k.accountName)
	output, err := cmd.Output()
	if err == nil {
		return string(output), nil
	}

	// Fallback: try keyring
	cmd = exec.Command("keyring", "get", k.serviceName, k.accountName)
	output, err = cmd.Output()
	if err != nil {
		return "", ErrNotFound
	}
	return string(output), nil
}

func (k *Keychain) setLinux(secret string) error {
	// Try secret-tool first (libsecret) — password is read from stdin
	cmd := exec.Command("secret-tool", "store", "--label", k.serviceName, "service", k.serviceName, "user", k.accountName)
	cmd.Stdin = strings.NewReader(secret)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return nil
}

func (k *Keychain) deleteLinux() error {
	cmd := exec.Command("secret-tool", "clear", "service", k.serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}
	return nil
}

// Helper function to get API key from keychain
func GetAPIKey() (string, error) {
	keychain := New("synesis", "api-key")
	return keychain.Get()
}

// Helper function to set API key in keychain
func SetAPIKey(key string) error {
	keychain := New("synesis", "api-key")
	return keychain.Set(key)
}

// Helper function to delete API key from keychain
func DeleteAPIKey() error {
	keychain := New("synesis", "api-key")
	return keychain.Delete()
}

// Helper function to check if API key exists in keychain
func HasAPIKey() (bool, error) {
	keychain := New("synesis", "api-key")
	return keychain.Exists()
}

// ProfileKeychain returns a keychain scoped to a named profile.
func ProfileKeychain(profileName string) *Keychain {
	if profileName == "" {
		return New("synesis", "api-key")
	}
	return New("synesis-profile", profileName)
}

// GetProfileAPIKey retrieves a key for a named profile from the keychain.
func GetProfileAPIKey(profileName string) (string, error) {
	return ProfileKeychain(profileName).Get()
}

// SetProfileAPIKey stores a key for a named profile in the keychain.
func SetProfileAPIKey(profileName, key string) error {
	return ProfileKeychain(profileName).Set(key)
}

// DeleteProfileAPIKey removes a key for a named profile from the keychain.
func DeleteProfileAPIKey(profileName string) error {
	return ProfileKeychain(profileName).Delete()
}

// HasProfileAPIKey checks if a key exists for a named profile in the keychain.
func HasProfileAPIKey(profileName string) (bool, error) {
	return ProfileKeychain(profileName).Exists()
}
