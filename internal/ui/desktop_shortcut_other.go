//go:build !windows && !darwin

package ui

func CreateDesktopShortcut() error {
	return nil
}
