//go:build windows

package vpn

import (
	"strings"
	"testing"
)

func TestEnforceSplitTunnelScriptResolvesAlias(t *testing.T) {
	// Build the script the same way EnforceSplitTunnel does and assert it
	// resolves the interface via Get-VpnConnection (alias/index), not bare $name.
	script := enforceSplitTunnelScript("Vepeen")
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
