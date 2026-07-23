# Fix Plan: PRD-002 post-review polish

**Related PRD:** PRD-002 (`docs/planning/prd-002-l2tp-split-tunnel.md`)  
**Severity:** Medium (Major nits + Security Medium residuals; no Critical)  
**Reported by:** Debugger/Reviewer (PASS WITH NITS), Security (APPROVED for MVP)  
**Date:** 2026-07-22  
**Status:** Ready for implementation  
**Version:** v1.0.0

---

## Bug / polish summary

PRD-002 implementation is shippable for MVP, but review found **Major** UX/correctness and **Medium** security hygiene items that should land before release polish. No Critical findings; full bug-fix loop not required for ship, but these items are in-scope for this pass.

| # | Severity | Area | Symptom / gap |
| - | -------- | ---- | ------------- |
| 1 | Major | UI connect result | `ConnectFull` / rasdial “already connected” maps to `UserError` code `"already"`, but UI treats any error as `StatusError` → Disconnect stays disabled |
| 2 | Major | Secrets (CredMan) | `CredWrite` uses pointers into Go-managed buffers without `runtime.KeepAlive` → GC may free blob/target before native call completes |
| 3 | Major | UI Save | `onSave` runs CredMan + `config.Save` on the UI thread (unlike Connect) → possible freeze |
| 4 | Medium | Temp scripts | Orphaned `%TEMP%\vepeen\vpn-*.ps1` if process dies mid-`EnsureProfile` |
| 5 | Medium | CredMan persist | Uses `CRED_PERSIST_ENTERPRISE` (roaming); prefer non-roaming `CRED_PERSIST_LOCAL_MACHINE` |
| 6 | Minor | Validation UX | `validateConnect` returns `fyne.Focusable` but callers ignore it; empty-check / trim policy inconsistent for password vs other fields |
| 7 | Minor | Window chrome | No `SetMinSize`; only `Resize(480, 640)` |
| 8 | Minor | go.mod | `golang.org/x/sys` is indirect though `internal/secrets` imports it |

## Root cause analysis

1. **Already-connected:** `MapExecError` correctly emits code `"already"` (`internal/vpn/errors.go`), but `onConnect` completion always does `setStatus(StatusError, …)` on any non-nil error (`internal/ui/main_window.go`).
2. **KeepAlive:** Classic Go + Windows API hazard in `credManStore.Set` after `procCredWriteW.Call` with `unsafe.Pointer` into stack/heap slices.
3. **Save sync:** `onSave` was implemented as a short path; Connect was async from day one — inconsistency, not a design requirement.
4. **Orphans:** `runPowerShellScript` defers `os.Remove` only for the current process lifetime; crash/kill leaves files.
5. **Persist flag:** Constant `credPersist = credPersistEnterprise` chosen as “user-scoped”; Security prefers non-roaming local machine persist for secrets.
6. **Focus/trim:** Focusable return value unused; password empty-check uses raw `== ""` while name/server/user use `TrimSpace`.
7. **Min size:** Starter window only resized, never min-constrained.
8. **go.mod:** Dependency pulled transitively via Fyne; direct import in secrets should be a direct `require`.

## Fix strategy

**Recommended:** Single polish pass (Option A — minimal, targeted). No architecture rewrite. All work assigned to **Backend Developer** (Go + Fyne ownership per PRD-002).

### Out of scope (this pass)

- Full RAS API rewrite to remove `rasdial` argv password
- Perfect secret-value redaction overhaul (tiny opportunistic cleanups OK)
- Dependency CVE bumps unless trivial
- Non-Windows feature work

---

## Implementation tasks

**Agent for all implementation tasks:** Backend Developer  
**Parallelizable:** Yes within Phase 1 where files do not overlap (see notes).  
**Phase 2 (review)** depends on Phase 1.

### Phase 1: Code fixes

