//go:build darwin

// macOS VPN engine: drives a VPN service the user has already created in System
// Settings (Network) via `scutil --nc`. Unlike Windows, Vepeen does not create
// or configure the L2TP/PSK/MS-CHAPv2 service itself — macOS owns that. We only
// start/stop/query it and add split-tunnel routes after connect.
package vpn

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"vepeen/internal/vpn/shared"
)

// scutil --nc list line, e.g.:
// * (Disconnected)   <UUID> PPP --> L2TP  "VPN O"  [PPP:L2TP]
var ncListLine = regexp.MustCompile(`^\s*\*?\s*\(([^)]+)\)\s+(\S+)\s+.*?"([^"]+)"`)

// ListProfiles enumerates VPN services from `scutil --nc list`.
func ListProfiles() ([]shared.ProfileSummary, error) {
	out, err := exec.Command("scutil", "--nc", "list").Output()
	if err != nil {
		return nil, shared.NewUserError("platform", "Failed", "Could not list VPN services: "+shared.SanitizeOutput(err.Error()))
	}
	var res []shared.ProfileSummary
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		m := ncListLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		res = append(res, shared.ProfileSummary{
			Name:           m[3],
			TunnelType:     "L2TP",
			SplitTunneling: true,
			Status:         parseStatusWord(m[1]),
		})
	}
	return res, nil
}

// EnsureNATRegistry is a no-op on macOS; the OS handles NAT-T.
func EnsureNATRegistry() (shared.NATResult, error) { return shared.NATOK, nil }

// EnsureSplitTunneling is a no-op; split tunnel is applied as routes post-connect.
func EnsureSplitTunneling(name string) error { return nil }

// Connect starts the named VPN service. Credentials are optional; when empty the
// service's saved credentials are used.
func Connect(p shared.ConnectParams) error {
	args := []string{"--nc", "start", p.Name}
	if strings.TrimSpace(p.Username) != "" {
		args = append(args, "--user", p.Username)
	}
	if strings.TrimSpace(p.Password) != "" {
		args = append(args, "--password", p.Password)
	}
	if out, err := exec.Command("scutil", args...).CombinedOutput(); err != nil {
		return shared.MapExecError("connect", err, string(out))
	}
	// scutil start is async; wait briefly for the service to reach Connected.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := QueryStatus(p.Name)
		switch st {
		case shared.StatusConnected:
			return nil
		case shared.StatusDisconnected, shared.StatusError:
			return shared.NewUserError("dial", "Connection failed", "The VPN service did not connect. Check credentials, PSK, and server address in System Settings › Network.")
		}
		time.Sleep(500 * time.Millisecond)
	}
	return shared.NewUserError("dial", "Connection timed out", "The VPN did not connect within 30 seconds.")
}

// Disconnect stops the named VPN service.
func Disconnect(name string) error {
	if out, err := exec.Command("scutil", "--nc", "stop", name).CombinedOutput(); err != nil {
		return shared.MapExecError("disconnect", err, string(out))
	}
	return nil
}

// DisconnectAllExcept stops every connected VPN service except exceptName.
func DisconnectAllExcept(exceptName string) ([]string, error) {
	profs, err := ListProfiles()
	if err != nil {
		return nil, err
	}
	var stopped []string
	for _, pr := range profs {
		if pr.Name == exceptName || pr.Status != shared.StatusConnected {
			continue
		}
		if Disconnect(pr.Name) == nil {
			stopped = append(stopped, pr.Name)
		}
	}
	return stopped, nil
}

// QueryStatus returns the lifecycle state from `scutil --nc status`.
func QueryStatus(name string) (shared.ConnStatus, error) {
	out, err := exec.Command("scutil", "--nc", "status", name).Output()
	if err != nil {
		return shared.StatusUnknown, shared.MapExecError("status", err, "")
	}
	// First line is the status word (Connected/Disconnected/Connecting/...).
	first := strings.TrimSpace(string(out))
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	return parseStatusWord(first), nil
}

// ProfileExists reports whether a service with this name is configured.
func ProfileExists(name string) (bool, error) {
	profs, err := ListProfiles()
	if err != nil {
		return false, err
	}
	for _, p := range profs {
		if p.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// EnforceSplitTunnel is handled by route.SyncRoutes on macOS; nothing to do here.
func EnforceSplitTunnel(name string) (string, error) { return "", nil }

// ProfileDiagnostics returns the raw scutil status for troubleshooting.
func ProfileDiagnostics(name string) (string, error) {
	out, err := exec.Command("scutil", "--nc", "status", name).Output()
	if err != nil {
		return "", shared.MapExecError("diagnostics", err, "")
	}
	return shared.SanitizeOutput(string(out)), nil
}

// PurgeOrphanScripts is a no-op on macOS (no helper scripts written).
func PurgeOrphanScripts() {}

// TrafficCounters is not tracked on macOS in v1.
func TrafficCounters(name string) (uint64, uint64, error) {
	return 0, 0, shared.NewUserError("platform", "Unavailable", "Traffic counters are not available on macOS.")
}

// ActiveConnections is not enumerated on macOS in v1.
func ActiveConnections(name string) ([]shared.ActiveConn, error) { return nil, nil }

// PingHost pings host with a total timeout and returns round-trip time in ms.
func PingHost(host string, timeoutMs uint32) (uint32, error) {
	secs := (timeoutMs + 999) / 1000
	if secs == 0 {
		secs = 1
	}
	out, err := exec.Command("ping", "-c", "1", "-t", strconv.Itoa(int(secs)), host).Output()
	if err != nil {
		return 0, fmt.Errorf("ping failed")
	}
	return parsePingMs(string(out))
}

// InterfaceInfo returns the tunnel interface index and its assigned IPv4 nets.
// macOS L2TP surfaces as a ppp* interface; index is not meaningful here so we
// return 0 and let callers use the addresses.
func InterfaceInfo(name string) (uint32, []net.IPNet, error) {
	iface, err := tunnelInterface()
	if err != nil {
		return 0, nil, err
	}
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return 0, nil, err
	}
	addrs, _ := ifi.Addrs()
	var nets []net.IPNet
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
			nets = append(nets, *ipn)
		}
	}
	return uint32(ifi.Index), nets, nil
}

func parseStatusWord(s string) shared.ConnStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "connected":
		return shared.StatusConnected
	case "connecting":
		return shared.StatusConnecting
	case "disconnecting":
		return shared.StatusDisconnecting
	case "disconnected":
		return shared.StatusDisconnected
	default:
		return shared.StatusUnknown
	}
}

var pingRTT = regexp.MustCompile(`time[=<]([0-9.]+)\s*ms`)

func parsePingMs(out string) (uint32, error) {
	m := pingRTT.FindStringSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("no rtt in ping output")
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	return uint32(f + 0.5), nil
}

// tunnelInterface returns the first ppp/utun interface that has an IPv4 address,
// which is the active VPN tunnel on macOS.
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
