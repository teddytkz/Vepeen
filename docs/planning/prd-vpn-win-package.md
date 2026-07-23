# PRD: Group Windows VPN code into `internal/vpn/win`

**Version:** v1.0.0
**Status:** Draft
**Author:** Planner Agent
**Created:** 2026-07-23
**Updated:** 2026-07-23

---

## Overview

`vepeen` is a Windows-only L2TP/IPsec VPN client (Fyne v2 GUI) that orchestrates the OS-native Windows VPN stack via PowerShell, `rasdial.exe`, and Win32 APIs. Today all Windows-specific VPN logic lives directly in `package vpn` under `internal/vpn/*_windows.go`, intermixed with platform-agnostic orchestration. This PRD groups every Windows-only VPN implementation into a dedicated `internal/vpn/win` package (`package win`), leaving `vpn` as a thin, platform-neutral facade. This is a pure refactor: no behavior changes, no new features.

## Problem Statement

- The `vpn` package mixes OS-specific code (registry, iphlpapi, PowerShell, rasdial) with cross-platform orchestration (`Manager`, types, errors). This makes the eventual Linux port (`vpn/linux`) harder: there is no clean seam to drop a second implementation beside the Windows one.
- The existing `Manager` function-pointer seam (`connectFn`, `syncRoutesFn`, `natCheckFn`, `ensureSplitTunnelingFn`) already isolates the *connect* path, but the rest of the public API (`QueryStatus`, `ListProfiles`, `TrafficCounters`, `ActiveConnections`, `Disconnect`, etc.) is implemented inline in `vpn` and must be duplicated by hand in `stub_other.go` for non-Windows builds.
- Consolidating the Windows cluster into `vpn/win` makes the platform boundary explicit, removes the need to scatter `//go:build windows` files across `vpn`, and gives Linux a clear sibling package to fill in later.

## Goals

- Move all 9 Windows-only VPN files into `internal/vpn/win` (`package win`).
- Keep `vpn` as the public API surface: every symbol currently exported from `vpn` (and consumed by `internal/ui`) remains exported from `vpn`, delegating to `win` on Windows and to `stub_other.go` elsewhere.
- Preserve the `Manager` function-pointer seam; wire it to `win.*` on Windows and to `stub_other.go` impls otherwise.
- No change to runtime behavior, error messages, or Indonesian strings.
- `go build`, `go vet`, `go test ./...` pass on Windows; `GOOS=linux go build ./...` succeeds via the stub path without compiling `win`.

## Non-Goals

- Implementing the Linux backend (`vpn/linux`) — future work.
- Moving `internal/route`, `internal/config`, `internal/secrets`, or `internal/ui` Windows pairs into their own `win` subpackages — future work (consistent pattern, not in scope here).
- Deleting or migrating `internal/secrets/store_windows.go` — explicitly left as-is (migration-only read-only).
- Adding new functionality or changing any public signature.

---

## Feature Specification

### User Stories

- As a maintainer, I want all Windows VPN code isolated in `vpn/win`, so that adding `vpn/linux` later is a parallel, non-invasive change.
- As a Backend Developer, I want `vpn`'s public API unchanged, so that `internal/ui` and `cmd/vepeen` require no call-site edits for moved symbols.

### Acceptance Criteria

- [ ] `internal/vpn/win` contains exactly the 9 Windows files listed in Scope, each declaring `package win` and carrying `//go:build windows`.
- [ ] `internal/vpn` still exports every symbol previously exported there: `Manager`, `NewManager`, `ConnectRequest`, `Phase`, `ProgressFunc`, `ConnStatus`, `ProfileSummary`, `ConnectParams`, `ActiveConn`, `NATResult`, `UserError`, `AsUserError`, `MapExecError`, `ValidateName`, `natValueOK`, `ListProfiles`, `EnsureNATRegistry`, `Connect`, `Disconnect`, `DisconnectAllExcept`, `QueryStatus`, `ProfileExists`, `EnforceSplitTunnel`, `ProfileDiagnostics`, `EnsureSplitTunneling`, `PurgeOrphanScripts`, `TrafficCounters`, `ActiveConnections`, `PingHost`, `Status`, `DisconnectFull`, `PhaseDetail`, `ConnectFull`.
- [ ] `internal/ui/main_window.go` and `internal/ui/ping_windows.go` compile unchanged (no `vpn.` → `win.` edits required).
- [ ] `go build ./...` and `go vet ./...` succeed on Windows.
- [ ] `GOOS=linux go build ./...` succeeds; `internal/vpn/win` is not compiled (build-tag gated).
- [ ] `go test ./...` passes on Windows, including `dial_windows_test.go` and `profile_windows_test.go` (now in `win`).
- [ ] No import cycle: `vpn` imports `win`; `win` does not import `vpn`.
- [ ] Legacy `// +build windows` tags are removed; only `//go:build windows` remains.

