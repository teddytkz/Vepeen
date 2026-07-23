# Design Spec: Landscape Two-Column UI (Fix-007 Issue B)

**Type:** Modified Screen (landscape layout only — no logic change)
**Target file:** `internal/ui/main_window.go` (`NewMainWindow` + `build()`)
**Related:** `docs/planning/fix-007-split-tunnel-landscape.md` (Issue B), `design-ui006-issueB.md`
**Status:** Ready for Frontend Developer
**Date:** 2026-07-22

---

## 0. Goal

Convert the portrait, scroll-required window into a **landscape, no-scroll** window.
Four existing `Card`s are preserved and rearranged into **two columns**:

- **Left column:** `Koneksi VPN` (form) + `Rute Split Tunnel`
- **Right column:** `Status` + `Log`

No handler, controller field, `fyne.Do` call, `minSizeWrap` widget, app ID, `FyneApp.toml`,
or Indonesian label changes. This is a **pure layout** redesign.

---

## 1. Window size & minSizeWrap floor

In `NewMainWindow`:

| Property | Current | New |
| --- | --- | --- |
| `w.Resize(...)` | `fyne.NewSize(480, 720)` | `fyne.NewSize(960, 600)` |
| `w.CenterOnScreen()` | kept | kept |
| `newMinSizeWrap(..., floor)` | `fyne.NewSize(420, 600)` | **`fyne.NewSize(900, 560)`** |

Rationale for the floor `900×560`:
- Width `900` guarantees each of the two columns gets ≥ ~440px (after a 16px gap + padding),
  enough for the `Koneksi VPN` form and the `Rute` multiline entry without clipping.
- Height `560` guarantees the tallest column (left: form + routes) fits without vertical scroll
  at the smallest allowed window. The window opens at `960×600`, which is comfortably above the floor.

> The `minSizeWrap` widget already takes `max(inner.MinSize(), floor)`, so raising the floor alone
> prevents the window from shrinking below the two-column content. **Do not remove `minSizeWrap`** —
> removing it reintroduces the fix-002/fix-005 blank/collapsed-window regression.

---

## 2. Theme

**Preserve the existing dark theme.** No change required.

- `cmd/vepeen/main.go` keeps `a.Settings().SetTheme(theme.DarkTheme())`.
- No custom theme, no brand color, no `FYNE_THEME` coupling for the baseline.
- All `widget.HighImportance` / default importance styling stays as-is (`Hubungkan` = HighImportance).

---

## 3. Two-column layout structure

**Recommended approach:** two `VBox` columns placed inside `container.NewHBox(left, hGap(16), right)`,
wrapped in `container.NewPadded(...)`, placed in the `Border` center.

Why HBox-of-VBox (per Planner recommendation): each column sizes to its own content height
independently, so the shorter right column top-aligns cleanly while the left column defines the
row height. Equal-width alternative `container.NewGridWithColumns(2)` is acceptable if the
developer prefers perfectly balanced columns — both satisfy the spec; HBox is the primary.

**No full-height `VScroll` of all four cards.** Content fits at `960×600` / floor `900×560`.
- Primary: `Border` center = `Padded(HBox(leftCol, hGap(16), rightCol))`. No scroll.
- Fallback (optional safety only): wrap the HBox in `container.NewVScroll(...)` so an unusually
  small OS font scale still scrolls instead of clipping. If used, keep it as the *outer* wrapper of
  the two columns only — do **not** return to the old single vertical stack of all four cards.

### Layout tree (pseudo-code — NOT final Go)

