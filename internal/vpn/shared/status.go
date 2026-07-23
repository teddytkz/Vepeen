package shared

// ConnStatus is the high-level connection lifecycle state for UI and domain.
type ConnStatus string

const (
	StatusDisconnected  ConnStatus = "Disconnected"
	StatusConnecting    ConnStatus = "Connecting"
	StatusConnected     ConnStatus = "Connected"
	StatusDisconnecting ConnStatus = "Disconnecting"
	StatusError         ConnStatus = "Error"
	StatusUnknown       ConnStatus = "Unknown"
)

// ProfileSummary is a platform-neutral view of an existing Windows VPN
// connection, as enumerated by ListProfiles. It carries no secrets.
type ProfileSummary struct {
	Name           string     `json:"name"`
	ServerAddress  string     `json:"serverAddress"`
	TunnelType     string     `json:"tunnelType"`
	SplitTunneling bool       `json:"splitTunneling"`
	Status         ConnStatus `json:"status"`
}

// ConnectParams are PPP credentials for rasdial. Username and Password are
// optional: when both are empty/whitespace the OS-saved credentials are used.
type ConnectParams struct {
	Name     string
	Username string
	Password string
}

// ActiveConn describes one established TCP connection routed through the VPN.
type ActiveConn struct {
	RemoteAddr string // remote IP
	RemotePort string // remote port
	Hostname   string // reverse-DNS hostname if resolved, else ""
}
