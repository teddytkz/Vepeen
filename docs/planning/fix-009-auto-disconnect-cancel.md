# Fix Plan: Auto-disconnect other VPNs + Cancel connecting

**Related PRD:** PRD-002 (L2TP/IPsec split tunnel)
**Severity:** Medium (Feature 1 — robustness) + Medium (Feature 2 — UX control)
**Reported by:** Planner Agent
**Date:** 2026-07-22

## Bug / Feature Summary

Two related UX/robustness gaps in the connect flow:

- **Feature 1 — Auto-disconnect other VPNs:** When the user clicks `Hubungkan`, the app may leave *other* active Windows VPN connections up. Multiple simultaneous tunnels cause routing conflicts and ambiguous split-tunnel behavior. The app should first tear down every OTHER active VPN connection so only the target Vepeen tunnel is up.
- **Feature 2 — Cancel connecting:** Once `Hubungkan` is clicked, there is no way to abort the in-progress connect. `onConnect` launches a goroutine running `c.mgr.ConnectFull` with no cancellation mechanism. A `Cancel` button should appear/enabled while `StatusConnecting` and abort the connect (best-effort cleanup of a half-open tunnel).

## Root Cause Analysis

- `internal/vpn` exposes `Connect`, `Disconnect(name)`, `DisconnectFull(name)`, but **no "list all connections"** or **"disconnect all except"** helper. `Get-VpnConnection` (no `-Name`) lists all connections with `.Name` and `.ConnectionStatus`, but nothing in the package consumes it for bulk teardown.
- `ConnectFull(req, progress)` has no `context.Context`. The sequential phases (`EnsureProfile` → `SyncRoutes` → `Dial` → `SplitEnforce`) cannot be interrupted. `onConnect` stores no cancel handle, so the UI cannot signal abort.
- `applyEnablement` toggles `btnConn`/`btnDisc`/`btnSave` by state but has no `btnCancel` concept; the button row is `Simpan / Spacer / Putuskan + Hubungkan`.

## Fix Strategy

Both features are additive and isolated. Recommended: implement both together (single PRD-style fix plan) because Feature 2's `context.Context` plumbing is small and Feature 1's `DisconnectAllExcept` is called at the new `PhaseDisconnectOthers` phase inside `ConnectFull`.

**Recommended:** Full implementation of both features (Option A — single combined plan). Risk: Low/Med. Effort: M.

### Constraints (must hold)

- Do NOT change route parsing (`internal/route`), secrets, config, or split-tunnel enforcement (`EnforceSplitTunnel` stays called after dial).
- Keep app ID `com.vepeen.app`, `FyneApp.toml`, `fyne.Do` threading, Indonesian labels.
- Keep all existing handlers' behavior except adding cancel + auto-disconnect.
- Non-Windows stubs must compile (`//go:build !windows`).

---

## Implementation Tasks

### Phase 1: Auto-disconnect other VPNs (Feature 1)

**Depends on:** Nothing
**Parallelizable:** No (Feature 2 touches same `ConnectFull` signature + UI; do after or together)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/vpn/disconnectall_windows.go` (new, `//go:build windows`) | Add `DisconnectAllExcept(exceptName string) ([]string, error)`. Run `Get-VpnConnection` (no `-Name`) via `runPowerShell`; parse each connection's `Name` + `ConnectionStatus` (PowerShell `Select-Object Name,ConnectionStatus \| ConvertTo-Json` or line parse). For any `ConnectionStatus -eq 'Connected'` and `Name != exceptName`, call `rasdial <Name> /DISCONNECT` (reuse `Disconnect(name)`). Collect per-disconnect errors but do NOT abort; return the slice of names successfully disconnected (for logging). Best-effort: if `Get-VpnConnection` itself fails, return the error. |
| 1.2 | Backend Developer | `internal/vpn/stub_other.go` | Add non-Windows stub: `func DisconnectAllExcept(exceptName string) ([]string, error) { return nil, unsupported() }` so the package compiles off-Windows. |
| 1.3 | Backend Developer | `internal/vpn/manager.go` | Add `PhaseDisconnectOthers Phase = "disconnect_others"` to the `Phase` const block. Add `PhaseDisconnectOthers` case to `PhaseDetail` returning `"Memutuskan VPN lain…"`. In `ConnectFull`, at the very START (before `EnsureProfile`), `notify(PhaseDisconnectOthers)` then call `DisconnectAllExcept(name)`; log/ignore returned names (best-effort — errors collected inside, not fatal). Do NOT fail connect if teardown is partial. |