```
NewMainWindow(a fyne.App) fyne.Window:
  w := a.NewWindow("Vepeen")
  w.Resize(fyne.NewSize(960, 600))          // was (480, 720)
  w.CenterOnScreen()
  ctrl := newController()
  ctrl.win = w
  w.SetContent(newMinSizeWrap(ctrl.build(), fyne.NewSize(900, 560)))  // floor was (420,600)
  ctrl.loadInitial()
  return w

build() fyne.CanvasObject:

  header = VBox(
    Label("Vepeen")              [TextStyle Bold]
    Label("L2TP/IPsec · split tunnel")   [Wrapping = Word]
    Separator()
  )

  // ---- LEFT COLUMN ----
  cardKoneksi = Card("Koneksi VPN", "",
    Form(
      FormItem("IP",        serverEntry)          // Entry, placeholder "vpn.contoh.com atau IP"
      FormItem("Key (PSK)", pskEntry)             // PasswordEntry
      FormItem("Username",  userEntry)            // Entry, placeholder "nama.pengguna"
      FormItem("Password",  passEntry)            // PasswordEntry
    )
  )

  cardRute = Card("Rute Split Tunnel", "",
    VBox(
      Label("Wajib · satu IP/CIDR per baris. Hanya daftar ini lewat VPN.")  [Word]
      routesEntry                                          // MultiLineEntry, MinRowsVisible(3), WrapOff
      Label("Contoh: 10.10.0.0/16 atau 203.0.113.50. Kosong diabaikan. # = komentar.") [Word]
    )
  )

  leftCol = VBox(cardKoneksi, vGap(12), cardRute)

  // ---- RIGHT COLUMN ----
  cardStatus = Card("Status", "",
    VBox(
      statusPri   // Label [Bold, Word]  ("Terputus" / "Terhubung" / ...)
      statusDet   // Label [Word]
    )
  )

  cardLog = Card("Log", "",
    VBox(
      Border(nil, nil, logTitle, btnClearLog)   // logTitle [Bold] "Log"  |  btnClearLog "Bersihkan log"
      logEntry                                 // MultiLineEntry, MinRowsVisible(5), WrapOff, Disabled
    )
  )

  rightCol = VBox(cardStatus, vGap(12), cardLog)

  // ---- BODY (two columns) ----
  columns = HBox(leftCol, hGap(16), rightCol)   // alt: GridWithColumns(2)
  body     = Padded(columns)

  // ---- BUTTON ROW (bottom) ----
  buttonRow = Padded(
    HBox(
      btnSave "Simpan"
      Spacer()
      btnDisc "Putuskan"
      hGap(8)
      btnConn "Hubungkan"   [Importance = HighImportance]
    )
  )

  root = Border(header, buttonRow, nil, nil, body)
  return Padded(root)
```

---

## 4. Card placement & content (all preserved)

| Card | Column | Content (unchanged) |
| --- | --- | --- |
| `cardKoneksi` | Left (top) | `widget.NewForm` — IP, Key (PSK), Username, Password |
| `cardRute` | Left (bottom) | `routesDuty` label + `routesEntry` (MultiLine, 3 rows) + `routesHelp` label |
| `cardStatus` | Right (top) | `statusPri` (bold) + `statusDet` |
| `cardLog` | Right (bottom) | `logHeader` (Border: `logTitle` + `btnClearLog`) + `logEntry` (MultiLine, 5 rows, Disabled) |

All widget instances (`serverEntry`, `pskEntry`, `userEntry`, `passEntry`, `routesEntry`,
`logEntry`, `statusPri`, `statusDet`, `btnSave`, `btnDisc`, `btnConn`, `btnClearLog`) are created
exactly as today and bound to the same controller fields/handlers. Only their *containers* change.

---

## 5. Spacing & padding rules

- **Outer margin:** `container.NewPadded(...)` around `root` (≈ Fyne default padding, ~8px).
- **Between the two columns:** `hGap(16)` inside the HBox (or rely on `GridWithColumns(2)`'s
  built-in gap). Keep it ≥ 12px for clear visual separation.
- **Between cards within a column:** `vGap(12)` (existing helper — unchanged).
- **Card internals:** unchanged from current `build()` (form item spacing, multiline min rows).
- **Button row:** `Padded` HBox; `Simpan` left, `Spacer()` pushes `Putuskan` + `Hubungkan`
  to the right; `hGap(8)` between the two right buttons.

