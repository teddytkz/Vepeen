# Fix Plan: Blank window when second instance re-shows a tray-hidden window

**Related PRD:** PRD-001 (golang-fyne starter), PRD-002 (l2tp-split-tunnel)
**Severity:** High
**Reported by:** Debugger/Reviewer
**Date:** 2026-07-24

## Bug Summary

When the app is hidden to the system tray (X → `w.Hide()`) and the user launches the `.exe` again, the existing window reappears but renders a blank/white canvas with no UI. The bug only reproduces after the window has been hidden to tray, not when it was already visible.

## Root Cause Analysis

`AcquireSingleInstance()` (in `internal/ui/single_instance_windows.go`) detects the existing global mutex `Global\VepeenSingleInstance` and calls `bringExistingToFront()`, which uses native Win32 `ShowWindow(hwnd, SW_RESTORE)` + `SetForegroundWindow(hwnd)` directly on the first instance's HWND. Because the first instance was hidden via Fyne's `w.Hide()` (Fyne internal `visible=false`), the native `ShowWindow` makes the OS window visible again, but Fyne still believes it is hidden and its render loop **skips repainting** → blank/white canvas. When the window was never hidden, `SW_RESTORE` merely activates it and Fyne keeps painting, so the bug is invisible on that path.

## Fix Strategy

### Option A: Event-signaling via Fyne (Recommended)

- The first instance creates a named event `Global\VepeenShowEvent` and runs a goroutine that waits on it and, when signaled, calls `fyne.Do(func(){ w.Show(); w.RequestFocus() })` so Fyne repaints correctly.
- The second instance, on detecting the mutex already exists, signals the event (`SetEvent`) and exits — it never touches the first instance's HWND via native ShowWindow/SetForegroundWindow.
- `AllowSetForegroundWindow(ASFW_ANY)` is called by the second instance so the first instance is permitted to take foreground when it calls `RequestFocus()`.

**Recommended:** Option A — keeps all window visibility changes inside Fyne's render loop, eliminating the blank-canvas race entirely. Removes the fragile native-show path.

## Files to MODIFY

### `internal/ui/single_instance_windows.go` (`//go:build windows`)
- Add a new named-event constant `showEventName = "Global\\VepeenShowEvent"`.
- Add lazy DLL procs for `kernel32`: `CreateEventW`, `OpenEventW`, `SetEvent`, `WaitForSingleObject`, `CloseHandle` (CloseHandle already used for mutex), and `user32`: `GetWindowThreadProcessId`, `AllowSetForegroundWindow`.
- Change `AcquireSingleInstance()` so that on the **first** (owning) instance it also creates the show event and returns its handle via the `release` func (closed on exit). On the **second** instance it calls `SignalExistingInstance()` instead of `bringExistingToFront()`, then returns `alreadyRunning=true`.
- Remove (or demote to a no-op fallback) the native `bringExistingToFront()` `ShowWindow`/`SetForegroundWindow` logic.
- Add `SignalExistingInstance()` (see helpers) — opens the existing event and sets it; best-effort, ignores errors.
- Add `ListenForShowSignal(w fyne.Window)` (see helpers) — spawns a goroutine that waits on the event handle and calls `fyne.Do(func(){ w.Show(); w.RequestFocus() })`.

### `cmd/vepeen/main.go`
- After `AcquireSingleInstance()` returns (first instance path), call `ui.ListenForShowSignal(w)` **before** `a.Run()` so the wait goroutine is live.
- Defer closing the show-event handle: extend the existing `defer releaseMutex()` to also close the event handle (or have `AcquireSingleInstance` return a combined release func that closes both mutex and event).

### `internal/ui/tray_windows.go` (`//go:build windows`)
- No behavioral change required. `SetupTray`'s `SetCloseIntercept(func(){ w.Hide() })` and the "Show" menu item (`w.Show(); w.RequestFocus()`) stay as-is. Confirmed compatible with the event mechanism.

### `internal/ui/window_pos_windows.go` (`//go:build windows`)
- No change. `ShowCentered(w)` (first-launch path) is unaffected.

### `internal/ui/single_instance_other.go` (`//go:build !windows`)
- Add a no-op `ListenForShowSignal(w fyne.Window)` so non-Windows builds compile. `AcquireSingleInstance()` already returns `alreadyRunning=false` (no-op) — leave unchanged.

