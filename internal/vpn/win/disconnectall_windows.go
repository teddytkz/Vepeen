//go:build windows

package win

import (
	"strings"
)

// DisconnectAllExcept disconnects every connected Windows VPN profile except
// exceptName. Returns the list of names it attempted to disconnect (for logging).
// Best-effort: individual failures are ignored so connect can proceed.
func DisconnectAllExcept(exceptName string) ([]string, error) {
	except := strings.TrimSpace(exceptName)
	script := `Get-VpnConnection | Where-Object { $_.ConnectionStatus -eq 'Connected' } | ForEach-Object { $_.Name }`
	out, err := runPowerShell(script)
	if err != nil {
		return nil, err
	}
	var disconnected []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == except {
			continue
		}
		if derr := Disconnect(name); derr != nil {
			// best-effort; ignore
			continue
		}
		disconnected = append(disconnected, name)
	}
	return disconnected, nil
}