---

## Technical Design

### Architecture Overview

`vpn` becomes a facade. It owns the platform-neutral types, errors, and `Manager` orchestration, and re-exports the OS-specific functions. On Windows, those re-exports call into `win.*`; on other platforms they call the `stub_other.go` no-ops. `Manager.NewManager()` wires its function pointers to `win.*` (Windows) or to the stub functions (non-Windows) — the seam already exists and only the right-hand side changes.

```
internal/vpn (package vpn)            internal/vpn/win (package win)
┌──────────────────────────┐          ┌──────────────────────────────────┐
│ status.go                │          │ status_windows.go                │
│ natt.go                  │          │ connections_windows.go            │
│ errors.go                │          │ dial_windows.go                  │
│ manager.go  (facade +   │  imports │ disconnectall_windows.go         │
│   re-exports + Manager)  │ ──────▶ │ natt_windows.go                 │
│ stub_other.go (!windows) │          │ netapi_windows.go                │
│ (re-exports → stub)     │          │ powershell_windows.go            │
└──────────────────────────┘          │ profile_windows.go               │
                                      │ traffic_windows.go               │
                                      └──────────────────────────────────┘
```

### Codebase Context (verified)

- `manager.go` `NewManager()` currently assigns `connectFn: Connect`, `natCheckFn: EnsureNATRegistry`, `ensureSplitTunnelingFn: EnsureSplitTunneling`, `syncRoutesFn: route.SyncRoutes`. `ConnectFull` also calls `DisconnectAllExcept`, `EnforceSplitTunnel`, and `m.Status` → `QueryStatus` directly (not via pointers).
- `stub_other.go` (`//go:build !windows`) provides no-op versions of all 18 public functions plus `unsupported()`.
- `internal/ui/main_window.go` references the moved symbols **only** through the `vpn.` prefix (e.g. `vpn.ListProfiles`, `vpn.TrafficCounters`, `vpn.ActiveConnections`, `vpn.PurgeOrphanScripts`, `vpn.QueryStatus` via `m.Status`, `vpn.PingHost`). `internal/ui/ping_windows.go` calls `vpn.PingHost`. None reference `win.` — so keeping `vpn.*` re-exports satisfies them with zero UI edits.
- `internal/config`, `internal/secrets`, `internal/route` do **not** import `vpn`, so they are unaffected.
- `cmd/vepeen/main.go` imports only `internal/ui` — unaffected.
- `errors.go` (`UserError`, `AsUserError`, `MapExecError`, `sanitizeOutput`, `ValidateName`, `newUserError`) and `manager.go` (`sanitizeDetail`) are **agnostic** and stay in `vpn`. The Windows files call these via the `vpn.` prefix after the move (e.g. `vpn.ValidateName`, `vpn.MapExecError`, `vpn.newUserError`, `vpn.sanitizeOutput`, `vpn.sanitizeDetail`).

### Data Model

No data-model changes. `ProfileSummary`, `ConnectParams`, `ActiveConn`, `ConnStatus`, `NATResult` remain defined in `vpn` (they are platform-neutral types).

### API Changes

None at the public boundary. The internal package boundary changes: symbols move from `vpn` → `win`, but `vpn` re-exports them. The `internal/ui` and `cmd` consumers see identical symbols.

### UI Changes

None. `internal/ui` is untouched (it only uses `vpn.` symbols, which remain exported).

---

## Implementation Plan

### Phase 1: Create `internal/vpn/win` and move files

**Depends on:** Nothing
**Parallelizable:** No (single cohesive move; must be done together to keep the build green)

| Task | Agent | Files | Description |
| ---- | ----- | ------ | ----------- |
| 1.1 | Backend Developer | `internal/vpn/win/{status_windows,connections_windows,dial_windows,disconnectall_windows,natt_windows,netapi_windows,powershell_windows,profile_windows,traffic_windows}.go` | Create `internal/vpn/win/`, move the 9 files in, change `package vpn` → `package win`, keep `//go:build windows` (drop legacy `// +build windows`), add `import "vepeen/internal/vpn"` where needed, and prefix all references to agnostic helpers with `vpn.` (`ValidateName`, `MapExecError`, `newUserError`, `sanitizeOutput`, `sanitizeDetail`, `NATResult`, `NATOK`, `NATElevationRequired`, `NATSet`, `natValueOK`, `natValueTarget`, `ProfileSummary`, `ConnStatus`, `StatusUnknown`, `ConnectParams`, `ActiveConn`). |
| 1.2 | Backend Developer | `internal/vpn/win/*_test.go` | Move `dial_windows_test.go` and `profile_windows_test.go` into `win`; change `package vpn` → `package win`. They reference only `win`-local symbols (`evaluateRasdialResult`, `enforceSplitTunnelScript`) so no `vpn.` prefix needed. |

