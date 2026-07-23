package shared

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
	NatPolicyAgentKey = `SYSTEM\CurrentControlSet\Services\PolicyAgent`
	NatValueName      = "AssumeUDPEncapsulationContextOnSendRule"
	NatValueTarget    = uint32(2)
)

// NatValueOK reports whether the registry value enables UDP encapsulation.
func NatValueOK(v uint32) bool {
	return v == NatValueTarget
}
