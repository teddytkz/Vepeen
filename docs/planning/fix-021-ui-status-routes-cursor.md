# Fix Plan: fix-021 — Status detail, routes area height, entry cursor

**Related PRD:** PRD-002 / PRD-006 (split tunnel UI + Route All Traffic)
**Severity:** Low (UI polish only)
**Reported by:** User
**Date:** 2026-07-24
**Scope:** Minor — changelog + this short fix plan (no full PRD)

---

## Outcomes

1. **Status detail (footer):** When VPN has active traffic, footer detail shows only `Traffic Route On` — no URL/IP list. Activity log still logs full `VPN traffic: …` details.
2. **Routes text area height:** Split-tunnel routes multi-line entry expands to fill empty space above the "Route All Traffic" checkbox (no dead gap under the checkbox).
3. **Entry caret visibility:** Focused text entries show a clearly visible blinking caret on the dark input fill. Prefer theme-level fix; no custom Entry widget.

---

## Root cause (brief)

| # | Symptom | Cause |
| - | ------- | ----- |
| 1 | Footer shows hosts/IPs next to Connected | `startTraffic` passes `"VPN traffic: "+sig` into `setStatus` detail |
| 2 | Empty space under Route All Traffic | `cardRoutes` is `VBox` — extra height sits below last child (`routeAllCheck`), not in `routesEntry` |
| 3 | Cursor hard to see / “invisible” | Fyne v2.8 has **no** `ColorNameCursor`. Entry caret color is `theme.Color(theme.ColorNamePrimary)` in `widget/entry_cursor_anim.go` (app-global, not `ColorForWidget`). Current Primary is teal `#2dd4bf` — should be visible; if still weak on this fill, only theme lever is Primary (global). |

---

## Fix strategy (minimal)

### Fix 1 — status detail string only

**File:** `internal/ui/main_window.go`  
**Symbol:** `(*controller).startTraffic` (~line 595)

```go
// KEEP log with details:
c.appendLog("VPN traffic: " + sig)
// CHANGE status detail only:
c.setStatus(vpn.StatusConnected, "Connected", "Traffic Route On")
```

Empty-traffic branch stays: `"No active connections through the VPN."`  
Do **not** change `ActiveConnections`, log format, or other `setStatus` call sites.

### Fix 2 — Border layout for routes card

**File:** `internal/ui/main_window.go`  
**Symbol:** `(*controller).build` — `cardRoutes` (~lines 220–230)

Today:

```go
cardRoutes := card(container.NewVBox(
    routesHeader,
    helperText("Only these destinations route through the VPN."),
    c.routesEntry,
    c.routeAllCheck,
))
```

Change to mirror `cardLog` expand pattern:

```go
routesTop := container.NewVBox(
    routesHeader,
    helperText("Only these destinations route through the VPN."),
)
cardRoutes := card(container.NewBorder(
    routesTop,       // top
    c.routeAllCheck, // bottom
    nil, nil,
    c.routesEntry,   // center — expands
))
```

Optional only if still short after Border: bump `SetMinRowsVisible(6)` → `10`. Prefer layout fix first; bump only if needed after visual check.

`leftCol` already uses `Border(..., center=cardRoutes)` — center expansion will flow into the entry once the card content is Border-based.

### Fix 3 — visible caret (theme)

**File:** `internal/ui/theme.go` primarily  
**Symbol:** `(*vepeenTheme).Color`

Facts (Fyne v2.8.0):

- No `theme.ColorNameCursor`.
- Caret fill = `theme.Color(theme.ColorNamePrimary)` (global).
- `container.NewThemeOverride` does **not** recolor the caret (anim uses `theme.Color`, not `ColorForWidget`).
- Custom Entry: **out of scope** (YAGNI).

**Recommended (shortest, brand-safe):**

1. Keep `ColorNamePrimary` → `accentColor` (teal). Teal caret is the supported theme path and is high-contrast on `inputFill`.
2. No new theme case unless visual check fails.
3. If caret is still effectively invisible after Fix 2 (focus/layout), **then** one-line global change is acceptable per product note “global OK for all entries”:

   ```go
   case theme.ColorNamePrimary:
       return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff} // white caret + HighImportance side effect
   ```

   Side effect: `HighImportance` buttons (Connect CTA) also use Primary → become white. Only take this path if teal caret fails visual check; do not add a custom Entry.

**Do not:** new deps, custom Entry subclass, ThemeOverride wrapper, VPN/backend changes.

---

## Implementation tasks

All **Frontend Developer**. No parallel file conflicts if ordered 1→2→3 in the same pass (fixes 1–2 same file; fix 3 other file — 1+3 or 2+3 can be parallel; 1+2 sequential in one edit session).

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1 | Frontend Developer | `internal/ui/main_window.go` | In `startTraffic`, when `sig != ""`: keep `appendLog("VPN traffic: "+sig)`; change `setStatus` detail to `"Traffic Route On"`. |
| 2 | Frontend Developer | `internal/ui/main_window.go` | Rebuild `cardRoutes` as `Border(top=header+helper, bottom=routeAllCheck, center=routesEntry)`. Optionally `SetMinRowsVisible(10)` only if still short. |
| 3 | Frontend Developer | `internal/ui/theme.go` | Confirm no `ColorNameCursor`. Keep Primary=teal unless caret still invisible; only then Primary→white (global). No custom Entry. |
| 4 | Debugger/Reviewer | — | Verify acceptance criteria; `go build ./...` and `go vet ./internal/ui/...`. |

**Documentation:** No — internal UI polish only. Do not update README.

---

## Acceptance criteria

- [ ] Connected + active VPN traffic: footer detail is exactly `Traffic Route On` (no host/IP/URL in status).
- [ ] Same condition: activity log still contains a line `VPN traffic: …` with host/IP details.
- [ ] Connected + no traffic: detail remains `No active connections through the VPN.`
- [ ] Routes multi-line entry grows vertically to fill space between helper text and "Route All Traffic"; no large empty band under the checkbox.
- [ ] Focusing `routesEntry` (and other entries) shows a visible blinking caret on the dark field.
- [ ] No VPN backend / `ActiveConnections` / connect-path behavior changes.
- [ ] `go build ./...` and `go vet ./internal/ui/...` pass.

---

## Regression risk

| Risk | Mitigation |
| ---- | ---------- |
| Status string used elsewhere for parsing | Only display via `setStatus`; log keeps machine-useful detail |
| Border layout clips routes or checkbox | Mirror proven `cardLog` Border pattern; visual check at default window size |
| White Primary washes out Connect CTA | Prefer keep teal Primary; white only if caret still invisible |

## Rollback

Revert the 1–2 string/layout edits in `main_window.go` and any Primary tweak in `theme.go`. No schema/API/migration.

---

## Implementation Summary (for Orchestrator)

**Scope:** Minor UI polish — fix plan + changelog (no PRD)  
**Agent:** Frontend Developer → Debugger/Reviewer  
**Files:**
- `internal/ui/main_window.go` — `startTraffic` status detail; `build` `cardRoutes` Border
- `internal/ui/theme.go` — caret via existing Primary only if needed

**Order:** Task 1 → Task 2 → Task 3 → Review  
**Docs:** none  
**Acceptance:** status `Traffic Route On` without IPs; routes entry fills card; visible blinking caret; build/vet clean
