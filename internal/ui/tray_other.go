//go:build !windows

package ui

import "fyne.io/fyne/v2"

// SetupTray is a no-op on non-Windows platforms.
// System tray behaviour is only wired on Windows.
func SetupTray(a fyne.App, w fyne.Window, onQuit func()) { //nolint:revive // intentionally empty stub
}
