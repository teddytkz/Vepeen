//go:build darwin

package vpn

import (
	"testing"

	"vepeen/internal/vpn/shared"
)

func TestParseStatusWord(t *testing.T) {
	cases := map[string]shared.ConnStatus{
		"Connected":       shared.StatusConnected,
		"  connecting  ":  shared.StatusConnecting,
		"Disconnecting":   shared.StatusDisconnecting,
		"Disconnected":    shared.StatusDisconnected,
		"something weird": shared.StatusUnknown,
	}
	for in, want := range cases {
		if got := parseStatusWord(in); got != want {
			t.Errorf("parseStatusWord(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePingMs(t *testing.T) {
	out := "64 bytes from 1.1.1.1: icmp_seq=0 ttl=57 time=12.3 ms\n"
	ms, err := parsePingMs(out)
	if err != nil {
		t.Fatal(err)
	}
	if ms != 12 { // 12.3 rounds to 12
		t.Errorf("parsePingMs = %d, want 12", ms)
	}
	if _, err := parsePingMs("no rtt here"); err == nil {
		t.Error("expected error on output without rtt")
	}
}

func TestNcListLine(t *testing.T) {
	line := `* (Disconnected)   F025F3CC-57C8-428D-BD38-B070F011B2D5 PPP --> L2TP       "VPN O"                          [PPP:L2TP]`
	m := ncListLine.FindStringSubmatch(line)
	if m == nil {
		t.Fatal("regex did not match a real scutil --nc list line")
	}
	if m[3] != "VPN O" {
		t.Errorf("name = %q, want %q", m[3], "VPN O")
	}
	if parseStatusWord(m[1]) != shared.StatusDisconnected {
		t.Errorf("status = %q, want Disconnected", m[1])
	}
}
