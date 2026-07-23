package vpn

// NATResult describes the outcome of EnsureNATRegistry.
type NATResult int

const (
	// NATOK means the NAT-T registry value is already set (==2); no action needed.
	NATOK NATResult = iota
	// NATSet means the value was missing/incorrect and has been set to 2.
	NATSet
	// NATElevationRequired means writing HKLM requires administrator rights.
	NATElevationRequired
)

const (
	natPolicyAgentKey = `SYSTEM\CurrentControlSet\Services\PolicyAgent`
	natValueName      = "AssumeUDPEncapsulationContextOnSendRule"
	natValueTarget    = uint32(2)
)

// natValueOK reports whether the registry value enables UDP encapsulation.
func natValueOK(v uint32) bool {
	return v == natValueTarget
}
