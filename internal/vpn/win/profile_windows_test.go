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
