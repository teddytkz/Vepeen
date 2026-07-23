# Design Spec — Issue B: UI Visual Redesign (Card-grouped layout)

**Project:** vepeen (Fyne v2.8.0 desktop app, `com.vepeen.app`)
**Source plan:** `docs/planning/fix-006-route-and-ui.md` → Issue B
**Type:** Modified Component (layout-only redesign of `internal/ui/main_window.go` `build()`)
**Status:** Ready for Frontend Developer
**Constraints preserved:** all handlers/logic (`onSave`, `onConnect`, `onDisconnect`, `onClearLog`, `appendLog`, `loadInitial`, `applyConfig`, `applyEnablement`, `setStatus`, `profileName`), `fyne.Do` threading, `minSizeWrap` widget, app ID, `FyneApp.toml`, Indonesian labels, same controls.

---

## 1. Theme

**Recommendation: built-in Dark theme + accent via widget importance.** Keep it simple — no custom theme required for the baseline.

- Set in `cmd/vepeen/main.go` right after `app.NewWithID("com.vepeen.app")` (do NOT change the app ID or move it):

```go
a := app.NewWithID("com.vepeen.app")
a.Settings().SetTheme(theme.DarkTheme()) // built-in dark variant
```

- `theme.DarkTheme()` is deprecated in v2.8 but still functional and the simplest path. Do **not** introduce `FYNE_THEME` env coupling.
- Brand accent is expressed through `widget.Importance`, not a custom palette: `Hubungkan` uses `widget.HighImportance` (already set in code). Secondary buttons stay `MediumImportance` (default).

**Optional custom theme (stretch, only if a brand color is wanted):** implement `fyne.Theme` wrapping `theme.DefaultTheme()` and override `Color(theme.ColorNamePrimary, variant)` to a teal/indigo (e.g. `#0FB5AE` teal or `#4F46E5` indigo). Wire via `a.Settings().SetTheme(&brandTheme{variant: theme.VariantDark})`. This is **out of scope for the baseline**; the Frontend Developer may add `internal/ui/theme.go` only if the user opts in. Keep `minSizeWrap` and app ID untouched.

---

## 2. Layout Structure (4 Cards in a vertical scroll)

Replace the current flat `VBox`/`Border`/`Separator` stack with **four `container.NewCard` groups** inside a single `container.NewVScroll`. Keep the proven Border skeleton (header top, button row bottom, scroll center) so the `minSizeWrap` floor and scroll behavior from fix-002/fix-005 are preserved.

### Card 1 — "Koneksi VPN"
- `container.NewCard("Koneksi VPN", "", content)`
- `content` = `widget.NewForm(...)` with `FormItem`s (Indonesian labels, same widgets):
  - `widget.NewFormItem("IP", c.serverEntry)`
  - `widget.NewFormItem("Key (PSK)", c.pskEntry)`  ← `widget.NewPasswordEntry()`
  - `widget.NewFormItem("Username", c.userEntry)`
  - `widget.NewFormItem("Password", c.passEntry)` ← `widget.NewPasswordEntry()`
- Keep placeholders: server `vpn.contoh.com atau IP`, user `nama.pengguna`.

### Card 2 — "Rute Split Tunnel"
- `container.NewCard("Rute Split Tunnel", "", content)`
- `content` = `container.NewVBox(...)`:
  1. `routesDuty` label: `"Wajib · satu IP/CIDR per baris. Hanya daftar ini lewat VPN."` (`TextWrapWord`)
  2. `c.routesEntry` = `widget.NewMultiLineEntry()`, `SetMinRowsVisible(3)`, `Wrapping = TextWrapOff`
  3. `routesHelp` label: `"Contoh: 10.10.0.0/16 atau 203.0.113.50. Kosong diabaikan. # = komentar."` (`TextWrapWord`)

### Card 3 — "Status"
- `container.NewCard("Status", "", content)`
- `content` = `container.NewVBox(...)`:
  1. `c.statusPri` label — emphasize: `TextStyle{Bold: true}` (primary state text)
  2. `c.statusDet` label (`TextWrapWord`, secondary detail)

### Card 4 — "Log"
- `container.NewCard("Log", "", content)`
- `content` = `container.NewVBox(...)`:
  1. `logHeader` = `container.NewBorder(nil, nil, logTitle, c.btnClearLog)` where `logTitle` = `widget.NewLabel("Log")` bold
  2. `c.logEntry` = `widget.NewMultiLineEntry()`, `SetMinRowsVisible(5)`, `Wrapping = TextWrapOff`, `Disable()` (read-only)

---

## 3. Button Row

A single clean horizontal row, fixed at the bottom of the window (Border bottom) so the primary action is always visible:

- `container.NewHBox(c.btnSave, layout.NewSpacer(), c.btnDisc, hGap(8), c.btnConn)`
  - `c.btnSave` ("Simpan") — left, `MediumImportance` (default)
  - `layout.NewSpacer()` pushes the right group to the far edge
  - `c.btnDisc` ("Putuskan") — `MediumImportance`
  - `c.btnConn` ("Hubungkan") — `widget.HighImportance` (prominent, right-most)
- Wrap the row in `container.NewPadded(...)` for outer side/bottom margin.

---

## 4. Spacing / Padding

Fyne `VBox`/`HBox` have no built-in gap, so use fixed spacers. Add two tiny layout-only helpers (no logic change):

```go
func vGap(h float32) fyne.CanvasObject {
    r := canvas.NewRectangle(nil)
    r.Resize(fyne.NewSize(0, h)) // canvas.Rectangle.MinSize() returns its Size
    return r
}
func hGap(w float32) fyne.CanvasObject {
    r := canvas.NewRectangle(nil)
    r.Resize(fyne.NewSize(w, 0))
    return r
}
```

