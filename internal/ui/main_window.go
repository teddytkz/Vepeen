package ui

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"vepeen/internal/config"
	"vepeen/internal/route"
	"vepeen/internal/vpn"
)

// maxLogLines caps in-memory activity log lines (drop oldest).
const maxLogLines = 300

// NewMainWindow builds the L2TP/IPsec split-tunnel main window.
// It returns the window and a disconnectAndQuit closure that disconnects the
// VPN (best-effort, 5 s timeout) then calls a.Quit().
func NewMainWindow(a fyne.App) (fyne.Window, func()) {
	w := a.NewWindow("Vepeen")
	w.Resize(fyne.NewSize(1040, 780)) // redesign brief's target size

	ctrl := newController()
	ctrl.win = w
	// Window has no SetMinSize in Fyne v2.8; enforce via content MinSize wrapper.
	w.SetContent(newMinSizeWrap(ctrl.build(), fyne.NewSize(1000, 720)))
	// Centering is now performed synchronously at show time by ui.ShowCentered
	// (called from main.go after NewMainWindow returns), which positions the
	// window at the work-area center before the first frame is composited. This
	// avoids the teleport blink from the old deferred goroutine.
	ctrl.loadInitial()

	// disconnectAndQuit shows a non-dismissible progress dialog, disconnects the
	// VPN (best-effort, 5 s timeout) off the UI thread, then quits. a.Quit() is
	// guaranteed to run exactly once, after the dialog is hidden.
	disconnectAndQuit := func() {
		if !ctrl.quitting.CompareAndSwap(false, true) {
			return // already quitting
		}
		name := ctrl.profileName()

		statusLabel := widget.NewLabel("Disconnecting VPN…")
		dlg := dialog.NewCustomWithoutButtons("Quitting Vepeen",
			container.NewVBox(statusLabel, widget.NewProgressBarInfinite()), w)
		w.Show() // ensure window visible even if quitting from tray
		dlg.Show()

		go func() {
			if name != "" {
				done := make(chan struct{})
				go func() {
					defer close(done)
					// Only attempt disconnect if connected (or unknown-but-name-present).
					st, _ := ctrl.mgr.Status(name)
					if st == vpn.StatusConnected || st == vpn.StatusUnknown {
						fyne.Do(func() { statusLabel.SetText("Disconnecting " + name + "…") })
						log.Printf("quit: disconnecting %s…", name)
						if err := ctrl.mgr.DisconnectFull(name); err != nil {
							log.Printf("quit: disconnect result: %v", err)
						} else {
							log.Printf("quit: disconnect result: ok")
						}
						// Verify; retry once if still connected.
						if st2, _ := ctrl.mgr.Status(name); st2 == vpn.StatusConnected {
							fyne.Do(func() { statusLabel.SetText("Still connected, retrying…") })
							log.Printf("quit: still connected, retrying disconnect %s…", name)
							if err := ctrl.mgr.DisconnectFull(name); err != nil {
								log.Printf("quit: retry disconnect result: %v", err)
							} else {
								log.Printf("quit: retry disconnect result: ok")
							}
						}
					}
				}()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					log.Printf("quit: disconnect timed out after 5s, proceeding to quit")
				}
			}
			fyne.Do(func() { statusLabel.SetText("Closing…") })
			fyne.Do(func() {
				dlg.Hide()
				a.Quit()
			})
		}()
	}

	// Run on Startup is a checkable item. State lives in the HKCU Run key
	// (single source of truth), so read it at build time and toggle it on click.
	// Fyne caches whether a menu contains checkmarks when the menu is built
	// (containsCheck in widget/menu.go), so the checkmark would overlap the
	// label after toggling. Rebuilding the menu re-runs that layout pass; the
	// menu is already dismissed when the action fires, so the rebuild is not
	// visible to the user.
	runOnStartup := fyne.NewMenuItem("Run on Startup", nil)
	optionsItem := fyne.NewMenuItem("Options", nil)
	rebuildMenu := func() {
		runOnStartup.Checked = IsRunOnStartup()
		optionsItem.ChildMenu = fyne.NewMenu("Options",
			runOnStartup,
			fyne.NewMenuItem("Create Desktop Shortcut", func() {
				if err := CreateDesktopShortcut(); err != nil {
					ctrl.appendLog("Failed to create desktop shortcut: " + err.Error())
				} else {
					ctrl.appendLog("Desktop shortcut created.")
				}
			}),
		)
		w.SetMainMenu(fyne.NewMainMenu(
			fyne.NewMenu("Menu",
				optionsItem,
				fyne.NewMenuItem("Quit", func() { disconnectAndQuit() }),
			),
		))
	}
	runOnStartup.Action = func() {
		enabled := !IsRunOnStartup()
		if err := SetRunOnStartup(enabled); err != nil {
			ctrl.appendLog("Failed to update run on startup: " + err.Error())
			return
		}
		runOnStartup.Checked = enabled
		ctrl.appendLog("Run on startup " + map[bool]string{true: "enabled", false: "disabled"}[enabled] + ".")
		rebuildMenu()
	}
	rebuildMenu()

	return w, disconnectAndQuit
}

