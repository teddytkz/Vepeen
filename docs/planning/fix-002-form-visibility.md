# Fix Plan: Form inputs not visible (layout collapse)

**Related PRD / UI:** PRD-002, ui-003 (`docs/planning/ui-003-simplified-form-log.md`)  
**Severity:** Critical  
**Reported by:** Debugger/Reviewer  
**Date:** 2026-07-22  
**Status:** Ready for implementation  
**Version:** v1.0.0

---

## Bug summary

Main window form fields (IP, Key, Username, Password, routes) appear missing or collapsed. Users cannot see or use primary inputs. Status/actions/log may still show.

## Root cause analysis

Fyne `Border` layout assigns **bottom** its full `MinSize().Height` first; **center** gets only leftover height.

Current `build()` in `internal/ui/main_window.go`:

```go
footer := container.NewVBox(actions, statusBox, logBox) // log SetMinRowsVisible(8)
root := container.NewBorder(header, footer, nil, nil, scroll)
```

- Footer VBox (actions + status + log@8 rows) MinSize height ≈ **376px**
- Center is form `VScroll` with MinSize height only ~**32px** → Border does not reserve form space
- At 480×720 center ≈ 249px; at 420×600 ≈ 129px; with DPI/wrap, center can approach **0** → form invisible

Confirmed by Debugger (session review + Fyne `borderlayout.go`: `bottomHeight := bottom.MinSize().Height`, no shrink).

## Fix strategy

### Option A: Compact footer + nested center (recommended)

- **Files:** `internal/ui/main_window.go` only (`build()`)
- **Risk:** Low — layout-only; no VPN/secrets changes
- **Effort:** S

### Option B: Shrink log only in footer

- Keep log in footer but lower min rows — still competes with form for fixed bottom height; residual collapse risk on small windows
- **Not recommended** as sole fix

**Recommended:** Option A

### Out of scope

- VPN connect/disconnect/status logic
- Secrets / CredMan
- Label redesign, new fields, Designer pass
- Changing window default size policy beyond layout floor

---

## Implementation tasks

**Agent for implementation:** Backend Developer only (no Designer)  
**Parallelizable:** No — single file, single function

### Phase 1: Layout fix

**Depends on:** Nothing  
**Parallelizable:** No

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/ui/main_window.go` | In `build()` only: **Footer** = `actions` + `statusBox` only (remove `logBox` from footer VBox). |
| 1.2 | Backend Developer | `internal/ui/main_window.go` | Lower `c.logEntry.SetMinRowsVisible` from **8** to **4–6** (prefer **5**). Keep log read-only + clear button + separator/header structure. |
| 1.3 | Backend Developer | `internal/ui/main_window.go` | After creating form `scroll` (`NewVScroll(formBody)`), call `scroll.SetMinSize(fyne.NewSize(0, 200))` (height floor ~200; width 0 = unconstrained). |
| 1.4 | Backend Developer | `internal/ui/main_window.go` | Nested center: `center := container.NewBorder(scroll, nil, nil, nil, logBox)` — **Top** = form VScroll (min height floor), **Center** = log box (flexible remainder). |
| 1.5 | Backend Developer | `internal/ui/main_window.go` | Outer Border: `container.NewBorder(header, footer, nil, nil, center)` then existing `NewPadded`. Do not change controllers, handlers, or non-`build` logic. |

**Target structure (conceptual):**

```text
Padded
└─ Border
   ├─ Top:    header (title + subtitle + sep)
   ├─ Bottom: footer VBox(actions, statusBox)   // compact
   └─ Center: Border
              ├─ Top:    form VScroll (MinSize h≈200)
              └─ Center: logBox (title + clear + entry@4–6 rows)
```

**Sub-agent guidance:**

- Tasks 1.1–1.5 are one sequential edit session on `build()` — do not split across agents.
- Preserve widget construction order for fields/buttons if possible; only re-parent containers.
- Do not move log into a second window or remove log entirely (ui-003 still requires activity log).

### Phase 2: Verify & review

**Depends on:** Phase 1

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 2.1 | Backend Developer | `go build -o vepeen.exe ./cmd/vepeen`; run app; confirm form fields visible at default 480×720 and near min 420×600; scroll form if needed; log still shows and clears. |
| 2.2 | Debugger/Reviewer | Confirm Critical resolved: all primary fields + routes visible; footer no longer steals center; no regression on Connect/Save/log append. |
| 2.3 | Documentation | Skip unless README UI layout claims change; optional one-line note only if docs assert log-in-footer. |

---

## Acceptance criteria

- [ ] At default window size (480×720), **IP, Key (PSK), Username, Password** labels and entries are visible without relying on zero-height center
- [ ] **Rute (split tunnel)** block is reachable (visible or scrollable in form VScroll)
- [ ] Footer is **actions + status only** (no log in bottom Border slot)
- [ ] Log remains on main window, flexible in nested center, `SetMinRowsVisible` in **4–6** range
- [ ] Form VScroll has **MinSize height floor ≈ 200** so Border reserves form space
- [ ] Outer layout: header top, compact footer bottom, nested form+log center
- [ ] No changes to VPN, secrets, or non-layout business logic
- [ ] `go build ./cmd/vepeen` succeeds

## Regression risk

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Log area too small on short windows | Low | Med | Min rows 4–6 + center flex; form still prioritized via Top min size |
| Form min 200 + compact footer still tight at 420×600 | Med | Low | Footer without log should free ~150–200px; verify at min size |
| Accidental handler/widget nil order change | High | Low | Only re-parent existing widgets in `build()` |

## Rollback strategy

Revert `build()` layout in `internal/ui/main_window.go` to previous footer-with-log Border (single file, no data migration).

---

## Version history

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-22 | Initial Critical fix plan — form visibility / Border footer collapse |
