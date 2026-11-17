//go:build !windows

package ui

import "fyne.io/fyne/v2"

// WatchDisplayChanges is a no-op outside Windows.
func WatchDisplayChanges(a fyne.App, w fyne.Window) {}