type controller struct {
	win   fyne.Window
	mgr   *vpn.Manager
	mu    sync.Mutex
	busy  bool
	state vpn.ConnStatus

	// cancelConnect cancels an in-progress ConnectFull (nil when not connecting).
	cancelConnect context.CancelFunc

	// connectionName is the Windows VPN profile name (hidden in UI; default Vepeen).
	connectionName string

	// cfg is the in-memory non-secret settings projection of stored.
	cfg config.Config

	// stored is the full persisted state (settings + per-profile credentials).
	stored config.Stored

	profileSelect *widget.Select
	routesEntry   *widget.Entry
	userEntry     *widget.Entry
	passEntry     *widget.Entry
	logView       *logView

	btnSave     *widget.Button
	btnConn     *widget.Button
	btnClearLog *widget.Button
	statusPri   *widget.Label
	statusDet   *widget.Label

	rememberCheck *tealCheck
	routeAllCheck *tealCheck
	hostArea      *widget.Entry
	dlLabel       *widget.Label
	tickBusy      atomic.Bool
	quitting      atomic.Bool
	ulLabel       *widget.Label
	profiles      map[string]vpn.ProfileSummary // name -> summary (for ServerAddress)
	trafficStop   chan struct{}                 // closed to stop the traffic ticker
	trafficName   string                        // active connection name for the ticker

	pingLabel *widget.Label // gateway ping status text
	pingStop  chan struct{} // closed to stop the ping ticker

	// Redesign widgets.
	hero       *heroRing      // focal connect/disconnect ring
	heroName   *canvas.Text   // profile name under the ring
	heroSub    *canvas.Text   // "L2TP/IPsec · host" sub line
	statDown   *canvas.Text   // Down tile value
	statUp     *canvas.Text   // Up tile value
	statPing   *canvas.Text   // Ping tile value
	routeCount *canvas.Text   // "N routes" counter
	footerDot  *canvas.Circle // footer state dot
	footerText *canvas.Text   // footer status text
}

func newController() *controller {
	return &controller{
		mgr:            vpn.NewManager(),
		state:          vpn.StatusDisconnected,
		connectionName: config.DefaultConnectionName,
		stored:         config.DefaultStored(),
		cfg:            config.Default(),
	}
}

func (c *controller) build() fyne.CanvasObject {
	// ---- Left column ----

	c.profileSelect = widget.NewSelect([]string{}, c.onProfileChanged)
	c.profileSelect.PlaceHolder = "Select VPN profile…"

	cardProfile := card(container.NewVBox(
		sectionLabel("CONNECTION PROFILE"),
		c.profileSelect,
	))

	c.userEntry = widget.NewEntry()
	c.userEntry.SetPlaceHolder("Username")
	c.passEntry = widget.NewPasswordEntry()
	c.passEntry.SetPlaceHolder("Password")
	c.rememberCheck = newTealCheck("Remember credentials", nil)
	c.rememberCheck.SetChecked(true)

	cardCreds := card(container.NewVBox(
		sectionLabel("CREDENTIALS"),
		c.userEntry,
		c.passEntry,
		c.rememberCheck,
		helperText("Leave blank to use credentials saved in Keychain."),
	))

	c.routesEntry = widget.NewMultiLineEntry()
	c.routesEntry.SetMinRowsVisible(6)
	c.routesEntry.Wrapping = fyne.TextWrapOff
	c.routesEntry.TextStyle = fyne.TextStyle{Monospace: true}
	c.routesEntry.SetPlaceHolder("10.10.0.0/16\n203.0.113.50\nmail.foofle.com")
	c.routesEntry.OnChanged = func(string) { c.updateRouteCount() }

	c.routeCount = mono("0 routes", 11, textFaint)
	routesHeader := container.NewBorder(nil, nil, sectionLabel("SPLIT TUNNEL ROUTES"), c.routeCount)

	c.routeAllCheck = newTealCheck("Route All Traffic", nil)

	cardRoutes := card(container.NewBorder(
		container.NewVBox(routesHeader, helperText("Only these destinations route through the VPN.")),
		c.routeAllCheck,
		nil, nil,
		c.routesEntry,
	))

	leftCol := container.NewBorder(
		container.NewVBox(cardProfile, cardCreds),
		nil, nil, nil,
		cardRoutes,
	)

	// ---- Right column: hero + log ----

	c.hero = newHeroRing(c.onHeroTap)

	c.heroName = canvas.NewText("No profile selected", textPrimary)
	c.heroName.TextSize = 16
	c.heroName.TextStyle = fyne.TextStyle{Bold: true}
	c.heroName.Alignment = fyne.TextAlignCenter
	c.heroSub = mono("choose a profile to begin", 12, monoFaint)
	c.heroSub.Alignment = fyne.TextAlignCenter

	c.statDown = mono("—", 15, textSecondary)
	c.statUp = mono("—", 15, textSecondary)
	c.statPing = mono("not connected", 13, textSecondary)
	stats := container.NewGridWithColumns(3,
		statTile("DOWN", c.statDown),
		statTile("UP", c.statUp),
		statTile("PING", c.statPing),
	)

	// keep legacy labels alive so existing tickers compile; not shown.
	c.dlLabel = widget.NewLabel("")
	c.ulLabel = widget.NewLabel("")
	c.pingLabel = widget.NewLabel("")
	c.hostArea = widget.NewMultiLineEntry()

	cardHero := cardPad(container.NewVBox(
		container.NewCenter(c.hero),
		container.NewCenter(c.heroName),
		container.NewCenter(c.heroSub),
		stats,
	), 26, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0a}, 20) // brighter fill, r20

	c.btnClearLog = widget.NewButton("Clear", c.onClearLog)
	c.btnClearLog.Importance = widget.LowImportance
	logHeader := container.NewBorder(nil, nil, sectionLabel("ACTIVITY LOG"), c.btnClearLog)

	c.logView = newLogView(maxLogLines)

	cardLog := cardPad(container.NewBorder(logHeader, nil, nil, nil, c.logView), 16, cardFill, 16)

	rightCol := container.NewBorder(cardHero, nil, nil, nil, cardLog)

	// legacy status labels retained (updated by state machine, surfaced in footer).
	c.statusPri = widget.NewLabel("Disconnected")
	c.statusDet = widget.NewLabel("")

	body := container.New(newRatioHBox(1, 1.08, theme.Padding()*2), leftCol, rightCol)

	// ---- Footer ----

	c.footerDot = &canvas.Circle{FillColor: ringIdle}
	c.footerText = canvas.NewText("Disconnected · settings loaded", textMuted)
	c.footerText.TextSize = 12.5
	// Status dot sized to the text and vertically centered with the label.
	footerLeft := container.NewHBox(
		container.NewCenter(container.NewGridWrap(fyne.NewSize(11, 11), c.footerDot)),
		c.footerText,
	)

	c.btnSave = widget.NewButton("Save Settings", c.onSave)
	c.btnConn = widget.NewButton("Connect", c.onHeroTap)
	c.btnConn.Importance = widget.HighImportance

	footer := container.NewPadded(container.NewBorder(nil, nil, footerLeft, nil,
		container.NewHBox(layout.NewSpacer(), c.btnSave, c.btnConn),
	))

	// Title strip (fix #5): app name left, protocol label right, in mono.
	appName := canvas.NewText("Vepeen", textPrimary)
	appName.TextStyle = fyne.TextStyle{Bold: true}
	appName.TextSize = 15
	proto := mono("L2TP/IPsec · split tunnel", 12, color.NRGBA{R: 0x56, G: 0x66, B: 0x6b, A: 0xff})
	titleStrip := container.NewPadded(container.NewBorder(nil, nil, appName, proto))

	content := container.NewBorder(titleStrip, footer, nil, nil, container.NewPadded(body))
	return container.NewStack(bgLayer(), content)
}

