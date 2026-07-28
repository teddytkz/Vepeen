# PRD-006: Route All Traffic checkbox

**Version:** v0.1.0
**Status:** Draft
**Author:** Planner Agent
**Created:** 2026-07-24

---

## Overview

Add a "Route All Traffic" checkbox under the split-tunnel routes text area. When checked, the VPN carries ALL traffic (split tunneling disabled, server default gateway used). Default state on open: **unchecked**.

## Goals

- Persist the flag in config (`Config` + `Stored`), default `false`.
- Wire the checkbox into UI build / applyConfig / onSave / onConnect / validateConnect.
- In `ConnectFull`, skip the 4 split-tunnel steps when the flag is set.

## Non-Goals

- No UI disabling of the routes text area (left editable but unused when checked — optional only).
- No change to NAT-T check, `DisconnectAllExcept`, or `connectFn` (rasdial).

---

## Files to modify

- `internal/config/config.go`
  - Add `RouteAllTraffic bool \`json:"routeAllTraffic"\`` to `Config` and `Stored`.
  - `Default()`: add `RouteAllTraffic: false`.
  - `DefaultStored()`: add `RouteAllTraffic: false`.
  - `Config()` (Stored→Config): project `RouteAllTraffic: s.RouteAllTraffic`.
  - `withCreds` (Config→Stored): set `RouteAllTraffic: c.RouteAllTraffic`.

- `internal/ui/main_window.go`
  - Add field `routeAllCheck *tealCheck` to `controller` struct (near `rememberCheck`, ~line 150).
  - `build()` (~line 219): in the routes card top VBox, after `helperText(...)`, insert `c.routeAllCheck = newTealCheck("Route All Traffic", nil)`.
  - `applyConfig()` (~line 456): after setting `c.routesEntry`, set `c.routeAllCheck.SetChecked(cfg.RouteAllTraffic)`.
  - `onSave()` (~line 884): add `c.stored.RouteAllTraffic = c.routeAllCheck.Checked` next to the `RememberCredentials` write.
  - `onConnect()` (~line 962): add `RouteAllTraffic: c.routeAllCheck.Checked` to the `vpn.ConnectRequest{...}` literal.
  - `validateConnect()` (~line 1135): relax the empty-routes guard — if `c.routeAllCheck.Checked`, skip the `len(prefixes) == 0` error (return `""`, `nil`).

- `internal/vpn/manager.go`
  - Add `RouteAllTraffic bool` to `ConnectRequest` struct (~line 38).
  - Branch in `ConnectFull` (see below).

---

## VPN layer — `ConnectRequest` + `ConnectFull` branch

New field:
```go
RouteAllTraffic bool // when true, route ALL traffic via VPN (split tunneling off)
```

Inside `ConnectFull`, after the `name` validation and NAT-T check, gate the split-tunnel steps on `!req.RouteAllTraffic`:

- **Empty-prefix guard** (currently `if len(prefixes) == 0 { return nil, ... "Enter at least one destination..." }`): only enforce when `!req.RouteAllTraffic`. When the flag is true, allow `prefixes`` to be empty and skip resolution.
- **SKIP `ensureSplitTunnelingFn(name)`** (`PhaseSplitTunnelEnsure`) — keep SplitTunneling OFF so the server default gateway is used.
- **SKIP `syncRoutesFn(name, prefixes)`** (`PhaseSyncRoutes`) — no per-destination routes.
- **SKIP `EnforceSplitTunnel(name, prefixes)`** (`PhaseSplitEnforce`) — keep the `0.0.0.0/0` default route so all traffic goes through the VPN.

Still always run: NAT-T check, `DisconnectAllExcept(name)`, and `connectFn` (rasdial).

Suggested shape (no code change beyond this intent):
```
if !req.RouteAllTraffic {
    // resolve prefixes; if empty -> user error
    // ensureSplitTunnelingFn
    // syncRoutesFn
    // EnforceSplitTunnel
}
```

---

## UI wiring points (summary)

- **build**: create `routeAllCheck` via `newTealCheck("Route All Traffic", nil)`, place in routes card VBox below helper text.
- **applyConfig**: `c.routeAllCheck.SetChecked(cfg.RouteAllTraffic)`.
- **onSave**: `c.stored.RouteAllTraffic = c.routeAllCheck.Checked`.
- **onConnect**: `RouteAllTraffic: c.routeAllCheck.Checked` in the request literal.
- **validateConnect**: when `c.routeAllCheck.Checked`, do not require ≥1 route.

---

## Default state (unchecked on open)

Falls out naturally: `DefaultStored()` returns `RouteAllTraffic: false`, `newController()` seeds `stored`/`cfg` from it, and `applyConfig` sets the checkbox from config on load. No extra init needed.

---

## Risks / edge cases

- **Routes text area stays editable when checked** — leave it as-is (user may uncheck and reuse). Do NOT disable unless flagged optional. Its content is simply ignored by `ConnectFull` when the flag is true.
- **`onSave` with flag checked**: still parses routes (harmless) but they are unused until unchecked.
- **`EnforceSplitTunnel` skipped** means the server-pushed `0.0.0.0/0` default route is retained — verify the Windows profile does NOT have `DisableDefaultRoutes`/split-only policy that would drop it.
- **Tests**: `manager_test.go` / `dial_windows_test.go` may assert the split-tunnel steps run; add a `RouteAllTraffic: true` case asserting those steps are skipped (use the overridable `Fn` stubs).

---

## Acceptance criteria

- [ ] App opens with "Route All Traffic" unchecked.
- [ ] Checking it and connecting routes all traffic (no per-dest routes, default gateway via VPN).
- [ ] Unchecking restores split-tunnel behavior.
- [ ] Flag persists across save/reload.
- [ ] `go build ./...` and `go vet ./internal/ui/... ./internal/vpn/...` pass.

## Rollback

Revert the three files to HEAD; config JSON with unknown `routeAllTraffic` key is ignored by `json.Unmarshal` (backward safe).
