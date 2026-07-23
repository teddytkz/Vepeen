# Fix Plan: Blank window (form not rendered) + console window on launch

**Related PRD:** PRD-002 (`docs/planning/prd-002-l2tp-split-tunnel.md`)
**Related plans:** fix-002 (`docs/planning/fix-002-form-visibility.md`), fix-004 (`docs/planning/fix-004-fyne-do-migration.md`)
**Severity:** Critical (Bug 1) / Medium (Bug 2)
**Reported by:** Debugger/Reviewer
**Date:** 2026-07-22
**Status:** Ready for implementation
**Version:** v1.0.0

---

## Bug Summary

### Bug 1 — Blank/empty window (form not rendered)
The Fyne window opens but shows a blank page; the form built by `controller.build()` in `internal/ui/main_window.go` does not appear. The window title and frame render, but no widgets are visible.

### Bug 2 — Console window appears when launching the `.exe`
Running the built `vepeen.exe` also opens a `cmd.exe` console window alongside the GUI. This is the default Windows console subsystem behavior of the Go linker.

---

## Root Cause Analysis

### Bug 1
`internal/ui/main_window.go` defines a custom `minSizeWrap` struct (lines ~625–640) that **embeds the raw `fyne.CanvasObject` interface** and only overrides `MinSize()`:

```go
type minSizeWrap struct {
	fyne.CanvasObject
	min fyne.Size
}
```

It does **not** implement `fyne.Widget` (no `CreateRenderer()`). Fyne's canvas obtains a renderer for each object in the content tree via `renderer(obj)`. For a non-Widget, non-canvas-primitive object, `renderer()` returns `nil`, so the canvas skips rendering the entire content subtree → blank window.

The previous fix-002 "PASS" was **static-only** (geometry probe of `MinSize()`), never an actual render, so this defect was never caught at runtime.

### Bug 2
Go's Windows linker defaults to the **console** subsystem. Building with `go build` (no `-ldflags`) produces an `.exe` that allocates a console on launch. The GUI-subsystem flag `-H windowsgui` is required to suppress it.

---

## Fix Strategy

### Bug 1 — Convert `minSizeWrap` into a proper `fyne.Widget`

**Recommended:** Single, self-contained fix in `internal/ui/main_window.go`.
**Risk:** Low — UI-only; no VPN/secrets/config/route changes.
**Effort:** S

Steps:
- Embed `widget.BaseWidget` instead of the raw `fyne.CanvasObject` interface.
- Store the inner `fyne.CanvasObject` as a field (`inner fyne.CanvasObject`).
- Override `MinSize()` to return `max(inner.MinSize(), min)`.
- Implement `CreateRenderer()` returning a renderer whose:
  - `Objects()` returns `[]fyne.CanvasObject{inner}`
  - `Layout(size fyne.Size)` calls `inner.Resize(size)`
  - `MinSize()` returns the widget's `MinSize()`
- Call `w.ExtendBaseWidget(self)` in the constructor.
- Update `NewMainWindow` to wrap with the new constructor instead of the struct literal.

### Bug 2 — GUI subsystem build + hidden-console diagnostics

**Recommended:** Add `build.ps1`, update `README.md`, and add a minimal panic/startup file logger in `cmd/vepeen/main.go`.
**Risk:** Low — build/tooling + entrypoint only; no logic changes to VPN/secrets/config/route.
**Effort:** S

Because `-H windowsgui` hides `stderr`/`stdout`, runtime panics/startup errors become invisible. A minimal file logger writes them to `%AppData%\vepeen\vepeen.log` (no new dependencies — use `os`, `path/filepath`, `runtime/debug`, `time`).

---

## Constraints (must hold)

- Do **NOT** change VPN / route / secrets / config logic.
- Keep app ID `com.vepeen.app` and `FyneApp.toml` intact (incl. `[Migrations] fyneDo = true`).
- Keep `fyne.Do` usage; no threading changes.
- CGo must stay enabled (MinGW-w64 gcc on PATH); builds set `CGO_ENABLED=1`.

---

## File Scope

