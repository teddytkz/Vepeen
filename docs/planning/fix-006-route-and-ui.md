# Fix Plan: Route sync bug + UI visual redesign

**Related PRD / UI:** PRD-002 (`docs/planning/prd-002-l2tp-split-tunnel.md`), ui-003 (`docs/planning/ui-003-simplified-form-log.md`)  
**Severity:** Issue A = Critical (blocks connect); Issue B = Medium (visual quality)  
**Reported by:** Planner (code review of `internal/route/sync_windows.go` + UI quality pass)  
**Date:** 2026-07-22  
**Status:** Ready for implementation  
**Version:** v1.0.0

---

## Issue A — Route sync bug (CRITICAL, blocks connect)

### Bug summary

`listRoutes` in `internal/route/sync_windows.go` invokes `Get-VpnConnectionRoute -ConnectionName X`, a cmdlet that **does not exist** in the Windows `VpnClient` PowerShell module. The module only ships: `Add-VpnConnection`, `Set-VpnConnection`, `Remove-VpnConnection`, `Add-VpnConnectionRoute`, `Remove-VpnConnectionRoute`, `Get-VpnConnection`. Because the cmdlet is unknown, PowerShell errors on every `SyncRoutes` call, so `listRoutes` returns an error and `SyncRoutes` aborts before adding/removing any split-tunnel route. The connect flow (`EnsureProfile` → `SyncRoutes` → `rasdial`) therefore fails — connect is blocked.

### Root cause analysis

- `listRoutes` (around line 64) builds a script using `Get-VpnConnectionRoute`, which is not a real cmdlet → PowerShell throws "The term 'Get-VpnConnectionRoute' is not recognized...".
- `runPowerShell` returns that error text; `listRoutes` only degrades to empty when `out == ""` **and** `isSoftRouteListError(err)` is true. The current `isSoftRouteListError` (lines ~113–121) only matches "not found" / "tidak ditemukan" / "no vpn" / "cannot find" — it does **not** match the "not recognized" / "tidak dikenali" wording PowerShell uses for an unknown command, so the error propagates and aborts connect.
- Existing routes are actually readable via `Get-VpnConnection -Name X` and its `.Routes` property (each item exposes `.DestinationPrefix`).

### Fix strategy

**Option A: Correct cmdlet + broaden soft-error guard (recommended)**

- **Files:** `internal/route/sync_windows.go`, `README.md`
- **Risk:** Low — only changes how routes are *read*; `addRoute`/`removeRoute` (valid cmdlets) untouched; no VPN profile/status/secrets changes.
- **Effort:** S

**Option B: Ignore the error entirely**

- Would mask real failures (e.g., profile missing) and silently skip route sync — unsafe. Not recommended.

**Recommended:** Option A

### File scope

- `internal/route/sync_windows.go` — `listRoutes` (line ~64) + `isSoftRouteListError` (lines ~113–121)
- `README.md` — Troubleshooting row (line 236)
- **Out of scope:** `addRoute`/`removeRoute`, `internal/vpn/*` (profile/status/powershell), `internal/secrets`, `internal/config`, `internal/ui`

### Implementation tasks

**Agent for implementation:** Backend Developer  
**Parallelizable:** No — single Go file + one README line

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| A.1 | Backend Developer | `internal/route/sync_windows.go` | In `listRoutes`, replace the `Get-VpnConnectionRoute` script with: `$ErrorActionPreference='Stop'; $c = Get-VpnConnection -Name %s -ErrorAction SilentlyContinue; if ($null -eq $c -or $null -eq $c.Routes) { exit 0 }; @($c.Routes) \| ForEach-Object { $_.DestinationPrefix }` — keep `psQuote(connectionName)` as the format arg. |
| A.2 | Backend Developer | `internal/route/sync_windows.go` | Broaden `isSoftRouteListError` to also treat "not recognized" and "tidak dikenali" as soft (degrade to empty list), so future cmdlet regressions don't abort connect. |
| A.3 | Backend Developer | `README.md` | Update Troubleshooting line 236: remove the "check `Get-VpnConnectionRoute`" hint; state routes are read via `Get-VpnConnection` (`.Routes` / `.DestinationPrefix`). |
| A.4 | Backend Developer | `internal/route/parse_test.go` (or new test) | Add/extend a unit test for `isSoftRouteListError` covering "not recognized" and "tidak dikenali" inputs; add a test asserting the `listRoutes` script string contains `Get-VpnConnection -Name` and `DestinationPrefix` (string-level, no real PowerShell needed). |