// updateRouteCount refreshes the "N routes" label: non-empty, non-# lines.
func (c *controller) updateRouteCount() {
	if c.routeCount == nil {
		return
	}
	n := 0
	for _, ln := range strings.Split(c.routesEntry.Text, "\n") {
		s := strings.TrimSpace(ln)
		if s != "" && !strings.HasPrefix(s, "#") {
			n++
		}
	}
	suffix := "routes"
	if n == 1 {
		suffix = "route"
	}
	c.routeCount.Text = fmt.Sprintf("%d %s", n, suffix)
	c.routeCount.Refresh()
}

// onHeroTap routes a ring tap to connect or disconnect based on state.
func (c *controller) onHeroTap() {
	switch c.state {
	case vpn.StatusConnected:
		c.onDisconnect()
	case vpn.StatusConnecting:
		c.onCancel()
	default:
		c.onConnect()
	}
}

// appendLog adds a timestamped, color-coded line to the activity log (UI thread
// only). Never pass PSK, password, or secret-bearing command lines.
func (c *controller) appendLog(msg string) {
	if c.logView == nil {
		return
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	// Defensive cap: a runaway line must never widen the window (log scrolls, but
	// keep rows sane). 300 chars is plenty for any real status message.
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	c.logView.Append(time.Now().Format("15:04:05"), msg, classifyLog(msg))
}

// classifyLog picks a log row color from the message content.
func classifyLog(msg string) logKind {
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "connected") || strings.Contains(l, "applied") ||
		strings.Contains(l, "saved") || strings.Contains(l, "active") ||
		strings.Contains(l, "vpn traffic"):
		return logOK
	case strings.Contains(l, "fail") || strings.Contains(l, "error") ||
		strings.Contains(l, "skipped") || strings.Contains(l, "not ") ||
		strings.Contains(l, "cannot") || strings.Contains(l, "invalid") ||
		strings.Contains(l, "warn"):
		return logWarn
	case strings.Contains(l, "cancel") || strings.Contains(l, "cleared") ||
		strings.Contains(l, "ready") || strings.Contains(l, "diagnostics"):
		return logMuted
	default:
		return logInfo
	}
}

func (c *controller) appendLogf(format string, args ...any) {
	c.appendLog(fmt.Sprintf(format, args...))
}

func (c *controller) onClearLog() {
	if c.logView == nil {
		return
	}
	c.logView.Clear()
	c.appendLog("Log cleared.")
}

