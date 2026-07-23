# PRD-003: Global Routes (replace per-profile routes map)

**Version:** 1.0.0
**Status:** Draft
**Author:** Planner Agent
**Created:** 2026-07-23
**Updated:** 2026-07-23

---

## Overview

Replace the per-Windows-profile `profiles` map with a single global `routes` list on `Config`. Routes become a single shared split-tunnel list that applies to whichever Windows VPN profile is selected, instead of being stored per profile.

## Problem Statement

Currently `Config.Profiles` is a `map[string]ProfileEntry` where each entry holds its own `Routes []string`. The profile dropdown is populated from the OS (`vpn.ListProfiles()`), not from `cfg.Profiles`, so the per-profile map is redundant bookkeeping. The user wants one global routes list — "just make it global" — so changing the profile dropdown no longer changes the routes text.

## Goals

- Remove `Profiles` map and `ProfileEntry` type from `internal/config/config.go`.
- Add a single global `Routes []string` field on `Config`.
- UI routes entry always shows the global list; profile change does NOT re-populate routes.
- Backward-compatible migration from old `profiles` shape to new `routes` shape.

## Non-Goals

- Changing how the profile dropdown is populated (still `vpn.ListProfiles()`).
- Changing route parsing/resolution/sync (`internal/route/*`) — operates on `[]string`, unaffected.
- Changing `vpn.ConnectRequest.Routes` consumption in `internal/vpn/manager.go` — already global per connect.
- Changing secrets handling.

---

## Feature Specification

### User Stories

- As a user, I want one shared split-tunnel routes list so I don't have to re-enter routes per profile.
- As a user, I want switching the VPN profile to keep my routes unchanged.

### Acceptance Criteria

- [ ] `config.json` on disk uses `"routes": [...]` at top level and has no `"profiles"` key after first save.
- [ ] Loading an old `profiles`-shaped config migrates routes into the global `Routes` without data loss.
- [ ] `applyConfig` pre-fills `routesEntry` from `cfg.Routes` (global), not per-profile.
- [ ] `onProfileChanged` does NOT modify `routesEntry` text.
- [ ] `onSave` and `persistQuiet` write `cfg.Routes` (global), not `cfg.Profiles[name]`.
- [ ] `go build -o vepeen.exe ./cmd/vepeen` succeeds; `go vet ./internal/config/...` clean.
- [ ] New unit test covers old→new migration and new-shape load.

---

## Technical Design

### Architecture Overview

`Config` is the single source of non-secret persisted settings. `internal/ui/main_window.go` reads/writes it; `internal/vpn/manager.go` only consumes `ConnectRequest.Routes` (already a flat `[]string`). The change is confined to the config schema + the four UI touch points that read/write routes.

### Codebase Context

- `internal/config/config.go`: struct (lines 21-37), `Default()` (lines 39-44), `configWire` (lines 119-123), `parseConfig` (lines 125-160), `Save` (lines 162-194), `cleanRoutes` (lines 196-208).
- `internal/ui/main_window.go`: `applyConfig` (lines 301-307), `onProfileChanged` (lines 311-316), `onSave` (lines 548-550), `persistQuiet` (lines 793-803).
- `internal/vpn/manager.go:91` consumes `req.Routes` — no change needed.
- No existing config tests (`internal/config/*_test.go` absent) — add one.

### Data Model

**New `Config` struct** (replaces lines 21-37):

```go
// Config holds non-secret settings persisted as JSON. The app no longer creates
// VPN profiles or stores PSK; it only manages split-tunnel routing for a
// selected, pre-existing Windows VPN connection. Routes are GLOBAL — a single
// shared split-tunnel list applied to whichever profile is selected.
type Config struct {
	SelectedProfile     string   `json:"selectedProfile"`
	Routes              []string `json:"routes"`
	RememberCredentials bool     `json:"rememberCredentials"`
}
```

`ProfileEntry` type is deleted entirely.

### API Changes

None (config is internal JSON only).

### UI Changes

- `applyConfig`: set `routesEntry` from `cfg.Routes` (global).
- `onProfileChanged`: remove the routes re-population block; only update `connectionName` + `loadCredentials()`.
- `onSave`: write `cfg.Routes = routes` instead of `cfg.Profiles[name] = ProfileEntry{Routes: routes}`.
- `persistQuiet`: write `cur.Routes = prefixes` instead of `cur.Profiles[req.Name] = ...`.

---

## Implementation Plan

### Phase 1: Config schema + migration (Backend)