---

## 6. Header & button row (preserved positions)

- **Header (Border top):** bold `Vepeen` + subtitle `L2TP/IPsec · split tunnel` + `Separator()`.
  Unchanged from current `build()`.
- **Button row (Border bottom):** `Simpan` (left) · spacer · `Putuskan` + `Hubungkan`
  (right, `Hubungkan` = `HighImportance`). Unchanged logic/order; only the surrounding
  `Border` center changes from the old single VScroll to the two-column body.

---

## 7. Component states (must remain intact)

These are driven by `applyEnablement()` and are **unchanged** by this redesign:

| Element | States preserved |
| --- | --- |
| Form entries (`serverEntry`, `pskEntry`, `userEntry`, `passEntry`, `routesEntry`) | Enabled when disconnected/error; Disabled when connecting/connected/busy |
| `logEntry` | Always Disabled (read-only) |
| `btnSave` | Disabled while busy; else Enabled |
| `btnConn` | Enabled only when disconnected & not busy; Disabled otherwise (incl. connecting/disconnecting) |
| `btnDisc` | Enabled only when connected & not busy |
| `btnClearLog` | Always Enabled |

No new visual states are introduced. The dark theme + `HighImportance` affordance for
`Hubungkan` already provides the required focus/active/disabled distinction.

---

## 8. Accessibility & responsiveness

- **No scroll at target size:** verified by the `900×560` floor — both columns fit within it.
- **Keyboard:** all controls remain focusable in DOM/tab order; `Border` + `HBox`/`VBox`
  preserve natural tab order (header → left column → right column → button row). No `outline:none`
  or focus removal is introduced.
- **Screen reader:** semantic `Card`/`Form`/`Label` widgets unchanged; no ARIA needed beyond
  what Fyne provides.
- **Contrast:** dark theme unchanged → existing AA contrast preserved.
- **Smaller screens:** `minSizeWrap` floor prevents shrink below content; if the optional
  VScroll fallback is used, it only engages below `900×560`.

---

## 9. Frontend Developer guidance

**Files to edit:** `internal/ui/main_window.go` only (`NewMainWindow` + `build()`).

**Change (B.2 — `NewMainWindow`):**
- `w.Resize(fyne.NewSize(480, 720))` → `fyne.NewSize(960, 600)`.
- `newMinSizeWrap(ctrl.build(), fyne.NewSize(420, 600))` → `fyne.NewSize(900, 560)`.
- Keep `w.CenterOnScreen()` and `ctrl.loadInitial()`.

**Change (B.3 — `build()`):**
- Replace the single `container.NewVScroll(container.NewPadded(container.NewVBox(...)))` block
  with: `leftCol = VBox(cardKoneksi, vGap(12), cardRute)`,
  `rightCol = VBox(cardStatus, vGap(12), cardLog)`,
  `body = Padded(HBox(leftCol, hGap(16), rightCol))` (or `GridWithColumns(2)`).
- `root = container.NewBorder(header, buttonRow, nil, nil, body)`; return `Padded(root)`.

**Keep intact (do NOT touch):**
- All widget creation code, controller fields, and handler bindings.
- `vGap` / `hGap` helpers (reuse them).
- `fyne.Do` threading in `loadInitial`, `onConnect`, `onSave`, `onClearLog`, `appendLog`.
- `minSizeWrap` widget definition and `newMinSizeWrap` constructor.
- App ID `com.vepeen.app`, `FyneApp.toml`, Indonesian labels, dark theme.

**Verification:**
- `go build ./cmd/vepeen` and `go vet ./internal/ui ./cmd/vepeen` succeed.
- App launches centered, landscape, **no vertical scroll** for the four cards at `960×600`.
- Window cannot be dragged smaller than `900×560` (floor enforced).
- No blank window (fix-005) and no migration warning (fix-004) — both depend on keeping
  `minSizeWrap` + the raised floor.
