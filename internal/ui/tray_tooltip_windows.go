//go:build windows

package ui

import "fyne.io/systray"

// setTrayTooltip sets the Windows tray tooltip shown on hover.
func setTrayTooltip(title string) {
	systray.SetTooltip(title)
}
