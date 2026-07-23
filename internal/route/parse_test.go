package route

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestParseLines_Valid(t *testing.T) {
	text := `
# office
10.10.0.0/16
203.0.113.50
203.0.113.50/32

10.0.0.0/8
`
	got, err := ParseLines(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.10.0.0/16", "203.0.113.50/32", "10.0.0.0/8"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestParseLines_InvalidLineNumber(t *testing.T) {
	text := "10.0.0.0/24\nabc\n1.2.3.4"
	_, err := ParseLines(text)
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("want *ParseError, got %T", err)
	}
	if pe.Line != 2 {
		t.Errorf("line=%d want 2", pe.Line)
	}
}

func TestParseLines_IPv6Rejected(t *testing.T) {
	_, err := ParseLines("2001:db8::/32")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseLines_Empty(t *testing.T) {
	got, err := ParseLines("\n# only comment\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestNormalizeNetworkBits(t *testing.T) {
	// Host bits should be zeroed in canonical form.
	got, err := ParseLines("10.10.5.7/16")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "10.10.0.0/16" {
		t.Fatalf("got %v", got)
	}
}

func TestNormalizeEntry_IPv4(t *testing.T) {
	got, err := normalizeEntry("203.0.113.50")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "203.0.113.50/32" {
		t.Fatalf("got %q want %q", got, "203.0.113.50/32")
	}
}

func TestNormalizeEntry_CIDR(t *testing.T) {
	got, err := normalizeEntry("10.0.0.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "10.0.0.0/24" {
		t.Fatalf("got %q want %q", got, "10.0.0.0/24")
	}
}

func TestNormalizeEntry_DomainLowercased(t *testing.T) {
	got, err := normalizeEntry("Git-RBI.Xxx.xxx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "git-rbi.xxx.xxx" {
		t.Fatalf("got %q want %q", got, "git-rbi.xxx.xxx")
	}
}

func TestNormalizeEntry_InvalidString(t *testing.T) {
	_, err := normalizeEntry("not a domain")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeEntry_WildcardRejected(t *testing.T) {
	_, err := normalizeEntry("*.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeEntry_IPv6Rejected(t *testing.T) {
	_, err := normalizeEntry("::1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRoutes_DomainExpandsToIPv4(t *testing.T) {
	saved := lookupHost
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		return []string{"203.0.113.50"}, nil
	}
	defer func() { lookupHost = saved }()

	got, err := ResolveRoutes(context.Background(), []string{"example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"203.0.113.50/32"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveRoutes_IPPassesThrough(t *testing.T) {
	saved := lookupHost
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		return nil, nil
	}
	defer func() { lookupHost = saved }()

	got, err := ResolveRoutes(context.Background(), []string{"10.0.0.0/24"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.0/24"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveRoutes_Dedupe(t *testing.T) {
	saved := lookupHost
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		return []string{"203.0.113.50"}, nil
	}
	defer func() { lookupHost = saved }()

	got, err := ResolveRoutes(context.Background(), []string{"example.com", "203.0.113.50/32"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry after dedupe, got %v", got)
	}
	if got[0] != "203.0.113.50/32" {
		t.Fatalf("got %q want %q", got[0], "203.0.113.50/32")
	}
}

func TestResolveRoutes_LookupError(t *testing.T) {
	saved := lookupHost
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		return nil, fmt.Errorf("NXDOMAIN")
	}
	defer func() { lookupHost = saved }()

	_, err := ResolveRoutes(context.Background(), []string{"example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gagal mengresolve") {
		t.Errorf("error %q should contain 'gagal mengresolve'", err)
	}
}

func TestResolveRoutes_OnlyIPv6(t *testing.T) {
	saved := lookupHost
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		return []string{"::1", "2001:db8::1"}, nil
	}
	defer func() { lookupHost = saved }()

	_, err := ResolveRoutes(context.Background(), []string{"ipv6only.example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "tidak memiliki alamat IPv4") {
		t.Errorf("error %q should contain 'tidak memiliki alamat IPv4'", err)
	}
}
