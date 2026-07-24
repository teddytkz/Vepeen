//go:build darwin

package route

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// SyncRoutes adds the desired split-tunnel prefixes to the active VPN tunnel
// interface via `route add`. This needs root; without it the commands fail and
// SyncRoutes returns an error the manager treats as best-effort (connect still
// succeeds, user sees a warning). connectionName is unused on macOS — routes are
// bound to the tunnel interface, not the profile.
//
// ponytail: no route reconciliation (add-only, no diff/remove). macOS tears down
// tunnel routes on disconnect, so stale routes aren't an issue in the dial-existing
// -service model. Add diffing if that assumption breaks.
func SyncRoutes(connectionName string, desired []string) error {
	if len(desired) == 0 {
		return nil
	}
	iface, err := tunnelInterface()
	if err != nil {
		return fmt.Errorf("no active VPN tunnel interface for routes: %w", err)
	}
	var failed []string
	for _, prefix := range desired {
		net := toNet(prefix)
		out, err := exec.Command("route", "-n", "add", "-net", net, "-interface", iface).CombinedOutput()
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s (%s)", prefix, strings.TrimSpace(string(out))))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not add %d route(s) (needs root; run the app with sudo for split tunnel): %s",
			len(failed), strings.Join(failed, "; "))
	}
	return nil
}

// toNet normalizes a bare IP to a /32 so `route -net` accepts it.
func toNet(prefix string) string {
	if strings.Contains(prefix, "/") {
		return prefix
	}
	return prefix + "/32"
}

// tunnelInterface returns the first ppp/utun interface with an IPv4 address.
func tunnelInterface() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, ifi := range ifaces {
		if !strings.HasPrefix(ifi.Name, "ppp") && !strings.HasPrefix(ifi.Name, "utun") {
			continue
		}
		addrs, _ := ifi.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
				return ifi.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no active tunnel interface")
}