func (c *controller) loadInitial() {
	// Best-effort cleanup of orphaned temp VPN scripts from prior runs.
	vpn.PurgeOrphanScripts()

	c.appendLog("Ready. Select a VPN connection, enter routes, then click Connect.")

	stored, err := config.LoadStored()
	if err != nil {
		c.setStatus(vpn.StatusDisconnected, "Disconnected", "Failed to load settings; using defaults.")
		c.appendLog("Failed to load settings; using defaults.")
	} else {
		c.stored = stored
		c.cfg = stored.Config()
		c.applyConfig(c.cfg)
		c.setStatus(vpn.StatusDisconnected, "Disconnected", "Settings loaded.")
		c.appendLog("Settings loaded.")
	}

	go func() {
		profiles, err := vpn.ListProfiles()
		if err != nil {
			c.appendLog("Failed to load VPN connection list.")
			return
		}
		fyne.Do(func() {
			names := make([]string, 0, len(profiles))
			for _, p := range profiles {
				names = append(names, p.Name)
			}
			c.profiles = make(map[string]vpn.ProfileSummary, len(profiles))
			for _, p := range profiles {
				c.profiles[p.Name] = p
			}
			c.profileSelect.Options = names
			if c.connectionName != "" {
				c.profileSelect.SetSelected(c.connectionName)
			}
			c.profileSelect.Refresh()
		})
	}()

	go func() {
		st, err := c.mgr.Status(c.profileName())
		if err != nil || st == vpn.StatusUnknown {
			return
		}
		fyne.Do(func() {
			if st == vpn.StatusConnected {
				c.state = vpn.StatusConnected
				c.setStatus(vpn.StatusConnected, "Connected", "No active connections through the VPN.")
				c.refreshLocalIP()
				c.appendLog("Already connected (OS status).")
				c.applyEnablement()
			}
		})
	}()

	c.applyEnablement()
	c.loadCredentials()
}

func (c *controller) applyConfig(cfg config.Config) {
	c.cfg = cfg
	if cfg.SelectedProfile != "" {
		c.connectionName = cfg.SelectedProfile
	} else {
		c.connectionName = config.DefaultConnectionName
	}
	if len(cfg.Routes) > 0 {
		c.routesEntry.SetText(strings.Join(cfg.Routes, "\n"))
	}
	c.routeAllCheck.SetChecked(cfg.RouteAllTraffic)
}

// onProfileChanged updates the selected connection. The routes text is global
// and must NOT change when switching profiles.
func (c *controller) onProfileChanged(selected string) {
	c.connectionName = strings.TrimSpace(selected)
	c.loadCredentials()
	c.syncIdentity()
	if c.logView != nil && c.connectionName != "" {
		c.appendLog("Profile selected · " + c.connectionName)
	}
}

// profileName returns the Windows VPN / CredMan profile name (never empty).
func (c *controller) profileName() string {
	name := strings.TrimSpace(c.connectionName)
	if name == "" {
		return config.DefaultConnectionName
	}
	return name
}

// loadCredentials pre-fills the username/password entries from the store when
// the remember checkbox is checked. Never logs the values.
func (c *controller) loadCredentials() {
	if !c.rememberCheck.Checked {
		return
	}
	name := c.profileName()
	entry, ok := c.stored.Credentials[name]
	if !ok {
		return
	}
	c.userEntry.SetText(entry.Username)
	c.passEntry.SetText(entry.Password)
}

// persistCredentials stores or deletes the username/password for a connection
// based on the remember checkbox. Empty values delete. Never logs the values.
func (c *controller) persistCredentials(name, user, pass string) {
	if c.stored.Credentials == nil {
		c.stored.Credentials = map[string]config.CredEntry{}
	}
	if c.rememberCheck.Checked && strings.TrimSpace(user) != "" && strings.TrimSpace(pass) != "" {
		c.stored.Credentials[name] = config.CredEntry{Username: user, Password: pass}
	} else {
		delete(c.stored.Credentials, name)
	}
	_ = config.SaveStored(c.stored)
}

// formatRate converts a bytes-per-second value to bits-per-second and renders
// it as Kbps or Mbps (network rate units), auto-scaling to the appropriate unit.
func formatRate(bytesPerSec float64) string {
	bitsPerSec := bytesPerSec * 8
	if bitsPerSec >= 1e6 {
		return fmt.Sprintf("%.1f Mbps", bitsPerSec/1e6)
	}
	return fmt.Sprintf("%.0f Kbps", bitsPerSec/1e3)
}

