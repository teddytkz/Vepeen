package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	w.Resize(fyne.NewSize(960, 600))

	ctrl := newController()
	ctrl.win = w
	// Window has no SetMinSize in Fyne v2.8; enforce via content MinSize wrapper.
	w.SetContent(newMinSizeWrap(ctrl.build(), fyne.NewSize(900, 560)))
	// Centering is now performed synchronously at show time by ui.ShowCentered
	// (called from main.go after NewMainWindow returns), which positions the
	// window at the work-area center before the first frame is composited. This
	// avoids the teleport blink from the old deferred goroutine.
	ctrl.loadInitial()

	// disconnectAndQuit disconnects the VPN (best-effort, 5 s) then quits.
	disconnectAndQuit := func() {
		name := ctrl.profileName()
		if name != "" {
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = ctrl.mgr.DisconnectFull(name)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
		a.Quit()
	}

	w.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("Menu",
			fyne.NewMenuItem("Create Desktop Shortcut", func() {
				if err := CreateDesktopShortcut(); err != nil {
					ctrl.appendLog("Failed to create desktop shortcut: " + err.Error())
				} else {
					ctrl.appendLog("Desktop shortcut created.")
				}
			}),
		),
	))

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
	logEntry      *widget.Entry

	btnSave     *widget.Button
	btnDisc     *widget.Button
	btnConn     *widget.Button
	btnCancel   *widget.Button
	btnClearLog *widget.Button
	statusPri   *widget.Label
	statusDet   *widget.Label

	rememberCheck *widget.Check
	hostArea      *widget.Entry
	dlLabel       *widget.Label
	tickBusy      atomic.Bool
	ulLabel       *widget.Label
	profiles      map[string]vpn.ProfileSummary // name -> summary (for ServerAddress)
	trafficStop   chan struct{}                 // closed to stop the traffic ticker
	trafficName   string                        // active connection name for the ticker

	pingLabel *widget.Label // gateway ping status text
	pingStop  chan struct{} // closed to stop the ping ticker
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

