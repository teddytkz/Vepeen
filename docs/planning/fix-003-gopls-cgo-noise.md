# Fix Plan: gopls go-gl false error + IDE noise

**Related PRD / UI:** PRD-001 (Fyne/CGo), fix-002 (form layout — already correct)  
**Severity:** Low  
**Reported by:** Debugger/Reviewer  
**Date:** 2026-07-22  
**Status:** Ready for implementation  
**Version:** v1.0.0

---

## Bug summary

IDE / gopls surfaces errors like:

```text
build constraints exclude all Go files ... go-gl ... [darwin]
```

Users may think the project does not build, or that form/layout work is broken. **Real Windows build succeeds** with `CGO_ENABLED=1`. Form layout from fix-002 is correct; a rebuilt binary should show the form.

## Root cause analysis

- Fyne depends on `go-gl` packages that use OS/CGo build tags.
- gopls/analysis runs with a mismatched environment (`GOOS=darwin` and/or `CGO_ENABLED=0`), so it analyzes the wrong file set and reports “build constraints exclude all Go files”.
- This is **analysis noise**, not a compile failure of `./cmd/vepeen` on Windows with CGo.
- `cmd/diag_layout` was an investigation-only helper for fix-002 layout MinSize; not product code.

## Fix strategy

### Option A: Workspace gopls env + README + remove diag (recommended)

- Pin gopls tool env for this workspace: `CGO_ENABLED=1`, `GOOS=windows`.
- Document the false positive under README Troubleshooting.
- Delete `cmd/diag_layout` (investigation artifact).
- **Risk:** Low — no app/VPN logic.
- **Effort:** S

### Option B: README only

- Leaves IDE red squiggles; higher user confusion.
- **Not recommended** as sole fix.

**Recommended:** Option A

### Out of scope

- VPN connect/disconnect/status logic
- Form layout changes (fix-002 already correct)
- Pinning or upgrading `go-gl` / Fyne versions
- Global user gopls settings outside this workspace
- CI / DevOps

---

## Implementation tasks

**Agent for implementation:** Backend Developer only  
**Parallelizable:** Yes for 1.1 / 1.2 / 1.3 (different files; no shared edits)

### Phase 1: Workspace + docs + cleanup

**Depends on:** Nothing  
**Parallelizable:** Yes

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `.vscode/settings.json` (create) | Set `go.toolsEnvVars` so gopls/go tools use Windows + CGo: `CGO_ENABLED` = `"1"`, `GOOS` = `"windows"`. Do not change other editor settings unless required for Go. |
| 1.2 | Backend Developer | `README.md` | Under **Troubleshooting**, add a row for gopls/`go-gl` “build constraints exclude all Go files … [darwin]” (or CGo-off analysis): explain it is IDE noise when env is wrong; real fix is `CGO_ENABLED=1` + Windows target; verify with `go build -o vepeen.exe ./cmd/vepeen`. Optionally note workspace `.vscode/settings.json` sets tools env. |
| 1.3 | Backend Developer | `cmd/diag_layout/` (delete) | Remove investigation-only `cmd/diag_layout` package (entire directory). Do not leave stubs. If README/project layout mentions it, drop that mention (currently layout lists only `cmd/vepeen`). |

**Sub-Agent Guidance:**

- Tasks 1.1, 1.2, 1.3 touch different paths — safe in parallel.
- Do **not** edit `internal/**` or `go.mod` for this fix.
- Do **not** pin go-gl versions.

### Phase 2: Review

**Depends on:** Phase 1

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 2.1 | Debugger/Reviewer | Confirm `go build -o vepeen.exe ./cmd/vepeen` still succeeds with CGo. Confirm `cmd/diag_layout` gone. Spot-check README troubleshooting row. Optional: reload window / gopls and confirm go-gl darwin noise reduced when tools env applies. |
| 2.2 | Documentation | No separate pass required if Backend owns README row; Reviewer verifies accuracy only. |

**Security:** Not required (no auth/data/API/VPN changes).

---

## Acceptance criteria

- [ ] `.vscode/settings.json` exists with `go.toolsEnvVars.CGO_ENABLED` = `"1"` and `go.toolsEnvVars.GOOS` = `"windows"`
- [ ] README Troubleshooting documents the gopls/`go-gl` build-constraints false positive and how to verify a real build
- [ ] `cmd/diag_layout` is removed from the tree
- [ ] `go build -o vepeen.exe ./cmd/vepeen` still succeeds (no product code regression)
- [ ] No changes to VPN logic, form layout, or go-gl/Fyne dependency pins

## Regression risk

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Workspace `GOOS=windows` confuses non-Windows contributors | Low | Low | Product is Windows-only; README already states Windows target |
| gopls still noisy if GCC missing | Low | Med | README already covers `gcc` not found; CGo=1 alone does not install compiler |
| Someone still needs layout dump | Low | Low | Re-run one-off diagnostics if needed; do not keep `diag_layout` in tree |

## Rollback strategy

- Delete `.vscode/settings.json` or revert the `go.toolsEnvVars` block.
- Revert README troubleshooting row.
- Restore `cmd/diag_layout` from VCS only if diagnostics are needed again (prefer not).

---

## Implementation summary (for Orchestrator)

**Scope:** Minor tooling/docs/cleanup — full fix plan (not changelog-only) for clear Backend tasks  
**Plan path:** `docs/planning/fix-003-gopls-cgo-noise.md`

**Backend Developer tasks:**

| Task | Files | What |
| ---- | ----- | ---- |
| 1.1 | `.vscode/settings.json` | `go.toolsEnvVars`: `CGO_ENABLED=1`, `GOOS=windows` |
| 1.2 | `README.md` | Troubleshooting bullet for gopls go-gl / build-constraints noise |
| 1.3 | `cmd/diag_layout/` | Delete investigation artifact |

**Agent:** Backend Developer → Debugger/Reviewer  
**Acceptance:** Real build still OK; IDE env documented; diag_layout gone; no VPN/layout/go-gl pin changes

---

## Version history

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-22 | Initial fix plan from Debugger findings |