| File | Change |
| ---- | ------ |
| `internal/ui/main_window.go` | Rewrite `minSizeWrap` → `fyne.Widget`; add constructor; update `NewMainWindow` wrap call |
| `cmd/vepeen/main.go` | Add panic recovery + startup error file logger to `%AppData%\vepeen\vepeen.log` |
| `build.ps1` (new) | `CGO_ENABLED=1 go build -ldflags="-H windowsgui" -o vepeen.exe ./cmd/vepeen` |
| `README.md` | Update Setup/build section to use `.\build.ps1` and explain `-H windowsgui` + log file |

---

## Implementation Tasks

**Agents:** Backend Developer (all code/tooling) → Debugger/Reviewer (verify render + no console) → Documentation (README).
**Parallelizable:** Bug 1 (`main_window.go`) and Bug 2 (`main.go` + `build.ps1`) touch different files and can be done in parallel; README update depends on both.

### Phase 1: Bug 1 — Widget conversion

**Depends on:** Nothing
**Parallelizable:** Yes (vs Bug 2 files)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/ui/main_window.go` | Replace `minSizeWrap` struct: embed `widget.BaseWidget`; add field `inner fyne.CanvasObject`; keep `min fyne.Size`. |
| 1.2 | Backend Developer | `internal/ui/main_window.go` | Add constructor `newMinSizeWrap(inner fyne.CanvasObject, min fyne.Size) *minSizeWrap` that sets fields and calls `w.ExtendBaseWidget(w)`. |
| 1.3 | Backend Developer | `internal/ui/main_window.go` | Override `MinSize()` → `max(inner.MinSize(), min)` (width/height independently). |
| 1.4 | Backend Developer | `internal/ui/main_window.go` | Implement `CreateRenderer()` returning a `widget.BaseWidgetRenderer` whose `Objects()` → `[]fyne.CanvasObject{inner}`, `Layout(size)` → `inner.Resize(size)`, `MinSize()` → `w.MinSize()`. |
| 1.5 | Backend Developer | `internal/ui/main_window.go` | In `NewMainWindow`, change `w.SetContent(&minSizeWrap{CanvasObject: ctrl.build(), min: fyne.NewSize(420, 600)})` to `w.SetContent(newMinSizeWrap(ctrl.build(), fyne.NewSize(420, 600)))`. |

**Conceptual new code (illustrative):**

```go
type minSizeWrap struct {
	widget.BaseWidget
	inner fyne.CanvasObject
	min   fyne.Size
}

func newMinSizeWrap(inner fyne.CanvasObject, min fyne.Size) *minSizeWrap {
	w := &minSizeWrap{inner: inner, min: min}
	w.ExtendBaseWidget(w)
	return w
}

func (m *minSizeWrap) MinSize() fyne.Size {
	base := m.inner.MinSize()
	w, h := base.Width, base.Height
	if m.min.Width > w {
		w = m.min.Width
	}
	if m.min.Height > h {
		h = m.min.Height
	}
	return fyne.NewSize(w, h)
}

