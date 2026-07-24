# Fix Plan: Quit via menu does not disconnect VPN

**Related PRD:** PRD-002 (L2TP split tunnel) / changelog `quit-disconnect` (2026-07-23)
**Severity:** Medium
**Reported by:** User (Indonesian) — "kalau quit lewat menu kenapa tidak disconnect ya" (why doesn't it disconnect when quitting via the menu)
**Date:** 2026-07-24

## Bug Summary

When the user quits the app, the VPN connection is sometimes left up. Two distinct
perceptions feed this:

1. **Tray "Quit" path** (`internal/ui/tray_windows.go:38-40` → `onQuit()` = `disconnectAndQuit`)
   attempts a best-effort disconnect but the failure is invisible and unreliable.
2. **Main window menu** (`internal/ui/main_window.go:64-72`) has **no Quit item at all** —
   only "Create Desktop Shortcut". The user's phrase "quit lewat menu" most likely
   refers to either the window X button (which only `Hide()`s to tray by design, see
   `tray_windows.go:48-50`) or an expected-but-missing menu Quit. The X-hide path
   correctly does NOT disconnect (hide-to-tray), but the user perceives it as
   "quit without disconnect."

## Root Cause Analysis

`disconnectAndQuit` in `internal/ui/main_window.go:46-60`:

```go
disconnectAndQuit := func() {
    name := ctrl.profileName()
    if name != "" {
        done := make(chan struct{})
        go func() {
            defer close(done)
            _ = ctrl.mgr.DisconnectFull(name)  // ERROR SWALLOWED
        }()
        select {
        case <-done:
        case <-time.After(5 * time.Second):
        }
    }
    a.Quit()
}
```

Ranked root causes:

1. **Silent failure / wrong name (most likely).** The disconnect error is discarded
   with `_ =`. `profileName()` (`main_window.go:378-384`) **never returns empty** —
   it falls back to `config.DefaultConnectionName`. If the live RAS entry name does
   not match `c.connectionName` (e.g. profile renamed, or the user connected a
   different profile), `rasdial.exe <name> /DISCONNECT` fails, the error is ignored,
   and the app quits with the VPN still up.
2. **No post-disconnect verification.** The code never re-queries OS status, so a
   soft-failed disconnect is indistinguishable from success. No retry.
3. **Only the named profile is targeted.** No fallback to disconnect other active
   VPNs if the name is wrong.
4. **Missing main-menu Quit item.** "Quit lewat menu" is ambiguous; the only menu
   action is the desktop-shortcut creator. Users expect a Quit in the menu bar.
5. **Race (low likelihood).** The `select` waits for `DisconnectFull` to finish, so
   the only race is if the 5 s timeout fires mid-`rasdial` — rare.

## Fix Strategy

### Option A: Robust quit-disconnect in the UI closure + add menu Quit (RECOMMENDED)

- Rewrite `disconnectAndQuit` to: query `ctrl.mgr.Status(name)`; if `Connected`,
  call `ctrl.mgr.DisconnectFull(name)`, then re-query `Status`; if still
  `Connected`, retry `DisconnectFull` once. Log every outcome with `log.Printf`
  (never secrets/PSK). Keep the 5 s best-effort timeout so quit never hangs.
- Add a **"Quit"** item to the main window `fyne.NewMainMenu` that calls the same
  `disconnectAndQuit` closure, so "quit lewat menu" is unambiguous.
- Keep X-button hide-to-tray behavior (`tray_windows.go:48-50`) unchanged — document
  the distinction (hide ≠ quit; hide intentionally keeps the VPN up).

**Files:** `internal/ui/main_window.go` only.
**Risk:** Low. Reuses existing `Status` + `DisconnectFull` primitives; no vpn-package change.
**Effort:** S

### Option B: Add a backend `Manager.DisconnectIfConnected(name)` helper

Move the status-check + disconnect + verify + retry logic into `internal/vpn/manager.go`
(or `internal/vpn/win`), then call it from the UI closure. Cleaner separation but
touches the vpn package and is unnecessary given the primitives already exist.

**Recommended:** Option A — keeps the change in the UI layer (Frontend), reuses
`Status`/`DisconnectFull`, and adds the missing menu Quit. No backend change required.

## Implementation Tasks

| Task | Agent            | Files                          | Description                                                                 |
| ---- | ---------------- | ------------------------------ | --------------------------------------------------------------------------- |
| 1    | Frontend Dev     | `internal/ui/main_window.go`   | Import `log`; rewrite `disconnectAndQuit` to status-check → disconnect → verify → retry-once, logging each step with `log.Printf` (no secrets). Keep 5 s timeout. |
| 2    | Frontend Dev     | `internal/ui/main_window.go`   | Add a "Quit" `fyne.NewMenuItem` to the existing `fyne.NewMainMenu` that invokes `disconnectAndQuit`. |
| 3    | Debugger/Reviewer| `internal/ui/main_window.go`   | Verify `go build ./...` and `go vet ./internal/ui/...` pass; confirm X still hides (no disconnect) and both tray Quit and menu Quit disconnect. |

## Acceptance Criteria

- [ ] Tray "Quit" disconnects the active/selected VPN profile (verified via OS status) before the app exits.
- [ ] Main window menu now contains a "Quit" item that performs the same disconnect-then-quit.
- [ ] If the disconnect fails or the profile name is wrong, the failure is logged to `%AppData%/vepeen/vepeen.log` (no passwords/PSK) instead of being silently swallowed.
- [ ] A single retry is attempted when the OS still reports `Connected` after the first disconnect.
- [ ] Quit never blocks beyond the 5 s best-effort timeout.
- [ ] Window X button still only hides to tray (VPN intentionally stays up); this behavior is unchanged.
- [ ] `go build ./...` and `go vet ./internal/ui/...` pass.

## Regression Risk

- X-button hide-to-tray must remain disconnect-free (do not reuse `disconnectAndQuit` there).
- The new menu Quit must not double-fire with tray Quit (both call the same closure; Fyne invokes one at a time, safe).
- `log` import added to `main_window.go` must not conflict with existing imports.
