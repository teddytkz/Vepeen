package vpn

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type errStub string

func (e errStub) Error() string { return string(e) }

func TestConnectFull_SyncRoutesErrorIsBestEffort(t *testing.T) {
	m := NewManager()
	m.natCheckFn = func() (NATResult, error) { return NATOK, nil }
	m.syncRoutesFn = func(string, []string) error {
		return errors.New("transient route sync failure")
	}
	m.connectFn = func(ConnectParams) error { return nil }
	m.enforceSplitTunnelFn = func(string, []string) (string, error) { return "", nil }

	warnings, err := m.ConnectFull(context.Background(), ConnectRequest{
		Name:       "Vepeen",
		RoutesText: "10.0.0.0/24",
	}, nil)
	if err != nil {
		t.Fatalf("expected connect to proceed, got err: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected non-empty warnings when SyncRoutes fails")
	}
}

func TestConnectFull_EmptyRoutesBlocked(t *testing.T) {
	m := NewManager()
	m.natCheckFn = func() (NATResult, error) { return NATOK, nil }
	m.syncRoutesFn = func(string, []string) error { return nil }
	m.connectFn = func(ConnectParams) error { return nil }
	m.enforceSplitTunnelFn = func(string, []string) (string, error) { return "", nil }

	_, err := m.ConnectFull(context.Background(), ConnectRequest{
		Name:       "Vepeen",
		RoutesText: "",
	}, nil)
	if err == nil {
		t.Fatal("expected validation error for empty routes")
	}
	ue, ok := AsUserError(err)
	if !ok {
		t.Fatalf("expected UserError, got %T %v", err, err)
	}
	if ue.Code != "validation" {
		t.Errorf("code=%q want validation", ue.Code)
	}
	if !strings.Contains(strings.ToLower(ue.Error()), "split tunnel") {
		t.Errorf("message should mention split tunnel, got: %s", ue.Error())
	}
}

// TestConnectFull_ResolvesDomainsBeforeDial is the regression test for the bug:
// domain entries (e.g. git-rbi.jatismobile.com, resolvable via public DNS) must
// be resolved BEFORE the VPN dials, so their IPs are synced into the profile and
// the live split-tunnel routes are added after dial.
func TestConnectFull_ResolvesDomainsBeforeDial(t *testing.T) {
	m := NewManager()
	m.natCheckFn = func() (NATResult, error) { return NATOK, nil }
	m.syncRoutesFn = func(string, []string) error { return nil }

	dialed := false
	m.connectFn = func(ConnectParams) error {
		dialed = true
		return nil
	}
	resolvedBeforeDial := false
	m.resolveRoutesFn = func(_ context.Context, entries []string) ([]string, []string, error) {
		resolvedBeforeDial = !dialed
		return []string{"10.0.0.0/24", "203.0.113.50/32"}, []string{"Domain example.com → 203.0.113.50 (injected as /32 route)"}, nil
	}

	var enforced []string
	m.enforceSplitTunnelFn = func(_ string, prefixes []string) (string, error) {
		enforced = prefixes
		return "", nil
	}

	warnings, err := m.ConnectFull(context.Background(), ConnectRequest{
		Name:       "Vepeen",
		RoutesText: "10.0.0.0/24\nexample.com",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !dialed {
		t.Fatal("connectFn was never called")
	}
	if !resolvedBeforeDial {
		t.Fatal("domains were resolved after the VPN dialed — expected pre-dial resolution")
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "example.com") {
		t.Fatalf("expected a domain→IP log note, got %v", warnings)
	}
	// Both static and resolved prefixes must reach the live enforce step.
	found := map[string]bool{}
	for _, p := range enforced {
		found[p] = true
	}
	if !found["10.0.0.0/24"] || !found["203.0.113.50/32"] {
		t.Fatalf("expected static + resolved prefixes enforced, got %v", enforced)
	}
}

// TestConnectFull_DomainResolveFailureIsFatal verifies that a domain that fails
// to resolve pre-dial aborts the connect with a clear validation error (the
// tunnel is not established for an unresolvable destination).
func TestConnectFull_DomainResolveFailureIsFatal(t *testing.T) {
	m := NewManager()
	m.natCheckFn = func() (NATResult, error) { return NATOK, nil }
	m.syncRoutesFn = func(string, []string) error { return nil }
	m.connectFn = func(ConnectParams) error { return nil }
	m.resolveRoutesFn = func(_ context.Context, entries []string) ([]string, []string, error) {
		return nil, nil, errors.New("failed to resolve domain \"example.com\": NXDOMAIN")
	}
	m.enforceSplitTunnelFn = func(string, []string) (string, error) { return "", nil }

	_, err := m.ConnectFull(context.Background(), ConnectRequest{
		Name:       "Vepeen",
		RoutesText: "example.com",
	}, nil)
	if err == nil {
		t.Fatal("expected connect to fail on unresolvable domain")
	}
	if !strings.Contains(err.Error(), "example.com") {
		t.Errorf("error should name the failing domain, got %q", err.Error())
	}
}
