package vpn

import (
	"context"
	"log"
	"strings"
	"time"

	"vepeen/internal/route"
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
}

// NewManager returns a VPN manager.
func NewManager() *Manager {
	return &Manager{
		syncRoutesFn:           route.SyncRoutes,
		connectFn:              Connect,
		natCheckFn:             EnsureNATRegistry,
		ensureSplitTunnelingFn: EnsureSplitTunneling,
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
		log.Printf("ConnectFull: NAT-T memerlukan hak administrator: %s", sanitizeDetail(natErr.Error()))
		warnings = append(warnings, "NAT-T memerlukan hak administrator (AssumeUDPEncapsulationContextOnSendRule=2). Hubungkan mungkin gagal di belakang NAT; coba jalankan sebagai administrator.")
	}
	if natRes == NATSet {
		log.Printf("ConnectFull: NAT-T diatur (AssumeUDPEncapsulationContextOnSendRule=2). Mungkin perlu restart agar berlaku.")
		warnings = append(warnings, "NAT-T diatur (AssumeUDPEncapsulationContextOnSendRule=2). Mungkin perlu restart agar berlaku.")
	}

	prefixes := req.Routes
	if len(prefixes) == 0 {
		var err error
		prefixes, err = route.ParseLines(req.RoutesText)
		if err != nil {
			if pe, ok := err.(*route.ParseError); ok {
				return nil, newUserError("validation", "Tidak dapat menghubungkan", pe.Error())
			}
			return nil, newUserError("validation", "Tidak dapat menghubungkan", err.Error())
		}
	}
	// Windows VPN routes require IP prefixes, so resolve any domain entries
	// to their IPv4 addresses before syncing.
	resolveCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resolved, rerr := route.ResolveRoutes(resolveCtx, prefixes)
	if rerr != nil {
		return nil, newUserError("validation", "Tidak dapat menghubungkan", rerr.Error())
	}
	prefixes = resolved
	if len(prefixes) == 0 {
		return nil, newUserError("validation", "Tidak dapat menghubungkan", "Isi minimal satu IP, CIDR, atau nama domain tujuan untuk split tunnel (mis. 10.0.0.0/24 atau example.com).")
	}

	notify(PhaseDisconnectOthers)
	if _, derr := DisconnectAllExcept(name); derr != nil {
		// best-effort; ignore so connect can proceed
		_ = derr
	}

	notify(PhaseSplitTunnelEnsure)
	if err := m.ensureSplitTunnelingFn(name); err != nil {
		log.Printf("ConnectFull: gagal mengaktifkan split tunnel: %s Koneksi dilanjutkan; rute mungkin tidak diterapkan.", sanitizeDetail(err.Error()))
		warnings = append(warnings, "Gagal mengaktifkan split tunnel: "+sanitizeDetail(err.Error())+". Koneksi dilanjutkan; rute split tunnel mungkin tidak diterapkan.")
	}

	notify(PhaseSyncRoutes)
	if err := m.syncRoutesFn(name, prefixes); err != nil {
		// Best-effort: do not abort connect for a transient route-sync error.
		log.Printf("ConnectFull: penyelarasan rute dilewati: %s Koneksi dilanjutkan; rute split tunnel mungkin perlu disimpan ulang.", sanitizeDetail(err.Error()))
		warnings = append(warnings, "Penyelarasan rute dilewati: "+sanitizeDetail(err.Error())+". Koneksi dilanjutkan; rute split tunnel mungkin perlu disimpan ulang.")
	}
	if ctx.Err() != nil {
		return nil, newUserError("canceled", "Dibatalkan", "Penghubungan dibatalkan.")
	}

	notify(PhaseDial)
	if err := m.connectFn(ConnectParams{Name: name, Username: req.Username, Password: req.Password}); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, newUserError("canceled", "Dibatalkan", "Penghubungan dibatalkan.")
	}

	notify(PhaseSplitEnforce)
	if _, eerr := EnforceSplitTunnel(name); eerr != nil {
		// best-effort; do not fail connect
	}
	if ctx.Err() != nil {
		return nil, newUserError("canceled", "Dibatalkan", "Penghubungan dibatalkan.")
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

func sanitizeDetail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	// Never invent secrets; just strip common secret flags if present.
	lower := strings.ToLower(s)
	if strings.Contains(lower, "l2tppsk") || strings.Contains(lower, "password") {
		return "Detail teknis disembunyikan demi keamanan."
	}
	return s
}

// PhaseDetail returns Indonesian status detail for a connect phase.
func PhaseDetail(p Phase) string {
	switch p {
	case PhaseSplitTunnelEnsure:
		return "Mengaktifkan split tunnel…"
	case PhaseSyncRoutes:
		return "Menyelaraskan rute (split tunnel)…"
	case PhaseDial:
		return "Menghubungi server…"
	case PhaseSplitEnforce:
		return "Menegakkan split tunnel…"
	case PhaseDisconnectOthers:
		return "Memutuskan VPN lain…"
	case PhaseDone:
		return "Hanya IP/CIDR pada daftar yang melewati VPN."
	default:
		return ""
	}
}