## New Helpers to ADD

| Helper | Location | Responsibility |
| ------ | -------- | -------------- |
| `ListenForShowSignal(w fyne.Window)` | `single_instance_windows.go` (windows) / `single_instance_other.go` (no-op) | First instance: create/spawn a goroutine that blocks on `WaitForSingleObject(eventHandle)` and, on signal, runs `fyne.Do(func(){ w.Show(); w.RequestFocus() })`. Must run on a goroutine so it does not block `main`. |
| `SignalExistingInstance()` | `single_instance_windows.go` | Second instance: `OpenEventW(EVENT_MODIFY_STATE, false, showEventName)`, call `AllowSetForegroundWindow(ASFW_ANY)` (0xFFFF) so the first instance may take foreground, then `SetEvent(handle)`, then `CloseHandle`. Best-effort; ignore errors and return. |
| `createShowEvent() (windows.Handle, error)` | `single_instance_windows.go` | First instance: `CreateEventW(nil, 0, 0, showEventName)` and return the handle for the release func to close. |

## Constraints

- Keep `//go:build windows` on all Windows-only code; keep the `!windows` stub parity (no-op `ListenForShowSignal`).
- Preserve the single-instance mutex behavior (`Global\VepeenSingleInstance`) — only the *show* mechanism changes, not the *detection*.
- Must not break the normal first-launch path: `ShowCentered` + `a.Run()` unchanged; the show-event goroutine is inert until signaled.
- Must not break tray hide/show: `w.Hide()` on X and the tray "Show" menu item continue to work via Fyne directly.
- Any Fyne call from the wait goroutine (`w.Show()`, `w.RequestFocus()`) MUST be wrapped in `fyne.Do(...)` to satisfy Fyne's UI-thread requirement.
- The show-event handle must be created by the first instance BEFORE `a.Run()` and closed via `defer` on exit.
- No new third-party dependencies — use `golang.org/x/sys/windows` (already a dependency) lazy DLL procs, consistent with existing single-instance code.

## Implementation Tasks

| Task | Agent   | Files | Description |
| ---- | ------- | ----- | ---------- |
| 1    | Backend Developer | `internal/ui/single_instance_windows.go` | Add event constants, lazy procs, `createShowEvent`, `ListenForShowSignal`, `SignalExistingInstance`; rewire `AcquireSingleInstance` to create event on first instance and signal on second; remove native `bringExistingToFront` show path. |
| 2    | Backend Developer | `cmd/vepeen/main.go` | Call `ui.ListenForShowSignal(w)` before `a.Run()`; ensure event handle closed via `defer` (extend release func). |
| 3    | Backend Developer | `internal/ui/single_instance_other.go` | Add no-op `ListenForShowSignal(w fyne.Window)`. |

## Acceptance Criteria

- [ ] `go build ./...` and `go vet ./internal/ui/...` pass.
- [ ] First launch: window shows centered with full UI (no blank/white canvas).
- [ ] Hide to tray via X, then launch the `.exe` again: the existing window reappears **with the full UI rendered** (not blank/white), and is focused.
- [ ] Tray "Show" menu item and left-click tray icon still show the window correctly.
- [ ] Only one instance runs (second launch does not open a second window); second instance exits after signaling.
- [ ] No console window regression (`-H windowsgui` preserved).

## Validation Steps

1. `.\build.ps1` (or `go build ./...`) — confirm clean build.
2. `go vet ./internal/ui/...` — confirm no issues.
3. Manual repro:
   - Launch `bin\vepeen.exe` → window appears with UI.
   - Click X → window hides to tray (icon remains).
   - Launch `bin\vepeen.exe` again → existing window reappears **with UI visible** (not blank/white) and focused.
   - Right-click tray → "Show" → window shows correctly.
   - Right-click tray → "Quit" → app exits, second launch starts fresh normally.

## Regression Risk

- If `AllowSetForegroundWindow(ASFW_ANY)` is omitted, the first instance's `RequestFocus()` may be silently rejected by Windows foreground-lock rules (window still shows, just may not steal focus) — low impact.
- Event handle leak if `defer` release is not wired in `main.go` — mitigated by extending the existing `release` func returned from `AcquireSingleInstance`.
- Non-Windows builds must keep the no-op stub so `ListenForShowSignal` resolves.
