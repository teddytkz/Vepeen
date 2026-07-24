## [Unreleased]

### Fixed

- [2026-07-24] **fix-018** Low: quit re-entrancy guard (reviewer suggestion, user: "buatin atomic.Bool, apps matang tanpa bugs"). Root cause: `disconnectAndQuit` in `internal/ui/main_window.go` is called from both the main-menu "Quit" item and the tray "Quit" item (`internal/ui/tray_windows.go:39`); if both fire in quick succession, two progress dialogs spawn and `a.Quit()` can be invoked twice. Fix: add `quitting atomic.Bool` field to the `controller` struct (next to the existing `tickBusy atomic.Bool`, line 152) and insert `if !ctrl.quitting.CompareAndSwap(false, true) { return }` at the very top of the `disconnectAndQuit` closure (before `name := ctrl.profileName()`). `sync/atomic` is already imported. No other behavior change — dialog, 5 s timeout, `fyne.Do` usage, and X-button hide-to-tray stay as-is. `go build ./...` and `go vet ./internal/ui/...` must pass. Agent: Frontend Developer → Debugger/Reviewer.

- [2026-07-24] **fix-017** Low: quitting blocks silently with no feedback (user: "saat quit jangan ke blocking diam tapi munculkan dialog proses disconnectnya saja"). Root cause: `disconnectAndQuit` in `internal/ui/main_window.go` runs the disconnect entirely off the UI thread (goroutine + `select`) with no visible status, so the wait appears to "block silently". Fix: add a non-dismissible progress dialog (`dialog.NewCustomWithoutButtons("Quitting Vepeen", container.NewVBox(widget.NewLabel("Disconnecting VPN…"), widget.NewProgressBarInfinite()), w)`), call `w.Show()` then `dlg.Show()` so it appears even from the tray, update the label at each step via `fyne.Do` ("Disconnecting <name>…", "Still connected, retrying…", "Closing…"), and on completion OR the 5 s timeout hide the dialog + `a.Quit()` inside `fyne.Do`. Add the `fyne.io/fyne/v2/dialog` import. No confirmation dialog; X-button hide-to-tray unchanged; best-effort + 5 s timeout preserved; no secrets logged. `go build ./...` and `go vet ./internal/ui/...` must pass. Agent: Frontend Developer → Debugger/Reviewer.

- [2026-07-24] **fix-016** Medium: quit does not disconnect VPN reliably (user: "kalau quit lewat menu kenapa tidak disconnect ya"). Root cause: `disconnectAndQuit` in `internal/ui/main_window.go` swallowed the disconnect error (`_ =`), never verified OS status, and `profileName()` always returns a non-empty name (falls back to `DefaultConnectionName`) so a name mismatch leaves the VPN up silently; also the main window menu had **no Quit item** (only "Create Desktop Shortcut"), making "quit lewat menu" ambiguous. Fix: rewrite `disconnectAndQuit` to status-check → `DisconnectFull` → verify via `Status` → retry once if still `Connected`, logging each step with `log.Printf` (no secrets); add a "Quit" `fyne.NewMenuItem` to the main window `fyne.NewMainMenu` using the same closure. Keep X-button hide-to-tray (no disconnect) unchanged. `go build ./...` and `go vet ./internal/ui/...` must pass. Agent: Frontend Developer → Debugger/Reviewer.

### Added

- [2026-07-24] **prd-005** Minor: show local IP + subnet next to "Connected" status, e.g. `Connected - 192.168.1.1/255.255.255.0`. New `vpn.InterfaceInfo(name)` (win impl in `internal/vpn/win/netapi_windows.go`, re-export `internal/vpn/win_exports_windows.go`, stub `internal/vpn/stub_other.go`); UI `refreshLocalIP()` goroutine with retry in `internal/ui/main_window.go` called from the 3 Connected `setStatus` paths. Agent: Backend → Frontend → Debugger/Reviewer.

### Fixed

