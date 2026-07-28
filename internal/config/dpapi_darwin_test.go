//go:build darwin

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// keychainAvailable reports whether `security` can reach a real login keychain.
// It cannot in sandboxed/CI environments, where these tests are skipped.
func keychainAvailable(t *testing.T) {
	t.Helper()
	if _, err := securityFind(keychainService, keychainKeyAcct); err != nil && !errors.Is(err, errNoKey) {
		t.Skipf("keychain unavailable: %v", err)
	}
}

// Encrypt then decrypt must round-trip using the SAME key across separate calls.
// Guards the bug where any keychain read failure minted a fresh key mid-flight,
// orphaning vepeen.bin so saved routes silently loaded as defaults.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	keychainAvailable(t)

	plain := []byte(`{"routes":["10.10.0.0/16","mail.foofle.com"]}`)
	blob, err := encryptDPAPI(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := decryptDPAPI(blob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plain)
	}
}

// storeKey must be stable across calls. If it ever re-mints, an existing store
// becomes permanently undecryptable.
func TestStoreKeyIsStable(t *testing.T) {
	keychainAvailable(t)

	a, err := storeKey()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b, err := storeKey()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("storeKey returned a different key on the second call")
	}
}

// A keychain read failure must propagate, never fall through to minting a new
// key over the existing one.
func TestStoreKeyFailsClosedOnReadError(t *testing.T) {
	if _, err := securityFind("vepeen-definitely-absent", "nobody"); !errors.Is(err, errNoKey) {
		t.Skipf("keychain unavailable: %v", err)
	}
}

// An existing-but-undecryptable store must surface an error rather than
// masquerading as a fresh install, so callers don't overwrite good data.
func TestUndecryptableStoreReportsError(t *testing.T) {
	path, err := BinPath()
	if err != nil {
		t.Skip("no bin path")
	}
	// Work on a copy so the developer's real store is never touched.
	if _, err := os.Stat(path); err == nil {
		t.Skip("real store present; skipping to avoid touching it")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Skip("cannot create config dir")
	}
	if err := os.WriteFile(path, []byte("not a valid gcm blob"), 0o600); err != nil {
		t.Skip("cannot write probe store")
	}
	defer os.Remove(path)

	if _, err := LoadStored(); err == nil {
		t.Fatal("expected error for undecryptable store, got nil")
	}
	if err := Save(Config{Routes: []string{"1.2.3.4"}}); err == nil {
		t.Fatal("Save overwrote an unreadable store")
	}
}