// startTraffic launches a ~1/sec ticker that samples live download/upload rates
// for the named connection. Any prior ticker is stopped first.
func (c *controller) startTraffic(name string) {
	c.stopTraffic()
	c.trafficName = name
	stop := make(chan struct{})
	c.trafficStop = stop
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var prevRx, prevTx uint64
		var prevAt time.Time
		var havePrev bool
		var prevConns string
		var connBusy atomic.Bool
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if !c.tickBusy.CompareAndSwap(false, true) {
					continue
				}

				rx, tx, err := vpn.TrafficCounters(name)
				var dl, ul string
				setRates := false
				now := time.Now()
				if err == nil {
					switch {
					case rx == 0 && tx == 0:
						// Soft-fail or empty: drop baseline so next real sample does not spike.
						havePrev = false
					case havePrev && (rx < prevRx || tx < prevTx):
						// Counter reset / rebind: re-baseline, no rate this tick.
						prevRx, prevTx, prevAt = rx, tx, now
					case !havePrev:
						prevRx, prevTx, prevAt = rx, tx, now
						havePrev = true
						dl, ul, setRates = "0 Kbps", "0 Kbps", true
					default:
						elapsed := now.Sub(prevAt).Seconds()
						if elapsed >= 0.1 {
							dl = formatRate(float64(rx-prevRx) / elapsed)
							ul = formatRate(float64(tx-prevTx) / elapsed)
							prevRx, prevTx, prevAt = rx, tx, now
							setRates = true
						}
					}
				}
				if setRates {
					fyne.Do(func() {
						if c.state != vpn.StatusConnected {
							return
						}
						c.setStat(c.statDown, dl)
						c.setStat(c.statUp, ul)
					})
				}
				// Release before ActiveConnections (slow reverse DNS must not block rate ticks).
				c.tickBusy.Store(false)

				// Surface hosts routed through the VPN; skip if a poll is already in flight.
				if !connBusy.CompareAndSwap(false, true) {
					continue
				}
				go func() {
					defer connBusy.Store(false)
					conns, cerr := vpn.ActiveConnections(name)
					if cerr != nil {
						return
					}
					var parts []string
					for _, ac := range conns {
						if ac.Hostname != "" {
							parts = append(parts, ac.Hostname+" ("+ac.RemoteAddr+":"+ac.RemotePort+")")
						} else {
							parts = append(parts, ac.RemoteAddr+":"+ac.RemotePort)
						}
					}
					sig := strings.Join(parts, ", ")
					if sig == prevConns {
						return
					}
					prevConns = sig
					fyne.Do(func() {
						if c.state != vpn.StatusConnected {
							return
						}
						if sig != "" {
							c.appendLog("VPN traffic: " + sig)
							c.setStatus(vpn.StatusConnected, "Connected", "Traffic Route On")
						} else {
							c.setStatus(vpn.StatusConnected, "Connected", "No active connections through the VPN.")
						}
					})
				}()
			}
		}
	}()
}

// stopTraffic halts the traffic ticker (if any) and resets the rate labels.
func (c *controller) stopTraffic() {
	if c.trafficStop != nil {
		close(c.trafficStop)
		c.trafficStop = nil
	}
	c.trafficName = ""
	c.setStat(c.statDown, "—")
	c.setStat(c.statUp, "—")
}

// setStat updates a hero stat tile value (nil-safe).
func (c *controller) setStat(t *canvas.Text, v string) {
	if t == nil {
		return
	}
	t.Text = v
	t.Refresh()
}

// gatewayHost returns the connected VPN gateway address, or "" when not connected.
func (c *controller) gatewayHost() string {
	if c.state != vpn.StatusConnected {
		return ""
	}
	p, ok := c.profiles[c.profileName()]
	if !ok {
		return ""
	}
	return strings.TrimSpace(p.ServerAddress)
}

// startPingTicker launches a ~2s ticker that updates the gateway ping status.
// Any prior ticker is stopped first.
func (c *controller) startPingTicker() {
	c.stopPingTicker()
	stop := make(chan struct{})
	c.pingStop = stop
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				result := pingGateway(c.gatewayHost())
				fyne.Do(func() {
					c.setStat(c.statPing, result)
				})
			}
		}
	}()
}

// stopPingTicker halts the ping ticker (if any) and resets the label.
func (c *controller) stopPingTicker() {
	if c.pingStop != nil {
		close(c.pingStop)
		c.pingStop = nil
	}
	fyne.Do(func() {
		c.setStat(c.statPing, "not connected")
	})
}

// refreshLocalIP asynchronously resolves the VPN adapter's local IPv4 address
// and subnet mask and appends it to the Connected status label, e.g.
// "Connected - 192.168.1.1/255.255.255.0". The adapter IP may not be assigned
// the instant ConnectFull returns, so it retries briefly.
func (c *controller) refreshLocalIP() {
	name := c.profileName()
	go func() {
		var info string
		for attempt := 0; attempt < 10; attempt++ {
			ifIndex, addrs, err := vpn.InterfaceInfo(name)
			if err == nil && ifIndex != 0 && len(addrs) > 0 {
				ip := addrs[0].IP
				mask := addrs[0].Mask
				if ip != nil && mask != nil {
					ones, _ := mask.Size()
					info = fmt.Sprintf("%s/%s/%d", ip.String(), net.IP(mask).String(), ones)
					break
				}
			}
			time.Sleep(300 * time.Millisecond)
		}
		if info == "" {
			return
		}
		fyne.Do(func() {
			if c.state != vpn.StatusConnected {
				return
			}
			if strings.Contains(c.statusPri.Text, info) {
				return
			}
			c.statusPri.SetText(c.statusPri.Text + " - " + info)
			c.heroSub.Text = info
			c.heroSub.Refresh()
		})
	}()
}

// setStatus updates status labels, the hero ring, and the footer bar.
func (c *controller) setStatus(state vpn.ConnStatus, primary, detail string) {
	c.state = state
	c.statusPri.SetText(primary)
	c.statusDet.SetText(detail)
	c.syncVisualState(primary, detail)
}

