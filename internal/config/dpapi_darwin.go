//go:build darwin

package config

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
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

// storeKey returns the 32-byte AES key, creating and persisting it on first use.
func storeKey() ([]byte, error) {
	if b64, err := securityFind(keychainService, keychainKeyAcct); err == nil {
		if key, derr := base64.StdEncoding.DecodeString(b64); derr == nil && len(key) == 32 {
			return key, nil
		}
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

// securityFind returns the password for service/account, or an error if absent.
func securityFind(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(out, "\n")), nil
}

// securitySet upserts the password (delete-then-add; add alone fails if it exists).
func securitySet(service, account, value string) error {
	_ = exec.Command("security", "delete-generic-password",
		"-s", service, "-a", account).Run()
	return exec.Command("security", "add-generic-password",
		"-s", service, "-a", account, "-w", value, "-U").Run()
}

// securityDelete removes the item; a missing item is not an error.
func securityDelete(service, account string) error {
	_ = exec.Command("security", "delete-generic-password",
		"-s", service, "-a", account).Run()
	return nil
}