// smallTitle renders a compact bold teal section heading used inside cards.
func smallTitle(text string) *canvas.Text {
	t := canvas.NewText(text, color.NRGBA{R: 0x0f, G: 0xb5, B: 0xae, A: 0xff})
	t.TextSize = theme.Size(theme.SizeNameText) * 0.85
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func (c *controller) build() fyne.CanvasObject {
	title := widget.NewLabel("Vepeen")
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := widget.NewLabel("L2TP/IPsec · split tunnel")
	subtitle.Wrapping = fyne.TextWrapWord

	header := container.NewPadded(container.NewVBox(title, subtitle, widget.NewSeparator()))

	c.profileSelect = widget.NewSelect([]string{}, c.onProfileChanged)
	c.profileSelect.PlaceHolder = "Select Windows VPN connection…"

	c.userEntry = widget.NewEntry()
	c.userEntry.SetPlaceHolder("Username (optional)")
	c.passEntry = widget.NewPasswordEntry()
	c.passEntry.SetPlaceHolder("Password (optional)")
	credNote := widget.NewLabel("Leave username/password blank to use credentials saved in Windows Credential Manager.")
	credNote.Wrapping = fyne.TextWrapWord

	c.rememberCheck = widget.NewCheck("Remember credentials", nil)
	c.rememberCheck.SetChecked(true)

	cardKoneksi := widget.NewCard("", "",
		container.NewVBox(
			smallTitle("VPN Connection"),
			widget.NewLabel("Select an existing VPN profile from Windows."),
			c.profileSelect,
			c.userEntry,
			c.passEntry,
			credNote,
			c.rememberCheck,
		),
	)

	routesDuty := widget.NewLabel("Required · one IP, CIDR, or domain name per line. Only these destinations route through the VPN.")
	routesDuty.Wrapping = fyne.TextWrapWord

	c.routesEntry = widget.NewMultiLineEntry()
	c.routesEntry.SetMinRowsVisible(3)
	c.routesEntry.Wrapping = fyne.TextWrapOff

	routesHelp := widget.NewLabel("Examples: 10.10.0.0/16, 203.0.113.50, or mail.foofle.com. Blank lines ignored. # = comment.")
	routesHelp.Wrapping = fyne.TextWrapWord

	cardRute := widget.NewCard("", "",
		container.NewVBox(smallTitle("Split Tunnel Routes"), routesDuty, c.routesEntry, routesHelp),
	)

	c.statusPri = widget.NewLabel("Disconnected")
	c.statusPri.TextStyle = fyne.TextStyle{Bold: true}
	c.statusPri.Wrapping = fyne.TextWrapWord
	c.statusDet = widget.NewLabel("Select a VPN connection, enter routes, then click Connect.")
	c.statusDet.Wrapping = fyne.TextWrapWord

	cardStatus := widget.NewCard("", "",
		container.NewVBox(smallTitle("Status"), c.statusPri, c.statusDet),
	)

	c.btnClearLog = widget.NewButton("Clear log", c.onClearLog)
	logHeader := container.NewBorder(nil, nil, smallTitle("Log"), c.btnClearLog)

	c.logEntry = widget.NewMultiLineEntry()
	c.logEntry.SetMinRowsVisible(5)
	c.logEntry.Wrapping = fyne.TextWrapOff
	c.logEntry.Disable() // read-only activity history

	cardLog := widget.NewCard("", "",
		container.NewVBox(logHeader, c.logEntry),
	)

	c.hostArea = widget.NewMultiLineEntry()
	c.hostArea.SetMinRowsVisible(4)
	c.hostArea.Wrapping = fyne.TextWrapOff
	c.hostArea.Disable()
	cardInfo := widget.NewCard("", "", container.NewVBox(smallTitle("Connection Info"), c.hostArea))

	c.dlLabel = widget.NewLabel("Download: —")
	c.ulLabel = widget.NewLabel("Upload: —")
	c.dlLabel.Wrapping = fyne.TextWrapWord
	c.ulLabel.Wrapping = fyne.TextWrapWord
	cardTraffic := widget.NewCard("", "", container.NewVBox(smallTitle("Traffic"), c.dlLabel, c.ulLabel))

	c.pingLabel = widget.NewLabel("not connected")
	cardPing := widget.NewCard("", "", container.NewVBox(
		smallTitle("Ping Status"),
		c.pingLabel,
	))

	leftCol := container.NewBorder(cardKoneksi, nil, nil, nil, cardRute)
	rightCol := container.NewBorder(cardStatus, nil, nil, nil,
		container.NewVBox(cardLog, cardInfo, cardTraffic, cardPing),
	)

	body := container.NewPadded(
		container.NewGridWithColumns(2, leftCol, rightCol),
	)

	c.btnSave = widget.NewButton("Save", c.onSave)
	c.btnDisc = widget.NewButton("Disconnect", c.onDisconnect)
	c.btnCancel = widget.NewButton("Cancel", c.onCancel)
	c.btnConn = widget.NewButton("Connect", c.onConnect)
	c.btnConn.Importance = widget.HighImportance

	buttonRow := container.NewPadded(
		container.NewHBox(
			c.btnSave,
			layout.NewSpacer(),
			container.NewHBox(c.btnDisc, c.btnCancel, c.btnConn),
		),
	)

	root := container.NewBorder(header, buttonRow, nil, nil, body)
	return container.NewPadded(root)
}

// appendLog adds a timestamped line to the activity log (UI thread only).
// Never pass PSK, password, or secret-bearing command lines.
func (c *controller) appendLog(msg string) {
	if c.logEntry == nil {
		return
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	line := time.Now().Format("15:04:05") + "  " + msg
	cur := c.logEntry.Text
	var next string
	if cur == "" {
		next = line
	} else {
		next = cur + "\n" + line
	}
	lines := strings.Split(next, "\n")
	if len(lines) > maxLogLines {
		lines = lines[len(lines)-maxLogLines:]
		next = strings.Join(lines, "\n")
	}
	c.logEntry.SetText(next)
	// Scroll toward end for long history.
	c.logEntry.CursorRow = len(lines) - 1
	c.logEntry.Refresh()
}

func (c *controller) appendLogf(format string, args ...any) {
	c.appendLog(fmt.Sprintf(format, args...))
}

func (c *controller) onClearLog() {
	if c.logEntry == nil {
		return
	}
	c.logEntry.SetText("")
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
				c.setStatus(vpn.StatusConnected, "Connected", "Only the listed IPs/CIDRs route through the VPN.")
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
}

// onProfileChanged updates the selected connection. The routes text is global
// and must NOT change when switching profiles.
func (c *controller) onProfileChanged(selected string) {
	c.connectionName = strings.TrimSpace(selected)
	c.loadCredentials()
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

// formatRate renders a bytes-per-second value as KB/s or MB/s.
func formatRate(bytesPerSec float64) string {
	if bytesPerSec >= float64(1<<20) {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/float64(1<<20))
	}
	return fmt.Sprintf("%d KB/s", int(bytesPerSec/float64(1<<10)))
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
		var havePrev bool
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if !c.tickBusy.CompareAndSwap(false, true) {
					continue
				}

				rx, tx, err := vpn.TrafficCounters(name)
				dl, ul := "0 KB/s", "0 KB/s"
				if err == nil {
					if !havePrev && (rx > 0 || tx > 0) {
						havePrev = true
					}
					if havePrev {
						dRx := float64(rx - prevRx)
						dTx := float64(tx - prevTx)
						if dRx < 0 {
							dRx = 0
						}
						if dTx < 0 {
							dTx = 0
						}
						dl = formatRate(dRx)
						ul = formatRate(dTx)
					}
					prevRx, prevTx = rx, tx
				}

				hostText := "No active TCP connections through the VPN"
				if conns, cerr := vpn.ActiveConnections(name); cerr == nil && len(conns) > 0 {
					var b strings.Builder
					for _, ac := range conns {
						if ac.Hostname != "" {
							b.WriteString(ac.Hostname)
							b.WriteString(" (")
							b.WriteString(ac.RemoteAddr)
							b.WriteString(":")
							b.WriteString(ac.RemotePort)
							b.WriteString(")\n")
						} else {
							b.WriteString(ac.RemoteAddr)
							b.WriteString(":")
							b.WriteString(ac.RemotePort)
							b.WriteString("\n")
						}
					}
					hostText = strings.TrimRight(b.String(), "\n")
				}

				fyne.Do(func() {
					c.dlLabel.SetText("Download: " + dl)
					c.ulLabel.SetText("Upload: " + ul)
					c.hostArea.SetText(hostText)
				})
				c.tickBusy.Store(false)
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
	if c.dlLabel != nil {
		c.dlLabel.SetText("Download: —")
	}
	if c.ulLabel != nil {
		c.ulLabel.SetText("Upload: —")
	}
	if c.hostArea != nil {
		c.hostArea.SetText("—")
	}
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
					c.pingLabel.SetText(result)
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
		c.pingLabel.SetText("not connected")
	})
}

