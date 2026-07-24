# Fix Plan: Show disconnect progress dialog on quit

**Related PRD:** PRD-002 (L2TP split tunnel) / fix-016 (quit-disconnect)
**Severity:** Low
**Reported by:** User (Indonesian) — "saat quit jangan ke blocking diam tapi munculkan dialog proses disconnectnya saja" (when quitting, don't just block silently — show a dialog that displays the disconnect progress)
**Date:** 2026-07-24

## Bug Summary

After fix-016, `disconnectAndQuit` in `internal/ui/main_window.go` (lines 45-79)
correctly status-checks → `DisconnectFull` → verifies → retries once, with a 5 s
best-effort timeout, then calls `a.Quit()`. However, during the disconnect the app
gives **no visible feedback** — the window may already be hidden to tray, and the
goroutine + `select` block silently. The user perceives this as "blocking silently"
and wants a visible progress dialog showing the disconnect is happening.

## Root Cause Analysis

The quit path performs the disconnect entirely off the UI thread (goroutine +
`select`) and never surfaces any status to the user. There is no dialog, label, or
progress indicator on the quit path. The 5 s `time.After` timeout guarantees quit
never hangs, but the wait is invisible.

## Fix Strategy

### Option A: Non-dismissible progress dialog on the quit path (RECOMMENDED)

Add a visible, non-dismissible progress dialog to `disconnectAndQuit` so the user
sees the disconnect is in progress. Reuses verified Fyne v2.8.0 APIs and the
existing `fyne.Do` UI-thread mutation pattern already used throughout the file
(e.g. lines 342, 500, 563).

**Files:** `internal/ui/main_window.go` only.
**Risk:** Low. No vpn-package change; only UI additions using stable Fyne widgets.
**Effort:** S

### Option B: Confirmation dialog before disconnect

Rejected — user explicitly did NOT ask for a confirmation step, only a progress
indicator. Adding a confirm would change quit behavior (extra click) and is out of
scope.

## Implementation Tasks

| Task | Agent   | Files                        | Description                                                                                       |
| ---- | ------- | ---------------------------- | ------------------------------------------------------------------------------------------------- |
| 1    | Frontend Developer | `internal/ui/main_window.go` | Add `"fyne.io/fyne/v2/dialog"` to the import block (currently imports `fyne`, `widget`, `container`, `theme`, `layout`, `canvas` but NOT `dialog`). |
| 2    | Frontend Developer | `internal/ui/main_window.go` | In `disconnectAndQuit`, before starting the disconnect goroutine, build a non-dismissible progress dialog: `content := container.NewVBox(widget.NewLabel("Disconnecting VPN…"), widget.NewProgressBarInfinite())`; `dlg := dialog.NewCustomWithoutButtons("Quitting Vepeen", content, w)`. |
| 3    | Frontend Developer | `internal/ui/main_window.go` | Ensure the window is visible so the dialog shows even when quitting from the tray: call `w.Show()` before `dlg.Show()`, then `dlg.Show()`. (`widget.NewProgressBarInfinite()` auto-starts its animation on Show — no manual Start.) |
| 4    | Frontend Developer | `internal/ui/main_window.go` | Update the status label text at each step via `fyne.Do` (e.g. "Disconnecting <name>…", "Still connected, retrying…", "Closing…"). Use the dialog's label widget reference. |
| 5    | Frontend Developer | `internal/ui/main_window.go` | Keep the existing 5 s safety timeout. On completion OR timeout, hide the dialog and quit: perform `dlg.Hide()` + `a.Quit()` inside `fyne.Do` for UI-thread safety. |
| 6    | Frontend Developer | `internal/ui/main_window.go` | Do NOT add a confirmation dialog. Keep X-button hide-to-tray behavior unchanged. Keep best-effort + 5 s timeout. Never log secrets/PSK. |

### Notes for the Frontend Developer

- `dialog.NewCustomWithoutButtons` returns a `*CustomDialog` that embeds `*dialog`
  and exposes `Hide()` — no close/dismiss buttons, so the user cannot cancel the
  quit (correct: quit must proceed).
- The disconnect goroutine already logs each step with `log.Printf` (no secrets).
  Mirror those steps into the dialog label via `fyne.Do` so the UI reflects them.
- `ctrl.win` (== `w`) is the `fyne.Window` to attach the dialog to.
- `go build ./...` and `go vet ./internal/ui/...` must pass.

## Acceptance Criteria

- [ ] Quitting via the main-menu "Quit" item shows a "Quitting Vepeen" dialog with an infinite progress bar (no dismiss buttons) before/while disconnecting.
- [ ] The dialog is visible even when the app was hidden to tray (window is shown before the dialog).
- [ ] The dialog label updates to reflect disconnect steps (disconnecting, retrying, closing) via `fyne.Do`.
- [ ] On disconnect completion OR the 5 s timeout, the dialog is hidden and `a.Quit()` is called (inside `fyne.Do`).
- [ ] Quit never hangs longer than ~5 s (existing safety timeout preserved).
- [ ] X-button still only hides to tray (no dialog, no disconnect) — unchanged.
- [ ] No confirmation dialog is added.
- [ ] No secrets/PSK are logged or displayed.
- [ ] `go build ./...` and `go vet ./internal/ui/...` pass.

## Regression Risk

- The dialog is created on the quit path only; normal connect/disconnect UI flows
  are untouched. Risk is low. Verify the tray "Quit" path also shows the dialog
  (it calls the same `disconnectAndQuit` closure via `onQuit`).
- Ensure `w.Show()` does not cause a flicker/teleport on the already-visible window
  (it is a no-op if already shown); the centering logic in `ui.ShowCentered` is
  unaffected because quit happens after show.
