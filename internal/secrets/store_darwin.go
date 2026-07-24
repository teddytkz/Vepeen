//go:build darwin

// Package secrets on macOS backs onto the login Keychain via the `security`
// CLI. Secrets are keyed by Target(connectionName, kind) under service "vepeen".
package secrets

import (
	"bytes"
	"fmt"
	"os/exec"
)

const keychainService = "vepeen"

// NewStore returns a Keychain-backed secret store.
func NewStore() Store { return keychainStore{} }

// ReadLegacy has no legacy source on macOS.
func ReadLegacy(connectionName string, kind Kind) (string, bool) { return "", false }

// DeleteLegacy is a no-op on macOS.
func DeleteLegacy(connectionName string, kind Kind) error { return nil }

type keychainStore struct{}

func (keychainStore) Set(connectionName string, kind Kind, value string) error {
	if !ValidateKind(kind) {
		return fmt.Errorf("unsupported secret kind %q", kind)
	}
	acct := Target(connectionName, kind)
	if value == "" {
		return keychainStore{}.Delete(connectionName, kind)
	}
	_ = exec.Command("security", "delete-generic-password",
		"-s", keychainService, "-a", acct).Run()
	return exec.Command("security", "add-generic-password",
		"-s", keychainService, "-a", acct, "-w", value, "-U").Run()
}

func (keychainStore) Get(connectionName string, kind Kind) (string, error) {
	if !ValidateKind(kind) {
		return "", fmt.Errorf("unsupported secret kind %q", kind)
	}
	out, err := exec.Command("security", "find-generic-password",
		"-s", keychainService, "-a", Target(connectionName, kind), "-w").Output()
	if err != nil {
		return "", nil // not found → ("", nil) per Store contract
	}
	return string(bytes.TrimRight(out, "\n")), nil
}

func (keychainStore) Delete(connectionName string, kind Kind) error {
	if !ValidateKind(kind) {
		return fmt.Errorf("unsupported secret kind %q", kind)
	}
	_ = exec.Command("security", "delete-generic-password",
		"-s", keychainService, "-a", Target(connectionName, kind)).Run()
	return nil
}
