# Fix Plan: fix-008 — UI Polish (landscape layout balance + accent theme)

**Related PRD:** PRD-002 (L2TP/IPsec split-tunnel client)
**Related fixes:** fix-007 (landscape layout), fix-006 (card redesign), fix-005 (blank window)
**Severity:** Low (cosmetic / polish — no functional regression risk to VPN logic)
**Reported by:** User ("tidak proper" — unpolished)
**Date:** 2026-07-22

---

## Bug Summary

The landscape UI introduced in fix-007 is functional but visually unpolished:

1. **Dead space at bottom** — `container.NewHBox(leftCol, hGap(16), rightCol)` top-aligns the two columns. The Log card uses `SetMinRowsVisible(5)` and does NOT expand, so at `960×600` with content ~450px there is ~150px of empty space at the bottom. Looks unbalanced.
2. **Unequal column heights** — left column (Koneksi + Rute) is taller than right column (Status + Log), producing a visible visual imbalance.
3. **No brand/accent color** — uses the default Fyne `DarkTheme`; looks generic. Only `Hubungkan` uses `HighImportance`.
4. **Plain header** — just a bold title + subtitle + separator, no branding or comfortable padding.

All defects are layout/theme only. No handler logic, threading, or data flow is affected.

---

## Root Cause Analysis

- `build()` (in `internal/ui/main_window.go`) composes columns with `container.NewVBox` + fixed `vGap`/`hGap` spacers and a top-level `container.NewHBox`. VBox/HBox do not stretch children to fill available height; the Log card's `SetMinRowsVisible(5)` fixes its height, so the right column stops short and the body leaves dead space under the `Border` center.
- `cmd/vepeen/main.go` calls `a.Settings().SetTheme(theme.DarkTheme())` — the stock theme, no accent override.
- Header is a bare `container.NewVBox(title, subtitle, separator)` with no padding/branding.

---

## Fix Strategy

### Option A: Minimal Fix (recommended)

- Add a custom `fyne.Theme` (new `internal/ui/theme.go`) that wraps `theme.DarkTheme()` and overrides `ColorNamePrimary` to a tasteful accent (teal `#0FB5AE` or indigo `#4F46E5`). Apply via `a.Settings().SetTheme(...)` in `cmd/vepeen/main.go`.
- Rework `build()` layout so both columns fill full height with no dead space:
  - Use `container.NewGridWithColumns(2)` for the two columns → equal width AND equal height (grid stretches both cells to the tallest).
  - Inside each grid cell, use `container.NewBorder(top=upperCard, nil, nil, nil, lowerCard)` so the lower card (Rute on left, Log on right) expands to fill remaining height.
  - Replace `hGap`/`vGap` usage in the body with the grid (keep the helper functions if referenced elsewhere, or remove if unused).