// setStatus updates short status labels only (does not append log).
func (c *controller) setStatus(state vpn.ConnStatus, primary, detail string) {
	c.state = state
	c.statusPri.SetText(primary)
	c.statusDet.SetText(detail)
}

func (c *controller) applyEnablement() {
	formEnabled := c.state == vpn.StatusDisconnected || c.state == vpn.StatusError
	busyConnect := c.state == vpn.StatusConnecting
	busyDisc := c.state == vpn.StatusDisconnecting
	connected := c.state == vpn.StatusConnected

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

	// Log is always non-editable; Clear is always available (UI-only buffer).
	if c.logEntry != nil {
		c.logEntry.Disable()
	}
	if c.btnClearLog != nil {
		c.btnClearLog.Enable()
	}

	if busyConnect || busyDisc || c.busy {
		c.btnSave.Disable()
	} else {
		c.btnSave.Enable()
	}

	if formEnabled && !c.busy {
		c.btnConn.Enable()
	} else {
		c.btnConn.Disable()
	}

	if connected && !c.busy {
		c.btnDisc.Enable()
	} else {
		c.btnDisc.Disable()
	}

	if busyConnect || busyDisc {
		c.btnConn.Disable()
		c.btnDisc.Disable()
	}

	if busyConnect {
		c.btnCancel.Enable()
	} else {
		c.btnCancel.Disable()
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
		Name:       name,
		Username:   strings.TrimSpace(c.userEntry.Text),
		Password:   c.passEntry.Text,
		RoutesText: c.routesEntry.Text,
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
				c.setStatus(vpn.StatusConnected, "Connected", "Only the listed IPs/CIDRs route through the VPN.")
				c.appendLog("Connected. Split tunnel active.")
				if diag, derr := vpn.ProfileDiagnostics(name); derr == nil && diag != "" {
					c.appendLog("Diagnostics: " + diag)
				}
				for _, w := range warnings {
					c.appendLog(w)
				}
				c.persistCredentials(name, strings.TrimSpace(c.userEntry.Text), c.passEntry.Text)
				c.hostArea.SetText("Waiting for host list…")
				c.startTraffic(name)
				c.startPingTicker()
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
	if len(prefixes) == 0 {
		return "Enter at least one IP, CIDR, or domain name for split tunnel.", c.routesEntry
	}
	return "", nil
}

func (c *controller) persistQuiet(req vpn.ConnectRequest) {
	prefixes, _ := route.ParseLines(req.RoutesText)
	cur, _ := config.LoadStored()
	cur.SelectedProfile = req.Name
	cur.Routes = prefixes
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
