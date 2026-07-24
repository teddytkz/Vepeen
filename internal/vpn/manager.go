package vpn

import (
	"context"
	"log"
	"strings"
	"time"

	"vepeen/internal/route"
	"vepeen/internal/vpn/shared"
)

// Phase is a connect-progress step for UI status detail.
type Phase string

const (
	PhaseNATCheck          Phase = "nat_check"
	PhaseSyncRoutes        Phase = "sync_routes"
	PhaseDial              Phase = "dial"
	PhaseSplitEnforce      Phase = "split_enforce"
	PhaseDisconnectOthers  Phase = "disconnect_others"
	PhaseSplitTunnelEnsure Phase = "split_tunnel_ensure"
	PhaseDone              Phase = "done"
)

// ProgressFunc is called with phase updates (may run off the UI thread).
type ProgressFunc func(phase Phase)

// ConnectRequest is the full connect input from the UI layer.
type ConnectRequest struct {
	Name string
	// Username is OPTIONAL pass-through to rasdial. Never persisted to config.
	Username string
	// Password is OPTIONAL pass-through to rasdial. Never persisted to config.
	Password string
	// RoutesText is multi-line IP/CIDR list from the form.
	RoutesText string
	// Routes optional pre-parsed prefixes; if empty, RoutesText is parsed.
	Routes []string
	// RouteAllTraffic, when true, skips split-tunnel setup and routes all
	// traffic through the VPN (an empty routes list is valid in this mode).
	RouteAllTraffic bool
}

// Manager orchestrates route sync and dial for an existing Windows VPN profile.
type Manager struct {
	// syncRoutesFn syncs profile routes; overridable for tests.
	syncRoutesFn func(string, []string) error
	// connectFn dials the VPN; overridable for tests.
	connectFn func(ConnectParams) error
	// natCheckFn ensures the NAT-T registry; overridable for tests.
	natCheckFn func() (NATResult, error)
	// ensureSplitTunnelingFn enables split tunneling on the profile; overridable for tests.
	ensureSplitTunnelingFn func(string) error
	// disableSplitTunnelingFn disables split tunneling on the profile; overridable for tests.
	disableSplitTunnelingFn func(string) error
}

// NewManager returns a VPN manager.
func NewManager() *Manager {
	return &Manager{
		syncRoutesFn:           route.SyncRoutes,
		connectFn:              Connect,
		natCheckFn:             EnsureNATRegistry,
		ensureSplitTunnelingFn: EnsureSplitTunneling,
		disableSplitTunnelingFn: DisableSplitTunneling,
	}
}

