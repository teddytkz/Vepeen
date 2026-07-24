# Fix Plan: Quit Re-entrancy Guard (atomic.Bool)

**Related PRD:** PRD-002 (L2TP split-tunnel) — quit path hardening
**Severity:** Low
**Reported by:** Debugger/Reviewer (code review)
**Date:** 2026-07-24

## Bug Summary

The `disconnectAndQuit` closure in `internal/ui/main_window.go` (lines 45-98) is invoked from two places: the main-menu "Quit" item (`NewMainWindow`, ~line 109) and the tray "Quit" item (`internal/ui/tray_windows.go:39`, via `onQuit`). If both are triggered in quick succession (e.g. user clicks menu Quit then tray Quit, or vice-versa), the closure can run twice concurrently. This spawns two progress dialogs and can call `a.Quit()` twice, risking a double-quit race / duplicate dialog flicker.

## Root Cause Analysis

`disconnectAndQuit` has no re-entrancy guard. The closure builds a dialog and spawns a goroutine that eventually calls `a.Quit()`. Because the two call sites share the same closure but no shared "already quitting" flag, there is nothing preventing a second invocation from proceeding while the first is still in flight. The `controller` struct already carries an `atomic.Bool` (`tickBusy`, line 152) and `sync/atomic` is already imported, so the project pattern for cheap, lock-free one-shot guards is established — it just isn't applied to the quit path.

## Fix Strategy

### Option A: Minimal Guard (Recommended)

- Files: `internal/ui/main_window.go` (add 1 struct field + 3 lines at top of closure)
- Risk: Negligible — pure additive guard, no behavior change on the single-invocation path.
- Effort: S

### Option B: Mutex-based guard

- Files: `internal/ui/main_window.go` (reuse `ctrl.mu`)
- Risk: Works, but heavier than needed and mixes concerns with the existing `sync.Mutex` used for UI state.
- Effort: S

**Recommended:** Option A — matches the existing `tickBusy atomic.Bool` pattern already in the file and keeps the change minimal and lock-free.

## Implementation Tasks

| Task | Agent   | Files                        | Description                                                                                       |
| ---- | ------- | ---------------------------- | ------------------------------------------------------------------------------------------------- |
| 1    | Frontend Developer | `internal/ui/main_window.go` | Add `quitting atomic.Bool` field to `controller` struct, placed next to `tickBusy atomic.Bool` (line 152). |
| 2    | Frontend Developer | `internal/ui/main_window.go` | At the very top of the `disconnectAndQuit` closure (line 46, before `name := ctrl.profileName()`), insert the guard: `if !ctrl.quitting.CompareAndSwap(false, true) { return }`. |

No other behavior changes: the dialog, `widget.NewProgressBarInfinite()`, 5 s timeout, `fyne.Do` usage, and X-button hide-to-tray all remain as-is. `sync/atomic` is already imported, so no import change is required.

## Acceptance Criteria

- [ ] `controller` struct has a `quitting atomic.Bool` field adjacent to `tickBusy atomic.Bool`.
- [ ] The first line(s) of the `disconnectAndQuit` closure perform `CompareAndSwap(false, true)` and `return` early when already quitting.
- [ ] A single Quit (menu or tray) still shows the progress dialog and calls `a.Quit()` exactly once.
- [ ] Rapidly triggering menu Quit + tray Quit (or twice on either) spawns only one dialog and calls `a.Quit()` at most once.
- [ ] `go build ./...` and `go vet ./internal/ui/...` pass with no new warnings.

## Regression Risk

Low. The guard only short-circuits a second concurrent invocation; the normal single-quit path is unchanged. The `tickBusy` field and its usages are untouched.