**Sub-Agent Guidance:**

- Task 1.1 is atomic — all 9 files must move together or the package won't compile.
- Within 1.1, the `vpn.` prefix additions are mechanical: search each moved file for the agnostic symbols listed above and qualify them. `NATResult`/`NATOK`/`NATElevationRequired`/`NATSet`/`natValueOK`/`natValueTarget` are defined in `natt.go` (stays in `vpn`); `ProfileSummary`/`ConnStatus`/`StatusUnknown`/`ConnectParams`/`ActiveConn` are in `status.go` (stays in `vpn`).

### Phase 2: Re-export from `vpn` and wire `NewManager`

**Depends on:** Phase 1
**Parallelizable:** No

| Task | Agent | Files | Description |
| ---- | ----- | ------ | ----------- |
| 2.1 | Backend Developer | `internal/vpn/stub_other.go` | Keep as-is (already provides all 18 no-op re-exports for `!windows`). No change. |
| 2.2 | Backend Developer | `internal/vpn/win_exports_windows.go` (NEW, `//go:build windows`) | Add a new file in `vpn` that re-exports every moved symbol by delegating to `win`: `func ListProfiles() ([]ProfileSummary, error) { return win.ListProfiles() }`, and similarly for `EnsureNATRegistry`, `Connect`, `Disconnect`, `DisconnectAllExcept`, `QueryStatus`, `ProfileExists`, `EnforceSplitTunnel`, `ProfileDiagnostics`, `EnsureSplitTunneling`, `PurgeOrphanScripts`, `TrafficCounters`, `ActiveConnections`, `PingHost`. This file imports `vepeen/internal/vpn/win`. |
| 2.3 | Backend Developer | `internal/vpn/manager.go` | Update `NewManager()` to wire Windows impls: `connectFn: win.Connect`, `natCheckFn: win.EnsureNATRegistry`, `ensureSplitTunnelingFn: win.EnsureSplitTunneling`. Add `import "vepeen/internal/vpn/win"` (build-tag gated — see note). `syncRoutesFn` stays `route.SyncRoutes`. On non-Windows, `win` is not compiled, so these references must be guarded: see Build-tag strategy below. |

**Sub-Agent Guidance:**

- The cleanest way to avoid a non-Windows compile break in 2.3 is to keep `NewManager()` in `manager.go` (agnostic) calling the `vpn.`-level re-exports (`connectFn: Connect`, etc.) rather than `win.Connect` directly. Because `Connect` is re-exported by `win_exports_windows.go` (Windows) and `stub_other.go` (non-Windows), `manager.go` needs **no** `win` import and **no** build-tag change. This is the recommended approach and keeps `vpn` import-cycle-free.
- Alternative (only if direct `win.` wiring is desired): split `NewManager()` into `newManagerWindows` (`//go:build windows`, imports `win`) and `newManagerOther` (`//go:build !windows`, uses stubs), both called from an agnostic wrapper. This adds files; the re-export approach (preferred) avoids it.

### Phase 3: Duplication consolidation (FOLLOW-UP — out of scope for this PRD)

**Depends on:** Phase 2
**Parallelizable:** N/A

