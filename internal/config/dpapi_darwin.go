//go:build darwin

package config

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// macOS analogue of Windows DPAPI: vepeen.bin is AES-256-GCM encrypted with a
// per-user random key kept in the login Keychain (service "vepeen", account
// "store-key"). Keychain is unlocked with the user's login, matching DPAPI's
// user-scope, no-prompt behaviour.
const (
	keychainService = "vepeen"
	keychainKeyAcct = "store-key"
)

// errNoKey reports that no key item exists yet (as opposed to one existing but
// being unreadable — denied ACL, locked keychain, transient `security` failure).
var errNoKey = errors.New("no store key in keychain")

// storeKey returns the 32-byte AES key, creating and persisting it on first use.
//
// A read failure must NEVER fall through to minting a new key: doing so
// overwrites the existing key and permanently orphans vepeen.bin, which then
// silently decrypts to defaults and loses saved routes/credentials. Only a
// confirmed-absent item (exit status 44) may create one.
func storeKey() ([]byte, error) {
	b64, err := securityFind(keychainService, keychainKeyAcct)
	switch {
	case err == nil:
		key, derr := base64.StdEncoding.DecodeString(b64)
		if derr != nil || len(key) != 32 {
			return nil, errors.New("store key in keychain is malformed")
		}
		return key, nil
	case !errors.Is(err, errNoKey):
		return nil, fmt.Errorf("read store key: %w", err)
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := securitySet(keychainService, keychainKeyAcct, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

func encryptDPAPI(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("empty plaintext")
	}
	key, err := storeKey()
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func decryptDPAPI(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, errors.New("empty blob")
	}
	key, err := storeKey()
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("blob too short")
	}
	return gcm.Open(nil, blob[:ns], blob[ns:], nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// errItemNotFound is `security`'s exit status for "the item does not exist".
const securityNotFoundStatus = 44

// securityFind returns the password for service/account. A confirmed-absent item
// yields errNoKey; every other failure is returned as-is so callers can tell
// "nothing stored yet" apart from "stored but unreadable".
func securityFind(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == securityNotFoundStatus {
			return "", errNoKey
		}
		return "", err
	}
	return string(bytes.TrimRight(out, "\n")), nil
}

// securitySet adds the password. -U updates an existing item in place, so no
// delete-first is needed — and deleting first would destroy the only copy of the
// key if the subsequent add failed.
func securitySet(service, account, value string) error {
	return exec.Command("security", "add-generic-password",
		"-s", service, "-a", account, "-w", value, "-U").Run()
}

// securityDelete removes the item; a missing item is not an error.
func securityDelete(service, account string) error {
	_ = exec.Command("security", "delete-generic-password",
		"-s", service, "-a", account).Run()
	return nil
}
