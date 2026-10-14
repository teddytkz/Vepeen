//go:build windows

package win

import (
	"strings"
	"testing"
)

func TestEnforceSplitTunnelScriptResolvesAlias(t *testing.T) {
	// Build the script the same way EnforceSplitTunnel does and assert it
	// resolves the interface via Get-VpnConnection (alias/index), not bare $name.
	script := enforceSplitTunnelScript("Vepeen", nil)
	if !strings.Contains(script, "Get-VpnConnection -Name") {
		t.Error("script should call Get-VpnConnection -Name")
	}
	if !strings.Contains(script, "InterfaceAlias") {
		t.Error("script should reference InterfaceAlias")
	}
	if !strings.Contains(script, "InterfaceIndex") {
		t.Error("script should reference InterfaceIndex")
	}
}

func TestEnforceSplitTunnelScriptLiveAddsRoutes(t *testing.T) {
	// The live route add must use New-NetRoute in the ActiveStore so it does not
	// duplicate the profile-persisted routes, and must include each prefix.
	script := enforceSplitTunnelScript("Vepeen", []string{"10.0.0.0/24", "203.0.113.50/32"})
	if !strings.Contains(script, "New-NetRoute") {
		t.Error("script should live-add routes with New-NetRoute")
	}
	if !strings.Contains(script, "ActiveStore") {
		t.Error("script should use -PolicyStore ActiveStore to avoid duplicating profile routes")
	}
	if !strings.Contains(script, "10.0.0.0/24") {
		t.Error("script should include the 10.0.0.0/24 prefix")
	}
	if !strings.Contains(script, "203.0.113.50/32") {
		t.Error("script should include the 203.0.113.50/32 prefix")
	}
}

// TestEnforceSplitTunnelScriptIsIdempotent guards the regression where
// New-NetRoute on an already-present route emitted an error record that, under
// `powershell -Command`, produced a spurious non-zero exit ("exit status 1").
// The script must check for an existing route before adding, and end with
// `exit 0` so a clean run always reports success.
func TestEnforceSplitTunnelScriptIsIdempotent(t *testing.T) {
	script := enforceSplitTunnelScript("Vepeen", []string{"43.133.132.75/32"})
	if !strings.Contains(script, "Get-NetRoute") {
		t.Error("script should check for an existing route before adding (idempotent add)")
	}
	if !strings.Contains(script, "exit 0") {
		t.Error("script should end with `exit 0` so a clean run reports success")
	}
	// The add must be guarded by the existence check, not unconditional.
	if strings.Index(script, "Get-NetRoute") > strings.Index(script, "New-NetRoute") {
		t.Error("Get-NetRoute existence check should precede New-NetRoute")
	}
}

// TestEnforceSplitTunnelScriptResolvesIndexFromAlias guards the case where
// Get-VpnConnection returns an empty InterfaceIndex (common for PPP/L2TP
// adapters). The script must fall back to resolving the index from the alias.
func TestEnforceSplitTunnelScriptResolvesIndexFromAlias(t *testing.T) {
	script := enforceSplitTunnelScript("Jatis - 122.50.6.195", []string{"43.133.132.75/32"})
	if !strings.Contains(script, "Get-NetIPConfiguration") {
		t.Error("script should resolve InterfaceIndex from the alias when Get-VpnConnection leaves it empty")
	}
}