- Polish header: add `container.NewPadded` padding, bold title, subtitle, separator.
- Keep window `Resize(960, 600)` (or tune to `900×640`); keep `CenterOnScreen`; keep `minSizeWrap` floor `fyne.NewSize(900, 560)` (adjust only if the new layout's natural min size exceeds it — verify with a build/screenshot).
- Keep button row at bottom (`Border` bottom = buttonRow) with Simpan (left), Spacer, Putuskan + Hubungkan (HighImportance, right).
- Optionally set `a.SetIcon(...)` only if a simple PNG resource is trivially available; otherwise skip (no regression either way).

- Files: `internal/ui/theme.go` (new), `internal/ui/main_window.go`, `cmd/vepeen/main.go`, `README.md`
- Risk: Low — pure presentation; no handler/threading/data changes.
- Effort: S–M

### Option B: Thorough Fix (not needed)

Full design-system pass (typography scale, spacing tokens, icon asset pipeline). Out of scope for this polish request.

**Recommended:** Option A — addresses all four defects with minimal, low-risk changes.

---

## Implementation Tasks

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| A.1 | Frontend Developer | `internal/ui/theme.go` (new) | Create `vepeenTheme` struct embedding `theme.DarkTheme()`; override `Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color` to return accent for `theme.ColorNamePrimary` (and `ColorNamePrimaryHover`/`ColorNameButton` if desired), else delegate to base. Use teal `#0FB5AE` (or indigo `#4F46E5`). Keep Dark variant default. |
| A.2 | Frontend Developer | `cmd/vepeen/main.go` | Replace `a.Settings().SetTheme(theme.DarkTheme())` with `a.Settings().SetTheme(ui.NewTheme())`. Import `vepeen/internal/ui`. Optionally `a.SetIcon(...)` if a PNG resource exists; otherwise leave commented/skipped. |
| B.1 | Frontend Developer | `internal/ui/main_window.go` | In `build()`: wrap each column's two cards in `container.NewBorder(top=upperCard, nil, nil, nil, lowerCard)` so the lower card expands. Compose the two bordered columns with `container.NewGridWithColumns(2)` (equal width + equal height, no dead space). Remove/replace `hGap(16)` between columns; keep `vGap` only if still used, else drop. |
| B.2 | Frontend Developer | `internal/ui/main_window.go` | Keep `NewMainWindow` `Resize(960, 600)` (or `900×640`), `CenterOnScreen`, and `newMinSizeWrap(..., fyne.NewSize(900, 560))`. Verify the new layout's natural min size; raise the floor only if content exceeds it (avoid blank-window regression — floor must be ≤ window size). |
| C.1 | Frontend Developer | `internal/ui/main_window.go` | Polish header: wrap `header` in `container.NewPadded` (comfortable padding), keep bold title + subtitle + `widget.NewSeparator()`. No branding icon required. |
| D.1 | Frontend Developer | `internal/ui/main_window.go` | Keep button row at `Border` bottom: Simpan (left), `layout.NewSpacer()`, Putuskan, gap, Hubungkan (`HighImportance`, right). Ensure it sits flush at the bottom and looks clean. |
| E.1 | Documentation | `README.md` | Note the custom accent theme (teal/indigo) and that the layout is a balanced two-column grid filling full height. |

---

## Constraints (must preserve)

- Do NOT change any handler logic: `onSave`, `onConnect`, `onDisconnect`, `onClearLog`, `appendLog`, `applyEnablement`, `loadInitial`, `applyConfig`, `setStatus`, `profileName`, `finishSave`, etc.
- Keep `fyne.Do` threading, `minSizeWrap` widget, app ID `com.vepeen.app`, `FyneApp.toml`, dark theme base, Indonesian labels, and all controls (entries, cards, buttons, log).
- Keep split-tunnel enforcement logic untouched (in `internal/vpn`, `internal/route`).
- No blank-window regression: `minSizeWrap` floor must remain ≤ window size and the wrapped object must stay a proper `fyne.Widget` (per fix-005).

---

## Acceptance Criteria

- [ ] Both columns fill the full window height — no dead space at the bottom (Log card expands to fill right column; Rute card expands to fill left column).
- [ ] The two columns are equal height and visually balanced (grid stretches both cells).
- [ ] Accent color is visible (primary buttons / focus / highlights use the custom accent, e.g. teal `#0FB5AE` or indigo `#4F46E5`); Dark variant retained.
- [ ] Header has comfortable padding, bold title, subtitle, and separator (no bare cramped header).
- [ ] Button row sits at the bottom: Simpan (left), Spacer, Putuskan + Hubungkan (HighImportance, right); looks clean.
- [ ] `go build ./...` and `go vet ./...` are clean.
- [ ] No blank window on launch (minSizeWrap floor ≤ window size; widget-based wrap intact).
- [ ] `fyne.Do` usage, app ID `com.vepeen.app`, `FyneApp.toml`, and Indonesian labels are unchanged.
- [ ] All handlers and split-tunnel enforcement logic are byte-for-byte unchanged in behavior.

---

## Regression Risk

- Low. Changes are confined to `build()` layout composition and theme application. The only risk is a too-large `minSizeWrap` floor causing a blank/oversized window — mitigated by keeping floor ≤ window size and reusing the existing `minSizeWrap` widget from fix-005.
- Verify the Log card still scrolls/renders correctly when expanded (it previously used `SetMinRowsVisible(5)`; with Border expansion it should grow, not clip).

## Rollback Strategy

- Revert `cmd/vepeen/main.go` theme line to `theme.DarkTheme()` and delete `internal/ui/theme.go`; revert `build()` to the pre-fix `NewHBox`/`NewVBox` composition. No data or config changes to roll back.
