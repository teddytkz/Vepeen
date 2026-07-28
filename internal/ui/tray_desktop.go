//go:build windows || darwin

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
)

// SetupTray configures the system tray icon and menu, and intercepts the
// window close button so that clicking X hides the window to tray instead of
// quitting the application.
//
// Left-clicking the tray icon shows the window (via SetSystemTrayWindow).
// Right-clicking shows the menu with "Show" and "Quit".
func SetupTray(a fyne.App, w fyne.Window, onQuit func()) {
	desk, ok := a.(desktop.App)
	if !ok {
		// Not running as a desktop app (e.g. test driver) — skip tray setup.
		return
	}

	// Use the app icon if available, otherwise fall back to a built-in icon.
	icon := a.Icon()
	if icon == nil {
		icon = theme.ComputerIcon()
	}
	desk.SetSystemTrayIcon(icon)

	// Build right-click context menu.
	menu := fyne.NewMenu("Vepeen",
		fyne.NewMenuItem("Show", func() {
			w.Show()
			w.RequestFocus()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			onQuit()
		}),
	)
	desk.SetSystemTrayMenu(menu)

	// Left-click on tray icon shows the window (Fyne 2.7+).
	desk.SetSystemTrayWindow(w)

	// Intercept the X button: hide to tray instead of quitting.
	w.SetCloseIntercept(func() {
		w.Hide()
	})
}
