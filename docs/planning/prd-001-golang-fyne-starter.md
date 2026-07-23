# PRD-001: Golang + Fyne Desktop Starter (vepeen)

**Version:** v0.1.0  
**Status:** Draft  
**Author:** Planner Agent  
**Created:** 2026-07-22  
**Updated:** 2026-07-22  

---

## Overview

Scaffold a greenfield desktop application named **vepeen** using Go and the Fyne v2 GUI toolkit. The deliverable is a minimal but complete, runnable Windows-friendly starter: module setup, a main window with basic interactive UI, and a README covering prerequisites and run steps so the project can be extended immediately.

## Problem Statement

The workspace `d:\1.Project\vepeen` is empty. The user needs a working Go + Fyne project foundation (not a blank folder) so they can run a GUI app on Windows and grow features without redoing bootstrap, dependency, or layout decisions.

## Goals

- Initialize a Go module named `vepeen` with Fyne v2 as a dependency.
- Deliver a runnable desktop app: titled window, label, and interactive button (or equivalent simple form).
- Provide a small, extensible package layout suitable for future UI and app logic growth.
- Document Windows run instructions and Fyne native toolchain requirements (Go + C compiler / CGo).
- Keep scope to a solid starter — no product features, CI, Docker, or packaging pipeline.

## Non-Goals

- Application-specific business features beyond the demo UI.
- Mobile/web targets, packaging/installers, or code signing.
- CI/CD, Docker, Makefile-heavy DevOps, or release automation.
- Full design system, theming engine, or multi-window architecture.
- Internationalization, persistence, networking, or auth.
- Using Fyne v1 APIs.

---

## Feature Specification

### User Stories

- As a developer, I want a Go module with Fyne wired up, so that I can run a GUI app with standard Go tooling.
- As a developer, I want a visible main window with basic controls, so that I can confirm the stack works on Windows.
- As a developer, I want a clear README with Windows prerequisites and run commands, so that I can onboard without hunting docs.
- As a developer, I want a simple folder layout, so that I can add screens and logic without restructuring from scratch.

### Acceptance Criteria

- [ ] Repository is no longer empty: `go.mod`, application source, and `README.md` exist.
- [ ] Module path is `vepeen`.
- [ ] `go.mod` declares a modern Go version (1.21+ recommended) and depends on `fyne.io/fyne/v2`.
- [ ] `go.sum` is generated after dependency resolution.
- [ ] App uses Fyne **v2** APIs only (`fyne.io/fyne/v2/...`).
- [ ] Running the app opens a desktop window titled **Vepeen** (or equivalent product title).
- [ ] Window shows at least one label and one interactive control (button recommended); interaction updates UI state (e.g. label text).
- [ ] Layout is readable on a default desktop window size (not a single unstyled dump of widgets without container layout).
- [ ] Project structure separates entrypoint from UI composition in a way that is easy to extend.
- [ ] `README.md` documents: prerequisites (Go, C compiler/CGo on Windows), install/run commands, and basic troubleshooting for common Windows Fyne build issues.
- [ ] No CI/Docker/infra files are introduced unless required for the app to compile (they are not required).

---

## Technical Design

### Architecture Overview

Single-process desktop GUI app:

```
main (entrypoint)
  → creates Fyne app + main window
  → builds UI via internal UI package
  → ShowAndRun event loop
```

No backend server, database, or IPC in this phase.

### Codebase Context

- Workspace is empty — no existing patterns, modules, or configs.
- Stack is fixed by request: **Go + Fyne**.
- Prefer official Fyne v2 hello-world patterns: `app.New()`, `NewWindow`, `SetContent`, `ShowAndRun`, `widget` + `container` layouts.

### Defaults (non-blocking decisions)

| Decision | Default | Rationale |
| -------- | ------- | --------- |
| Module name | `vepeen` | Matches workspace folder; simple local module path |
| App / window title | `Vepeen` | Product-facing name from folder |
| Fyne major | v2 | Modern API as required |
| Go version in go.mod | `1.21` or newer available on machine | Fyne requires Go ≥ 1.19; 1.21+ is a safe modern baseline |
| Entry layout | `cmd/vepeen/main.go` + `internal/ui` | Conventional Go layout; keeps root clean and UI testable/extendable |
| Demo UI | Label + Button in a vertical box (optional centered content) | Matches Fyne starter examples; proves interactivity |
| Designer phase | Optional / light constraints only | Minimal starter; no full design sprint |
| Primary implementer | Backend Developer | Go desktop code; Fyne UI is still Go |
| DevOps | Out of scope | Explicitly not requested |

### Data Model

None for this phase. UI state may be in-memory widget state only (e.g. label text).

### API Changes

None (no HTTP/RPC surface).

### UI Changes

**Main window**

- Title: `Vepeen`
- Content: simple vertical layout containing:
  - A heading/label (e.g. welcome text)
  - A button that updates the label (or similar clear feedback)
- Reasonable default size so content is not cramped (implementer chooses a sensible `Resize` if needed)
- No multi-page navigation required

**Design constraints (light, no separate Designer phase required)**

- Prefer Fyne layout containers (`VBox` / `Center` / padding) over absolute positioning.
- Keep copy short and neutral English (or bilingual only if trivial); default English is fine.
- Do not invent a custom theme unless trivial; system/default Fyne theme is acceptable.

### Project Structure (target)

```
vepeen/
├── cmd/
│   └── vepeen/
│       └── main.go          # Entrypoint: app lifecycle only
├── internal/
│   └── ui/
│       └── main_window.go   # Window + widget composition
├── docs/
│   └── planning/
│       ├── prd-001-golang-fyne-starter.md
│       └── changelog.md
├── go.mod
├── go.sum
└── README.md
```

