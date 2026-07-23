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
