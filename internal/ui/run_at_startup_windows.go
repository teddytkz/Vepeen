//go:build windows

package ui

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// runKey is the HKCU Run key where autostart entries live.
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// runValueName is the value name under the Run key. The value data is the
// quoted executable path plus the --autostart flag. Quoting matters: without
// it, paths with spaces are misparsed by ShellExecute. The flag lets the app
// distinguish an autostart launch (hide to tray) from a manual double-click
// (show the window).
const runValueName = "Vepeen"

// IsRunOnStartup reports whether the Vepeen autostart entry exists in the
// HKCU Run key. The registry is the single source of truth, so a user
// disabling Vepeen in Task Manager's Startup tab is reflected here.
func IsRunOnStartup() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValueName)
	return err == nil
}

// SetRunOnStartup adds or removes the Vepeen autostart entry in the HKCU
// Run key, pointing at the current executable path.
func SetRunOnStartup(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if enabled {
		return k.SetStringValue(runValueName, `"`+exe+`" --autostart`)
	}
	err = k.DeleteValue(runValueName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
