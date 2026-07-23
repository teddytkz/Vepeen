//go:build windows

package config

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modCrypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
	procLocalFree          = windows.NewLazySystemDLL("kernel32.dll").NewProc("LocalFree")
)

// crYPTPROTECT_UI_FORBIDDEN suppresses any DPAPI prompt; protection is bound to
// the current Windows user account (non-roaming, equivalent to the prior CredMan scope).
const crYPTPROTECT_UI_FORBIDDEN = 0x1

// dataBlob mirrors the Win32 DATA_BLOB structure used by CryptProtectData /
// CryptUnprotectData. Data is a pointer (matching the native BYTE* pbData) so the
// output can be sliced without an unsafe.Pointer(uintptr) conversion that vet
// would flag.
type dataBlob struct {
	Size uint32
	Data *byte
}

// encryptDPAPI wraps CryptProtectData with user-scoped, no-prompt protection and
// no optional entropy. The output blob must be freed with LocalFree.
func encryptDPAPI(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("empty plaintext")
	}
	in := dataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var out dataBlob
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // description (optional)
		0, // optional entropy (none)
		0, // reserved
		0, // prompt struct (nil)
		crYPTPROTECT_UI_FORBIDDEN,
		uintptr(unsafe.Pointer(&out)),
	)
	// Keep the Go-managed input buffer alive for the duration of the native call.
	runtime.KeepAlive(plain)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.Data)))
	// Copy the native buffer into a Go-managed slice before LocalFree invalidates it.
	buf := make([]byte, out.Size)
	copy(buf, unsafe.Slice(out.Data, out.Size))
	return buf, nil
}

// decryptDPAPI wraps CryptUnprotectData (symmetric to encryptDPAPI): same user
// scope, no prompt, no entropy. The output blob must be freed with LocalFree.
func decryptDPAPI(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, errors.New("empty blob")
	}
	in := dataBlob{Size: uint32(len(blob)), Data: &blob[0]}
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // description output (optional)
		0, // optional entropy (none)
		0, // reserved
		0, // prompt struct (nil)
		crYPTPROTECT_UI_FORBIDDEN,
		uintptr(unsafe.Pointer(&out)),
	)
	// Keep the Go-managed input buffer alive for the duration of the native call.
	runtime.KeepAlive(blob)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.Data)))
	// Copy the native buffer into a Go-managed slice before LocalFree invalidates it.
	buf := make([]byte, out.Size)
	copy(buf, unsafe.Slice(out.Data, out.Size))
	return buf, nil
}
