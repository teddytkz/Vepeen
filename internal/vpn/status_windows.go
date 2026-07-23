//go:build windows

package vpn

import (
	"fmt"
	"strings"
)

// QueryStatus returns the OS connection status for a named VPN profile.
func QueryStatus(name string) (ConnStatus, error) {
	if err := ValidateName(name); err != nil {
		return StatusUnknown, err
	}
	script := fmt.Sprintf(
		`$ErrorActionPreference='SilentlyContinue'; $c = Get-VpnConnection -Name %s; if ($null -eq $c) { Write-Output 'Missing'; exit 0 }; Write-Output $c.ConnectionStatus`,
		psQuote(name),
	)
	out, err := runPowerShell(script)
	if err != nil {
		return StatusUnknown, MapExecError("Status", err, out)
	}
	return mapConnectionStatus(out), nil
}

func mapConnectionStatus(raw string) ConnStatus {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch {
	case s == "connected":
		return StatusConnected
	case s == "disconnected", s == "missing":
		return StatusDisconnected
	case strings.Contains(s, "connect"):
		if strings.Contains(s, "disconnect") {
			return StatusDisconnecting
		}
		return StatusConnecting
	default:
		if s == "" {
			return StatusDisconnected
		}
		return StatusUnknown
	}
}
