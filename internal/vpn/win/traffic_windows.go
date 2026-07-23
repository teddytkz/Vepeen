//go:build windows

package win

import (
	"vepeen/internal/vpn/shared"
)

// TrafficCounters returns cumulative received and transmitted bytes for the
// named VPN connection by reading its network adapter statistics via the native
// iphlpapi GetIfEntry2 call. When the connection is not connected (or the
// interface cannot be resolved), it returns (0, 0, nil) without error. No
// secrets are embedded or logged.
func TrafficCounters(name string) (rxBytes, txBytes uint64, err error) {
	if err := shared.ValidateName(name); err != nil {
		return 0, 0, err
	}

	ifIndex, _, err := resolveVPNInterface(name)
	if err != nil {
		// Best-effort monitoring: degrade gracefully rather than crash.
		return 0, 0, nil
	}
	if ifIndex == 0 {
		return 0, 0, nil
	}

	rxBytes, txBytes, err = getIfEntry2(ifIndex)
	if err != nil {
		return 0, 0, nil
	}
	return rxBytes, txBytes, nil
}
