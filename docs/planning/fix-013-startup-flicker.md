# Fix Plan: Startup window flicker / blink ("kedipan apps")

**Related PRD:** PRD-001 (golang-fyne-starter), PRD-002 (l2tp-split-tunnel)
**Severity:** Medium (visual UX defect on first launch)
**Reported by:** User (Indonesian: "kedipan apps" — app blinks when first opened)
**Date:** 2026-07-23

## Bug Summary

When Vepeen is launched, the main window is briefly visible at one position (top-left / 0,0), then jumps to the work-area center, and a white/light frame flashes before the dark themed content paints. The result is a visible blink on first open.

## Root Cause Analysis

Four ranked causes were identified by the Explorer:

- **Rank 1 (primary):** `internal/ui/window_pos_windows.go:59-76` — `centerOnWorkArea` spawns a goroutine that polls every 20ms (3s deadline) for the GLFW HWND, then calls `SetWindowPos` to move the window to the work-area center. Because `ShowAndRun()` (`cmd/vepeen/main.go:24`) shows the window at GLFW's default position first, the window appears at 0,0 and then teleports to center → visible blink. The `SWP_NOACTIVATE` flag (line ~52) adds a focus/visibility flicker.
- **Rank 2 (secondary):** GLFW/OpenGL clears to a default (white/light) background for the first frame before Fyne's dark theme paints. No application manifest exists and no `WS_EX_COMPOSITED`/`WS_CLIPCHILDREN`/double-buffering mitigation is applied. `build.ps1` only sets `-H windowsgui`.
- **Rank 3 (secondary):** `internal/ui/main_window.go:31-38` — staged `Resize` → `SetContent` → `centerOnWorkArea`. The deferred centering (Rank 1) means the final size used for centering is not known until after show.
- **Rank 4 (minor):** `internal/ui/main_window.go:155-220` — `loadInitial()` runs `config.LoadStored()` (DPAPI decrypt) synchronously in `NewMainWindow` before `ShowAndRun`, delaying first paint (does not itself cause a blink, but slows startup).

This plan addresses **Rank 1** and **Rank 2** (the two concrete, code-level causes of the visible flicker), with optional notes on Rank 3/4.

## Fix Strategy

### Option A: Synchronous show-time positioning + composited window style (RECOMMENDED)

- **Rank 1:** Eliminate the deferred polling goroutine. Position the window synchronously at window-show time, before the first frame is composited, so it never appears at 0,0.
- **Rank 2:** Apply `WS_EX_COMPOSITED` (and optionally `WS_CLIPCHILDREN`) extended window style at the same show-time hook to enable composited double-buffered painting, reducing the white first-frame flash. Add a Windows application manifest (`.syso`) for DPI awareness so the window is not blurry/flashing on HiDPI displays.
- **Rank 3:** Naturally resolved — positioning at show time reads the actual window rect via `GetWindowRect`, so the final size is correct.

### Option B: Minimal — just use Fyne's `CenterOnScreen()`

- Delete the custom goroutine and call `w.CenterOnScreen()` in `NewMainWindow` (before `ShowAndRun`). Fyne applies centering at GLFW window-creation time → no blink. Downside: centers against the full monitor rect (taskbar not excluded), so the window sits slightly lower than the work-area center the team originally wanted. Does NOT address Rank 2 white flash.

**Recommended:** Option A — it fixes both visible causes (Rank 1 + Rank 2) and preserves the work-area centering the team explicitly wanted, without reintroducing the blink.

## Implementation Tasks

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1 | Frontend Developer | `internal/ui/window_pos_windows.go`, `internal/ui/window_pos_other.go`, `cmd/vepeen/main.go` | Replace the deferred `centerOnWorkArea` goroutine with a synchronous show-time positioning helper. Add a build-tagged `showCentered(w fyne.Window)` (Windows) that (a) shows the window, (b) positions it at the work-area center via `driver.NativeWindow.RunNative` (HWND is valid after `Show()`), and (c) applies `WS_EX_COMPOSITED` (+ optional `WS_CLIPCHILDREN`) via `GetWindowLongPtr`/`SetWindowLongPtr`. Keep `SPI_GETWORKAREA` work-area computation. The non-Windows stub keeps `w.CenterOnScreen()` + `ShowAndRun`. `main.go` calls the helper instead of `w.ShowAndRun()` (helper internally calls `a.Run()` on Windows). |
| 2 | Frontend Developer | `build.ps1`, new `*.syso` / `*.manifest` (repo root or `cmd/vepeen/`) | Add a Windows application manifest embedded as a `.syso` resource (via `rsrc` or `goversioninfo`, or a hand-written `.manifest` compiled with `windres`/MinGW to `.syso`). Manifest must declare `dpiAware`/`dpiAwareness` (Per-Monitor v2) and `windowsSettings`. Must remain compatible with `-H windowsgui` (GUI subsystem, no console). Verify the build still produces a no-console GUI exe. |
| 3 (optional) | Frontend Developer | `internal/ui/main_window.go` | Move the synchronous `config.LoadStored()` + `applyConfig` portion of `loadInitial()` into a `fyne.Do` goroutine (mirroring the existing profile/status goroutines) so the window paints sooner. Keep `applyEnablement`/`loadCredentials` ordering correct. Addresses Rank 4 only; optional. |

