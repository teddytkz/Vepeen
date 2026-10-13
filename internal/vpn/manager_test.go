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

// TestConnectFull_ResolvesDomainsAfterDial is the regression test for the bug:
// domain entries must be resolved AFTER the VPN dials (so the VPN's own DNS can
// answer split-DNS lookups), not before.
func TestConnectFull_ResolvesDomainsAfterDial(t *testing.T) {
	m := NewManager()
	m.natCheckFn = func() (NATResult, error) { return NATOK, nil }
	m.syncRoutesFn = func(string, []string) error { return nil }

	dialed := false
	m.connectFn = func(ConnectParams) error {
		dialed = true
		return nil
	}
	resolvedAfterDial := false
	m.resolveDomainsFn = func(_ context.Context, domains []string) ([]string, []string) {
		resolvedAfterDial = dialed
		return []string{"203.0.113.50/32"}, nil
	}
	m.enforceSplitTunnelFn = func(string, []string) (string, error) { return "", nil }

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
	if !resolvedAfterDial {
		t.Fatal("domains were resolved before the VPN dialed — bug not fixed")
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

// TestConnectFull_DomainResolveFailureWarnsButConnects verifies that a domain
// that still fails to resolve after dialing is a warning, not a fatal error.
func TestConnectFull_DomainResolveFailureWarnsButConnects(t *testing.T) {
	m := NewManager()
	m.natCheckFn = func() (NATResult, error) { return NATOK, nil }
	m.syncRoutesFn = func(string, []string) error { return nil }
	m.connectFn = func(ConnectParams) error { return nil }
	m.resolveDomainsFn = func(_ context.Context, domains []string) ([]string, []string) {
		return nil, []string{"Could not resolve domain \"example.com\": NXDOMAIN. Its routes were skipped."}
	}
	m.enforceSplitTunnelFn = func(string, []string) (string, error) { return "", nil }

	warnings, err := m.ConnectFull(context.Background(), ConnectRequest{
		Name:       "Vepeen",
		RoutesText: "example.com",
	}, nil)
	if err != nil {
		t.Fatalf("connect should not fail on domain resolve error, got: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning about the unresolved domain")
	}
	if !strings.Contains(warnings[0], "example.com") {
		t.Errorf("warning should name the failing domain, got %q", warnings[0])
	}
}
