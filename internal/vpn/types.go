package vpn

import "vepeen/internal/vpn/shared"

// Type aliases preserve the public API of package vpn while the underlying
// definitions live in the platform-agnostic shared package.
type (
	ConnStatus     = shared.ConnStatus
	ProfileSummary = shared.ProfileSummary
	ConnectParams  = shared.ConnectParams
	ActiveConn     = shared.ActiveConn
	UserError      = shared.UserError
	NATResult      = shared.NATResult
)

// Status constants are re-exported so callers can keep using vpn.StatusConnected etc.
const (
	StatusDisconnected  = shared.StatusDisconnected
	StatusConnecting    = shared.StatusConnecting
	StatusConnected     = shared.StatusConnected
	StatusDisconnecting = shared.StatusDisconnecting
	StatusError         = shared.StatusError
	StatusUnknown       = shared.StatusUnknown
)

// NATResult constants are re-exported for the same reason.
const (
	NATOK                = shared.NATOK
	NATSet               = shared.NATSet
	NATElevationRequired = shared.NATElevationRequired
)

// Function/constructor re-exports keep the public API stable.
var (
	MapExecError   = shared.MapExecError
	ValidateName   = shared.ValidateName
	NewUserError   = shared.NewUserError
	AsUserError    = shared.AsUserError
	SanitizeOutput = shared.SanitizeOutput
	// NatValueOK is re-exported for tests that assert on the NAT registry value.
	NatValueOK = shared.NatValueOK
)
