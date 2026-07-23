package vpn

import (
	"errors"
	"strings"
	"testing"
)

func TestMapExecError_Auth(t *testing.T) {
	err := MapExecError("Connect", errors.New("exit 1"), "Error 691: The remote connection was denied because the user name and password combination is not recognized")
	ue, ok := AsUserError(err)
	if !ok {
		t.Fatalf("want UserError, got %T %v", err, err)
	}
	if ue.Code != "auth" {
		t.Errorf("code=%q want auth", ue.Code)
	}
	if ue.Primary != "Gagal autentikasi" {
		t.Errorf("primary=%q", ue.Primary)
	}
}

func TestMapExecError_Elevation(t *testing.T) {
	err := MapExecError("EnsureProfile", errors.New("exit 1"), "Access is denied.")
	ue, ok := AsUserError(err)
	if !ok || ue.Code != "elevation" {
		t.Fatalf("got %#v", err)
	}
}

func TestMapExecError_NoSecretLeak(t *testing.T) {
	// Lines containing L2tpPsk are stripped by sanitizeOutput.
	err := MapExecError("EnsureProfile", errors.New("x"), "Add-VpnConnection -L2tpPsk supersecret123 failed\nSomething else")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecret123") {
		t.Fatalf("secret leaked: %s", err.Error())
	}
}

func TestValidateName(t *testing.T) {
	if ValidateName("") == nil {
		t.Fatal("empty should fail")
	}
	if ValidateName("Vepeen") != nil {
		t.Fatal("Vepeen should pass")
	}
}

func TestMapExecError_IPSec789(t *testing.T) {
	err := MapExecError("Connect", errors.New("exit 1"), "Error 789: L2TP connection attempt failed")
	ue, ok := AsUserError(err)
	if !ok {
		t.Fatalf("want UserError, got %T %v", err, err)
	}
	if ue.Code != "ipsec" {
		t.Errorf("code=%q want ipsec", ue.Code)
	}
	if ue.Primary != "Gagal terhubung (L2TP/IPsec)" {
		t.Errorf("primary=%q", ue.Primary)
	}
}

func TestMapExecError_IPSec800(t *testing.T) {
	err := MapExecError("Connect", errors.New("exit 1"), "Error 800: Unable to establish the L2TP connection")
	ue, ok := AsUserError(err)
	if !ok || ue.Code != "ipsec" {
		t.Fatalf("got %#v", err)
	}
	if ue.Primary != "Gagal terhubung (L2TP/IPsec)" {
		t.Errorf("primary=%q", ue.Primary)
	}
}

func TestMapExecError_IPSec809(t *testing.T) {
	err := MapExecError("Connect", errors.New("exit 1"), "Error 809: The network connection could not be established")
	ue, ok := AsUserError(err)
	if !ok || ue.Code != "ipsec" {
		t.Fatalf("got %#v", err)
	}
	if ue.Primary != "Gagal terhubung (L2TP/IPsec)" {
		t.Errorf("primary=%q", ue.Primary)
	}
}

func TestMapExecError_NetworkRegression(t *testing.T) {
	// A different network error must still map to "network" (case ordering guard).
	err := MapExecError("Connect", errors.New("exit 1"), "connection timed out")
	ue, ok := AsUserError(err)
	if !ok || ue.Code != "network" {
		t.Fatalf("got %#v want code network", err)
	}
}
