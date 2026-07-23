## [Unreleased]

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
