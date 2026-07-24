//go:build darwin

package route

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// SyncRoutes is a no-op on macOS. Routes attach to the live tunnel interface
// (ppp0), which does not exist until AFTER the VPN dials — so the manager's
// pre-dial sync step cannot do anything here. The real work runs post-dial in
// vpn.EnforceSplitTunnel → ApplySplitTunnel. Returning nil keeps the connect
// flow quiet (no misleading "route sync skipped" warning).
func SyncRoutes(connectionName string, desired []string) error { return nil }

// ApplySplitTunnel adds the desired split-tunnel prefixes to the just-connected
// macOS VPN tunnel interface. It only ADDS routes — it never touches the default
// route.
//
// Tunnel scope (full vs split) is owned by macOS, via the service's "Send all
// traffic over VPN connection" option (OverridePrimary):
//   - OFF  → macOS keeps the LAN as primary; only these added prefixes use the
//     tunnel. True split tunnel; general internet + DNS keep working.
//   - ON   → macOS routes everything through the VPN (full tunnel). Internet
//     still works; the added prefixes are redundant.
//
// Earlier this function deleted the VPN default route to force split tunnel, but
// that killed general internet (the LAN default route and DNS were not restored).
// Deferring the scope decision to macOS is the correct, robust design.
//
// Route edits need root, so they run in ONE osascript admin prompt. Returns a
// human-readable summary for the activity log.
//
// ponytail: add-only, no reconciliation. macOS drops tunnel routes on disconnect,
// so stale routes aren't a concern in the dial-existing-service model.
func ApplySplitTunnel(desired []string) (summary string, err error) {
	if len(desired) == 0 {
		return "", nil
	}
	iface, err := waitTunnelInterface(5 * time.Second)
	if err != nil {
		return "", fmt.Errorf("tunnel interface not ready for split-tunnel routes: %w", err)
	}

	var b strings.Builder
	for _, prefix := range desired {
		// `|| true` so an already-present route isn't fatal.
		b.WriteString(fmt.Sprintf("/sbin/route -n add -net %s -interface %s || true; ", toNet(prefix), iface))
	}
	if err := runAsAdmin(b.String()); err != nil {
		return "", fmt.Errorf("split-tunnel route setup failed (admin denied or error): %w", err)
	}
	return fmt.Sprintf("Split tunnel: %d route(s) added via %s. Tunnel scope (split vs full) is set by macOS in Network › VPN › Details › 'Send all traffic over VPN connection'.", len(desired), iface), nil
}

// runAsAdmin executes a shell command with administrator privileges via osascript,
// which shows the standard macOS authentication dialog once.
func runAsAdmin(script string) error {
	// Escape double quotes for embedding in the AppleScript string literal.
	esc := strings.ReplaceAll(script, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	osa := fmt.Sprintf(`do shell script "%s" with administrator privileges`, esc)
	out, err := exec.Command("osascript", "-e", osa).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
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

// waitTunnelInterface polls for the tunnel interface, which may lag a moment
// after the dial call returns.
func waitTunnelInterface(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		if iface, err := tunnelInterface(); err == nil {
			return iface, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no active tunnel interface")
		}
		time.Sleep(300 * time.Millisecond)
	}
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
