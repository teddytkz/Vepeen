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

// EnforceSplitTunnel makes the live VPN connection a split tunnel: it removes a
// server-pushed default route (0.0.0.0/0) on the VPN interface if present, then
// adds each desired prefix to the live interface. Profile routes added via
// Add-VpnConnectionRoute only apply on the NEXT connect, so this live step is
// what makes the current session's split routes active immediately. It is
// best-effort: errors are returned but should not abort the connect flow.
func EnforceSplitTunnel(name string, prefixes []string) (string, error) {
	script := enforceSplitTunnelScript(name, prefixes)
	out, err := runPowerShell(script)
	if err != nil {
		return shared.SanitizeOutput(strings.TrimSpace(out)), err
	}
	return strings.TrimSpace(out), nil
}

// enforceSplitTunnelScript builds the PowerShell that resolves the real VPN
// interface, removes a server-pushed default route (0.0.0.0/0), and live-adds
// each prefix with New-NetRoute in the ActiveStore (so it is not persisted and
// does not duplicate the profile routes; it disappears when the adapter goes down).
//
// Interface resolution: Get-VpnConnection often returns an empty InterfaceIndex
// and InterfaceAlias for PPP/L2TP adapters even while Connected, so we resolve
// the index from the alias (which equals the connection name for RAS/PPP) via
// Get-NetIPConfiguration.
//
// Route adds are idempotent (check-then-add): New-NetRoute on an already-present
// route emits an error record, which under `powershell -Command` turns the
// process exit code non-zero even when the route is correct. Checking first
// avoids the spurious failure.
func enforceSplitTunnelScript(name string, prefixes []string) string {
	var add strings.Builder
	for _, p := range prefixes {
		fmt.Fprintf(&add, `$p = %s
if (-not (Get-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix $p -ErrorAction SilentlyContinue)) {
  New-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix $p -PolicyStore ActiveStore -ErrorAction SilentlyContinue
}
`, psQuote(p))
	}
	return fmt.Sprintf(`$name = %s
$c = Get-VpnConnection -Name $name -ErrorAction SilentlyContinue
if (-not $c) { Write-Output 'connection-not-found'; exit 0 }

$ifIndex = $c.InterfaceIndex
$ifAlias = $c.InterfaceAlias
if (-not $ifIndex) {
  if (-not $ifAlias) { $ifAlias = $name }
  $ifIndex = (Get-NetIPConfiguration -InterfaceAlias $ifAlias -ErrorAction SilentlyContinue | Select-Object -First 1).InterfaceIndex
}
if (-not $ifIndex) { Write-Output 'no-interface-info'; exit 0 }

$r = Get-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue
if ($r) {
  Remove-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix '0.0.0.0/0' -Confirm:$false -ErrorAction SilentlyContinue
}
%s
exit 0`, psQuote(name), add.String())
}

// ProfileDiagnostics returns a short status line: SplitTunneling on/off and route count.
func ProfileDiagnostics(name string) (string, error) {
	script := fmt.Sprintf(`$c = Get-VpnConnection -Name %s -ErrorAction SilentlyContinue
if ($null -eq $c) { 'profile-not-found' }
else {
  $st = if ($c.SplitTunneling) { 'on' } else { 'off' }
  $rc = if ($null -eq $c.Routes) { 0 } else { @($c.Routes).Count }
  "SplitTunneling=$st routes=$rc"
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

// DisableSplitTunneling disables split tunneling on the named Windows VPN
// profile so all traffic routes through the tunnel (the VPN default gateway
// is used). Best-effort.
func DisableSplitTunneling(name string) error {
	if err := shared.ValidateName(name); err != nil {
		return err
	}
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'
$name = %s
$c = Get-VpnConnection -Name $name -ErrorAction SilentlyContinue
if ($null -eq $c) { Write-Output 'no-profile'; exit 0 }
if (-not $c.SplitTunneling) { Write-Output 'already-off' } else {
  Set-VpnConnection -Name $name -SplitTunneling $false -ErrorAction Stop
  Write-Output 'disabled'
}`, psQuote(name))
	out, err := runPowerShell(script)
	if err != nil {
		return shared.MapExecError("DisableSplitTunneling", err, out)
	}
	return nil
}