**Sub-Agent Guidance:**

- Task 1.1: parse robustly. Prefer `Get-VpnConnection \| Select-Object Name,ConnectionStatus \| ConvertTo-Json -Compress` and `encoding/json` unmarshal into a struct (handle both single-object and array JSON). Fall back to line scan only if JSON is awkward. Never log connection names that could be secret-bearing — names are profile names, safe to log at info level.
- Task 1.3: the `DisconnectAllExcept` call must come BEFORE `EnsureProfile` so the target profile is not torn down by the "except" filter (target is excluded by name anyway, but ordering keeps intent clear).

### Phase 2: Cancel connecting (Feature 2)

**Depends on:** Phase 1 (shares `ConnectFull` signature + `manager.go`)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Backend Developer | `internal/vpn/errors.go` | Add a new `UserError` code `"canceled"`: `newUserError("canceled", "Dibatalkan", "Penghubungan dibatalkan.")`. |
| 2.2 | Backend Developer | `internal/vpn/manager.go` | Change `ConnectFull` signature to `ConnectFull(ctx context.Context, req ConnectRequest, progress ProgressFunc) error`. After each phase boundary (`EnsureProfile`, `SyncRoutes`, before/after `Dial`, before `SplitEnforce`), check `if err := ctx.Err(); err != nil { return newUserError("canceled", "Dibatalkan", "Penghubungan dibatalkan.") }`. Keep all existing validation + phase order. |
| 2.3 | Backend Developer | `internal/ui/main_window.go` (controller struct) | Add fields `ctx context.Context` and `cancelConnect context.CancelFunc` to `controller`. |
| 2.4 | Frontend Developer | `internal/ui/main_window.go` (`build`) | Add `c.btnCancel = widget.NewButton("Batal", c.onCancel)`. Keep it in the button row: `Simpan` (left), `layout.NewSpacer()`, `container.NewHBox(c.btnDisc, c.btnCancel, c.btnConn)` (right). `btnCancel` is always in layout but disabled unless connecting. |
| 2.5 | Frontend Developer | `internal/ui/main_window.go` (`applyEnablement`) | When `c.state == vpn.StatusConnecting`, enable `btnCancel` and keep `btnConn` disabled; otherwise disable `btnCancel`. (Reuse existing `busyConnect` variable.) |
| 2.6 | Frontend Developer | `internal/ui/main_window.go` (`onConnect`) | Create `ctx, cancel := context.WithCancel(context.Background())`; store `c.cancelConnect = cancel`; pass `ctx` to `c.mgr.ConnectFull`. In the `fyne.Do` completion, detect `vpn.AsUserError(err)` with `Code == "canceled"` → set `StatusDisconnected`, best-effort `c.mgr.DisconnectFull(name)` to clean a half-open tunnel, log `"Dibatalkan."`, and clear `c.cancelConnect = nil`. |
| 2.7 | Frontend Developer | `internal/ui/main_window.go` (new `onCancel`) | Guard with `c.mu`. If `c.cancelConnect != nil`, call it; set `c.busy = true`; log `"Membatalkan…"`; set status detail to `"Membatalkan…"` (keep `StatusConnecting` so `btnCancel` stays visible until goroutine returns). The running `ConnectFull` returns the `canceled` error → handled in 2.6. |

**Sub-Agent Guidance:**

