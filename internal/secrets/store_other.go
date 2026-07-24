//go:build !windows && !darwin

// Package secrets is retained ONLY as a migration-only, read-only fallback for
// importing credentials from the legacy Windows Credential Manager into
// vepeen.bin. On non-Windows platforms there is no Credential Manager, so the
// read helpers are no-ops. This file exists only so the package compiles
// cross-platform; it will be deleted in a follow-up release.
package secrets

import (
	"errors"
	"fmt"
	"sync"
)

// NewStore returns an in-memory secret store on non-Windows platforms.
// Secrets are never written to disk. VPN features are Windows-only.
func NewStore() Store {
	return &memoryStore{m: make(map[string]string)}
}

type memoryStore struct {
	mu sync.Mutex
	m  map[string]string
}

// ReadLegacy is a no-op off Windows: there is no Credential Manager to migrate.
func ReadLegacy(connectionName string, kind Kind) (string, bool) {
	return "", false
}

// DeleteLegacy is a no-op off Windows.
func DeleteLegacy(connectionName string, kind Kind) error {
	return nil
}

func (s *memoryStore) Set(connectionName string, kind Kind, value string) error {
	return errors.New("secrets store is migration-only and read-only")
}

func (s *memoryStore) Get(connectionName string, kind Kind) (string, error) {
	if !ValidateKind(kind) {
		return "", fmt.Errorf("unsupported secret kind %q", kind)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[Target(connectionName, kind)], nil
}

func (s *memoryStore) Delete(connectionName string, kind Kind) error {
	if !ValidateKind(kind) {
		return fmt.Errorf("unsupported secret kind %q", kind)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, Target(connectionName, kind))
	return nil
}
