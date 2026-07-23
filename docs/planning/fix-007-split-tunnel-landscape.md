# Fix Plan: Split tunnel enforcement + Landscape UI

**Related PRD / UI:** PRD-002 (`docs/planning/prd-002-l2tp-split-tunnel.md`), fix-006 (`docs/planning/fix-006-route-and-ui.md`), ui-003 (`docs/planning/ui-003-simplified-form-log.md`)
**Severity:** Issue A = High (split tunnel silently ineffective → full tunnel); Issue B = Medium (UX/ergonomics)
**Reported by:** Planner (code review of `internal/vpn/profile_windows.go`, `internal/vpn/manager.go`, `internal/ui/main_window.go`)
**Date:** 2026-07-22
**Status:** Ready for implementation
**Version:** v1.0.0

---

## Issue A — Split tunnel not working (all traffic routes through VPN)

### Bug summary

User adds route `10.0.7.0/24`, but **all** traffic goes through the VPN instead of just that subnet. The profile is created with `-SplitTunneling` (correct), and `internal/route/sync_windows.go` only ever adds the user's specific prefixes via `Add-VpnConnectionRoute` (correct — it never adds a default route). The full-tunnel behavior comes from two gaps in the connect flow.

### Root cause analysis

1. **Stale connection not reset before profile update.** `EnsureProfile` (`internal/vpn/profile_windows.go`) calls `Set-VpnConnection -SplitTunneling $true`, but `Set-VpnConnection -SplitTunneling` only takes effect on the **next** connect. The manager's `ConnectFull` (`internal/vpn/manager.go`) runs `EnsureProfile → SyncRoutes → Connect(rasdial)`. If the VPN is **already connected** when `EnsureProfile` runs, `rasdial` reports "already connected" and the stale full-tunnel routing persists — the new `SplitTunneling` flag and routes never get applied. (The existing recreation branch does disconnect when tunnel type drifted, but the normal `Set` path does **not** disconnect.)
2. **Server-pushed default route not suppressed.** Some L2TP servers push a default route `0.0.0.0/0` onto the VPN interface. `SplitTunneling` should suppress it, but on some Windows builds the `0.0.0.0/0` route still appears on the VPN interface, causing full tunneling. Nothing in the connect flow removes it after dial.

`internal/route/sync_windows.go` is **not** the source — it only adds/removes the user's prefixes and never touches a default route.

### Fix strategy

**Option A: Disconnect-before-update + post-connect default-route enforcement + diagnostics (recommended)**

- **Files:** `internal/vpn/profile_windows.go`, `internal/vpn/manager.go`, `internal/ui/main_window.go`, `README.md`
- **Risk:** Low–Medium — PowerShell-only changes; no route parsing, secrets, config, app ID, `FyneApp.toml`, or `fyne.Do` changes; `Add/Remove-VpnConnectionRoute` usage preserved.
- **Effort:** M

**Option B: Document-only (tell user to disconnect first)**

- Does not fix the already-connected case or the server-pushed default route. Not recommended.

**Recommended:** Option A

### File scope

- `internal/vpn/profile_windows.go` — `EnsureProfile` (disconnect if connected at start); new `EnforceSplitTunnel(name)`; new `ProfileDiagnostics(name)`.
- `internal/vpn/manager.go` — `ConnectFull` calls `EnforceSplitTunnel` after `Connect` succeeds.
- `internal/ui/main_window.go` — after a successful `ConnectFull`, call `ProfileDiagnostics` and `appendLog` the result.
- `README.md` — Split tunnel behavior + Troubleshooting rows.
- **Out of scope (per constraints):** route parsing (`internal/route/parse*.go`), `internal/secrets`, `internal/config`, `Add/Remove-VpnConnectionRoute`, app ID, `FyneApp.toml`, `fyne.Do` threading.

### Implementation tasks