### Acceptance criteria

- [ ] `listRoutes` script uses `Get-VpnConnection -Name` + `.Routes` + `.DestinationPrefix`; no `Get-VpnConnectionRoute` remains.
- [ ] `isSoftRouteListError` returns true for messages containing "not recognized" and "tidak dikenali".
- [ ] `go build ./cmd/vepeen` succeeds; `go test ./internal/route` passes.
- [ ] README troubleshooting no longer references `Get-VpnConnectionRoute`; says routes read via `Get-VpnConnection`.
- [ ] Against a real profile, `SyncRoutes` no longer errors on the list step; connect flow proceeds to add/remove split-tunnel routes.

### Regression risk

- `addRoute`/`removeRoute` unchanged and remain valid; only the read path changed.
- Broadening the soft-error guard could in theory hide a genuinely missing profile, but `SyncRoutes` still validates the connection name and `addRoute`/`removeRoute` will surface real failures.

---

## Issue B — Ugly UI (MEDIUM, visual quality)

### Bug summary

The current `build()` in `internal/ui/main_window.go` uses default Fyne widgets with cramped spacing, no visual grouping, and weak hierarchy. User feedback: "jelek banget" (very ugly). All functionality, Indonesian labels, and controls are correct and must be preserved — this is a visual redesign only.

### Root cause analysis

- Controls are stacked in plain `VBox`/`Border` with `widget.NewSeparator()` dividers and no `Card` grouping → no clear visual sections.
- Default theme + default widget sizing; no consistent theme variant or brand color; primary action (`Hubungkan`) is emphasized only by `HighImportance` with no spatial prominence.
- Spacing/padding is minimal; form labels and entries feel cramped.

### Fix strategy

**Option A: Card-grouped redesign via Designer handoff (recommended)**

- **Files:** `internal/ui/main_window.go` (`build()`), optional `internal/ui/theme.go` (custom theme), `README.md` (UI section)
- **Risk:** Low–Medium — layout-only; must preserve `fyne.Do` threading, `minSizeWrap` widget, app ID, `FyneApp.toml`, all handlers/controllers.
- **Effort:** M

**Option B: Tweak spacing only**

- Keeps flat layout; does not address grouping/hierarchy complaints. Not recommended.

**Recommended:** Option A — route through the Designer (UI/UX) agent for a concrete layout spec, then Frontend Developer implements.

### Handoff to Designer (UI/UX)

The Planner defers the concrete layout to the **Designer (UI/UX) agent**, who must produce a layout spec covering:

- **Theme:** apply a consistent `fyne` theme variant (dark or light) via `app.Settings().SetTheme(...)` or a custom theme implementing `fyne.Theme` for a brand accent color; keep app ID `com.vepeen.app` and `FyneApp.toml` intact.
- **Card grouping** (`container.NewCard` with title + content):
  - **Koneksi** card — server (IP), Key (PSK), Username, Password form.
  - **Rute split tunnel** card — multi-line routes entry + helper text.
  - **Status** card — `statusPri` + `statusDet`.
  - **Log** card — read-only log entry + "Bersihkan log" button.