**Spacing rules:**
- Outer margin around the scroll column: wrap in `container.NewPadded(...)` (≈ theme padding, ~8px).
- Inter-card gap: `vGap(12)` between each card (12px; use 16px if more air is desired).
- Button row internal gap: `hGap(8)` between `Putuskan` and `Hubungkan`.
- Card internal content padding is provided automatically by `widget.Card` (theme padding). Do **not** double-pad form content.
- Entry comfort: keep `SetMinRowsVisible` values (routes 3, log 5); single-line entries use default height (comfortable in Dark theme).

---

## 5. Window

- Keep `w.Resize(fyne.NewSize(480, 720))` and `w.CenterOnScreen()` in `NewMainWindow`.
- Keep `w.SetContent(newMinSizeWrap(ctrl.build(), fyne.NewSize(420, 600)))` — **do not remove** (fix-005 floor; prevents blank window and form-collapse).
- Content is scrollable: the card column lives inside `container.NewVScroll(...)`.
- `build()` still returns `container.NewPadded(root)` (root = Border). No change to `NewMainWindow` except the optional theme line in `main.go`.

---

## 6. Hierarchy

- **Top:** bold title `Vepeen` + subtitle `L2TP/IPsec · split tunnel` + `widget.NewSeparator()` (header, fixed).
- **Middle (scroll):** four cards in order — Koneksi VPN → Rute Split Tunnel → Status → Log — separated by `vGap(12)`.
- **Bottom (fixed):** button row with `Hubungkan` (HighImportance) as the dominant, right-aligned primary action; `Simpan` left, `Putuskan` secondary-right.
- Primary action prominence comes from (a) `HighImportance` fill color and (b) right-edge placement + spacer separation.

---

## 7. Pseudo-code Layout Tree (Frontend Developer guide)

```
NewMainWindow(a)
└─ w.SetContent( newMinSizeWrap( build(), Size(420,600) ) )   // UNCHANGED
   build() returns container.NewPadded(root):

   root = container.NewBorder( header, buttonRow, nil, nil, scroll )

   header = VBox(
       Label("Vepeen")            [Bold]
       Label("L2TP/IPsec · split tunnel")  [WrapWord]
       Separator
   )

   scroll = VScroll( NewPadded( VBox(
       cardKoneksi
       vGap(12)
       cardRute
       vGap(12)
       cardStatus
       vGap(12)
       cardLog
   )))

   cardKoneksi = NewCard("Koneksi VPN", "",
       NewForm(
           FormItem("IP",        serverEntry)
           FormItem("Key (PSK)", pskEntry)      // PasswordEntry
           FormItem("Username",  userEntry)
           FormItem("Password",  passEntry)     // PasswordEntry
       ))

   cardRute = NewCard("Rute Split Tunnel", "",
       VBox(
           Label(routesDuty)  [WrapWord]
           routesEntry        // MultiLineEntry, MinRows 3, WrapOff
           Label(routesHelp)  [WrapWord]
       ))

   cardStatus = NewCard("Status", "",
       VBox(
           statusPri  [Bold]     // primary state
           statusDet  [WrapWord] // detail
       ))

   cardLog = NewCard("Log", "",
       VBox(
           Border(left=Label("Log")[Bold], right=btnClearLog)
           logEntry             // MultiLineEntry, MinRows 5, WrapOff, Disable()
       ))

   buttonRow = NewPadded( HBox(
       btnSave                       // "Simpan"      MediumImportance
       Spacer
       btnDisc                       // "Putuskan"    MediumImportance
       hGap(8)
       btnConn                       // "Hubungkan"   HighImportance  (right-most)
   ))
```

**Widget/controller fields reused (unchanged):** `serverEntry`, `pskEntry`, `userEntry`, `passEntry`, `routesEntry`, `logEntry`, `btnSave`, `btnDisc`, `btnConn`, `btnClearLog`, `statusPri`, `statusDet`. All handlers (`onSave`, `onConnect`, `onDisconnect`, `onClearLog`, `appendLog`, `applyEnablement`, `loadInitial`) stay exactly as-is — only the container tree in `build()` changes.

---

## 8. Acceptance Checklist (mirrors fix-006 Issue B)

- [ ] Four `Card` groups present: Koneksi VPN, Rute Split Tunnel, Status, Log.
- [ ] Consistent Dark theme applied via `a.Settings().SetTheme(theme.DarkTheme())` in `main.go`; app ID `com.vepeen.app` unchanged.
- [ ] `Hubungkan` visually prominent (HighImportance, right edge); `Simpan`/`Putuskan` in a clean secondary row.
- [ ] Comfortable spacing: `vGap(12)` between cards, `hGap(8)` between right buttons, `NewPadded` outer margin; entries comfortable height.
- [ ] Window ~480–520 wide, scrollable; `minSizeWrap(..., Size(420,600))` preserved.
- [ ] All functionality preserved: Indonesian labels, same controls, `fyne.Do` threading, `FyneApp.toml` unchanged.
- [ ] `go build ./cmd/vepeen` succeeds; app launches with no blank window (fix-005) and no migration warning (fix-004).

## 9. Frontend Developer Decomposition

- **Sub-agent A (layout):** rewrite `build()` per the tree above — 4 cards, scroll, button row, `vGap`/`hGap` helpers. Files: `internal/ui/main_window.go`.
- **Sub-agent B (theme):** add `a.Settings().SetTheme(theme.DarkTheme())` in `cmd/vepeen/main.go` after `app.NewWithID`. Optional `internal/ui/theme.go` only if brand color opted in.
- **Reviewer:** verify `minSizeWrap` + `NewVScroll` floor intact (no form-collapse regression), all handlers untouched, `go build` + launch clean.
