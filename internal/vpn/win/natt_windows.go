//go:build windows

package win

import (
	"errors"
	"log"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"vepeen/internal/vpn/shared"
)

// EnsureNATRegistry ensures the L2TP/IPsec NAT-T registry value is set so the
// connection can succeed behind a NAT/router. It never panics and never crashes
// on a non-admin host; it returns NATElevationRequired with an actionable error
// instead. The value may require a reboot to take effect (surfaced as NATSet).
func EnsureNATRegistry() (shared.NATResult, error) {
	// 1. Read current value. If already set to the target, nothing to do.
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, shared.NatPolicyAgentKey, registry.READ)
	if err == nil {
		defer k.Close()
		if v, _, gerr := k.GetIntegerValue(shared.NatValueName); gerr == nil && shared.NatValueOK(uint32(v)) {
			return shared.NATOK, nil
		}
	}

	// 2. Try to open for write. Access denied => elevation required.
	wk, werr := registry.OpenKey(registry.LOCAL_MACHINE, shared.NatPolicyAgentKey, registry.SET_VALUE)
	if werr != nil {
		if errors.Is(werr, windows.ERROR_ACCESS_DENIED) {
			return shared.NATElevationRequired, shared.NewUserError("nat",
				"Connection failed (NAT-T)",
				"Windows is blocking L2TP/IPsec behind a NAT. Set registry HKLM\\SYSTEM\\CurrentControlSet\\Services\\PolicyAgent\\AssumeUDPEncapsulationContextOnSendRule = 2 (run as administrator), then try again. Vepeen attempts to set this automatically but requires administrator privileges.")
		}
		return shared.NATElevationRequired, shared.NewUserError("nat",
			"Connection failed (NAT-T)",
			"Windows is blocking L2TP/IPsec behind a NAT. Set registry HKLM\\SYSTEM\\CurrentControlSet\\Services\\PolicyAgent\\AssumeUDPEncapsulationContextOnSendRule = 2 (run as administrator), then try again. Vepeen attempts to set this automatically but requires administrator privileges.")
	}
	defer wk.Close()

	// 3. Set value to 2.
	if serr := wk.SetDWordValue(shared.NatValueName, shared.NatValueTarget); serr != nil {
		log.Printf("EnsureNATRegistry: failed to write registry value: %v", serr)
		return shared.NATElevationRequired, shared.NewUserError("nat",
			"Connection failed (NAT-T)",
			"Windows is blocking L2TP/IPsec behind a NAT. Set registry HKLM\\SYSTEM\\CurrentControlSet\\Services\\PolicyAgent\\AssumeUDPEncapsulationContextOnSendRule = 2 (run as administrator), then try again. Vepeen attempts to set this automatically but requires administrator privileges.")
	}
	return shared.NATSet, nil
}
