//go:build windows
// +build windows

package ui

import "fyne.io/fyne/v2"

// ShowCentered centers the window and shows it. CenterOnScreen must be called
// BEFORE the window is shown; Fyne/GLFW then creates the native window already
// centered, so there is no 0,0 frame, no teleport, and no hide/show blink.
// Trade-off: centers on the full monitor (taskbar not excluded) — the only
// flicker-free option, since Fyne has no pre-show SetPosition API and the
// native HWND does not exist until after Show().
func ShowCentered(w fyne.Window) {
	w.CenterOnScreen()
	w.Show()
}
