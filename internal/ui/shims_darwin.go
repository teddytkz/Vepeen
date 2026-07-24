//go:build darwin

package ui

import (
	"fmt"

	"fyne.io/fyne/v2"

	"vepeen/internal/vpn"
)

// pingGateway pings host via the platform VPN helper and formats the result.
func pingGateway(host string) string {
	ms, err := vpn.PingHost(host, 2000)
	if err != nil {
		return "unreachable"
	}
	return fmt.Sprintf("%d ms", ms)
}

// ShowCentered just shows the window; macOS centers new windows itself.
func ShowCentered(w fyne.Window) { w.Show() }

// AcquireSingleInstance is a no-op on macOS (single-instance not enforced in v1).
func AcquireSingleInstance() (alreadyRunning bool, release func()) {
	return false, func() {}
}

// ListenForShowSignal is a no-op on macOS.
func ListenForShowSignal(w fyne.Window) {}

// CreateDesktopShortcut is unsupported on macOS in v1.
func CreateDesktopShortcut() error {
	return fmt.Errorf("desktop shortcut is not supported on macOS")
}
