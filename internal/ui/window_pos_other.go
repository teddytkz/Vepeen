//go:build !windows
// +build !windows

package ui

import "fyne.io/fyne/v2"

// centerOnWorkArea centers the window. On non-Windows platforms Fyne's
// CenterOnScreen is used directly (no taskbar work-area adjustment needed).
func centerOnWorkArea(w fyne.Window) {
	w.CenterOnScreen()
}
