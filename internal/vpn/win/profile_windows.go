//go:build windows

package win

import (
	"fmt"
	"strings"

	"vepeen/internal/vpn/shared"
)

// ListProfiles enumerates all Windows VPN connections via Get-VpnConnection.
// Returns an empty (non-nil) slice and no error when none exist.
func ListProfiles() ([]shared.ProfileSummary, error) {
	script := `Get-VpnConnection -ErrorAction SilentlyContinue | ForEach-Object {
        "$($_.Name)|$($_.ServerAddress)|$($_.TunnelType)|$([int]$_.SplitTunneling)|$($_.ConnectionStatus)"
    }`
	out, err := runPowerShell(script)
	if err != nil {
		return nil, shared.MapExecError("ListProfiles", err, out)
	}
	var res []shared.ProfileSummary
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}
		st := mapConnectionStatus(strings.TrimSpace(parts[4]))
		res = append(res, shared.ProfileSummary{
			Name:           strings.TrimSpace(parts[0]),
			ServerAddress:  strings.TrimSpace(parts[1]),
			TunnelType:     strings.TrimSpace(parts[2]),
			SplitTunneling: strings.TrimSpace(parts[3]) == "1",
			Status:         st,
		})
	}
	if res == nil {
		res = []shared.ProfileSummary{}
	}
	return res, nil
}

// EnforceSplitTunnel removes a default (0.0.0.0/0) route on the VPN interface if present,
// guaranteeing split-tunnel behavior even when the server pushes a default route.
// It is best-effort: errors are returned but should not abort the connect flow.
func EnforceSplitTunnel(name string) (string, error) {
	script := enforceSplitTunnelScript(name)
	out, err := runPowerShell(script)
	if err != nil {
		return shared.SanitizeOutput(strings.TrimSpace(out)), err
	}
	return strings.TrimSpace(out), nil
}

// enforceSplitTunnelScript builds the PowerShell that removes a server-pushed
// default route (0.0.0.0/0) using the real VPN interface resolved via
// Get-VpnConnection (alias/index), falling back to the profile name.
func enforceSplitTunnelScript(name string) string {
	return fmt.Sprintf(`$ErrorActionPreference='Stop'
$name = %s
try {
  $c = Get-VpnConnection -Name $name -ErrorAction SilentlyContinue
  $ifAlias = if ($c -and $c.InterfaceAlias) { $c.InterfaceAlias } else { $name }
  $ifIndex = if ($c -and $c.InterfaceIndex) { $c.InterfaceIndex } else { $null }
  if ($ifIndex -ne $null) {
    $r = Get-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue
    if ($r) {
      Remove-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix '0.0.0.0/0' -Confirm:$false -ErrorAction SilentlyContinue
      'removed-default'
    } else {
      'no-default'
    }
  } else {
    $r = Get-NetRoute -InterfaceAlias $ifAlias -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue
    if ($r) {
      Remove-NetRoute -InterfaceAlias $ifAlias -DestinationPrefix '0.0.0.0/0' -Confirm:$false -ErrorAction SilentlyContinue
      'removed-default'
    } else {
      'no-default'
    }
  }
} catch {
  'enforce-error'
}`, psQuote(name))
}

// ProfileDiagnostics returns a short status line: SplitTunneling on/off and route count.
func ProfileDiagnostics(name string) (string, error) {
	script := fmt.Sprintf(`$c = Get-VpnConnection -Name %s -ErrorAction SilentlyContinue
if ($null -eq $c) { 'profil tidak ada' }
else {
  $st = if ($c.SplitTunneling) { 'on' } else { 'off' }
  $rc = if ($null -eq $c.Routes) { 0 } else { @($c.Routes).Count }
  "SplitTunneling=$st rute=$rc"
}`, psQuote(name))
	out, err := runPowerShell(script)
	if err != nil {
		return shared.SanitizeOutput(strings.TrimSpace(out)), err
	}
	return strings.TrimSpace(out), nil
}

// ProfileExists reports whether a per-user VPN connection with the given name exists.
func ProfileExists(name string) (bool, error) {
	if err := shared.ValidateName(name); err != nil {
		return false, err
	}
	script := fmt.Sprintf(
		`$c = Get-VpnConnection -Name %s -ErrorAction SilentlyContinue; if ($null -ne $c) { '1' } else { '0' }`,
		psQuote(name),
	)
	out, err := runPowerShell(script)
	if err != nil {
		return false, shared.MapExecError("ProfileExists", err, out)
	}
	return strings.TrimSpace(out) == "1", nil
}

// EnsureSplitTunneling enables split tunneling on the named Windows VPN
// profile so only explicitly added routes traverse the tunnel. Best-effort.
func EnsureSplitTunneling(name string) error {
	if err := shared.ValidateName(name); err != nil {
		return err
	}
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$name = %s
$c = Get-VpnConnection -Name $name -ErrorAction SilentlyContinue
if ($null -eq $c) { Write-Output 'no-profile'; exit 0 }
if ($c.SplitTunneling) { Write-Output 'already-on' } else {
  Set-VpnConnection -Name $name -SplitTunneling $true -ErrorAction Stop
  Write-Output 'enabled'
}`, psQuote(name))
	out, err := runPowerShell(script)
	if err != nil {
		return shared.MapExecError("EnsureSplitTunneling", err, out)
	}
	return nil
}