// syncVisualState maps the connection state onto the redesign widgets.
func (c *controller) syncVisualState(primary, detail string) {
	if c.hero != nil {
		switch c.state {
		case vpn.StatusConnecting, vpn.StatusDisconnecting:
			c.hero.SetState("connecting")
		case vpn.StatusConnected:
			c.hero.SetState("connected")
		default:
			c.hero.SetState("disconnected")
		}
	}

	if c.footerDot != nil && c.footerText != nil {
		col := stateColor("disconnected")
		switch c.state {
		case vpn.StatusConnecting, vpn.StatusDisconnecting:
			col = stateColor("connecting")
		case vpn.StatusConnected:
			col = stateColor("connected")
		}
		c.footerDot.FillColor = col
		c.footerDot.Refresh()
		txt := primary
		if detail != "" {
			txt = primary + " · " + detail
		}
		c.footerText.Text = txt
		c.footerText.Refresh()
	}

	c.syncCTA()
	c.syncIdentity()
}

// syncCTA updates the single call-to-action button's label and importance
// to reflect the current connection state.
func (c *controller) syncCTA() {
	if c.btnConn == nil {
		return
	}
	switch c.state {
	case vpn.StatusDisconnected, vpn.StatusError, vpn.StatusUnknown:
		c.btnConn.SetText("Connect")
		c.btnConn.Importance = widget.HighImportance
	case vpn.StatusConnecting:
		c.btnConn.SetText("Cancel")
		c.btnConn.Importance = widget.DangerImportance
	case vpn.StatusConnected:
		c.btnConn.SetText("Disconnect")
		c.btnConn.Importance = widget.MediumImportance
	case vpn.StatusDisconnecting:
		c.btnConn.SetText("Disconnecting…")
		c.btnConn.Importance = widget.MediumImportance
	}
	c.btnConn.Refresh()
}

// syncIdentity updates the profile name + sub line shown under the hero ring.
func (c *controller) syncIdentity() {
	if c.heroName == nil {
		return
	}
	name := strings.TrimSpace(c.profileSelect.Selected)
	if name == "" {
		c.heroName.Text = "No profile selected"
		c.heroSub.Text = "choose a profile to begin"
	} else {
		c.heroName.Text = name
		if c.state == vpn.StatusConnected {
			// heroSub shows the VPN local IP/subnet once refreshLocalIP resolves it.
			c.heroSub.Text = "L2TP/IPsec · connected"
		} else {
			host := ""
			if p, ok := c.profiles[name]; ok {
				host = strings.TrimSpace(p.ServerAddress)
			}
			if host != "" {
				c.heroSub.Text = "L2TP/IPsec · " + host
			} else {
				c.heroSub.Text = "L2TP/IPsec"
			}
		}
	}
	c.heroName.Refresh()
	c.heroSub.Refresh()
}

func (c *controller) applyEnablement() {
	formEnabled := c.state == vpn.StatusDisconnected || c.state == vpn.StatusError
	busyConnect := c.state == vpn.StatusConnecting
	busyDisc := c.state == vpn.StatusDisconnecting

	setEntry := func(e *widget.Entry, on bool) {
		if on {
			e.Enable()
		} else {
			e.Disable()
		}
	}
	setEntry(c.routesEntry, formEnabled)
	setEntry(c.userEntry, formEnabled)
	setEntry(c.passEntry, formEnabled)

	// logView is inherently read-only; Clear is always available (UI-only buffer).
	if c.btnClearLog != nil {
		c.btnClearLog.Enable()
	}

	if busyConnect || busyDisc || c.state == vpn.StatusConnected || c.busy {
		c.btnSave.Disable()
	} else {
		c.btnSave.Enable()
	}

	// Single CTA button enablement, driven by state + generic busy lock.
	switch c.state {
	case vpn.StatusDisconnected, vpn.StatusError, vpn.StatusUnknown,
		vpn.StatusConnecting, vpn.StatusConnected:
		c.btnConn.Enable()
	default: // Disconnecting
		c.btnConn.Disable()
	}
	// Disable the CTA only when busy in a non-cancellable state, or while
	// disconnecting. During Connecting the button is "Cancel" and must stay
	// enabled so the user can abort the connection.
	if c.busy && c.state != vpn.StatusConnecting {
		c.btnConn.Disable()
	}
}

func (c *controller) onSave() {
	c.mu.Lock()
	if c.busy {
		c.mu.Unlock()
		return
	}
	c.busy = true
	c.mu.Unlock()
	c.applyEnablement()

	name := c.profileName()
	c.connectionName = name

	routesText := c.routesEntry.Text
	var routes []string
	if strings.TrimSpace(routesText) != "" {
		parsed, err := route.ParseLines(routesText)
		if err != nil {
			c.finishSave(false, "Cannot save", err.Error())
			c.appendLog("Validation: " + err.Error())
			if c.win != nil {
				c.win.Canvas().Focus(c.routesEntry)
			}
			return
		}
		routes = parsed
	}

	// Update in-memory config. Routes are global (not per-profile).
	c.stored.SelectedProfile = name
	c.stored.Routes = routes
	c.stored.RememberCredentials = c.rememberCheck.Checked
	c.stored.RouteAllTraffic = c.routeAllCheck.Checked
	c.cfg = c.stored.Config()
	cfg := c.stored
	keepState := c.state
	keepPrimary := c.statusPri.Text

	go func() {
		if err := config.SaveStored(cfg); err != nil {
			fyne.Do(func() {
				detail := sanitizeUIErr(err)
				c.finishSave(false, "Failed to save settings", detail)
				c.appendLog("Failed to save: " + detail)
			})
			return
		}
		fyne.Do(func() {
			c.persistCredentials(name, strings.TrimSpace(c.userEntry.Text), c.passEntry.Text)
			if keepState == vpn.StatusConnected {
				c.finishSaveKeep(keepState, keepPrimary, "Settings saved.")
			} else {
				c.finishSaveKeep(vpn.StatusDisconnected, "Disconnected", "Settings saved.")
			}
			c.appendLog("Settings saved.")
		})
	}()
}

