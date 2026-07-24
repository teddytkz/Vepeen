//go:build windows

package vpn

import (
	"net"

	"vepeen/internal/vpn/shared"
	"vepeen/internal/vpn/win"
)

// This file is the ONLY vpn file that imports the windows-only win package.
// It re-exports the public Windows VPN functions so the rest of the codebase
// (internal/ui, cmd/vepeen) keeps using vpn.* unchanged.

// ActiveConnections returns established TCP connections through the VPN.
func ActiveConnections(name string) ([]shared.ActiveConn, error) {
	return win.ActiveConnections(name)
}

// Connect dials the VPN via rasdial.
func Connect(p shared.ConnectParams) error {
	return win.Connect(p)
}

// Disconnect hangs up the VPN via rasdial /DISCONNECT.
func Disconnect(name string) error {
	return win.Disconnect(name)
}

// DisconnectAllExcept disconnects every connected Windows VPN except exceptName.
func DisconnectAllExcept(exceptName string) ([]string, error) {
	return win.DisconnectAllExcept(exceptName)
}

// EnsureNATRegistry ensures the L2TP/IPsec NAT-T registry value is set.
func EnsureNATRegistry() (shared.NATResult, error) {
	return win.EnsureNATRegistry()
}

// EnsureSplitTunneling enables split tunneling on the named profile.
func EnsureSplitTunneling(name string) error {
	return win.EnsureSplitTunneling(name)
}

// DisableSplitTunneling disables split tunneling on the named profile.
func DisableSplitTunneling(name string) error {
	return win.DisableSplitTunneling(name)
}

// EnforceSplitTunnel removes a server-pushed default route if present. On Windows
// routes are attached to the profile pre-dial, so prefixes are unused here.
func EnforceSplitTunnel(name string, prefixes []string) (string, error) {
	return win.EnforceSplitTunnel(name)
}

// ListProfiles enumerates all Windows VPN connections.
func ListProfiles() ([]shared.ProfileSummary, error) {
	return win.ListProfiles()
}

// PingHost sends a single ICMP echo and returns the round-trip time in ms.
func PingHost(host string, timeoutMs uint32) (uint32, error) {
	return win.PingHost(host, timeoutMs)
}

// ProfileDiagnostics returns a short status line for the profile.
func ProfileDiagnostics(name string) (string, error) {
	return win.ProfileDiagnostics(name)
}

// ProfileExists reports whether a per-user VPN connection exists.
func ProfileExists(name string) (bool, error) {
	return win.ProfileExists(name)
}

// QueryStatus returns the OS connection status for a named VPN profile.
func QueryStatus(name string) (shared.ConnStatus, error) {
	return win.QueryStatus(name)
}

// TrafficCounters returns cumulative received/transmitted bytes for the VPN.
func TrafficCounters(name string) (uint64, uint64, error) {
	return win.TrafficCounters(name)
}

// PurgeOrphanScripts deletes leftover vpn-*.ps1 scripts under %TEMP%\vepeen.
func PurgeOrphanScripts() {
	win.PurgeOrphanScripts()
}

// InterfaceInfo returns the local IPv4 address(es) and subnet mask of the
// connected VPN adapter. When not connected it returns (0, nil, nil).
func InterfaceInfo(name string) (uint32, []net.IPNet, error) {
	return win.InterfaceInfo(name)
}
