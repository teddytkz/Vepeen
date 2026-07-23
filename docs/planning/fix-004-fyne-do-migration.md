# Fix Plan: Fyne `fyne.Do` threading migration warning

**Related PRD / UI:** PRD-001 (Fyne), PRD-002 / ui-003 (UI already uses `fyne.Do`)  
**Severity:** Medium (deprecation warning; hard fail next major Fyne)  
**Reported by:** Debugger/Reviewer  
**Date:** 2026-07-22  
**Status:** Ready for implementation  
**Version:** v1.0.0

---

## Bug summary

On launch, Fyne prints:

```text
*** This application has not been migrated to the fyne.Do threading model ***
*** The next major Fyne release will remove this safety! ***
*** Read more at https://docs.fyne.io/started/goroutines ***
```

App behavior is otherwise fine. UI workers already marshal widget updates with `fyne.Do`.

## Root cause analysis

- Fyne v2.6+ requires an explicit **migration opt-in** for the `fyne.Do` threading model.
- Opt-in is via repo-root `FyneApp.toml` → `[Migrations] fyneDo = true` (or temporary build tag `migrated_fynedo`).
- Code audit (`internal/ui/main_window.go`): loadInitial, onSave, onConnect, onDisconnect already use `fyne.Do` for UI; `persistQuiet` has no UI. **No code change required** unless smoke fails after opt-in.
- App ID in code is `com.vepeen.app` (`cmd/vepeen/main.go` → `app.NewWithID`).

## Fix strategy

### Option A: `FyneApp.toml` migration flag (recommended)

- Add `FyneApp.toml` at repo root with `[Details]` Name/ID matching the app and `[Migrations] fyneDo = true`.
- Optional one-line README note under project layout / troubleshooting.
- **Risk:** Low — metadata only; code already migrated.
- **Effort:** S

### Option B: Build tag only (`-tags migrated_fynedo`)

- Useful for a one-off smoke test; not a permanent project signal.
- **Not recommended** as sole fix.

**Recommended:** Option A

### Out of scope

- Changes to `internal/ui/main_window.go` unless post-opt-in smoke shows UI races
- Fyne version upgrade
- VPN / secrets / form layout
- App icon packaging (Icon optional; omit if no asset)

---

## Implementation tasks

**Agent for implementation:** Backend Developer  
**Parallelizable:** Yes for 1.1 / 1.2 (different files)

### Phase 1: Migration metadata + docs

**Depends on:** Nothing  
**Parallelizable:** Yes

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `FyneApp.toml` (create at repo root) | Create Fyne metadata with Details matching the running app and enable `fyneDo` migration. Use exactly: |
| | | | ```toml |
| | | | [Details] |
| | | | Name = "Vepeen" |
| | | | ID = "com.vepeen.app" |
| | | | |
| | | | [Migrations] |
| | | | fyneDo = true |
| | | | ``` |
| | | | Do **not** invent an Icon path unless an icon file already exists. ID **must** match `app.NewWithID("com.vepeen.app")` in `cmd/vepeen/main.go`. |
| 1.2 | Backend Developer | `README.md` | Optional but preferred: (a) list `FyneApp.toml` in **Project layout**; (b) short Troubleshooting note that the `fyne.Do` migration warning is resolved by `FyneApp.toml` `[Migrations] fyneDo = true` once all background UI updates use `fyne.Do` (already true for this app). Keep it brief. |

**Sub-Agent Guidance:**

- Do **not** edit `internal/ui/main_window.go` or other Go sources in this fix unless Phase 2 smoke fails.
- Do **not** change `go.mod` / Fyne version.
- Optional pre-check: `go run -tags migrated_fynedo ./cmd/vepeen` should silence the warning before committing `FyneApp.toml` (same effect as the TOML flag for a single build).

### Phase 2: Review / smoke

**Depends on:** Phase 1

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 2.1 | Debugger/Reviewer (or user) | `go build -o vepeen.exe ./cmd/vepeen` then run `.\vepeen.exe`. Confirm console **no longer** prints the fyne.Do migration warning. Smoke: window opens, form visible, log works; no freeze on Simpan/Hubungkan UI updates. |
| 2.2 | Backend Developer (only if smoke fails) | If UI freezes or panics after opt-in, re-audit goroutine → UI paths in `internal/ui/main_window.go` and wrap any missed widget updates in `fyne.Do`. Do not change VPN packages for this. |

**Security:** Not required (metadata/docs only unless 2.2 triggers UI-only edits).

---

## Acceptance criteria

- [ ] `FyneApp.toml` exists at repo root with `Name = "Vepeen"`, `ID = "com.vepeen.app"`, and `[Migrations] fyneDo = true`
- [ ] App ID in TOML matches `cmd/vepeen/main.go` (`com.vepeen.app`)
- [ ] Launch no longer prints the “has not been migrated to the fyne.Do threading model” warning
- [ ] No intentional changes to VPN/secrets/route logic
- [ ] `go build -o vepeen.exe ./cmd/vepeen` still succeeds
- [ ] Optional: README mentions `FyneApp.toml` / migration note

## Regression risk

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Hidden UI update not wrapped in `fyne.Do` becomes a race after opt-in | Med | Low | Pre-audit already clean; smoke Simpan/Hubungkan/status; fix only if fails |
| Wrong app ID in TOML vs code | Low | Low | Use `com.vepeen.app` exactly |
| Fyne ignores TOML if not at module root | Low | Low | Place file at repo root next to `go.mod` |

## Rollback strategy

- Delete `FyneApp.toml` or set `fyneDo = false` (warning returns; app still runs on current Fyne).
- Revert README layout/troubleshooting lines.

---

## Implementation summary (for Orchestrator)

**Scope:** Minor metadata opt-in — full fix plan for clear Backend tasks  
**Plan path:** `docs/planning/fix-004-fyne-do-migration.md`

**Backend Developer tasks:**

| Task | Files | What to do |
| ---- | ----- | ---------- |
| 1.1 | `FyneApp.toml` | Create with Details Name/ID + Migrations `fyneDo = true` |
| 1.2 | `README.md` | Optional: project layout + short troubleshooting note |
| 2.2 | `internal/ui/main_window.go` | **Only if** smoke fails after opt-in |

**Next agent:** Backend Developer → Debugger/Reviewer (smoke: no migration warning)

**Acceptance (one-liner):** Running the app does not print the fyne.Do migration warning; UI still updates correctly from background work.
