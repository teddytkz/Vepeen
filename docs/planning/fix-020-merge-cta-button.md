# Fix Plan: Merge Connect / Cancel / Disconnect into a single stateful CTA button

**Related PRD:** PRD-002 (L2TP split tunnel) — UI button row
**Severity:** Low
**Reported by:** Planner (user request)
**Date:** 2026-07-24

## Bug Summary

The footer currently renders three separate buttons — `btnDisc` ("Disconnect"), `btnCancel` ("Cancel"), and `btnConn` ("Connect") — each wired to its own handler. This duplicates the state-routing logic that already exists in `onHeroTap()` and clutters the footer. The request is to collapse them into ONE button whose label and action change by connection state.

## Root Cause Analysis

Not a defect — a UX/consolidation improvement. The three-button layout was added incrementally (see `fix-009-auto-disconnect-cancel.md` which introduced `btnCancel`). The stateful router `onHeroTap()` (line 340) already performs the exact Connect/Cancel/Disconnect switch, so the three buttons are redundant. `syncCTA()` (line 752) is currently a stub that only ever sets "Connect".

## Fix Strategy

### Option A: Minimal Fix (recommended)

- Keep the existing `c.btnConn` field (rename to `c.btnCTA` is optional and only adds churn — **keep `c.btnConn`** to minimize references).
- Remove `c.btnDisc` and `c.btnCancel` fields + their creation lines.
- Wire the single button's `OnTapped` to the existing `c.onHeroTap`.
- Rewrite `syncCTA()` to set label + `Importance` by state.
- Rewrite the button-enable logic in `applyEnablement()` to drive the single button; delete all `btnDisc`/`btnCancel` references.
- Update the footer `container.NewHBox` to contain only `c.btnConn`.

- Files: `internal/ui/main_window.go` only
- Risk: Low — handlers unchanged, state machine unchanged
- Effort: S

### Option B: Rename field to `c.btnCTA`

- Same as A but also rename `c.btnConn` → `c.btnCTA` across all 7 references.
- Risk: Low, but more diff churn for no behavioral gain.
- **Not recommended** — keep `c.btnConn` to reduce surface area.

**Recommended:** Option A — keep `c.btnConn`, remove the other two.

## Implementation Tasks

All in `internal/ui/main_window.go`. Agent: Frontend Developer → Debugger/Reviewer.

| Task | File | Lines | Description |
| ---- | ---- | ----- | ----------- |
| 1 | `internal/ui/main_window.go` | 145-147 | Remove struct fields `btnDisc *widget.Button` and `btnCancel *widget.Button`. Keep `btnConn *widget.Button` (line 146). |
| 2 | `internal/ui/main_window.go` | 295-298 | Remove creation lines `c.btnDisc = widget.NewButton("Disconnect", c.onDisconnect)` (295) and `c.btnCancel = widget.NewButton("Cancel", c.onCancel)` (296). Change `c.btnConn = widget.NewButton("Connect", c.onConnect)` (297) to `c.btnConn = widget.NewButton("Connect", c.onHeroTap)` so the single button routes through the existing stateful router. Keep/remove the `c.btnConn.Importance = widget.HighImportance` line (298) — it will be overwritten by `syncCTA()`; safe to delete or leave. |
| 3 | `internal/ui/main_window.go` | 301 | Update footer layout `container.NewHBox(layout.NewSpacer(), c.btnSave, c.btnDisc, c.btnCancel, c.btnConn)` → `container.NewHBox(layout.NewSpacer(), c.btnSave, c.btnConn)`. |
| 4 | `internal/ui/main_window.go` | 752-766 | Rewrite `syncCTA()` to set label + Importance by state (see spec below). Remove the `if c.btnConn == nil { return }` early-out only if it stays valid (keep it — harmless). |
| 5 | `internal/ui/main_window.go` | 790-843 | Rewrite the button-enable block in `applyEnablement()` to drive only `c.btnConn`; delete every `btnDisc`/`btnCancel` reference (lines 829-831, 835-836, 840-842). |

### `syncCTA()` spec (Task 4)

```text
switch c.state {
case vpn.StatusConnected:
    c.btnConn.SetText("Disconnect")
    c.btnConn.Importance = widget.MediumImportance   // or LowImportance
case vpn.StatusConnecting:
    c.btnConn.SetText("Cancel")
    c.btnConn.Importance = widget.DangerImportance   // or WarningImportance
case vpn.StatusDisconnecting:
    c.btnConn.SetText("Disconnecting…")
    // importance left as-is (disabled anyway)
default: // StatusDisconnected, StatusError, StatusUnknown
    c.btnConn.SetText("Connect")
    c.btnConn.Importance = widget.HighImportance
}
c.btnConn.Refresh()
```

### `applyEnablement()` spec (Task 5)

Replace the three-button block with a single enable decision:

```text
ctaEnabled := (c.state == vpn.StatusDisconnected ||
               c.state == vpn.StatusError ||
               c.state == vpn.StatusUnknown ||
               c.state == vpn.StatusConnecting ||
               c.state == vpn.StatusConnected) && !c.busy
// StatusDisconnecting is intentionally excluded → disabled.
if ctaEnabled {
    c.btnConn.Enable()
} else {
    c.btnConn.Disable()
}
```

Remove the now-dead `busyConnect`/`busyDisc`/`connected` variables if they become unused (they are still used for `setEntry` form gating — `formEnabled` is reused; `busyConnect`/`busyDisc`/`connected` become unused after removing the btnDisc/btnCancel lines, so delete those three declarations to keep `go vet` clean).

## Acceptance Criteria

- [ ] `go build ./...` exits 0
- [ ] `go vet ./internal/ui/...` exits 0
- [ ] Only ONE button appears in the footer (next to Save)
- [ ] Disconnected / Error / Unknown → button reads "Connect", taps connect
- [ ] Connecting → button reads "Cancel", taps cancel
- [ ] Connected → button reads "Disconnect", taps disconnect
- [ ] Disconnecting → button reads "Disconnecting…" and is disabled
- [ ] During generic `busy` (e.g. Save in progress) the button is disabled
- [ ] `onConnect`, `onCancel`, `onDisconnect` handlers are unchanged
- [ ] No remaining references to `btnDisc` or `btnCancel` anywhere in the file

## Regression Risk

- `onHeroTap()` is already exercised by the hero ring tap; reusing it for the button is safe.
- `syncVisualState`/`setStatus` already call `syncCTA()` after every transition, so label updates automatically — no new call sites needed.
- `cancelConnect` context plumbing and `DisconnectFull` calls are untouched.
- Footer layout change is purely additive-removal of two widgets; no other container depends on `btnDisc`/`btnCancel`.