- **Spacing/padding:** comfortable gaps via `container.NewVBox` with `layout.NewSpacer()`/padding; cards separated by spacing; entries comfortable height.
- **Primary action prominence:** `Hubungkan` (HighImportance) visually dominant; `Simpan` + `Putuskan` in a clean secondary button row.
- **Form:** use `widget.NewForm` with proper Indonesian labels; entries comfortable height.
- **Window:** keep ~480–520 wide, scrollable if content exceeds height; preserve `minSizeWrap` floor and `fyne.Do` threading.

### File scope

- `internal/ui/main_window.go` — `build()` (layout only)
- `internal/ui/theme.go` — new optional custom theme (brand color)
- `README.md` — UI section note (optional)
- **Preserved intact:** `fyne.Do` calls, `minSizeWrap` widget, app ID, `FyneApp.toml`, all controller fields/handlers (`onSave`, `onConnect`, `onDisconnect`, `onClearLog`, `appendLog`, `loadInitial`, `applyConfig`, `applyEnablement`, `setStatus`, `profileName`), labels in Indonesian, same controls.

### Implementation tasks

**Agents:** Designer (UI/UX) → Frontend Developer → Debugger/Reviewer → Documentation  
**Parallelizable:** No — sequential handoff (design spec before implementation)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| B.1 | Designer (UI/UX) | (spec doc) | Produce concrete layout spec: theme variant/brand color, 4 `Card` groups (Koneksi / Rute split tunnel / Status / Log), spacing/padding rules, button row, window width/scroll. Reference fix-006 Issue B. |
| B.2 | Frontend Developer | `internal/ui/main_window.go` | Rebuild `build()` using `container.NewCard` for the four groups; apply theme variant; group form into Koneksi card; routes into Rute card; status into Status card; log into Log card. Keep all existing widgets/labels/handlers. |
| B.3 | Frontend Developer | `internal/ui/main_window.go` | Place `Simpan` + `Putuskan` in a clean secondary row; make `Hubungkan` prominent (HighImportance, clear spatial emphasis). Keep `minSizeWrap` floor and `fyne.Do` threading. |
| B.4 | Frontend Developer | `internal/ui/theme.go` (new, optional) | Add custom `fyne.Theme` for brand accent color if a custom theme is chosen; wire via `app.Settings().SetTheme(...)` in `NewMainWindow` without changing app ID. |
| B.5 | Frontend Developer | `internal/ui/main_window.go` | Keep window ~480–520 wide and scrollable; ensure `NewVScroll` wraps the card column; preserve `minSizeWrap(..., fyne.NewSize(420,600))`. |
| B.6 | Documentation | `README.md` | Update UI section to describe the card-grouped layout (optional, if wording changed). |

### Acceptance criteria

- [ ] Four `Card` groups present: Koneksi, Rute split tunnel, Status, Log.
- [ ] Consistent theme variant applied; optional brand color via custom theme.
- [ ] `Hubungkan` visually prominent; `Simpan`/`Putuskan` in a clean secondary row.
- [ ] Comfortable spacing/padding; entries comfortable height; window ~480–520 wide and scrollable.
- [ ] All existing functionality preserved: Indonesian labels, same controls, `fyne.Do` threading, `minSizeWrap`, app ID, `FyneApp.toml` unchanged.
- [ ] `go build ./cmd/vepeen` succeeds; app launches with no blank window (fix-005 floor preserved) and no migration warning (fix-004).

### Regression risk

- Layout change could reintroduce the form-collapse bug (fix-002/fix-005) if `minSizeWrap`/`NewVScroll` floor is removed — must keep them.
- Custom theme must not alter app ID or break `FyneApp.toml` migration opt-in.

---

## Rollback strategy

- Issue A: revert `sync_windows.go` `listRoutes` + `isSoftRouteListError` and README line via git; `addRoute`/`removeRoute` unaffected.
- Issue B: revert `main_window.go` `build()` (and remove `theme.go` if added) via git; functionality unchanged so safe to revert.

## Version history

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-22 | Initial fix plan: A (route read cmdlet + soft-error guard) + B (UI card redesign via Designer handoff) |