**Agents:** Backend Developer (A.1–A.4) → Debugger/Reviewer → Documentation
**Parallelizable:** A.1/A.2/A.3 can be authored together in `profile_windows.go`; A.4 (manager) depends on A.2; A.5 (UI) depends on A.3; A.6 (README) independent.

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| A.1 | Backend Developer | `internal/vpn/profile_windows.go` | In `EnsureProfile`, before the `Get-VpnConnection`/`if` block, add a disconnect-if-connected guard so `Set-VpnConnection -SplitTunneling $true` takes effect on the next connect. Sketch: `$conn = Get-VpnConnection -Name $name -ErrorAction SilentlyContinue; if ($null -ne $conn -and $conn.ConnectionStatus -eq 'Connected') { rasdial $name /DISCONNECT \| Out-Null }`. Keep the existing recreation branch (tunnel-type drift) logic intact. |
| A.2 | Backend Developer | `internal/vpn/profile_windows.go` | Add `EnforceSplitTunnel(name string) error` that runs a PowerShell script finding the VPN interface by `InterfaceAlias` matching the connection name and removing any `0.0.0.0/0` route on it (SplitTunneling is always on). Wrap in `try/catch`; return `nil` on any error (never fail the connect). Sketch: `$name='<conn>'; try { $r = Get-NetRoute -InterfaceAlias $name -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue; if ($r) { Remove-NetRoute -InterfaceAlias $name -DestinationPrefix '0.0.0.0/0' -Confirm:$false -ErrorAction SilentlyContinue; 'removed-default' } else { 'no-default' } } catch { 'skip' }`. Use `psQuote(name)`; do not log the command if it contained secrets (it does not). |
| A.3 | Backend Developer | `internal/vpn/profile_windows.go` | Add `ProfileDiagnostics(name string) (split bool, routes []string, err error)` returning `SplitTunneling` and the profile's `.Routes[].DestinationPrefix` via `Get-VpnConnection -Name $name`. Used by the UI to log verification info. |
| A.4 | Backend Developer | `internal/vpn/manager.go` | In `ConnectFull`, after `Connect(...)` returns `nil` and before `notify(PhaseDone)`, call `EnforceSplitTunnel(name)` and ignore its error (best-effort). Optionally add a `PhaseEnforceSplit` constant + `PhaseDetail` entry ("Menegakkan split tunnel…") and `notify` it around the call for UI progress. |
| A.5 | Backend Developer | `internal/ui/main_window.go` | In `onConnect`'s goroutine, after `c.mgr.ConnectFull` returns `nil`, call `vpn.ProfileDiagnostics(name)` and, inside `fyne.Do`, `appendLog` a line such as `Split tunnel: <on/off>; rute: <comma list or "(kosong)">`. Never log secrets. Keep all existing `fyne.Do`/handlers intact. |
| A.6 | Documentation | `README.md` | Update "Split tunnel behavior" (lines ~27–31) to state the app now disconnects-then-reconnects to apply `SplitTunneling` and enforces removal of any server-pushed `0.0.0.0/0` on the VPN interface. Add a Troubleshooting row: "All traffic goes through VPN / split tunnel not applied" → ensure not already connected (app auto-disconnects before re-applying) and that the app removes a pushed default route; verify via the Log line after connect. |

### Acceptance criteria

- [ ] `EnsureProfile` disconnects the connection first when it is currently `Connected`, so `Set-VpnConnection -SplitTunneling $true` applies on the subsequent connect.
- [ ] `EnforceSplitTunnel` removes a `0.0.0.0/0` route on the VPN interface when present; returns `nil` even on error (connect never fails because of it).
- [ ] `ConnectFull` invokes `EnforceSplitTunnel` after a successful `Connect`.
- [ ] After a successful connect, the app Log shows a split-tunnel diagnostics line (SplitTunneling on/off + route list).
- [ ] `go build ./cmd/vepeen` and `go vet ./internal/vpn ./cmd/vepeen` succeed.
- [ ] README documents the disconnect/reconnect requirement and the new enforcement; Troubleshooting has the "all traffic through VPN" row.
- [ ] `Add/Remove-VpnConnectionRoute` usage, route parsing, secrets, config, app ID, `FyneApp.toml`, and `fyne.Do` are unchanged.

### Regression risk

- Disconnecting an already-connected VPN at `EnsureProfile` start means a connect from a connected state will briefly drop and re-establish — expected and necessary for `SplitTunneling` to apply.
- `EnforceSplitTunnel` touches live routes; guarded by `try/catch` and `ErrorAction SilentlyContinue` so it cannot break connect.
- `ProfileDiagnostics` only reads; no mutation.

---

## Issue B — Landscape UI (no scroll)

### Bug summary

The window is portrait (`Resize(480, 720)`) and content is a single vertical `VScroll` column of four cards, requiring vertical scrolling on typical screens. The user wants a **landscape** window, tall/wide enough that scrolling is unnecessary for the four cards (Koneksi VPN, Rute Split Tunnel, Status, Log).

### Root cause analysis

- `NewMainWindow` (`internal/ui/main_window.go`) sets `w.Resize(fyne.NewSize(480, 720))` and wraps a single `container.NewVScroll` of a `VBox` of all four cards. With four stacked cards this exceeds typical viewport height → vertical scroll.
- `minSizeWrap` floor is `fyne.NewSize(420, 600)` — too narrow/short for a landscape two-column layout.

### Fix strategy

**Option A: Two-column landscape layout via Designer handoff (recommended)**

- **Files:** `internal/ui/main_window.go` (`NewMainWindow` + `build()`), `README.md` (UI note, optional)
- **Risk:** Low–Medium — layout-only; must preserve `fyne.Do` threading, `minSizeWrap` widget, app ID, `FyneApp.toml`, all handlers/controllers, Indonesian labels, and all controls.
- **Effort:** M

**Option B: Just widen the portrait window**

- Does not remove vertical scroll for four cards. Not recommended.

**Recommended:** Option A — route through the Designer (UI/UX) agent for a concrete landscape layout spec, then Frontend Developer implements.

### Handoff to Designer (UI/UX)

The Planner defers the concrete landscape spec to the **Designer (UI/UX) agent**, who must produce a layout spec covering:

- **Window:** `w.Resize(fyne.NewSize(960, 600))` (or `900×620`); keep `w.CenterOnScreen()`.
- **Layout:** two columns via `container.NewGridWithColumns(2)` (or two `VBox` columns inside an `HBox`):
  - **Left column:** Koneksi VPN card + Rute Split Tunnel card.
  - **Right column:** Status card + Log card.
- **Spacing:** keep `vGap`/`hGap` helpers and comfortable padding between cards and columns; preserve dark theme.
- **Button row:** keep at bottom (`Simpan` + spacer + `Putuskan` + `Hubungkan` with `HighImportance`); `Hubungkan` prominent.
- **minSizeWrap floor:** raise to `fyne.NewSize(900, 560)` so the window cannot shrink below the two-column content.
- **Preserved:** all four `Card`s, all entries/labels/handlers, `fyne.Do`, `minSizeWrap` widget, app ID, `FyneApp.toml`, Indonesian labels.

### File scope

- `internal/ui/main_window.go` — `NewMainWindow` (Resize + `minSizeWrap` floor) and `build()` (two-column layout).
- `README.md` — UI section note (optional).
- **Preserved intact:** `fyne.Do` calls, `minSizeWrap` widget, app ID, `FyneApp.toml`, all controller fields/handlers (`onSave`, `onConnect`, `onDisconnect`, `onClearLog`, `appendLog`, `loadInitial`, `applyConfig`, `applyEnablement`, `setStatus`, `profileName`), Indonesian labels, same controls.

### Implementation tasks

**Agents:** Designer (UI/UX) → Frontend Developer → Debugger/Reviewer → Documentation
**Parallelizable:** No — sequential handoff (design spec before implementation)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| B.1 | Designer (UI/UX) | (spec doc) | Produce concrete landscape layout spec: window size `960×600` (or `900×620`), two-column grid (left: Koneksi + Rute; right: Status + Log), spacing/padding rules, bottom button row, `minSizeWrap` floor `900×560`. Reference fix-007 Issue B. |
| B.2 | Frontend Developer | `internal/ui/main_window.go` | In `NewMainWindow`, change `w.Resize(fyne.NewSize(480, 720))` → `fyne.NewSize(960, 600)` (or `900×620`); keep `CenterOnScreen()`; raise `newMinSizeWrap(..., fyne.NewSize(900, 560))`. |
| B.3 | Frontend Developer | `internal/ui/main_window.go` | Rebuild `build()` to a landscape two-column layout: left `VBox` (cardKoneksi + vGap + cardRute), right `VBox` (cardStatus + vGap + cardLog), combined via `container.NewGridWithColumns(2)` (or `HBox` of the two columns with `hGap`). Keep header (top) and button row (bottom) via `container.NewBorder`. Remove the single full-height `VScroll` of all four cards (content should fit without vertical scroll at the target size). |
| B.4 | Frontend Developer | `internal/ui/main_window.go` | Keep the bottom button row (`Simpan` + spacer + `Putuskan` + `Hubungkan` HighImportance) and all existing widgets/labels/handlers; preserve `fyne.Do` threading and `minSizeWrap`. |
| B.5 | Documentation | `README.md` | Update UI section to describe the landscape two-column layout (optional, only if wording changed). |

### Acceptance criteria

- [ ] Window opens landscape (~960×600 / 900×620), centered; no vertical scroll needed for the four cards on a typical screen.
- [ ] Two-column layout: left = Koneksi VPN + Rute Split Tunnel; right = Status + Log.
- [ ] `minSizeWrap` floor raised to ~`fyne.NewSize(900, 560)`; window cannot shrink below content.
- [ ] Bottom button row preserved with `Hubungkan` (HighImportance) prominent; `Simpan`/`Putuskan` present.
- [ ] All existing functionality preserved: Indonesian labels, same controls, `fyne.Do` threading, `minSizeWrap` widget, app ID, `FyneApp.toml` unchanged.
- [ ] `go build ./cmd/vepeen` succeeds; app launches with no blank window (fix-005 floor preserved) and no migration warning (fix-004).

### Regression risk

- Removing the full-height `VScroll` could reintroduce the form-collapse bug (fix-002/fix-005) if `minSizeWrap`/`Border` floor is dropped — must keep `minSizeWrap` and the raised floor.
- Two-column grid must not exceed the raised `minSizeWrap` floor on small screens; verify at `900×560`.

---

## Rollback strategy

- Issue A: revert `profile_windows.go` (`EnsureProfile` guard, `EnforceSplitTunnel`, `ProfileDiagnostics`), `manager.go` call, UI diagnostics call, and README lines via git. Route sync (`Add/Remove-VpnConnectionRoute`) untouched.
- Issue B: revert `main_window.go` `NewMainWindow` + `build()` via git; functionality unchanged so safe to revert.

## Version history

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-22 | Initial fix plan: A (split-tunnel enforcement: disconnect-before-update + post-connect default-route removal + diagnostics) + B (landscape two-column UI via Designer handoff) |