**Depends on:** Nothing  
**Parallelizable:** Partial — Tasks 1.1+1.3+1.6+1.7 share `main_window.go` (one agent sequential on that file); 1.2+1.5 share `store_windows.go`; 1.4 and 1.8 are independent.

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/ui/main_window.go` | On Connect completion: if `vpn.AsUserError(err)` and `Code == "already"`, set `StatusConnected` with Primary/Detail from the error (or fixed Indonesian copy: `Terhubung` / `Sudah terhubung.`), enable Disconnect via `applyEnablement`. Do **not** use `StatusError`. |
| 1.2 | Backend Developer | `internal/secrets/store_windows.go` | After successful `CredWriteW`, call `runtime.KeepAlive` on `blob`, `targetPtr`, `userPtr`, and `cred` (anything whose address was passed to the native call). Import `runtime`. |
| 1.3 | Backend Developer | `internal/ui/main_window.go` | Make Save async like Connect: after validation/busy lock, capture form values, `go` worker for `store.Set` (PSK/password) + `config.Save`, then `fyne.Do` to `finishSave` / `finishSaveKeep`. Keep busy flag + button disablement for the duration. |
| 1.4 | Backend Developer | `internal/vpn/powershell_windows.go` (primary); optional call site in `internal/vpn/profile_windows.go` or `manager.go` / UI `loadInitial` | Add `PurgeOrphanScripts()` (or unexported helper): delete `vpn-*.ps1` under `filepath.Join(os.TempDir(), "vepeen")` best-effort (ignore errors). Invoke on app start path **and/or** at start of `EnsureProfile` / `runPowerShellScript` so orphans clear even without UI. Prefer: purge at beginning of `runPowerShellScript` **or** dedicated export called from `loadInitial` + `EnsureProfile` once. Do not log script contents. |
| 1.5 | Backend Developer | `internal/secrets/store_windows.go` | Set `credPersist = credPersistLocalMachine` (`CRED_PERSIST_LOCAL_MACHINE = 2`). Update comment: non-roaming, machine-local for current user credentials. Keep existing target naming. |
| 1.6 | Backend Developer | `internal/ui/main_window.go` | (a) Hold `fyne.Window` (or canvas) on `controller`; on validation failure in `onConnect` (and Save if validating routes), `w.Canvas().Focus(focusable)` for first invalid field. (b) **Trim policy:** empty-check all secret fields with `strings.TrimSpace(...) == ""` for “required”; **do not** strip password/PSK content when dialing/storing beyond what is already done — if product wants trim-on-use for PSK only, trim PSK when building `ConnectRequest` / Save, leave password bytes as entered except reject whitespace-only. Align `validateConnect` and any Save name checks. |
| 1.7 | Backend Developer | `internal/ui/main_window.go` | After `Resize`, call `w.SetMinSize(fyne.NewSize(420, 560))` (keep current default size ~480×640). |
| 1.8 | Backend Developer | `go.mod` (via `go get` / `go mod tidy`) | Promote `golang.org/x/sys` to a **direct** `require` (version already in tree, e.g. `v0.30.0` or tidy result). No version bump required unless tidy forces a compatible one. |

**Sub-agent guidance:**

- Tasks **1.2 + 1.5** can be one edit session on `store_windows.go`.
- Tasks **1.1 + 1.3 + 1.6 + 1.7** should be one sequential session on `main_window.go` to avoid merge conflicts.
- Task **1.4** independent of secrets; can parallel with 1.2/1.5/1.8.
- Task **1.8** independent; run after or with secrets work.

### Phase 2: Verify & re-review

**Depends on:** Phase 1

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 2.1 | Backend Developer | `go test ./internal/route ./internal/vpn`; `go build -o vepeen.exe ./cmd/vepeen`; smoke: Save while typing (UI stays responsive), Connect when already connected (status Terhubung + Putuskan enabled), validation focus on empty server. |
| 2.2 | Debugger/Reviewer | Re-check Major items 1–3 and Should-fix 4–8; confirm no regressions on connect/disconnect/status load. |
| 2.3 | Security | Spot-check CredMan persist flag, KeepAlive, temp purge; residual Medium risks remain documented (rasdial argv, brief PSK in temp script). |
| 2.4 | Documentation | Only if behavior/README claims change (CredMan non-roaming, temp purge). Touch `README.md` limitations if persist wording is wrong; otherwise skip. |

---

## Acceptance criteria

### Must fix (Major)

- [ ] **Already-connected:** When OS/VPN reports already connected (error code `"already"`), UI shows **Terhubung** (`StatusConnected`), detail indicates already connected, **Putuskan** enabled, **Hubungkan** disabled — not error state.
- [ ] **KeepAlive:** `Set` in `store_windows.go` keeps CredWrite-related buffers alive across the syscall (`runtime.KeepAlive` present and correct).
- [ ] **Async Save:** Save path does CredMan + config I/O off the UI thread; buttons respect busy; success/failure still update status on main thread via `fyne.Do`.

### Should fix

- [ ] **Temp purge:** Starting the app and/or ensuring a profile removes leftover `vpn-*.ps1` under `%TEMP%\vepeen\` without deleting unrelated files.
- [ ] **CredMan persist:** New writes use `CRED_PERSIST_LOCAL_MACHINE` (value `2`), not ENTERPRISE.
- [ ] **Validation:** First invalid field receives keyboard focus; whitespace-only password/PSK fails required check consistently with other fields.
- [ ] **Min size:** Window cannot be resized below 420×560.
- [ ] **go.mod:** `golang.org/x/sys` listed under direct `require`.

### Regression / quality

- [ ] `go test ./internal/route ./internal/vpn` pass
- [ ] `go build ./cmd/vepeen` pass on Windows
- [ ] Normal connect/disconnect/status-on-load still work
- [ ] No secrets logged; no new plain-text secret persistence

---

## Regression risk

| Change | Risk | Notes |
| ------ | ---- | ----- |
| Already → Connected | Low | May enable Disconnect when OS already has session; intended |
| KeepAlive | Low | Correctness-only; no API change |
| Async Save | Medium | Race if user edits fields mid-save — mitigate by capturing values before `go` and/or keeping form disabled while `busy` |
| Temp purge | Low | Glob only `vpn-*.ps1` in app temp dir |
| LOCAL_MACHINE persist | Low–Med | Existing ENTERPRISE credentials still readable by target name; new writes non-roaming. Optional: re-save on next Save/Connect overwrites with new persist |
| Focus + trim | Low | Whitespace-only password now rejected (stricter, correct) |
| MinSize | Low | Cosmetic |
| go.mod direct | None | Hygiene |

## Rollback strategy

- Revert the single polish commit(s) touching the files above.
- No schema/migration; CredMan entries remain valid if persist flag reverts (old blobs still match target names).
- Temp purge is best-effort only; rollback leaves orphans as before.

---

## Implementation summary (for Orchestrator)

**Scope:** Fix plan (not full PRD) — PRD-002 release polish  
**Plan file:** `docs/planning/fix-001-prd002-polish.md`  
**Primary agent:** Backend Developer  
**Order:**

1. `store_windows.go` — KeepAlive + LOCAL_MACHINE persist (1.2, 1.5)
2. `powershell_windows.go` (+ call site) — orphan purge (1.4)
3. `main_window.go` — already-connected, async save, focus/trim, min size (1.1, 1.3, 1.6, 1.7)
4. `go.mod` — direct `golang.org/x/sys` (1.8)
5. Build/test → Debugger/Reviewer → Security spot-check → docs only if needed

**Done when:** All acceptance checkboxes above pass; re-review can clear Major nits.

---

## Version history

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-22 | Initial polish fix plan from PASS WITH NITS + Security APPROVED |
