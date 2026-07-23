//go:build !windows

package vpn

import "fmt"

func unsupported() error {
	return newUserError("platform", "Gagal", "Fitur VPN hanya didukung di Windows.")
}

// ListProfiles is not supported outside Windows.
func ListProfiles() ([]ProfileSummary, error) { return nil, unsupported() }

// EnsureNATRegistry is a no-op outside Windows (NAT-T is Windows-specific).
func EnsureNATRegistry() (NATResult, error) { return NATOK, nil }

// Connect is not supported outside Windows.
func Connect(p ConnectParams) error { return unsupported() }

// Disconnect is not supported outside Windows.
func Disconnect(name string) error { return unsupported() }

// DisconnectAllExcept is not supported outside Windows.
func DisconnectAllExcept(exceptName string) ([]string, error) {
	return nil, unsupported()
}

// QueryStatus is not supported outside Windows.
func QueryStatus(name string) (ConnStatus, error) {
	return StatusUnknown, unsupported()
}

// ProfileExists is not supported outside Windows.
func ProfileExists(name string) (bool, error) {
	return false, unsupported()
}

// EnforceSplitTunnel is not supported outside Windows.
func EnforceSplitTunnel(name string) (string, error) { return "", unsupported() }

// ProfileDiagnostics is not supported outside Windows.
func ProfileDiagnostics(name string) (string, error) { return "", unsupported() }

// EnsureSplitTunneling is not supported outside Windows.
func EnsureSplitTunneling(name string) error { return unsupported() }

// PurgeOrphanScripts is a no-op outside Windows.
func PurgeOrphanScripts() {}

// TrafficCounters is not supported outside Windows.
func TrafficCounters(name string) (uint64, uint64, error) { return 0, 0, unsupported() }

// ActiveConnections is not supported outside Windows.
func ActiveConnections(name string) ([]ActiveConn, error) { return nil, unsupported() }

// PingHost is not supported outside Windows.
func PingHost(host string, timeoutMs uint32) (uint32, error) { return 0, unsupported() }

// Silence unused import on some toolchains.
var _ = fmt.Errorf
