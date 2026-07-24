# Fix Plan: fix-022 — Entry caret invisible (width 0)

**Related:** fix-021 (caret outcome incomplete — wrong root cause)
**Severity:** Critical (UI — focused entries show no caret)
**Reported by:** Debugger/Reviewer (fix-021 review)
**Date:** 2026-07-24

## Bug Summary

Focused text entries show no blinking caret. Color is fine; width is zero.

## Root Cause

`internal/ui/theme.go` `Size()` returns `0` for `theme.SizeNameInputBorder` (to kill pill stroke). Fyne v2.8 uses that size as **caret width** in `entryContentRenderer.moveCursor`. Width 0 → invisible caret regardless of `ColorNamePrimary`.

fix-021 assumed caret color (`ColorNamePrimary` / no `ColorNameCursor`). That path is a red herring while border size is 0.

## Fix Strategy

### Option A: Minimal Fix (recommended)

- File: `internal/ui/theme.go`
- Change only:

```go
case theme.SizeNameInputBorder:
	return 1 // caret width; border still invisible via ColorNameInputBorder=Transparent
```

- Keep `ColorNamePrimary` = teal (`accentColor`).
- Keep `ColorNameInputBorder` = `color.Transparent` (border stays invisible).
- Risk: Low — 1px transparent border is a no-op visually.
- Effort: S

### Option B: Primary→white

- Not needed after width fix. Do **not** change Primary unless visual check still fails (unlikely).

**Recommended:** Option A only.

## Implementation Tasks

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1 | Frontend Developer | `internal/ui/theme.go` | In `(*vepeenTheme).Size`, `SizeNameInputBorder`: `return 0` → `return 1` with comment that Fyne uses this as caret width; border remains transparent via Color. |
| 2 | Debugger/Reviewer | — | Focus any entry; confirm blinking caret. Confirm no visible pill border. `go build ./...` + `go vet ./internal/ui/...`. |

**Do not:** custom Entry, ThemeOverride, Primary→white, VPN/backend changes, README.

## Acceptance Criteria

- [ ] Focused entries (`routesEntry` and others) show a visible blinking caret on dark input fill.
- [ ] Input borders remain invisible (`ColorNameInputBorder` still Transparent).
- [ ] `ColorNamePrimary` remains teal; Connect CTA unchanged.
- [ ] `go build ./...` and `go vet ./internal/ui/...` pass.

## Regression Risk

| Risk | Mitigation |
| ---- | ---------- |
| 1px border becomes visible | Color is Transparent — stroke has no paint |
| fix-021 changelog implies color fixed caret | Amend changelog (this plan) |

## Rollback

Revert the single `return 1` → `return 0` in `theme.go` Size case.
