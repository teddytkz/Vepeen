//go:build !windows
// +build !windows

package ui

import "fyne.io/fyne/v2"

// ShowCentered centers the window and shows it. On non-Windows platforms Fyne's
// CenterOnScreen is used directly (no taskbar work-area adjustment needed).
func ShowCentered(w fyne.Window) {
	w.CenterOnScreen()
	w.Show()
}
