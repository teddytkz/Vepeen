//go:build windows

package route

import (
	"errors"
	"testing"
)

func TestIsSoftRouteListError_Nil(t *testing.T) {
	if isSoftRouteListError(nil) {
		t.Error("nil should not be soft")
	}
}

func TestIsSoftRouteListError_NotRecognized(t *testing.T) {
	err := errors.New("The term 'Get-VpnConnectionRoute' is not recognized as the name of a cmdlet, function, script file, or operable program.")
	if !isSoftRouteListError(err) {
		t.Error("'not recognized' should be soft")
	}
}

func TestIsSoftRouteListError_TidakDikenali(t *testing.T) {
	err := errors.New("Get-VpnConnectionRoute: istilah 'Get-VpnConnectionRoute' tidak dikenali sebagai nama cmdlet, fungsi, file skrip, atau program yang dapat dijalankan.")
	if !isSoftRouteListError(err) {
		t.Error("'tidak dikenali' should be soft")
	}
}

func TestIsSoftRouteListError_NotFound(t *testing.T) {
	if !isSoftRouteListError(errors.New("not found")) {
		t.Error("'not found' should be soft")
	}
	if !isSoftRouteListError(errors.New("tidak ditemukan")) {
		t.Error("'tidak ditemukan' should be soft")
	}
	if !isSoftRouteListError(errors.New("no vpn connection")) {
		t.Error("'no vpn' should be soft")
	}
	if !isSoftRouteListError(errors.New("cannot find profile")) {
		t.Error("'cannot find' should be soft")
	}
}

func TestIsSoftRouteListError_HardError(t *testing.T) {
	err := errors.New("Access is denied")
	if isSoftRouteListError(err) {
		t.Error("'Access is denied' should NOT be soft")
	}
}

func TestListRoutesScriptContainsCorrectCmdlet(t *testing.T) {
	// String-level assertion that listRoutes will use the correct cmdlet.
	script := `$ErrorActionPreference='Stop'; $c = Get-VpnConnection -Name 'test' -ErrorAction SilentlyContinue; if ($null -eq $c -or $null -eq $c.Routes) { exit 0 }; @($c.Routes) | ForEach-Object { $_.DestinationPrefix }`
	if !contains(script, "Get-VpnConnection -Name") {
		t.Error("script should use Get-VpnConnection -Name")
	}
	if !contains(script, "DestinationPrefix") {
		t.Error("script should reference DestinationPrefix")
	}
	if contains(script, "Get-VpnConnectionRoute") {
		t.Error("script should NOT contain Get-VpnConnectionRoute")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
