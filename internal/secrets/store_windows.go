//go:build windows

// Package secrets is retained ONLY as a migration-only, read-only fallback for
// importing credentials from the legacy Windows Credential Manager into
// vepeen.bin. It will be deleted in a follow-up release once no users remain on
// the old config.json + Credential Manager format. Do not add write paths back.
package secrets

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// NewStore is retained for source compatibility but returns a read-only store.
// Write operations are no longer supported; use config.Stored instead.
func NewStore() Store {
	return &credManStore{}
}

type credManStore struct{}

const (
	credTypeGeneric = 1
	// credPersistEnterprise keeps entries in the current user's credential set
	// (user-scoped, non-roaming) — the same scope DPAPI now uses.
	credPersistEnterprise = 3
)

var (
	modAdvapi32     = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW   = modAdvapi32.NewProc("CredReadW")
	procCredFree    = modAdvapi32.NewProc("CredFree")
	procCredDeleteW = modAdvapi32.NewProc("CredDeleteW")
)

// CREDENTIALW layout (subset used by CredRead/CredFree).
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// ReadLegacy reads a single secret from the legacy Credential Manager. It is used
// only during migration (config.json + CredMan -> vepeen.bin). Returns ("", false)
// when the entry is absent.
func ReadLegacy(connectionName string, kind Kind) (string, bool) {
	if !ValidateKind(kind) {
		return "", false
	}
	target := Target(connectionName, kind)
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return "", false
	}
	var pcred *credentialW
	r1, _, errno := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r1 == 0 {
		if errno == windows.ERROR_NOT_FOUND || errno == syscall.Errno(1168) {
			return "", false
		}
		if errors.Is(errno, windows.ERROR_NOT_FOUND) {
			return "", false
		}
		return "", false
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))

	if pcred.CredentialBlobSize == 0 || pcred.CredentialBlob == nil {
		return "", false
	}
	data := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	// Copy so free does not invalidate the result.
	out := string(append([]byte(nil), data...))
	return out, true
}

// DeleteLegacy removes a single legacy Credential Manager entry. Used only during
// migration to purge migrated sources after a successful vepeen.bin write.
func DeleteLegacy(connectionName string, kind Kind) error {
	if !ValidateKind(kind) {
		return fmt.Errorf("unsupported secret kind %q", kind)
	}
	target := Target(connectionName, kind)
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	r1, _, errno := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
	)
	if r1 == 0 {
		if errno == windows.ERROR_NOT_FOUND || errno == syscall.Errno(1168) {
			return nil
		}
		if errors.Is(errno, windows.ERROR_NOT_FOUND) {
			return nil
		}
		return fmt.Errorf("CredDelete: %w", errnoToError(errno))
	}
	return nil
}

// The following satisfy the Store interface for source compatibility but are
// intentionally read-only. Writes are rejected; deletes during migration go
// through DeleteLegacy. The app no longer uses these methods directly.
func (s *credManStore) Set(connectionName string, kind Kind, value string) error {
	return errors.New("secrets store is migration-only and read-only")
}

func (s *credManStore) Get(connectionName string, kind Kind) (string, error) {
	if !ValidateKind(kind) {
		return "", fmt.Errorf("unsupported secret kind %q", kind)
	}
	v, _ := ReadLegacy(connectionName, kind)
	return v, nil
}

func (s *credManStore) Delete(connectionName string, kind Kind) error {
	return DeleteLegacy(connectionName, kind)
}

func errnoToError(errno error) error {
	if errno == nil || errno == syscall.Errno(0) {
		return errors.New("unknown windows error")
	}
	return errno
}
