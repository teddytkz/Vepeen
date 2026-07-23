package vpn

import (
	"strings"
	"testing"
)

func TestNatValueOK(t *testing.T) {
	if !NatValueOK(2) {
		t.Error("NatValueOK(2) = false, want true")
	}
	if NatValueOK(0) {
		t.Error("NatValueOK(0) = true, want false")
	}
	if NatValueOK(1) {
		t.Error("NatValueOK(1) = true, want false")
	}
}

func TestEnsureNATRegistryNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EnsureNATRegistry panicked: %v", r)
		}
	}()
	res, err := EnsureNATRegistry()
	switch res {
	case NATOK, NATSet, NATElevationRequired:
		// valid enum value
	default:
		t.Fatalf("unexpected NATResult: %v", res)
	}
	if res == NATElevationRequired {
		if err == nil {
			t.Fatal("NATElevationRequired must return a non-nil error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "administrator") {
			t.Fatalf("elevation error should mention administrator, got: %v", err)
		}
	}
}