### Sub-Agent Guidance

- Task 1 is atomic (single file group, Windows + non-Windows stubs + main entry).
- Task 2 is independent of Task 1 (build/resource concern) and can run in parallel.
- Task 3 is independent and optional; do it last if included.

## Key Constraints (must respect)

- **No console window:** `build.ps1` uses `-ldflags="-H windowsgui"`. The manifest/`.syso` and any tooling must NOT switch the subsystem back to console. Verify the built `bin/vepeen.exe` has no console (e.g., `dumpbin /headers` shows `SUBSYSTEM:WINDOWS` / `IMAGE_SUBSYSTEM_WINDOWS_GUI`).
- **Centering before wrong-position paint:** The window must be positioned at the work-area center *before* it is composited at 0,0. Positioning must happen synchronously on the main thread at show time, not via a post-show polling goroutine.
- **Work-area centering preserved:** Keep `SPI_GETWORKAREA` (taskbar-excluded) centering; do not regress to full-monitor centering unless Option B is explicitly chosen.
- **Theme already set:** `a.Settings().SetTheme(ui.NewTheme())` runs in `main.go` before `NewMainWindow`, so no default-theme flash is expected — do not alter theme setup.
- **Non-Windows stubs must compile:** `window_pos_other.go` keeps `CenterOnScreen()` + `ShowAndRun`; do not introduce Windows-only symbols there.
- **No blank-window regression:** `minSizeWrap` is already a proper `fyne.Widget` (fix-005); do not touch it.
- **Indonesian labels / app ID / FyneApp.toml / `fyne.Do` threading:** all preserved; no handler/threading/data-logic changes beyond Task 3's optional refactor.

## Risks & Things the Implementer Must Verify

- **Fyne `CenterOnScreen()` availability/timing:** Confirm (via Context7 / Fyne v2.8 docs) that `CenterOnScreen()` positions at GLFW window-creation time when called before `ShowAndRun` (relevant if Option B is chosen, and as a fallback).
- **`RunNative` timing:** Verify that calling `w.RunNative(...)` immediately after `w.Show()` (before `a.Run()`) executes the callback on the main thread *before* the first frame is presented, so no 0,0 frame is visible. If a one-frame 0,0 paint is still possible, hide the window first (`ShowWindow(hwnd, SW_HIDE)`), position, then `ShowWindow(hwnd, SW_SHOW)` within the same synchronous callback.
- **`SWP_NOACTIVATE` removal:** The current code uses `SWP_NOACTIVATE` (line ~52). Since positioning now happens before show, drop `SWP_NOACTIVATE` (or pair with `SWP_SHOWWINDOW`) to avoid the focus/visibility flicker it introduces. Confirm the window still receives focus normally after launch.
- **`WS_EX_COMPOSITED` side effects:** `WS_EX_COMPOSITED` enables per-pixel alpha compositing; verify it does not break Fyne's GLFW canvas rendering or cause transparency/redraw issues. Test on a standard (non-transparent) desktop. `WS_CLIPCHILDREN` is lower-risk if `WS_EX_COMPOSITED` misbehaves.
- **Manifest + `-H windowsgui` coexistence:** Confirm the `.syso` `RT_MANIFEST` resource is merged correctly and does not conflict with the linker-set subsystem. If using `goversioninfo`, ensure it is configured for GUI (no console flag) and does not overwrite the `FyneApp.toml` app identity.
- **DPI awareness:** Per-Monitor v2 (`dpiAwareness`) is preferred over legacy `dpiAware` to avoid the Windows DPI-virtualization white flash on HiDPI. Verify the window is crisp, not blurred.
- **Toolchain availability:** `rsrc`/`goversioninfo`/`windres` may need `go install` or MinGW; ensure `build.ps1` remains self-contained or documents the one-time tool install.

## Acceptance Criteria

- [ ] On first launch, the window appears directly at the work-area center — no visible jump from 0,0 / top-left.
- [ ] No white/light first-frame flash before the dark themed content paints (or it is substantially reduced via composited style + manifest).
- [ ] Built `bin/vepeen.exe` remains a GUI-subsystem executable with no console window (`-H windowsgui` preserved).
- [ ] Work-area (taskbar-excluded) centering is preserved.
- [ ] Non-Windows build still compiles and centers via `CenterOnScreen()`.
- [ ] No regression: window renders fully (no blank window), all controls/handlers intact, Indonesian labels and app ID unchanged.
- [ ] (Optional, Task 3) First paint occurs sooner; no change in loaded settings/credentials behavior.

## Regression Risk

- Changing window styles (`WS_EX_COMPOSITED`) could affect GLFW/OpenGL rendering on some GPUs/drivers — test on at least one real Windows machine.
- A manifest that sets the wrong DPI mode could blur the UI or shift layout — verify visually.
- If `RunNative` timing is wrong, the blink may persist or a brief hidden-then-shown flash could appear — verify the hide/position/show sequence if needed.
