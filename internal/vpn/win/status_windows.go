//go:build windows

package win

import (
	"fmt"
	"strings"

	"vepeen/internal/vpn/shared"
)

// QueryStatus returns the OS connection status for a named VPN profile.
func QueryStatus(name string) (shared.ConnStatus, error) {
	if err := shared.ValidateName(name); err != nil {
		return shared.StatusUnknown, err
	}
	script := fmt.Sprintf(
		`$ErrorActionPreference='SilentlyContinue'; $c = Get-VpnConnection -Name %s; if ($null -eq $c) { Write-Output 'Missing'; exit 0 }; Write-Output $c.ConnectionStatus`,
		psQuote(name),
	)
	out, err := runPowerShell(script)
	if err != nil {
		return shared.StatusUnknown, shared.MapExecError("Status", err, out)
	}
	return mapConnectionStatus(out), nil
}

func mapConnectionStatus(raw string) shared.ConnStatus {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch {
	case s == "connected":
		return shared.StatusConnected
	case s == "disconnected", s == "missing":
		return shared.StatusDisconnected
	case strings.Contains(s, "connect"):
		if strings.Contains(s, "disconnect") {
			return shared.StatusDisconnecting
		}
		return shared.StatusConnecting
	default:
		if s == "" {
			return shared.StatusDisconnected
		}
		return shared.StatusUnknown
	}
}