**Depends on:** Nothing
**Parallelizable:** No (UI depends on new struct)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/config/config.go` | Delete `ProfileEntry` (lines 21-23). Replace `Config` struct (lines 25-37) with new shape (global `Routes []string`, drop `Profiles`). |
| 1.2 | Backend Developer | `internal/config/config.go` | Update `Default()` (lines 39-44): `Routes: []string{}` instead of `Profiles: map[...]{}`. |
| 1.3 | Backend Developer | `internal/config/config.go` | Update `configWire` (lines 119-123): replace `Profiles map[string]ProfileEntry` with `Routes []string`. |
| 1.4 | Backend Developer | `internal/config/config.go` | Rewrite `parseConfig` (lines 125-160) to handle both shapes (see Migration below). |
| 1.5 | Backend Developer | `internal/config/config.go` | Update `Save` (lines 162-194): remove `if cfg.Profiles == nil { cfg.Profiles = ... }` guard; marshal `cfg` directly (global `Routes` already serialized). |
| 1.6 | Backend Developer | `internal/config/config_test.go` (NEW) | Add tests: (a) new-shape load returns `Routes`; (b) old `profiles` shape migrates via selectedProfile; (c) old shape with missing selectedProfile unions all; (d) `Default()` has empty `Routes`. |

### Phase 2: UI wiring (Frontend)

**Depends on:** Phase 1

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Frontend Developer | `internal/ui/main_window.go` | `applyConfig` (lines 301-307): replace per-profile lookup with `c.routesEntry.SetText(strings.Join(cfg.Routes, "\n"))` (guard `len(cfg.Routes) > 0`). |
| 2.2 | Frontend Developer | `internal/ui/main_window.go` | `onProfileChanged` (lines 311-316): delete the `if entry, ok := c.cfg.Profiles[...]` block; keep `connectionName` set + `loadCredentials()`. |
| 2.3 | Frontend Developer | `internal/ui/main_window.go` | `onSave` (lines 548-550): replace `c.cfg.Profiles[name] = config.ProfileEntry{Routes: routes}` with `c.cfg.Routes = routes`. |
| 2.4 | Frontend Developer | `internal/ui/main_window.go` | `persistQuiet` (lines 793-803): delete `if cur.Profiles == nil {...}` guard; replace `cur.Profiles[req.Name] = config.ProfileEntry{Routes: prefixes}` with `cur.Routes = prefixes`. |

### Phase 3: Review & Documentation (Always Last)

**Depends on:** Phase 1 + Phase 2

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 3.1 | Debugger/Reviewer | Verify all acceptance criteria; run `go build` + new config test; confirm no remaining `Profiles`/`ProfileEntry` references in non-doc code. |
| 3.2 | Documentation | Update `README.md` (config section) to show new `routes` shape; note behavior change in changelog. |

---

## Migration Logic Decision (Task 1.4)

`parseConfig` must accept BOTH old (`profiles`) and new (`routes`) shapes. Decision:

1. **New shape first:** `json.Unmarshal` into `configWire`. If `w.Routes != nil` (the new key is present), use it directly:
   ```go
   cfg := Config{
       SelectedProfile:     w.SelectedProfile,
       Routes:              cleanRoutes(w.Routes),
       RememberCredentials: true,
   }
   if w.RememberCredentials != nil { cfg.RememberCredentials = *w.RememberCredentials }
   return cfg, nil
   ```
2. **Old shape fallback:** if `w.Routes == nil`, attempt to unmarshal into the legacy `struct { SelectedProfile string; Profiles map[string]ProfileEntry; RememberCredentials *bool }` (or reuse a second wire). Collapse per-profile routes into ONE global list:
   - If `selectedProfile` is set AND its entry exists, take `cleanRoutes(profiles[selectedProfile].Routes)`.
   - **Else (selectedProfile missing/empty or its entry absent):** take the **union** (dedup, order-preserving) of `Routes` across ALL profile entries. This guarantees no routes are silently dropped when the selected profile can't be identified.
   - Apply `cleanRoutes` to the result.
3. **Legacy pre-Profiles shape** (the `{connectionName, serverAddress, username, routes}` form already handled at lines 145-159) continues to map into a single global `Routes` (it already had one routes list) — keep that branch, just write to `cfg.Routes` instead of `cfg.Profiles[name]`.

**Rationale for union-on-missing-selected:** the on-disk `config.json` has `selectedProfile: "XXX - xxx.xxx.xxx.xxx"` whose entry contains both `10.0.7.0/24` and `git-rbi.xxx.xxx`, so the common case takes exactly that entry's two routes. The union branch is a safe fallback only when the selected profile can't be resolved, preventing data loss.

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| `git-rbi.xxx.xxx` route (previously only on the `XXX - xxx.xxx.xxx.xxx` profile) now applies to ALL selected profiles | Medium (user-facing behavior change) | High (by design) | Document explicitly in README + changelog; this is the intended "global" semantics the user requested. |
| Union fallback could add routes to profiles that previously had none (only if selectedProfile missing) | Low | Low | Only triggers when selectedProfile unresolvable; result is still a valid superset; user can edit in UI. |
| Stale `config.json` with `profiles` key lingers until first `Save` | Low | Certain | `Save` rewrites the file on next save/connect; optionally update `config.json` on disk directly in this change for cleanliness. |
| Remaining `cfg.Profiles` references after edit cause compile error | High (build break) | Medium | Grep `Profiles|ProfileEntry` across `internal/` (excluding docs) before marking done; only `main_window.go` + `config.go` reference them. |

## Rollback Strategy

Config schema change is forward-only on disk. To roll back: revert this PR; old code's `parseConfig` still understands the new `routes` shape? No — old code expects `profiles`. Mitigation: keep a backup of `config.json` before first save, or restore from `config.json.bak`. The migration is lossless (routes preserved), so reverting the binary and manually restoring the old `config.json` recovers per-profile state.

---

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| 1.0.0 | 2026-07-23 | Initial plan — global routes replacing per-profile map |