func (c *controller) finishSave(ok bool, primary, detail string) {
	c.mu.Lock()
	c.busy = false
	c.mu.Unlock()
	if !ok {
		c.setStatus(vpn.StatusError, primary, detail)
	}
	c.applyEnablement()
}

func (c *controller) finishSaveKeep(st vpn.ConnStatus, primary, detail string) {
	c.mu.Lock()
	c.busy = false
	c.mu.Unlock()
	c.setStatus(st, primary, detail)
	c.applyEnablement()
}

func (c *controller) onConnect() {
	c.mu.Lock()
	if c.busy || c.state == vpn.StatusConnected || c.state == vpn.StatusConnecting {
		if c.state == vpn.StatusConnected {
			c.setStatus(vpn.StatusConnected, "Connected", "Already connected.")
			c.appendLog("Already connected.")
		}
		c.mu.Unlock()
		return
	}
	c.busy = true
	c.mu.Unlock()

	if errMsg, focus := c.validateConnect(); errMsg != "" {
		c.mu.Lock()
		c.busy = false
		c.mu.Unlock()
		c.setStatus(vpn.StatusError, "Cannot connect", errMsg)
		c.appendLog("Validation: " + errMsg)
		c.applyEnablement()
		if focus != nil && c.win != nil {
			c.win.Canvas().Focus(focus)
		}
		return
	}

	name := c.profileName()
	c.connectionName = c.profileSelect.Selected

	req := vpn.ConnectRequest{
		Name:            name,
		Username:        strings.TrimSpace(c.userEntry.Text),
		Password:        c.passEntry.Text,
		RoutesText:      c.routesEntry.Text,
		RouteAllTraffic: c.routeAllCheck.Checked,
	}

	// Fall back to stored credentials when the form is empty and remember is on.
	if req.Username == "" && req.Password == "" && c.rememberCheck.Checked {
		if entry, ok := c.stored.Credentials[name]; ok {
			if entry.Username != "" {
				req.Username = entry.Username
			}
			if entry.Password != "" {
				req.Password = entry.Password
			}
		}
	}

	c.setStatus(vpn.StatusConnecting, "Connecting…", "Preparing split tunnel routes…")
	c.appendLogf("Connecting to %s…", name)
	c.applyEnablement()

	go c.persistQuiet(req)

	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancelConnect = cancel
	c.mu.Unlock()

	go func() {
		warnings, err := c.mgr.ConnectFull(ctx, req, func(phase vpn.Phase) {
			detail := vpn.PhaseDetail(phase)
			fyne.Do(func() {
				if c.state == vpn.StatusConnecting {
					c.statusDet.SetText(detail)
					if detail != "" {
						c.appendLog(detail)
					}
				}
			})
		})
		fyne.Do(func() {
			c.mu.Lock()
			c.busy = false
			c.cancelConnect = nil
			c.mu.Unlock()
			if err != nil {
				if ue, ok := vpn.AsUserError(err); ok && ue.Code == "canceled" {
					c.setStatus(vpn.StatusDisconnected, "Cancelled", "Connection cancelled.")
					c.appendLog("Cancelled.")
					// best-effort cleanup of a half-open tunnel, then refresh real status
					go func() {
						_ = c.mgr.DisconnectFull(name)
						st, _ := c.mgr.Status(name)
						fyne.Do(func() {
							c.mu.Lock()
							c.state = st
							c.mu.Unlock()
							switch st {
							case vpn.StatusConnected:
								c.setStatus(vpn.StatusConnected, "Still connected", "Disconnect manually.")
							default:
								c.setStatus(vpn.StatusDisconnected, "Cancelled", "Connection cancelled.")
							}
							c.applyEnablement()
						})
					}()
				} else if ue, ok := vpn.AsUserError(err); ok && ue.Code == "already" {
					// Already connected is a success-like state: enable Disconnect.
					c.setStatus(vpn.StatusConnected, ue.Primary, ue.Detail)
					c.refreshLocalIP()
					c.appendLog("Already connected.")
					c.hostArea.SetText("Waiting for host list…")
					c.startTraffic(name)
					c.startPingTicker()
				} else {
					primary, detail := formatVPNError(err)
					c.setStatus(vpn.StatusError, primary, detail)
					if detail != "" {
						c.appendLog(primary + ": " + detail)
					} else {
						c.appendLog(primary)
					}
				}
			} else {
				c.setStatus(vpn.StatusConnected, "Connected", "No active connections through the VPN.")
				c.appendLog("Connected. Split tunnel active.")
				// Diagnostics (raw scutil dump) go to the OS log only — it's one huge
				// unbreakable line, useless and layout-breaking in the UI.
				if diag, derr := vpn.ProfileDiagnostics(name); derr == nil && diag != "" {
					log.Printf("connect diagnostics: %s", diag)
				}
				for _, w := range warnings {
					c.appendLog(w)
				}
				c.persistCredentials(name, strings.TrimSpace(c.userEntry.Text), c.passEntry.Text)
				c.hostArea.SetText("Waiting for host list…")
				c.startTraffic(name)
				c.startPingTicker()
				c.refreshLocalIP()
			}
			c.applyEnablement()
		})
	}()
}

