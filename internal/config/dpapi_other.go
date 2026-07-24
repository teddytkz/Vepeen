//go:build !windows && !darwin

package config

import "errors"

// encryptDPAPI is unavailable off Windows; the encrypted store is Windows-only.
func encryptDPAPI(plain []byte) ([]byte, error) {
	return nil, errors.New("encrypted config requires Windows")
}

// decryptDPAPI is unavailable off Windows; the encrypted store is Windows-only.
func decryptDPAPI(blob []byte) ([]byte, error) {
	return nil, errors.New("encrypted config requires Windows")
}