`psQuote` and `runPowerShell` are duplicated between `internal/vpn/powershell_windows.go` and `internal/route/sync_windows.go` (route's version adds `sanitizePSError`). **Decision: defer to a separate follow-up PRD.** This PRD keeps `psQuote`/`runPowerShell` inside `win` (moved with `powershell_windows.go`) and leaves `internal/route` using its own copy. A later change can extract a shared `internal/winutil` package and update both `win` and `route` to import it. Not blocking for the Linux-prep goal.

### Phase 4: Review & Verification (Always Last)

**Depends on:** Phases 1–3

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 4.1 | Debugger/Reviewer | Run `go build ./...`, `go vet ./...`, `go test ./...` on Windows; confirm all acceptance criteria. Verify `internal/ui` needed zero edits. |
| 4.2 | Debugger/Reviewer | Run `GOOS=linux go build ./...` and confirm `internal/vpn/win` is excluded (build-tag gated) and the stub path compiles. |
| 4.3 | Documentation | Update `docs/planning/changelog.md` with the refactor entry (no user-facing doc changes needed). |

---

## Target Package Layout

```
internal/vpn/
├── manager.go              (package vpn)  — Manager, NewManager, ConnectFull, DisconnectFull, Status, PhaseDetail, sanitizeDetail
├── status.go               (package vpn)  — ConnStatus, ProfileSummary, ConnectParams, ActiveConn
├── errors.go               (package vpn)  — UserError, AsUserError, MapExecError, sanitizeOutput, ValidateName, newUserError
├── natt.go                 (package vpn)  — NATResult, natValueOK, natValueTarget, natPolicyAgentKey, natValueName
├── stub_other.go           (package vpn, //go:build !windows) — 18 no-op re-exports + unsupported()
├── win_exports_windows.go  (package vpn, //go:build windows)   — NEW: 14 delegating re-exports → win.*
├── manager_test.go         (package vpn)  — cross-platform
├── errors_test.go          (package vpn)  — cross-platform
├── natt_test.go            (package vpn)  — cross-platform
└── win/                    (package win, //go:build windows)
    ├── status_windows.go
    ├── connections_windows.go
    ├── dial_windows.go
    ├── disconnectall_windows.go
    ├── natt_windows.go
    ├── netapi_windows.go
    ├── powershell_windows.go
    ├── profile_windows.go
    ├── traffic_windows.go
    ├── dial_windows_test.go
    └── profile_windows_test.go
```

---

## Wiring Design

`vpn` re-exports the OS-specific functions. On Windows, `win_exports_windows.go` delegates to `win`; on other platforms, `stub_other.go` returns `unsupported()`. `Manager.NewManager()` keeps referencing the `vpn.`-level names, so it needs no `win` import and no build tags:

```go
// internal/vpn/win_exports_windows.go  (//go:build windows)
package vpn

import "vepeen/internal/vpn/win"

func ListProfiles() ([]ProfileSummary, error)        { return win.ListProfiles() }
func EnsureNATRegistry() (NATResult, error)          { return win.EnsureNATRegistry() }
func Connect(p ConnectParams) error                   { return win.Connect(p) }
func Disconnect(name string) error                    { return win.Disconnect(name) }
func DisconnectAllExcept(e string) ([]string, error) { return win.DisconnectAllExcept(e) }
func QueryStatus(name string) (ConnStatus, error)    { return win.QueryStatus(name) }
func ProfileExists(name string) (bool, error)         { return win.ProfileExists(name) }
func EnforceSplitTunnel(name string) (string, error) { return win.EnforceSplitTunnel(name) }
func ProfileDiagnostics(name string) (string, error) { return win.ProfileDiagnostics(name) }
func EnsureSplitTunneling(name string) error         { return win.EnsureSplitTunneling(name) }
func PurgeOrphanScripts()                             { win.PurgeOrphanScripts() }
func TrafficCounters(name string) (uint64, uint64, error) { return win.TrafficCounters(name) }
func ActiveConnections(name string) ([]ActiveConn, error) { return win.ActiveConnections(name) }
func PingHost(host string, ms uint32) (uint32, error) { return win.PingHost(host, ms) }
```

```go
// internal/vpn/manager.go  (unchanged NewManager — references vpn.-level names)
func NewManager() *Manager {
    return &Manager{
        syncRoutesFn:           route.SyncRoutes,
        connectFn:              Connect,              // vpn.Connect → win on Windows, stub otherwise
        natCheckFn:             EnsureNATRegistry,    // vpn.EnsureNATRegistry
        ensureSplitTunnelingFn: EnsureSplitTunneling, // vpn.EnsureSplitTunneling
    }
}
```

`internal/ui` continues to call `vpn.ListProfiles()`, `vpn.TrafficCounters()`, `vpn.ActiveConnections()`, `vpn.PingHost()`, `vpn.PurgeOrphanScripts()`, and `m.Status()` → `vpn.QueryStatus()` — all still exported from `vpn`. **No UI edits required.**

---

## Build-Tag Strategy

- All 9 files in `internal/vpn/win` carry `//go:build windows` (legacy `// +build windows` removed).
- `internal/vpn/win_exports_windows.go` carries `//go:build windows` and imports `win`.
- `internal/vpn/stub_other.go` carries `//go:build !windows` and provides the same 18 symbols as no-ops.
- `manager.go`, `status.go`, `errors.go`, `natt.go` are build-tag-free (compile on every platform).
- **No import cycle:** `vpn` imports `win` (via `win_exports_windows.go`); `win` imports `vpn` only for the agnostic helpers it needs (`vpn.ValidateName`, `vpn.MapExecError`, `vpn.newUserError`, `vpn.sanitizeOutput`, `vpn.sanitizeDetail`, and the types in `status.go`/`natt.go`). Go permits this mutual reference **only** because `win` is never compiled on non-Windows and `vpn` references `win` solely inside a `//go:build windows` file — there is no cycle on any single build target. To be safe and explicit, `win` should import `vpn` and `vpn` import `win` only within their respective `windows`-gated files; the cross-package type/func usage is one-directional per build.

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Import cycle (`vpn` ↔ `win`) breaks build | High | Low | Keep `win`'s `vpn.` references to agnostic helpers; keep `vpn`'s `win.` references inside `//go:build windows` files only. Verify with `go build ./...` and `GOOS=linux go build ./...`. |
| Forgotten `vpn.` prefix on a moved symbol → "undefined" compile error | Med | Med | Grep each moved file for `ValidateName`, `MapExecError`, `newUserError`, `sanitizeOutput`, `sanitizeDetail`, `NATResult`, `NATOK`, `NATElevationRequired`, `NATSet`, `natValueOK`, `natValueTarget`, `ProfileSummary`, `ConnStatus`, `StatusUnknown`, `ConnectParams`, `ActiveConn` and qualify with `vpn.`. |
| `NewManager()` wired to `win.Connect` directly → non-Windows compile break | Med | Med | Prefer the re-export approach (2.3 note): `manager.go` references `vpn.`-level names only; no `win` import in `manager.go`. |
| Test package mismatch (`package vpn` vs `package win`) | Low | Low | Move `dial_windows_test.go`/`profile_windows_test.go` to `win` and update their `package` decl. |
| Legacy `// +build windows` left behind → tooling warnings | Low | Low | Drop legacy tags during the move (Task 1.1). |
| `internal/ui` accidentally edited | Low | Low | Not required — `vpn.*` re-exports preserve the API. Verify with a no-diff check on `internal/ui`. |

---

## Rollback Strategy

The change is a self-contained refactor confined to `internal/vpn` and the new `internal/vpn/win`. Rollback = `git revert` of the single commit (or restore the pre-move file set). No migrations, no config changes, no external side effects. Because behavior is unchanged, no data or state rollback is needed.

---

## Migration / Verification Steps

1. `go build ./...` — must succeed on Windows (compiles `vpn` + `win`).
2. `go vet ./...` — no warnings.
3. `go test ./...` — all pass, including `win` package tests (`dial_windows_test.go`, `profile_windows_test.go`) and `vpn` cross-platform tests (`manager_test.go`, `errors_test.go`, `natt_test.go`).
4. `GOOS=linux go build ./...` — must succeed; confirm `internal/vpn/win` is excluded (build-tag gated) and the stub path compiles.
5. `git diff --stat internal/ui` — expect **no changes** to `internal/ui/main_window.go` or `internal/ui/ping_windows.go`.
6. Manual smoke (Windows): launch `bin/vepeen.exe`, list profiles, connect, verify traffic counters and active connections still populate.

---

## Out of Scope / Future Work

- **Linux backend (`vpn/linux`)** — implement `ListProfiles`, `Connect`, `Disconnect`, `QueryStatus`, `TrafficCounters`, `ActiveConnections`, `PingHost`, `EnsureNATRegistry` (no-op), `EnforceSplitTunnel`, `ProfileDiagnostics`, `EnsureSplitTunneling`, `DisconnectAllExcept`, `PurgeOrphanScripts`, `ProfileExists` via `nmcli`/`NetworkManager` or `strongswan` + `ip`. Then add `linux_exports_linux.go` in `vpn` mirroring `win_exports_windows.go`.
- **Shared `internal/winutil`** — extract `psQuote`/`runPowerShell` (and `sanitizePSError`) out of `win` and `internal/route` into one package; update both importers. Deferred (Phase 3).
- **Apply the same `win` subpackage pattern to `route`, `config`, `secrets`, `ui`** — e.g. `internal/route/win`, `internal/ui/win`. Consistent future direction, NOT part of this PRD.
- **Deletion of `internal/secrets/store_windows.go`** — explicitly NOT done here (migration-only read-only); revisit after `prd-004` encrypted-config migration is fully retired.

---

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-23 | Initial PRD: group Windows VPN code into `internal/vpn/win`. |