func (c *controller) onCancel() {
	c.mu.Lock()
	if c.state != vpn.StatusConnecting {
		c.mu.Unlock()
		return
	}
	cancel := c.cancelConnect
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.appendLog("Cancelling connection…")
	// The running ConnectFull will return a canceled error; completion handler cleans up.
}

func (c *controller) onDisconnect() {
	c.mu.Lock()
	if c.busy || c.state != vpn.StatusConnected {
		if c.state == vpn.StatusDisconnected {
			c.setStatus(vpn.StatusDisconnected, "Disconnected", "Already disconnected.")
			c.appendLog("Already disconnected.")
		}
		c.mu.Unlock()
		return
	}
	c.busy = true
	c.mu.Unlock()

	name := c.profileName()

	c.setStatus(vpn.StatusDisconnecting, "Disconnecting…", "")
	c.appendLog("Disconnecting…")
	c.applyEnablement()

	go func() {
		err := c.mgr.DisconnectFull(name)
		fyne.Do(func() {
			c.mu.Lock()
			c.busy = false
			c.mu.Unlock()
			if err != nil {
				primary, detail := formatVPNError(err)
				c.setStatus(vpn.StatusConnected, primary, detail)
				if detail != "" {
					c.appendLog(primary + ": " + detail)
				} else {
					c.appendLog(primary)
				}
			} else {
				c.setStatus(vpn.StatusDisconnected, "Disconnected", "Connection closed.")
				c.appendLog("Connection closed.")
				c.stopTraffic()
				c.stopPingTicker()
				c.hostArea.SetText("—")
			}
			c.applyEnablement()
		})
	}()
}

func (c *controller) validateConnect() (string, fyne.Focusable) {
	// Hidden name always defaults; never block connect on empty name.
	if strings.TrimSpace(c.profileSelect.Selected) == "" {
		return "Select a VPN connection first.", c.profileSelect
	}
	prefixes, err := route.ParseLines(c.routesEntry.Text)
	if err != nil {
		return err.Error(), c.routesEntry
	}
	if len(prefixes) == 0 && !c.routeAllCheck.Checked {
		return "Enter at least one IP, CIDR, or domain name for split tunnel.", c.routesEntry
	}
	return "", nil
}

func (c *controller) persistQuiet(req vpn.ConnectRequest) {
	prefixes, _ := route.ParseLines(req.RoutesText)
	cur, err := config.LoadStored()
	if err != nil {
		// Store present but unreadable: writing now would drop the credentials
		// we could not decrypt. Leave it alone.
		return
	}
	cur.SelectedProfile = req.Name
	cur.Routes = prefixes
	// Auto-save the remaining settings at connect time so the user does not
	// have to press Save Settings manually (the button still works too).
	cur.RememberCredentials = c.rememberCheck.Checked
	cur.RouteAllTraffic = c.routeAllCheck.Checked
	_ = config.SaveStored(cur)
}

func formatVPNError(err error) (primary, detail string) {
	if ue, ok := vpn.AsUserError(err); ok {
		return ue.Primary, ue.Detail
	}
	if pe, ok := err.(*route.ParseError); ok {
		return "Cannot connect", pe.Error()
	}
	return "Failed", sanitizeUIErr(err)
}

func sanitizeUIErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	lower := strings.ToLower(s)
	if strings.Contains(lower, "l2tppsk") || strings.Contains(lower, "password") {
		return "An error occurred (details hidden for security)."
	}
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}

// minSizeWrap enforces a minimum size on window content (Fyne Window has no SetMinSize).
// It is a proper fyne.Widget so the canvas can obtain a renderer and render the subtree.
type minSizeWrap struct {
	widget.BaseWidget
	inner fyne.CanvasObject
	min   fyne.Size
}

func newMinSizeWrap(inner fyne.CanvasObject, min fyne.Size) *minSizeWrap {
	w := &minSizeWrap{inner: inner, min: min}
	w.ExtendBaseWidget(w)
	return w
}

func (m *minSizeWrap) MinSize() fyne.Size {
	base := m.inner.MinSize()
	w, h := base.Width, base.Height
	if m.min.Width > w {
		w = m.min.Width
	}
	if m.min.Height > h {
		h = m.min.Height
	}
	return fyne.NewSize(w, h)
}

func (m *minSizeWrap) CreateRenderer() fyne.WidgetRenderer {
	return &minSizeWrapRenderer{w: m, inner: m.inner}
}

type minSizeWrapRenderer struct {
	w     *minSizeWrap
	inner fyne.CanvasObject
}

func (r *minSizeWrapRenderer) Layout(size fyne.Size) {
	r.inner.Resize(size)
}

func (r *minSizeWrapRenderer) MinSize() fyne.Size {
	return r.w.MinSize()
}

func (r *minSizeWrapRenderer) Refresh() {
	r.inner.Refresh()
}

func (r *minSizeWrapRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.inner}
}

func (r *minSizeWrapRenderer) Destroy() {}