- [2026-07-24] **fix-015** Medium: COM apartment refcount imbalance in `CreateDesktopShortcut()` (`internal/ui/desktop_shortcut_windows.go`). `CoUninitialize` was deferred unconditionally, including on the `S_FALSE` (already-initialized) success path, which could tear down COM for the owning thread (e.g. Fyne's UI thread) and cause `RPC_E_DISCONNECTED` / `CO_E_NOTINITIALIZED`. Now `CoUninitialize` is deferred only when `CoInitializeEx` returns `S_OK` (0); on `S_FALSE` (1) it proceeds without uninitializing; any other HRESULT returns an error. `go build ./...` and `go vet ./internal/ui/...` must still pass.

### Changed

- [2026-07-24] **menu-bar-apps** Minor: add a Fyne main menu bar with an "Apps" menu containing a "Create Desktop Shortcut" item that writes a Windows `.lnk` on the user's Desktop pointing at the running executable (`os.Executable()`), with the app icon and name "Vepeen".
  - **Menu location (recommended):** build and set the `fyne.MainMenu` inside `NewMainWindow` in `internal/ui/main_window.go` (after `ctrl` is constructed, before `return`). Rationale: the menu item must surface success/failure feedback via the controller's existing `appendLog` mechanism, and `ctrl` is only in scope there; `main.go` only receives `(fyne.Window, func())` and has no handle to the controller. Setting it in `main.go` would require exporting a callback or the controller — unnecessary indirection.
  - **Shortcut approach (recommended): (A) native IShellLinkW + IPersistFile via `golang.org/x/sys/windows`.** Rationale: zero new dependencies (consistent with DPAPI/single-instance/netapi which already use `NewLazySystemDLL`/`NewProc`/`Call`); no PowerShell spawn (aligns with fix-012's anti-powershell direction); Windows-only code is already isolated behind `//go:build windows` + `!windows` stubs in this package. (B) `go-ole` adds a dependency the project has avoided; (C) PowerShell is explicitly discouraged by project direction.
  - **Files:**
    - `internal/ui/desktop_shortcut_windows.go` (NEW, `//go:build windows`): `CreateDesktopShortcut() error` — resolve `os.Executable()`, resolve Desktop via `SHGetFolderPath(CSIDL_DESKTOP)` / `FOLDERID_Desktop`, build `IShellLinkW` + `IPersistFile` COM objects via `ole32.dll`/`shell32.dll` lazy procs, set target path + icon location (`os.Executable()`) + description "Vepeen", `Save` to `<Desktop>\Vepeen.lnk`, release COM. Returns a clear error on any failure (already-exists is non-fatal → return nil or a sentinel).
    - `internal/ui/desktop_shortcut_other.go` (NEW, `//go:build !windows`): `CreateDesktopShortcut() error` returns `nil` (no-op) so non-Windows builds compile.
    - `internal/ui/main_window.go`: inside `NewMainWindow`, after `ctrl` is built, construct `fyne.NewMainMenu(fyne.NewMenu("Apps", fyne.NewMenuItem("Create Desktop Shortcut", func(){ if err := CreateDesktopShortcut(); err != nil { ctrl.appendLog("Failed to create desktop shortcut: " + err.Error()) } else { ctrl.appendLog("Desktop shortcut created.") } }))` and call `w.SetMainMenu(menu)`.
  - **Feedback:** route result through `ctrl.appendLog(...)` (existing activity log, UI-thread safe, English strings only per i18n-en). No dialog needed; consistent with how Save/Connect surface status.
  - **Constraints honored:** no console window (`-H windowsgui` already set in `build.ps1`); single-instance mutex and tray "Show"/"Quit" behavior untouched; teal theme unaffected (menu uses Fyne default theming); English-only strings.
  - **Agent:** Frontend Developer → Debugger/Reviewer

- [2026-07-23] **fix-014** Low: embed Penelope icon into `vepeen.exe` so Windows Explorer / taskbar / title bar show it (`docs/planning/fix-014-exe-icon.md`)
  - **Root cause:** `cmd/vepeen/rsrc.syso` (1 132 bytes) contains only an `RT_MANIFEST` — the `-ico` flag was never passed to `rsrc`, so no `RT_ICON`/`RT_GROUP_ICON` resource was compiled in. `vepeen.exe.manifest` was also never committed.
  - **Task 1** — recreate `vepeen.exe.manifest` (repo root): standard Per-Monitor v2 DPI + UAC `asInvoker` manifest; content matches what originally produced the committed `.syso`.
  - **Task 2** — produce `docs/images/penelope.ico` from existing `docs/images/penelope.png`: `go install github.com/mat/besticon/ico/cmd/png2ico@latest` → `png2ico docs/images/penelope.ico docs/images/penelope.png` (multi-size 16/32/48/256 px). ImageMagick `magick -define icon:auto-resize` is an alternative.
  - **Task 3** — regenerate `cmd/vepeen/rsrc.syso`: `rsrc -manifest vepeen.exe.manifest -ico docs/images/penelope.ico -o cmd/vepeen/rsrc.syso`. Result grows from ~1 KB to ~200–300 KB. Commit new `.syso`.
  - **Task 4** — update `build.ps1`: add `-ico docs/images/penelope.ico` to both the comment (line ~14) and the conditional fallback invocation (line ~21) of `rsrc`.
  - No Go source changes. `go build ./...` and `.\build.ps1` workflows unchanged.
  - **Option A (quick alternative):** `fyne package -os windows -name Vepeen -appID com.vepeen.app -icon docs/images/penelope.png -release` handles everything automatically but outputs `Vepeen.exe` to CWD instead of `bin/`.
  - **Agent:** Frontend Developer → Debugger/Reviewer

- [2026-07-23] **app-icon** — Embed `docs/images/penelope.jpg` as the Fyne app icon and system tray icon.
  - **Step 1 — Convert JPEG → PNG** (Fyne's `bundle` tool requires PNG):
    `ffmpeg -i docs/images/penelope.jpg docs/images/penelope.png`
  - **Step 2 — Install fyne CLI** (once, if not present in `%GOPATH%\bin`):
    `go install fyne.io/fyne/v2/cmd/fyne@v2.8.0`
  - **Step 3 — Generate bundle** from repo root:
    `fyne bundle -name PenelopeIcon -package ui -o internal/ui/bundle.go docs/images/penelope.png`
    Produces `internal/ui/bundle.go` — package `ui`, `var PenelopeIcon *fyne.StaticResource`.
  - **Step 4 — `FyneApp.toml`** — add `Icon = "docs/images/penelope.png"` under `[Details]` so `fyne package` picks it up.
  - **Step 5 — `cmd/vepeen/main.go`** — call `a.SetIcon(ui.PenelopeIcon)` immediately after `app.NewWithID`.
  - `internal/ui/tray_windows.go` — **no change needed**; existing `a.Icon()` fallback logic already propagates the icon once `SetIcon` is called.

- [2026-07-23] **quit-disconnect** Minor: auto-disconnect VPN on tray Quit.
  - `internal/ui/main_window.go` — `NewMainWindow` now returns `(fyne.Window, func())`. The second value is a `disconnectAndQuit` closure that: checks the controller's current status, fires `mgr.DisconnectFull` with a 5 s timeout if connected (best-effort, does not block beyond the timeout), then calls `a.Quit()`. No new imports.
  - `internal/ui/tray_windows.go` — `SetupTray` signature changed to `SetupTray(a fyne.App, w fyne.Window, onQuit func())`. The "Quit" menu item calls `onQuit()` instead of `a.Quit()`.
  - `cmd/vepeen/main.go` — call site updated: `w, onQuit := ui.NewMainWindow(a)` → `ui.SetupTray(a, w, onQuit)`.
  - No UI updates during quit path; window may already be hidden to tray.

- [2026-07-23] **i18n-en** — Translated ALL Indonesian-language string literals and comments to English across the entire codebase. Pure text/localization change — zero logic changes. Files updated:
  - `internal/route/parse.go` — 6 error/format strings (ParseError messages, ResolveRoutes errors)
  - `internal/route/parse_test.go` — assertion strings updated to match new English messages
  - `internal/route/sync_other.go` — stub error message
  - `internal/route/sync_windows.go` — 6 error strings (connection name, route operations)
  - `internal/route/sync_windows_test.go` — test case names and `t.Error` assertion strings (Indonesian OS locale test data strings preserved with comments)
  - `internal/ui/ping_other.go` — stub error message
  - `internal/ui/tray_windows.go` — "Tampilkan"/"Keluar" menu item labels → "Show"/"Quit"
  - `internal/ui/main_window.go` — all UI labels, button text, placeholder text, card titles, status/log messages (~40 strings)
  - `internal/vpn/errors_test.go` — assertion strings for MapExecError results
  - `internal/vpn/manager.go` — UserError primary/detail strings, phase detail strings, comments
  - `internal/vpn/shared/errors.go` — all UserError primary/detail strings in MapExecError, ValidateName
  - `internal/vpn/stub_other.go` — stub UserError strings
  - `internal/vpn/win/dial_windows.go` — UserError validation message; OS-locale matching strings kept as-is
  - `internal/vpn/win/dial_windows_test.go` — test case names translated; Indonesian rasdial OS-locale text values preserved with explanatory comments
  - `internal/vpn/win/natt_windows.go` — log messages and UserError strings (3 occurrences)
  - `internal/vpn/win/profile_windows.go` — embedded PowerShell sentinel string

- [2026-07-23] Translated Indonesian string literals to English in `internal/ui/ping_windows.go`: `"tidak terhubung"` → `"not connected"`, `"timeout / tidak ada balasan"` → `"timeout / no reply"`. No logic changes.

- [2026-07-23] **prd-vpn-win-package** Medium: refactor — group all Windows-only VPN code into a new `internal/vpn/win` package (`package win`); `internal/vpn` becomes a thin platform-neutral facade that re-exports the OS-specific symbols (delegating to `win` on Windows, `stub_other.go` elsewhere). Moves 9 files: `status_windows.go`, `connections_windows.go`, `dial_windows.go`, `disconnectall_windows.go`, `natt_windows.go`, `netapi_windows.go`, `powershell_windows.go`, `profile_windows.go`, `traffic_windows.go` (+ their 2 tests). `Manager.NewManager()` keeps referencing `vpn.`-level names (no `win` import, no build-tag change). `internal/ui` and `cmd/vepeen` require zero edits. No behavior/string changes. `psQuote`/`runPowerShell` dedup deferred to follow-up. (`docs/planning/prd-vpn-win-package.md`)

### Fixed

- [2026-07-23] **fix-013** Medium: startup window flicker/blink ("kedipan apps") — `docs/planning/fix-013-startup-flicker.md`
  - **Rank 1 (primary):** deferred `centerOnWorkArea` goroutine (`internal/ui/window_pos_windows.go:59-76`) shows window at 0,0 then teleports to center → blink. Fix (Frontend): replaced with exported `ShowCentered(w fyne.Window)` that calls `w.Show()` then positions synchronously via `driver.NativeWindow.RunNative` (HWND valid after `Show()`, before `a.Run()`); drops `SWP_NOACTIVATE` (uses `SWP_NOSIZE|SWP_NOZORDER|SWP_SHOWWINDOW`); hides→positions→shows inside the same callback to avoid any 0,0 frame; keeps `SPI_GETWORKAREA` work-area centering; falls back to `CenterOnScreen()`. `main.go` now calls `ui.ShowCentered(w)` + `a.Run()` instead of `w.ShowAndRun()`. Non-Windows stub `window_pos_other.go` provides `ShowCentered` (`CenterOnScreen()` + `Show()`).
  - **Rank 2 (secondary):** GLFW/OpenGL white first-frame flash. Fix (Frontend): apply `WS_EX_COMPOSITED` (GWL_EXSTYLE=-20) and `WS_CLIPCHILDREN` (GWL_STYLE=-16) at the show-time hook; add Windows app manifest `vepeen.exe.manifest` (Per-Monitor v2 DPI via `dpiAwareness`, `uiAccess=false`) compiled to committed `cmd/vepeen/rsrc.syso` via `rsrc`; `build.ps1` regenerates `.syso` if missing. Verified: `bin/vepeen.exe` subsystem=2 (GUI, no console) and manifest strings embedded.
  - **Rank 3 (secondary):** staged Resize→SetContent→center resolved naturally by show-time positioning (reads actual rect).
  - **Rank 4 (minor, optional):** move synchronous `config.LoadStored()` in `loadInitial()` (`main_window.go:155-220`) into a `fyne.Do` goroutine to paint sooner.
  - Constraints: no console window (`-H windowsgui` preserved); work-area centering kept; theme/app ID/`FyneApp.toml`/`fyne.Do`/Indonesian labels untouched; non-Windows stubs compile; no blank-window regression.
  - Pipeline: Frontend Developer → Debugger/Reviewer → Documentation

### Changed

- [2026-07-23] **fix-012** Medium: replace PowerShell/exec in periodic tickers with Win32 syscalls (`docs/planning/fix-012-win32-ticker-perf.md`)
  - Eliminates ~4.5 subprocess spawns/second while connected (4× powershell.exe + 1× ping.exe)
  - New `internal/vpn/netapi_windows.go`: `iphlpapi.dll` procs (`GetAdaptersAddresses`, `GetIfEntry2`, `GetExtendedTcpTable`, `IcmpCreateFile`/`IcmpSendEcho`/`IcmpCloseHandle`), struct defs, shared `resolveVPNInterfaceIndex` helper
  - Rewrite `traffic_windows.go` (`TrafficCounters`) → `GetIfEntry2` for `InOctets`/`OutOctets`; delete `parseTrafficStats`/`extractNumber`
  - Rewrite `connections_windows.go` (`ActiveConnections`) → `GetExtendedTcpTable` filtered by VPN unicast IPs; `reverseLookup` preserved
  - Rewrite `ping_windows.go` (`pingGateway`) → `IcmpSendEcho` with 1000ms timeout; same Indonesian status strings
  - No signature changes, no new deps, stubs untouched
  - Pipeline: Backend Developer → Debugger/Reviewer

### Added

- [2026-07-23] **prd-004** Medium: single encrypted config file `vepeen.bin` — replace the two-store setup (`config.json` + Windows Credential Manager) with one DPAPI user-scoped encrypted blob next to the executable. New `Stored`/`CredEntry` structs in `internal/config/config.go` (settings + per-profile `username`/`password`/`psk`); new `internal/config/dpapi_windows.go` (`crypt32.dll` `CryptProtectData`/`CryptUnprotectData`, `CRYPTPROTECT_UI_FORBIDDEN`) + `dpapi_other.go` stub; `Load()`/`Save()` encrypt/decrypt `vepeen.bin` atomically; one-time migration merges legacy `config.json` (both locations) + CredMan entries into `vepeen.bin` then purges old sources (idempotent, safe fallback). `internal/secrets` becomes migration-only read-only. `internal/ui/main_window.go` credential flows (`loadCredentials`/`persistCredentials`/`onConnect`/`persistQuiet`/`onSave`) read/write `Stored.Credentials` instead of `secrets.Store`. Non-goal: PSK capture UI is a follow-up (field stored, not yet collected). (`docs/planning/prd-004-encrypted-config.md`)

- [2026-07-23] **prd-003** Medium: global routes — replace per-profile `profiles` map with a single top-level `routes []string` on `Config` (`internal/config/config.go`); delete `ProfileEntry`; `applyConfig`/`onSave`/`persistQuiet` in `internal/ui/main_window.go` read/write global `cfg.Routes`; `onProfileChanged` no longer re-populates routes; `parseConfig` migrates old `profiles` shape (selectedProfile entry, else union of all) into global `Routes`; new `internal/config/config_test.go`. Behavior change: `git-rbi.xxx.xxx` route now applies to all selected profiles, not just `XXX - xxx.xxx.xxx.xxx`. (`docs/planning/prd-003-global-routes.md`)

### Fixed

- [2026-07-23] **fix-011** High: optional credentials pass-through to rasdial (error 734) — `ConnectRequest` regains optional `Username`/`Password`; `ConnectFull` dial step passes them to `ConnectParams` (blank → `rasdial <name>` CredMan fallback, filled → `rasdial <name> <user> <pass>`); `MapExecError` gains a `734` → code `ppp` ("Gagal negosiasi PPP") case before `auth`; UI adds optional `userEntry`/`passEntry` (clearly marked optional, enabled with form, never persisted to config.json). (`docs/planning/fix-011-optional-credentials.md`)

### Added

- [2026-07-23] **fix-010** High: thorough VPN connection-failure fixes — NAT-T registry auto-set (elevated), localized rasdial success detection (exit-code primary), split-tunnel interface-alias resolution, best-effort route sync (no abort), specific 789/800/809 Indonesian messages; empty-routes gate kept (`docs/planning/fix-010-connection-failures.md`)
  - **F1 (High):** NAT-T registry `AssumeUDPEncapsulationContextOnSendRule=2` never set/checked → 789/809 behind NAT. Fix (Backend, new `internal/vpn/natt_windows.go` + `stub_other.go` + `manager.go` `PhaseNATCheck`): `EnsureNATRegistry()` reads/sets `HKLM\SYSTEM\CurrentControlSet\Services\PolicyAgent`; non-admin → `nat` UserError (admin instruction); admin set → warn reboot may be needed. `golang.org/x/sys/windows/registry` already a dep.
  - **F2 (High):** `Connect` (`dial_windows.go:36-46`) keys success on English substrings only → false failure on localized Windows. Fix: exit code 0 = success primary; text scan (EN+ID markers) fallback only; extract `evaluateRasdialResult`.
  - **F3 (High):** `EnforceSplitTunnel` (`profile_windows.go:72`) assumes VPN adapter alias == profile name → server-pushed `0.0.0.0/0` not removed when it differs. Fix: resolve alias/index via `Get-VpnConnection` before `Get/Remove-NetRoute`.
  - **F4 (Medium):** `ConnectFull` (`manager.go:107-114`) aborts connect on `SyncRoutes` error. Fix: make route sync best-effort (log + warning, continue); add `syncRoutesFn` seam; extend return to `([]string warnings, error)`; UI `onConnect` shows warnings.
  - **F5 (Medium):** `MapExecError` (`errors.go:60`) maps 789/800/809 to generic network message. Fix: add specific Indonesian `ipsec` case before `network`.
  - **F6 (Confirm):** empty-routes gate (`manager.go:84-90`) kept (split-tunnel needs ≥1 route); message clarified.
  - Constraints: secrets never logged; `-H windowsgui` → diagnostics to `%AppData%\vepeen\vepeen.log` + UI log; non-Windows stubs compile; split-tunnel behavior preserved; no auto-elevation.
  - Pipeline: Backend Developer → Frontend Developer → Debugger/Reviewer → Security → Documentation

- [2026-07-22] **fix-009** Medium + Medium: auto-disconnect other VPNs before connect + cancel connecting (`docs/planning/fix-009-auto-disconnect-cancel.md`)
  - **F1 (Medium):** Multiple simultaneous tunnels cause routing conflicts. Root cause: `internal/vpn` has `Connect`/`Disconnect(name)` but no "list all" / "disconnect all except" helper. Fix (Backend, `internal/vpn/disconnectall_windows.go` new `//go:build windows` + `stub_other.go` stub + `manager.go`): add `DisconnectAllExcept(exceptName string) ([]string, error)` — runs `Get-VpnConnection` (no `-Name`), parses `Name`+`ConnectionStatus`, `rasdial <Name> /DISCONNECT` for any `Connected` and `!= exceptName` (best-effort, errors collected, not fatal), returns disconnected names; call it at START of `ConnectFull` via new `PhaseDisconnectOthers` (`"disconnect_others"` → `"Memutuskan VPN lain…"`).
  - **F2 (Medium):** No way to abort an in-progress connect. Root cause: `ConnectFull` has no `context.Context`; `onConnect` stores no cancel handle. Fix (Backend + Frontend, `manager.go` + `errors.go` + `main_window.go`): `ConnectFull(ctx, req, progress)` checks `ctx.Err()` after each phase and returns new `UserError` code `"canceled"` (`"Dibatalkan"`); add `ctx`/`cancelConnect` to controller; `onConnect` creates `context.WithCancel`, stores cancel, passes ctx; new `onCancel` calls cancel + logs `"Membatalkan…"`; completion detects `canceled` → `StatusDisconnected` + best-effort `DisconnectFull(name)` + log `"Dibatalkan."`; add `btnCancel` ("Batal") in button row, enabled only while `StatusConnecting`.
  - Constraints: route parsing/secrets/config/`EnforceSplitTunnel` untouched; app ID `com.vepeen.app`, `FyneApp.toml`, `fyne.Do`, Indonesian labels preserved; non-Windows stubs compile.
  - Pipeline: Backend Developer → Frontend Developer → Documentation → Debugger/Reviewer → Security

### Fixed / Polish

- [2026-07-22] **fix-008** Low: UI polish — unbalanced landscape layout + no accent color (`docs/planning/fix-008-ui-polish.md`)
  - Root causes: `build()` uses top-aligned `NewHBox`/`NewVBox` with fixed `vGap`/`hGap`; Log card `SetMinRowsVisible(5)` does not expand → ~150px dead space at bottom; columns unequal height; stock `theme.DarkTheme()` (no accent); bare header.
  - Fix (Frontend Developer, `internal/ui/main_window.go` + new `internal/ui/theme.go` + `cmd/vepeen/main.go` + `README.md`): custom `fyne.Theme` overriding `ColorNamePrimary` to teal `#0FB5AE` (or indigo `#4F46E5`), applied via `a.Settings().SetTheme`; rework body to `container.NewGridWithColumns(2)` with each column `container.NewBorder(top=upperCard, nil, nil, nil, lowerCard)` so Rute (left) and Log (right) expand to fill full height (no dead space, equal columns); padded header (bold title + subtitle + separator); keep `Resize(960,600)`/`CenterOnScreen`/`minSizeWrap` floor `fyne.NewSize(900,560)`; keep bottom button row (Simpan / Spacer / Putuskan + Hubungkan HighImportance).
  - Constraints: no handler/threading/data changes; `fyne.Do`, app ID `com.vepeen.app`, `FyneApp.toml`, dark base, Indonesian labels, split-tunnel logic untouched; no blank-window regression.
  - Pipeline: Frontend Developer → Debugger/Reviewer → Documentation
- [2026-07-22] **fix-007** High + Medium: split tunnel not enforced (all traffic via VPN) + landscape UI (`docs/planning/fix-007-split-tunnel-landscape.md`)
  - **A (High):** Split tunnel silently ineffective. Root causes: (1) `EnsureProfile` (`internal/vpn/profile_windows.go`) sets `-SplitTunneling $true` but if already connected, `rasdial` reports "already connected" and the stale full-tunnel routing persists; (2) some L2TP servers push a `0.0.0.0/0` route on the VPN interface that `SplitTunneling` doesn't always suppress. Fix: disconnect-if-connected at start of `EnsureProfile`; add `EnforceSplitTunnel(name)` (post-connect removal of any `0.0.0.0/0` on the VPN interface, best-effort, never fails connect); add `ProfileDiagnostics(name)`; `ConnectFull` calls `EnforceSplitTunnel` after `Connect`; UI logs split-tunnel diagnostics after connect. README documents disconnect/reconnect requirement + enforcement. `internal/route` parsing, secrets, config, `Add/Remove-VpnConnectionRoute`, app ID, `FyneApp.toml`, `fyne.Do` untouched.
  - **B (Medium):** Portrait window scrolls. Fix: landscape two-column layout via Designer handoff — `Resize(960,600)` (or `900×620`), `minSizeWrap` floor raised to `fyne.NewSize(900,560)`, left column = Koneksi + Rute, right column = Status + Log, bottom button row preserved. Preserve `fyne.Do`, `minSizeWrap`, app ID, `FyneApp.toml`, Indonesian labels, all handlers.
  - Pipeline: A → Backend Developer → Debugger/Reviewer → Documentation; B → Designer (UI/UX) → Frontend Developer → Debugger/Reviewer → Documentation
- [2026-07-22] **fix-006** Critical + Medium: route sync bug (blocks connect) + ugly UI redesign (`docs/planning/fix-006-route-and-ui.md`)
  - **A (Critical):** `internal/route/sync_windows.go` `listRoutes` used non-existent `Get-VpnConnectionRoute` cmdlet → connect aborts. Fix: read routes via `Get-VpnConnection -Name X` + `.Routes[].DestinationPrefix`; broaden `isSoftRouteListError` to also treat "not recognized"/"tidak dikenali" as soft. README troubleshooting (line 236) no longer references `Get-VpnConnectionRoute`. `addRoute`/`removeRoute` and `internal/vpn` untouched.
  - **B (Medium):** UI visual redesign — group controls into `Card`s (Koneksi / Rute split tunnel / Status / Log), consistent theme variant + optional brand color, prominent `Hubungkan`, clean `Simpan`/`Putuskan` row, comfortable spacing, window ~480–520 scrollable. Designer (UI/UX) produces layout spec → Frontend Developer implements; preserve `fyne.Do`, `minSizeWrap`, app ID, `FyneApp.toml`, Indonesian labels, all handlers.
  - Pipeline: A → Backend Developer → Debugger/Reviewer; B → Designer → Frontend Developer → Debugger/Reviewer → Documentation
- [2026-07-22] **fix-005** Critical/Medium: blank window (form not rendered) + console window on `.exe` launch (`docs/planning/fix-005-blank-window-console.md`)
  - Bug 1 root cause: `minSizeWrap` embeds raw `fyne.CanvasObject` (no `CreateRenderer()`) → canvas skips render → blank window; fix-002 was static-only
  - Fix (Backend, `internal/ui/main_window.go`): convert `minSizeWrap` to `fyne.Widget` (embed `widget.BaseWidget`, `inner` field, `CreateRenderer()`, `ExtendBaseWidget`); `NewMainWindow` uses `newMinSizeWrap(...)`
  - Bug 2 root cause: Go Windows linker defaults to console subsystem
  - Fix: `build.ps1` (`CGO_ENABLED=1 go build -ldflags="-H windowsgui"`); `cmd/vepeen/main.go` panic/startup file logger → `%AppData%\vepeen\vepeen.log`; README documents `.\build.ps1` + flag + log
  - Out of scope: VPN/route/secrets/config logic, app ID, `FyneApp.toml`, `fyne.Do`/threading
  - Pipeline: Backend Developer → Debugger/Reviewer → Documentation

### Changed

- [2026-07-22] **ui-003** Simplify main UI + activity log (`docs/planning/ui-003-simplified-form-log.md`)
  - Primary fields: IP (server), Key (PSK), Username, Password, Connect
  - Add read-only multi-line log (timestamped phases/errors; never secrets)
  - Keep routes as secondary required field (split tunnel); Disconnect secondary; name default `Vepeen`
  - Pipeline: Designer → Backend Developer → Debugger/Reviewer → Security → Documentation
  - Touch: `internal/ui/main_window.go` (+ optional log helper), `README.md` UI section
  - Non-goals: full tunnel, VPN stack rewrite, disk debug logs
  - **Design (v1.1.0):** Ready for Backend — labels IP/Key(PSK)/Username/Password; Hubungkan primary; Simpan+Putuskan secondary; name hidden (`Vepeen`); log min 8 rows + Bersihkan log; window 480×720 / min 420×600

### Fixed / Polish

- [2026-07-22] **fix-004** Medium: Fyne `fyne.Do` threading migration warning (`docs/planning/fix-004-fyne-do-migration.md`)
  - Root cause: missing repo-root `FyneApp.toml` `[Migrations] fyneDo = true` (UI already uses `fyne.Do`)
  - Backend: create `FyneApp.toml` Details Name=`Vepeen` ID=`com.vepeen.app` + `fyneDo = true`; optional README note
  - No `main_window.go` changes unless post-opt-in smoke fails
  - Pipeline: Backend Developer → Debugger/Reviewer (user run: no migration warning)
- [2026-07-22] **fix-003** Low: gopls go-gl false error + IDE noise (`docs/planning/fix-003-gopls-cgo-noise.md`)
  - Real Windows build OK with `CGO_ENABLED=1`; gopls noise from darwin/`CGO=0` analysis
  - Backend: `.vscode/settings.json` `go.toolsEnvVars` (`CGO_ENABLED=1`, `GOOS=windows`); README Troubleshooting row; remove `cmd/diag_layout`
  - Out of scope: VPN, form layout (fix-002), go-gl version pin
  - Pipeline: Backend Developer → Debugger/Reviewer
- [2026-07-22] **fix-002** Critical: form inputs not visible (`docs/planning/fix-002-form-visibility.md`)
  - Root cause: footer VBox(actions+status+log@8) MinSize ~376px reserved first by Fyne Border; form VScroll center collapses
  - Fix (Backend only, `build()` in `internal/ui/main_window.go`): compact footer (actions+status); nested Border center (form VScroll MinSize h≈200 top, log center); log min rows 4–6
  - Out of scope: VPN, secrets, Designer/label redesign
  - Pipeline: Backend Developer → Debugger/Reviewer
- [2026-07-22] **fix-001** PRD-002 post-review polish plan (`docs/planning/fix-001-prd002-polish.md`)
  - Major: already-connected → `StatusConnected` + Disconnect; CredWrite `runtime.KeepAlive`; Save/CredMan off UI thread
  - Should: purge orphan `%TEMP%\vepeen\vpn-*.ps1`; CredMan `CRED_PERSIST_LOCAL_MACHINE`; validation focus + trim policy; `SetMinSize(420,560)`; direct `golang.org/x/sys` in go.mod
  - Agent: Backend Developer → Debugger/Reviewer → Security spot-check
  - Out of scope: RAS rewrite, full redaction overhaul, CVE bumps

### Added

- [2026-07-22] PRD-002 **implemented** (Backend Developer): L2TP/IPsec split-tunnel client on disk
  - Packages: `internal/config`, `internal/secrets` (CredMan), `internal/route`, `internal/vpn`, `internal/ui` form
  - Pipeline: EnsureProfile → SyncRoutes → rasdial; status states; secrets never in `config.json`
  - Tests: `go test ./internal/route` + `./internal/vpn` (error mapping); `go build ./cmd/vepeen` OK
  - README updated for dual-auth, config path, privileges, manual test checklist
- [2026-07-22] PRD-002: L2TP/IPsec VPN client with PSK + selective IP/CIDR routing (split tunnel) (`docs/planning/prd-002-l2tp-split-tunnel.md`)
  - Windows built-in VPN: PowerShell `VpnClient` + `rasdial`; per-user profile; `Add-VpnConnectionRoute`
  - Packages: `internal/vpn`, `internal/route`, `internal/config`, `internal/secrets`; replace demo `internal/ui`
  - UI: server, connection name (default `Vepeen`), PSK, username/password, multi-line CIDRs, connect/disconnect/status (Indonesian labels)
  - Secrets: no plain JSON for PSK/password; CredMan/DPAPI preferred
  - Pipeline: Designer → Backend Developer (all Go/Fyne) → Documentation → Debugger/Reviewer → Security → Documentation
  - Research: `docs/research/windows-l2tp-ipsec-split-tunnel.md`
  - Non-goals: full tunnel, other VPN types, Linux/macOS, SoftEther embed, DevOps
- [2026-07-22] PRD-001: Golang + Fyne desktop starter for greenfield project `vepeen` (`docs/planning/prd-001-golang-fyne-starter.md`)
  - Module `vepeen`, Fyne v2, `cmd/vepeen` + `internal/ui` layout
  - Minimal interactive main window (label + button)
  - Windows README prerequisites (Go, CGo/C compiler)
  - Implementation pipeline: Backend Developer → Debugger/Reviewer → Documentation
  - Explicit non-goals: CI/Docker/DevOps, product features beyond starter UI