- Task 2.2: `ctx.Err()` checks must be placed AFTER each `notify(phase)` so the UI shows the phase that was reached, then aborts. Do not check inside `EnsureProfile`/`SyncRoutes` internals — only at `ConnectFull` boundaries.
- Task 2.6: because `rasdial` may be mid-run when canceled, the post-cancel `DisconnectFull(name)` is best-effort (ignore its error) and only runs on the `canceled` code path.
- Task 2.7: do NOT set `StatusDisconnected` directly in `onCancel` — let the goroutine's completion (2.6) do final state transition so there is a single owner of `c.state` after connect.

### Phase 3: Documentation

**Depends on:** Phase 1 + Phase 2

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 3.1 | Documentation | `README.md` | Document auto-disconnect-other-VPNs behavior (Feature 1) under "What it does" / a new "Connecting" note: app disconnects other active Windows VPNs before connecting. Document the `Batal` (Cancel) button (Feature 2): appears while connecting; aborts and cleans up a half-open tunnel. Keep Indonesian label references. |

### Phase 4: Review & Verification (Always Last)

**Depends on:** All implementation phases

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 4.1 | Debugger/Reviewer | `go build ./cmd/vepeen` + `go vet ./internal/ui ./cmd/vepeen` + `go build ./...` (cross `GOOS=windows` and default) must pass. Verify non-Windows stub compiles. |
| 4.2 | Debugger/Reviewer | Verify acceptance criteria below; manual Windows test: connect with another VPN up → other VPN disconnected; click `Batal` mid-connect → `Dibatalkan.` + `StatusDisconnected`. |
| 4.3 | Security | Confirm no PSK/password logged in `DisconnectAllExcept` or cancel paths; `canceled` error carries no secrets. |

---

## Acceptance Criteria

- [ ] **F1:** Clicking `Hubungkan` disconnects every OTHER Windows VPN connection that is `Connected` (verified via `Get-VpnConnection` list), leaving only the target Vepeen tunnel.
- [ ] **F1:** The list of disconnected VPN names is logged; partial teardown errors do not abort the connect.
- [ ] **F1:** `PhaseDisconnectOthers` (`"disconnect_others"`) exists and `PhaseDetail` returns `"Memutuskan VPN lain…"`; UI shows this phase at connect start.
- [ ] **F1:** `DisconnectAllExcept` exists on Windows and has a compiling non-Windows stub.
- [ ] **F2:** `ConnectFull` accepts `context.Context` and returns a `canceled` `UserError` when `ctx` is canceled between phases.
- [ ] **F2:** A `Batal` button is present in the button row, enabled only while `StatusConnecting`, disabled otherwise.
- [ ] **F2:** Clicking `Batal` cancels the connect; the goroutine returns `canceled`; UI shows `"Dibatalkan."` and returns to `StatusDisconnected`; target tunnel best-effort disconnected.
- [ ] **F2:** `applyEnablement` keeps `Hubungkan` disabled and `Batal` enabled during `StatusConnecting`.
- [ ] **Constraints:** Route parsing, secrets, config, and `EnforceSplitTunnel` (post-dial) are unchanged; app ID, `FyneApp.toml`, `fyne.Do`, Indonesian labels preserved; non-Windows build compiles.

## Regression Risk

- `ConnectFull` signature change requires updating the single call site in `onConnect` (Task 2.6). No other callers exist.
- `DisconnectAllExcept` runs `Get-VpnConnection` + `rasdial /DISCONNECT` for other profiles — if a user intentionally runs two VPNs, Vepeen will now tear the other down. This is intended behavior per the feature; document it in README.
- Cancel during `rasdial` may leave a half-open tunnel; the best-effort `DisconnectFull(name)` after cancel mitigates but is not guaranteed instantaneous.

## Rollback Strategy

- Feature 1 + 2 are additive and isolated to `disconnectall_windows.go`, `stub_other.go`, `manager.go` (`ConnectFull` + `PhaseDisconnectOthers`), and `main_window.go` (button + handlers). Revert the four files (and README) to pre-fix state via git to roll back. No schema/data migration involved.