**Notes**

- `internal/ui` holds window construction so future screens/widgets can grow without bloating `main`.
- Do not add unused packages (no empty `pkg/`, no fake services).
- Optional later (out of scope now): `assets/`, `internal/app`, themes, multi-window.

### Dependencies

- Runtime/library: `fyne.io/fyne/v2` (and its transitive modules via `go get` / `go mod tidy`).
- System (Windows development):
  - Go toolchain installed and on `PATH`.
  - C compiler for CGo (Fyne uses native graphics). Common options: **MSYS2 MinGW-w64 toolchain**, or another supported GCC setup.
  - Graphics drivers / OpenGL-capable environment as required by Fyne on the host.
- End users of a built binary typically do not need Go/GCC; developers do.

### README Requirements

Must include:

1. What the project is (Go + Fyne desktop starter).
2. Prerequisites on Windows (Go version, C compiler / MSYS2 notes, PATH tips).
3. Setup: module already present; how to fetch deps (`go mod tidy`).
4. Run: e.g. `go run ./cmd/vepeen`.
5. Build: e.g. `go build -o vepeen.exe ./cmd/vepeen`.
6. Troubleshooting bullets for common failures (missing gcc, CGo disabled, PATH without mingw).

---

## Implementation Plan

### Phase 1: Project Scaffold & Runnable UI

**Depends on:** Nothing  
**Parallelizable:** Partially — module/README can be drafted alongside UI code if file ownership does not overlap; prefer single implementer for this small scope.

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `go.mod`, `go.sum` | Initialize module `vepeen`, add Fyne v2 dependency, tidy modules. |
| 1.2 | Backend Developer | `cmd/vepeen/main.go` | Create entrypoint that starts the Fyne app and shows the main window. |
| 1.3 | Backend Developer | `internal/ui/main_window.go` | Build main window content: title, label, button (or simple form), layout containers, interactive update. |
| 1.4 | Backend Developer | `README.md` | Document prerequisites, Windows CGo/toolchain notes, run/build commands, basic troubleshooting. |

**Sub-Agent Guidance:**

- Tasks 1.1–1.3 are sequential in practice (module before compile; UI before wiring in main).
- Task 1.4 can start after structure is known; finalize after the run command is verified.
- No Designer agent required; apply light design constraints above.
- No DevOps agent.

**Handoff notes (WHAT, not HOW):**

- Deliver a project that compiles and launches a GUI on Windows when prerequisites are met.
- Use Fyne v2 packages only.
- Keep `main` thin; put UI composition under `internal/ui`.
- Demo interaction must be obvious without reading source.
- Do not add CI, Docker, or extra frameworks.

### Phase 2: Review & Documentation

**Depends on:** Phase 1

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Debugger/Reviewer | All created sources + `go.mod` | Verify acceptance criteria: module name, Fyne v2 usage, window behavior, structure, no scope creep. |
| 2.2 | Documentation | `README.md` | Ensure README matches actual commands/paths and Windows notes are accurate enough to run. |

**Security review:** Not required for this starter (no auth, network, or sensitive data handling). Revisit if later features add those.

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Missing C compiler / CGo on Windows blocks `go run` | High | High for fresh machines | Document MSYS2/MinGW (or equivalent) clearly in README; state that GCC must be on PATH |
| `CGO_ENABLED=0` environment prevents build | High | Medium | Call out in troubleshooting that CGo must be enabled for Fyne |
| Over-scoping starter into full app architecture | Medium | Medium | Stick to entrypoint + `internal/ui` + demo widgets only |
| Wrong Fyne major / outdated import paths | Medium | Low | Require `fyne.io/fyne/v2` imports only |
| Module path later needs GitHub remote | Low | Medium | Local `vepeen` is fine; can re-module later if published |

## Rollback Strategy

- Greenfield: if scaffold is wrong, delete generated app files (`cmd/`, `internal/`, `go.mod`, `go.sum`, `README.md`) and re-run Phase 1.
- Planning docs under `docs/planning/` can remain as history.
- No production data or migrations involved.

---

## Open Questions

None blocking. Defaults above are approved for implementation:

- Module: `vepeen`
- Title: `Vepeen`
- Layout: `cmd/vepeen` + `internal/ui`
- Demo: label + button

---

## Implementation Summary (for Orchestrator)

**PRD path:** `docs/planning/prd-001-golang-fyne-starter.md`  
**Scope:** Major — greenfield Go + Fyne v2 desktop starter for Windows  
**Primary implementer:** Backend Developer  
**Ordered agent pipeline:**

1. **Backend Developer** — scaffold module, app entrypoint, UI, README  
2. **Debugger/Reviewer** — verify acceptance criteria and runnable structure  
3. **Documentation** — polish README accuracy (can merge lightly with Backend if single pass is cleaner)

**Files to create:**

| File | Purpose |
| ---- | ------- |
| `go.mod` | Module `vepeen` + Go version |
| `go.sum` | Dependency checksums |
| `cmd/vepeen/main.go` | Application entrypoint |
| `internal/ui/main_window.go` | Main window / UI composition |
| `README.md` | Windows-focused setup and run docs |

**Files already created by Planner:**

| File | Purpose |
| ---- | ------- |
| `docs/planning/prd-001-golang-fyne-starter.md` | This PRD |
| `docs/planning/changelog.md` | Planning changelog |

**Out of scope files:** CI configs, Docker, Makefiles (unless implementer needs a tiny helper — prefer pure `go` commands in README).

---

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v0.1.0 | 2026-07-22 | Initial draft for greenfield Go + Fyne starter |
