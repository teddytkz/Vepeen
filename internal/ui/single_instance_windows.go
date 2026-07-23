//go:build windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const mutexName = "Global\\VepeenSingleInstance"

var (
	modUser32         = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW   = modUser32.NewProc("FindWindowW")
	procShowWindow    = modUser32.NewProc("ShowWindow")
	procSetForeground = modUser32.NewProc("SetForegroundWindow")
)

const (
	swRestore = 9
)

// AcquireSingleInstance tries to create a global named mutex.
// If another instance already holds it, it finds the existing window by title,
// brings it to the foreground, and returns alreadyRunning=true.
// The caller should exit immediately when alreadyRunning is true.
// releaseFn closes the mutex handle and must be deferred in main on clean exit.
func AcquireSingleInstance() (alreadyRunning bool, release func()) {
	name, _ := windows.UTF16PtrFromString(mutexName)
	handle, err := windows.CreateMutex(nil, false, name)

	// err is non-nil AND matches ERROR_ALREADY_EXISTS when another instance owns it.
	if err != nil && err == windows.ERROR_ALREADY_EXISTS {
		bringExistingToFront()
		return true, func() {}
	}

	if handle == 0 {
		// Something went wrong creating the mutex — fail open (allow this instance).
		return false, func() {}
	}

	return false, func() {
		_ = windows.CloseHandle(handle)
	}
}

// bringExistingToFront finds the existing Vepeen window by title and raises it.
func bringExistingToFront() {
	titlePtr, err := windows.UTF16PtrFromString("Vepeen")
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindowW.Call(
		0, // lpClassName — nil matches any class
		uintptr(unsafe.Pointer(titlePtr)),
	)
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swRestore)
	procSetForeground.Call(hwnd)
}
