//go:build windows

package route

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// SyncRoutes makes the VPN profile's routes match desired IPv4 prefixes.
// Uses Add-VpnConnectionRoute / Remove-VpnConnectionRoute (per-user profile).
// desired must already be normalized (e.g. via ParseLines).
func SyncRoutes(connectionName string, desired []string) error {
	name := strings.TrimSpace(connectionName)
	if name == "" {
		return fmt.Errorf("connection name is empty")
	}
	if err := validateConnectionName(name); err != nil {
		return err
	}
	for _, p := range desired {
		if err := validatePrefix(p); err != nil {
			return err
		}
	}

	existing, err := listRoutes(name)
	if err != nil {
		return fmt.Errorf("list profile routes: %w", err)
	}

	desiredSet := make(map[string]struct{}, len(desired))
	for _, p := range desired {
		desiredSet[p] = struct{}{}
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, p := range existing {
		existingSet[p] = struct{}{}
	}

	// Remove stale
	for p := range existingSet {
		if _, ok := desiredSet[p]; !ok {
			if err := removeRoute(name, p); err != nil {
				return fmt.Errorf("remove route %s: %w", p, err)
			}
		}
	}
	// Add missing
	for p := range desiredSet {
		if _, ok := existingSet[p]; !ok {
			if err := addRoute(name, p); err != nil {
				return fmt.Errorf("add route %s: %w", p, err)
			}
		}
	}
	return nil
}

func listRoutes(connectionName string) ([]string, error) {
	// Output one prefix per line; empty if none.
	script := fmt.Sprintf(
		`$ErrorActionPreference='Stop'; $c = Get-VpnConnection -Name %s -ErrorAction SilentlyContinue; if ($null -eq $c -or $null -eq $c.Routes) { exit 0 }; @($c.Routes) | ForEach-Object { $_.DestinationPrefix }`,
		psQuote(connectionName),
	)
	out, err := runPowerShell(script)
	if err != nil {
		// Some systems error when no routes; treat empty as ok if message is soft.
		if strings.TrimSpace(out) == "" && isSoftRouteListError(err) {
			return nil, nil
		}
		return nil, err
	}
	var prefixes []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Normalize if possible
		norm, nerr := normalizePrefix(line)
		if nerr != nil {
			prefixes = append(prefixes, line)
			continue
		}
		prefixes = append(prefixes, norm)
	}
	return prefixes, nil
}

func addRoute(connectionName, prefix string) error {
	script := fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Add-VpnConnectionRoute -ConnectionName %s -DestinationPrefix %s`,
		psQuote(connectionName),
		psQuote(prefix),
	)
	_, err := runPowerShell(script)
	return err
}

func removeRoute(connectionName, prefix string) error {
	script := fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Remove-VpnConnectionRoute -ConnectionName %s -DestinationPrefix %s -ErrorAction Stop`,
		psQuote(connectionName),
		psQuote(prefix),
	)
	_, err := runPowerShell(script)
	return err
}

func runPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", err
		}
		return text, fmt.Errorf("%s", sanitizePSError(text))
	}
	return text, nil
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func validateConnectionName(name string) error {
	if strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Errorf("invalid connection name")
	}
	return nil
}

func validatePrefix(p string) error {
	if _, err := normalizePrefix(p); err != nil {
		return fmt.Errorf("invalid prefix: %s", p)
	}
	return nil
}

func isSoftRouteListError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "not recognized") ||
		strings.Contains(msg, "no vpn") ||
		strings.Contains(msg, "cannot find")
}

// sanitizePSError strips obvious secret-like patterns; keeps message short.
func sanitizePSError(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var kept []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "l2tppsk") || strings.Contains(lower, "password") {
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			kept = append(kept, line)
		}
	}
	out := strings.Join(kept, " ")
	if len(out) > 300 {
		out = out[:300] + "…"
	}
	return out
}