// ConnectFull validates, ensures profile, syncs routes, then rasdial.
// It returns non-fatal warnings (e.g. NAT-T set, route sync skipped) alongside
// any fatal error.
func (m *Manager) ConnectFull(ctx context.Context, req ConnectRequest, progress ProgressFunc) ([]string, error) {
	var warnings []string
	name := strings.TrimSpace(req.Name)
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	// NAT-T registry check (non-fatal: on NATElevationRequired append a warning
	// and continue; the connection may still succeed behind a NAT).
	notify := func(p Phase) {
		if progress != nil {
			progress(p)
		}
	}
	notify(PhaseNATCheck)
	natRes, natErr := m.natCheckFn()
	if natRes == NATElevationRequired {
		log.Printf("ConnectFull: NAT-T requires administrator privileges: %s", shared.SanitizeOutput(natErr.Error()))
		warnings = append(warnings, "NAT-T requires administrator privileges (AssumeUDPEncapsulationContextOnSendRule=2). Connection may fail behind a NAT; try running as administrator.")
	}
	if natRes == NATSet {
		log.Printf("ConnectFull: NAT-T set (AssumeUDPEncapsulationContextOnSendRule=2). A restart may be required for the change to take effect.")
		warnings = append(warnings, "NAT-T set (AssumeUDPEncapsulationContextOnSendRule=2). A restart may be required for the change to take effect.")
	}

	prefixes := req.Routes
	if len(prefixes) == 0 {
		var err error
		prefixes, err = route.ParseLines(req.RoutesText)
		if err != nil {
			if pe, ok := err.(*route.ParseError); ok {
				return nil, shared.NewUserError("validation", "Cannot connect", pe.Error())
			}
			return nil, shared.NewUserError("validation", "Cannot connect", err.Error())
		}
	}
	// Windows VPN routes require IP prefixes, so resolve any domain entries
	// to their IPv4 addresses before syncing.
	resolveCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resolved, rerr := route.ResolveRoutes(resolveCtx, prefixes)
	if rerr != nil {
		return nil, shared.NewUserError("validation", "Cannot connect", rerr.Error())
	}
	prefixes = resolved
	if len(prefixes) == 0 && !req.RouteAllTraffic {
		return nil, shared.NewUserError("validation", "Cannot connect", "Enter at least one destination IP, CIDR, or domain name for split tunnel (e.g. 10.0.0.0/24 or example.com).")
	}

	notify(PhaseDisconnectOthers)
	if _, derr := DisconnectAllExcept(name); derr != nil {
		// best-effort; ignore so connect can proceed
		_ = derr
	}
	if req.RouteAllTraffic {
		notify(PhaseSplitTunnelEnsure)
		if err := m.disableSplitTunnelingFn(name); err != nil {
			log.Printf("ConnectFull: failed to disable split tunnel: %s Connection will continue; all-traffic routing may not apply.", shared.SanitizeOutput(err.Error()))
			warnings = append(warnings, "Failed to disable split tunnel: "+shared.SanitizeOutput(err.Error())+". All-traffic routing may not apply.")
		}
	} else
	if !req.RouteAllTraffic {
		notify(PhaseSplitTunnelEnsure)
		if err := m.ensureSplitTunnelingFn(name); err != nil {
			log.Printf("ConnectFull: failed to enable split tunnel: %s Connection will continue; routes may not be applied.", shared.SanitizeOutput(err.Error()))
			warnings = append(warnings, "Failed to enable split tunnel: "+shared.SanitizeOutput(err.Error())+". Connection will continue; split tunnel routes may not be applied.")
		}

		notify(PhaseSyncRoutes)
		if err := m.syncRoutesFn(name, prefixes); err != nil {
			// Best-effort: do not abort connect for a transient route-sync error.
			log.Printf("ConnectFull: route sync skipped: %s Connection will continue; split tunnel routes may need to be re-saved.", shared.SanitizeOutput(err.Error()))
			warnings = append(warnings, "Route sync skipped: "+shared.SanitizeOutput(err.Error())+". Connection will continue; split tunnel routes may need to be re-saved.")
		}
		if ctx.Err() != nil {
			return nil, shared.NewUserError("canceled", "Cancelled", "Connection cancelled.")
		}
	}

	notify(PhaseDial)
	if err := m.connectFn(ConnectParams{Name: name, Username: req.Username, Password: req.Password}); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, shared.NewUserError("canceled", "Cancelled", "Connection cancelled.")
	}

	if !req.RouteAllTraffic {
		notify(PhaseSplitEnforce)
		if msg, eerr := EnforceSplitTunnel(name, prefixes); eerr != nil {
			// best-effort; do not fail connect, but surface why split tunnel may be off
			log.Printf("ConnectFull: split tunnel enforce: %s", shared.SanitizeOutput(eerr.Error()))
			warnings = append(warnings, "Split tunnel not fully applied: "+shared.SanitizeOutput(eerr.Error()))
		} else if msg != "" {
			warnings = append(warnings, msg)
		}
		if ctx.Err() != nil {
			return nil, shared.NewUserError("canceled", "Cancelled", "Connection cancelled.")
		}
	}

	notify(PhaseDone)
	return warnings, nil
}

// DisconnectFull disconnects the named connection.
func (m *Manager) DisconnectFull(name string) error {
	return Disconnect(name)
}

// Status returns OS status for the connection name.
func (m *Manager) Status(name string) (ConnStatus, error) {
	return QueryStatus(name)
}

// sanitizeDetail redacts secret-bearing content and caps detail length.
func sanitizeDetail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	// Never invent secrets; just strip common secret flags if present.
	lower := strings.ToLower(s)
	if strings.Contains(lower, "l2tppsk") || strings.Contains(lower, "password") {
		return "Technical details hidden for security."
	}
	return s
}

// PhaseDetail returns a status detail string for a connect phase.
func PhaseDetail(p Phase) string {
	switch p {
	case PhaseSplitTunnelEnsure:
		return "Enabling split tunnel…"
	case PhaseSyncRoutes:
		return "Syncing routes (split tunnel)…"
	case PhaseDial:
		return "Connecting to server…"
	case PhaseSplitEnforce:
		return "Enforcing split tunnel…"
	case PhaseDisconnectOthers:
		return "Disconnecting other VPNs…"
	case PhaseDone:
		return "Only the listed IPs/CIDRs route through the VPN."
	default:
		return ""
	}
}
