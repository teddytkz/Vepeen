//go:build !windows && !darwin

package ui

import (
	"fyne.io/fyne/v2"
)

// AcquireSingleInstance is a no-op on non-Windows platforms.
// Returns alreadyRunning=false so the app always starts normally.
func AcquireSingleInstance() (alreadyRunning bool, release func()) {
	return false, func() {}
}

// ListenForShowSignal is a no-op on non-Windows platforms.
func ListenForShowSignal(w fyne.Window) {}
