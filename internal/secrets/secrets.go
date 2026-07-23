// Package secrets stores VPN PSK and password outside plain config JSON.
// Preferred backend on Windows: Credential Manager. Never write secrets to config.json.
package secrets

import "strings"

// Kind identifies a secret type for a connection.
type Kind string

const (
	KindPassword Kind = "password"
	KindUsername Kind = "username"
)

// Store persists and retrieves secrets for a named VPN connection.
type Store interface {
	// Set stores a secret. Empty value deletes the secret when supported.
	Set(connectionName string, kind Kind, value string) error
	// Get returns the secret or ("", nil) if not found.
	Get(connectionName string, kind Kind) (string, error)
	// Delete removes a secret if present.
	Delete(connectionName string, kind Kind) error
}

// Target builds a Credential Manager / store key for a connection secret.
// Example: vepeen/Vepeen/psk
func Target(connectionName string, kind Kind) string {
	name := strings.TrimSpace(connectionName)
	if name == "" {
		name = "Vepeen"
	}
	return "vepeen/" + name + "/" + string(kind)
}

// ValidateKind returns true for supported secret kinds.
func ValidateKind(kind Kind) bool {
	switch kind {
	case KindPassword, KindUsername:
		return true
	default:
		return false
	}
}