func (m *minSizeWrap) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.inner)
}
```

> Note: `widget.NewSimpleRenderer(obj)` already implements `Objects()=[obj]`, `Layout(size)=obj.Resize(size)`, `MinSize()=obj.MinSize()`. If the wrapper's own `MinSize()` floor must be honored by the renderer, use a custom renderer returning `m.MinSize()` from `MinSize()`; otherwise `NewSimpleRenderer` is sufficient because the widget's `MinSize()` is what the layout queries. Prefer the explicit custom renderer to guarantee the floor is respected by the canvas.

### Phase 2: Bug 2 — GUI build + diagnostics

**Depends on:** Nothing
**Parallelizable:** Yes (vs Bug 1 files)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Backend Developer | `build.ps1` (new) | PowerShell script: `$env:CGO_ENABLED=1`; `go build -ldflags="-H windowsgui" -o vepeen.exe ./cmd/vepeen`; print result. Include a comment with the plain `go build` alternative. |
| 2.2 | Backend Developer | `cmd/vepeen/main.go` | Add `init`/startup logger: resolve `%AppData%\vepeen\vepeen.log` via `os.UserConfigDir()` (fallback `os.TempDir()`), create dir, open append file. Install `recover()` in `main()` that writes `panic` + `debug.Stack()` + timestamp to the log, then re-panic/exit. Wrap app creation/show in a guarded block that logs startup errors. |
| 2.3 | Backend Developer | `cmd/vepeen/main.go` | Keep `app.NewWithID("com.vepeen.app")`, `ui.NewMainWindow(a)`, `w.ShowAndRun()` intact; only wrap with recovery + error logging. No `fyne.Do`/threading changes. |

**Conceptual logger (illustrative, no new deps):**

```go
func logFatal(v any) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "vepeen")
	os.MkdirAll(path, 0o700)
	f, ferr := os.OpenFile(filepath.Join(path, "vepeen.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if ferr == nil {
		fmt.Fprintf(f, "%s FATAL: %v\n%s\n", time.Now().Format(time.RFC3339), v, debug.Stack())
		f.Close()
	}
}
```

### Phase 3: Documentation

**Depends on:** Phase 1 + Phase 2

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 3.1 | Documentation | `README.md` | Update "Setup / run / build" to use `.\build.ps1`; document `-H windowsgui` (no console) and the plain `go build` alternative; add a note that runtime errors/panics are written to `%AppData%\vepeen\vepeen.log` when run as a GUI `.exe`. |

### Phase 4: Review & Verification (always last)

**Depends on:** All implementation phases

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 4.1 | Debugger/Reviewer | Build with `.\build.ps1`; run `vepeen.exe`; confirm form (IP/Key/Username/Password/Rute/buttons/status/log) renders — no blank window. |
| 4.2 | Debugger/Reviewer | Confirm no `cmd.exe` console appears on launch. |
| 4.3 | Debugger/Reviewer | Confirm `go test ./internal/route ./internal/vpn` still pass; no regressions in VPN/secrets/config. |

---

## Verification

1. **Build:** `.\build.ps1` → `vepeen.exe` produced, exit 0. (Plain alt: `go build -ldflags="-H windowsgui" -o vepeen.exe ./cmd/vepeen`.)
2. **Run:** Launch `vepeen.exe` (double-click / from Explorer, not a terminal).
   - **Bug 1 check:** Window shows the full form (title "Vepeen", IP/Key/Username/Password entries, Rute block, Simpan/Hubungkan/Putuskan buttons, Status, Log). No blank area.
   - **Bug 2 check:** No separate `cmd.exe` console window opens.
3. **Diagnostics check:** Temporarily force a panic (or break startup) → `%AppData%\vepeen\vepeen.log` contains a timestamped stack; remove the trigger.
4. **Tests:** `go test ./internal/route ./internal/vpn` → pass.

---

## Acceptance Criteria

- [ ] `minSizeWrap` implements `fyne.Widget` (embeds `widget.BaseWidget`, has `CreateRenderer()`); `NewMainWindow` uses `newMinSizeWrap(...)`.
- [ ] Window content renders the full form (no blank window) when launched as a GUI `.exe`.
- [ ] `vepeen.exe` launches **without** opening a `cmd.exe` console window.
- [ ] `build.ps1` exists at repo root, sets `CGO_ENABLED=1`, and builds with `-ldflags="-H windowsgui"`.
- [ ] `cmd/vepeen/main.go` logs panics/startup errors to `%AppData%\vepeen\vepeen.log` (no new dependencies).
- [ ] `README.md` Setup section documents `.\build.ps1`, the `-H windowsgui` flag, and the log file location.
- [ ] App ID `com.vepeen.app` and `FyneApp.toml` unchanged; `fyne.Do` usage unchanged; CGo still required.
- [ ] `go test ./internal/route ./internal/vpn` passes; VPN/secrets/config/route logic untouched.

---

## Regression Risk

- **Bug 1:** Custom renderer must call `inner.Resize(size)` so nested scroll/layout still works; a no-op `Layout` would collapse children. Verify scroll + log still size correctly.
- **Bug 2:** `-H windowsgui` hides stdout/stderr — without the file logger, future runtime failures are silent. The logger must not itself panic on missing `%AppData%` (fallback to `TempDir`).
- No changes to VPN/route/secrets/config, so those paths carry no regression risk from this plan.

## Rollback Strategy

- Revert `internal/ui/main_window.go`, `cmd/vepeen/main.go`, `build.ps1`, and `README.md` to pre-change state via VCS. The previous `minSizeWrap` (interface-embed) is the only regression risk and is fully reversible by restoring the struct literal.
