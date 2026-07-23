# Fix Plan: VPN connection failures (NAT-T, localized success, split-tunnel alias, route-sync abort, error mapping)

**Related PRD:** PRD-002 (`docs/planning/prd-002-l2tp-split-tunnel.md`)
**Severity:** High (multiple independent root causes that each block/break L2TP/IPsec connect behind NAT)
**Reported by:** Planner (Explorer trace of the connect path `ConnectFull` → `EnsureProfile` → `SyncRoutes` → `Connect` → `EnforceSplitTunnel`)
**Date:** 2026-07-23
**Status:** Ready for implementation
**Version:** v1.0.0

---

## Overview

The connect flow has several app-controllable defects that cause L2TP/IPsec connections to fail (errors 789/800/809, false "failed" reports on localized Windows, server-pushed default route not removed, and connect aborted by a non-fatal route-sync error). This plan fixes each root cause so the client connects reliably "until there are no errors," while keeping secrets out of logs and preserving split-tunnel behavior.

## Problem Statement

On a typical Windows host behind a NAT/router, L2TP/IPsec fails unless the OS is told to allow UDP encapsulation (`AssumeUDPEncapsulationContextOnSendRule=2`). The app never checks or sets this. Additionally, success detection is English-only, the split-tunnel enforcement assumes the VPN adapter alias equals the profile name, a best-effort route sync aborts the whole connect on error, and 789/800/809 are mapped to a generic message. Any one of these produces a failed or misreported connection.

## Goals

- Reliably connect on localized and NATed Windows.
- Remove server-pushed `0.0.0.0/0` even when the VPN adapter alias differs from the profile name.
- Never abort connect for a non-fatal route-sync hiccup.
- Give the user specific, actionable Indonesian guidance for 789/800/809 (incl. NAT-T registry).
- Keep all diagnostics in `%AppData%\vepeen\vepeen.log` + UI log (no console; `-H windowsgui`).
- Never log PSK/password; never require elevation silently.

## Non-Goals

- Wrong PSK/credentials (691), firewall blocking UDP 500/4500/ESP, CGNAT — out of scope, but messaging is improved.
- Rewriting the RAS/PPP stack or the `internal/route` add/remove logic.
- Auto-elevation (UAC) — not attempted from a windowsgui process; we detect and instruct instead.

---

## Fix 1 — NAT-T registry (`AssumeUDPEncapsulationContextOnSendRule`)

### File(s) / function(s)
- **New:** `internal/vpn/natt_windows.go` (`//go:build windows`) — `EnsureNATRegistry() (NATResult, error)` + `NATResult` enum (`NATOK`, `NATSet`, `NATElevationRequired`).
- **Edit:** `internal/vpn/stub_other.go` — add `EnsureNATRegistry() (NATResult, error)` returning `NATOK, nil` (or `unsupported()` — choose `NATOK, nil` so non-Windows connect logic is unaffected).
- **Edit:** `internal/vpn/manager.go` — new `PhaseNATCheck Phase = "nat_check"`; call `EnsureNATRegistry` after validation, before `PhaseEnsureProfile`.

### What currently happens
Nothing — the app never reads or writes the PolicyAgent key, so behind-NAT L2TP/IPsec fails with 789/800/809.

### Precise change required
- Use `golang.org/x/sys/windows/registry` (already a dependency, v0.30.0).
- Key path: `HKLM\SYSTEM\CurrentControlSet\Services\PolicyAgent`, value name `AssumeUDPEncapsulationContextOnSendRule`, type `REG_DWORD`.
- Logic:
  1. `OpenKey(HKLM, path, READ)` → if value exists and `== 2` → return `NATOK` (no elevation needed, no reboot needed).
  2. Else attempt `OpenKey(HKLM, path, SET_VALUE)`. If it fails with access-denied (`windows.ERROR_ACCESS_DENIED` / `0x80070005`) → return `NATElevationRequired` (do **not** treat as fatal crash; return a clean result).
  3. If open succeeds, `SetValueEx(..., 2)` → return `NATSet` (change applied; a reboot may be required for it to take effect).
- In `ConnectFull`:
  - `NATElevationRequired` → return `newUserError("nat", "Gagal menghubungkan (NAT-T)", "<Indonesian message below>")` and stop (actionable: run elevated or set registry manually).
  - `NATSet` → `log.Printf` a warning that a reboot may be required, and append a warning to the connect warnings slice (see Fix 4 mechanism).
  - `NATOK` → proceed silently.

### New helper
`EnsureNATRegistry() (NATResult, error)`; optionally a pure `natValueOK(v uint32) bool` for unit testing.

### Error / message text (Indonesian, `UserError` style)
- Code `nat`, Primary `Gagal menghubungkan (NAT-T)`, Detail:
  `Windows memblokir L2TP/IPsec di belakang NAT. Atur registri HKLM\SYSTEM\CurrentControlSet\Services\PolicyAgent\AssumeUDPEncapsulationContextOnSendRule = 2 (jalankan sebagai administrator), lalu coba lagi. Vepeen mencoba mengaturnya otomatis tetapi memerlukan hak administrator.`
- `NATSet` warning (logged + UI): `NAT-T diatur (AssumeUDPEncapsulationContextOnSendRule=2). Mungkin perlu restart agar berlaku.`

### Testing / verification
- Add `TestNatValueOK` (pure helper): `2 → true`, `0/1 → false`.
- Add `TestEnsureNATRegistryNoPanic` (integration): call on the current OS; assert it returns one of the three enum values and does not panic; if `NATElevationRequired`, assert the returned error is non-nil and the message mentions "administrator".
- Manual: on a non-elevated run behind NAT, confirm the `nat` UserError is shown; after running elevated once, confirm `NATOK`/`NATSet` and a successful connect.

### Risks / careful notes
- **Elevation:** writing `HKLM` requires admin; from a `-H windowsgui` non-elevated process the open will fail → we must detect and instruct, never crash or silently proceed as if fixed.
- **Reboot:** the value may require a restart to take effect; surface that in the warning.
- **Non-Windows:** stub must compile and not break `ConnectFull` (return `NATOK, nil`).
- Secrets: this path touches no secrets.

---

## Fix 2 — Localized rasdial success detection

### File(s) / function(s)
- **Edit:** `internal/vpn/dial_windows.go` — `Connect` (lines 36-46). Extract a pure helper `evaluateRasdialResult(exitErr error, text string) bool`.

### What currently happens (lines 36-46)
```go
if err != nil {
    return MapExecError("Connect", err, text)
}
lower := strings.ToLower(text)
if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "gagal") {
    if !strings.Contains(lower, "successfully") && !strings.Contains(lower, "already connected") {
        return MapExecError("Connect", fmt.Errorf("rasdial failed"), text)
    }
}
return nil
```
On a non-English Windows, a real success message (e.g. Indonesian "koneksi berhasil dibuat") contains none of the English markers, so a success is misreported as failure whenever the text also contains a benign "error"/"failed" substring — or, worse, a localized success with exit 0 is fine, but a localized success that returns a non-zero exit is wrongly failed.

### Precise change required
- **Primary signal = exit code.** `cmd.CombinedOutput()` returns `err != nil` iff the process exit code ≠ 0. So: if `err == nil` → success, `return nil` immediately (do **not** scan text).
- **Fallback text scan only when `err != nil`.** If `err != nil`, treat as success **only if** the output contains a known success marker (English OR Indonesian): `successfully`, `already connected`, `berhasil`, `sudah terhubung`, `connected`. Otherwise call `MapExecError`.
- Move the helper out so it is unit-testable without invoking real `rasdial`.

### New helper
`func evaluateRasdialResult(exitErr error, text string) bool` — `exitErr == nil` → `true`; else scan `text` (lowercased) for the success markers above → `true`, else `false`.

### Error / message text
No new message; reuse `MapExecError` (which already produces Indonesian messages). Ensure the success-marker set includes Indonesian words so localized successes pass.

### Testing / verification
- Add `TestEvaluateRasdialResult` (table-driven, in `internal/vpn` or a new `dial_windows_test.go`):
  - exit 0, any text → `true`
  - exit ≠0, text contains "successfully" → `true`
  - exit ≠0, text contains "sudah terhubung" → `true`
  - exit ≠0, text contains "gagal" only → `false`
  - exit ≠0, empty text → `false`
- `go test ./internal/vpn` passes; `go build ./cmd/vepeen` succeeds.

### Risks / careful notes
- Do **not** remove the `err != nil → MapExecError` path; only reorder so exit code is authoritative.
- Keep password passed as arg and never logged (unchanged).
- Some localized `rasdial` returns non-zero on success — the fallback marker scan covers that; if a new locale uses an unknown word, it will be a false failure (acceptable; can extend marker list later).

---

## Fix 3 — EnforceSplitTunnel interface-alias mismatch

### File(s) / function(s)
- **Edit:** `internal/vpn/profile_windows.go` — `EnforceSplitTunnel` (currently uses `Get-NetRoute -InterfaceAlias $name`, ~line 72). Optionally add helper `resolveVpnInterfaceAlias(name string) (string, error)`.

### What currently happens
The script assumes the VPN adapter's `InterfaceAlias` equals the profile `name`:
```powershell
$r = Get-NetRoute -InterfaceAlias $name -DestinationPrefix '0.0.0.0/0' ...
Remove-NetRoute -InterfaceAlias $name -DestinationPrefix '0.0.0.0/0' ...
```
When the OS assigns a different adapter alias (common — e.g. "Vepeen" vs "Vepeen 2" or a GUID-ish name), `Get-NetRoute` finds nothing and the server-pushed `0.0.0.0/0` is **not** removed → full-tunnel despite split config.

### Precise change required
- Resolve the real interface from the VPN connection object first:
  ```powershell
  $c = Get-VpnConnection -Name $name -ErrorAction SilentlyContinue
  $ifAlias = if ($c -and $c.InterfaceAlias) { $c.InterfaceAlias } else { $name }
  $ifIndex = if ($c -and $c.InterfaceIndex) { $c.InterfaceIndex } else { $null }
  ```
- Then remove the default route using the resolved alias (prefer `-InterfaceIndex` when available, else `-InterfaceAlias $ifAlias`):
  ```powershell
  $r = Get-NetRoute -InterfaceAlias $ifAlias -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue
  if ($r) { Remove-NetRoute -InterfaceAlias $ifAlias -DestinationPrefix '0.0.0.0/0' -Confirm:$false -ErrorAction SilentlyContinue; 'removed-default' } else { 'no-default' }
  ```
- Keep the `try/catch` → `'enforce-error'` fallback and the best-effort contract (errors returned but must not abort connect — already the case in `ConnectFull`).

### New helper (optional)
`resolveVpnInterfaceAlias(name string) (string, error)` — runs the `Get-VpnConnection` snippet and returns `InterfaceAlias` (or `name` on miss). `EnforceSplitTunnel` can inline it or call the helper; either is fine.

### Error / message text
No new user-facing message (best-effort, silent on success). If `EnforceSplitTunnel` returns an error, `ConnectFull` already ignores it (lines ~116-120). Optionally log the `'enforce-error'` outcome via `log.Printf` for diagnostics.

### Testing / verification
- Add a string-level test (no real PowerShell needed) asserting the `EnforceSplitTunnel` script contains `Get-VpnConnection -Name` and references `InterfaceAlias`/`InterfaceIndex` (i.e., no longer relies solely on `$name` for `Get-NetRoute`).
- Manual: connect to a server that pushes `0.0.0.0/0`; after connect, verify via `Get-NetRoute` that no `0.0.0.0/0` exists on the VPN interface and that `ProfileDiagnostics` shows split-tunnel on.

### Risks / careful notes
- Must not break existing split-tunnel behavior when alias == name (the `$ifAlias` fallback to `$name` preserves that).
- `InterfaceIndex` is more reliable than alias; prefer it when present.
- Never log the resolved alias if it could echo anything sensitive — it can't (it's an adapter name), but keep `sanitizeOutput` on the returned string as today.

---

## Fix 4 — SyncRoutes abort (make best-effort)

### File(s) / function(s)
- **Edit:** `internal/vpn/manager.go` — `ConnectFull` (lines 107-114, the `if err := route.SyncRoutes(...) ; err != nil { return ... }` block).
- **Edit (testability seam):** add field `syncRoutesFn func(string, []string) error` to `Manager`, defaulting to `route.SyncRoutes` in `NewManager`; tests can override it.

### What currently happens (lines 107-114)
```go
notify(PhaseSyncRoutes)
if err := route.SyncRoutes(name, prefixes); err != nil {
    return newUserError("routes", "Gagal menyelaraskan rute",
        sanitizeDetail(err.Error())+" Koneksi tidak dilanjutkan.")
}
```
A transient `Add/Remove-VpnConnectionRoute` error aborts the entire connect even though the profile may already be correct and `rasdial` would succeed.

### Precise change required
- Make route sync **best-effort**: on error, `log.Printf` a warning (file log), append a warning to the connect warnings slice, and **continue** to `PhaseDial`. Do not return.
- Surface the warning to the UI (see mechanism below) so the user knows routes may be stale.
- Keep the empty-routes gate (Fix 6) so we never dial with zero prefixes.
- Use the `syncRoutesFn` seam so a failing sync can be unit-tested without real PowerShell.

### Warning mechanism (non-fatal surfacing)
`ConnectFull` currently returns `error`. Extend its signature to return `(warnings []string, err error)`:
- `warnings` carries non-fatal notes (NATSet reboot, SyncRoutes skipped).
- The UI `onConnect` handler appends `warnings` to the log area after a successful connect (no change to failure handling).
- If changing the signature is undesirable, the fallback is: write warnings only to the file log via `log.Printf` and have the UI show a generic "beberapa langkah penyelarasan dilewati (lihat log)" note on success. **Preferred: extend the return value** for clear UX.

### Error / message text
- Warning (logged + UI): `Penyelarasan rute dilewati: <sanitized detail>. Koneksi dilanjutkan; rute split tunnel mungkin perlu disimpan ulang.`
- Keep `sanitizeDetail` so no secrets leak into the warning.

### Testing / verification
- Add `TestConnectFull_SyncRoutesErrorIsBestEffort`: override `m.syncRoutesFn` to return an error; assert `ConnectFull` returns `err == nil` (connect proceeds) and `warnings` is non-empty. (Requires the `syncRoutesFn` seam and a way to stub `EnsureProfile`/`Connect` — e.g. also seam those, or test the specific block in isolation.)
- `go test ./internal/vpn` passes.

### Risks / careful notes
- Best-effort means a real profile/route problem won't block connect; `EnforceSplitTunnel` still runs after dial to suppress a pushed default route, mitigating full-tunnel risk.
- Do not swallow the error silently — always log + warn so failures are visible in `vepeen.log`.
- Secrets: `sanitizeDetail` already strips `l2tppsk`/`password`; keep it.

---

## Fix 5 — Specific 789/800/809 messages in `MapExecError`

### File(s) / function(s)
- **Edit:** `internal/vpn/errors.go` — `MapExecError` switch (currently the `network` case at ~line 60 already catches `789`/`800`/`809` with a generic message).

### What currently happens
The `network` case maps `789`/`800`/`809` → code `network`, Primary `Gagal terhubung`, Detail `Periksa server, jaringan, dan port UDP 500/4500 (L2TP/IPsec).` — correct but not specific about the NAT-T registry cause that Vepeen now auto-handles.

### Precise change required
- Add a **dedicated case BEFORE** the generic `network` case that matches `789`, `800`, `809` specifically and returns a more actionable Indonesian message referencing NAT-T and the auto-fix.
- Keep the existing `network` case as the fallback for other network errors.
- Order matters: the new case must precede `network` so it wins for these three codes.

### Error / message text (Indonesian, `UserError` style)
- Code `ipsec` (or `nat`), Primary `Gagal terhubung (L2TP/IPsec)`, Detail:
  `Kesalahan 789/800/809: biasanya karena NAT atau pengaturan IPsec. Vepeen mencoba mengatur registri NAT-T secara otomatis (perlu hak administrator). Pastikan port UDP 500 dan 4500 tidak diblokir firewall, dan PSK sudah benar.`
- (If the user is not admin, the Fix 1 `nat` error fires earlier; this message is the fallback for when the registry is already set but the underlying IPsec still fails.)

### Testing / verification
- Extend `internal/vpn/errors_test.go`:
  - `TestMapExecError_IPSec789`: input contains `789` → `ue.Code == "ipsec"` (or chosen code) and Primary `Gagal terhubung (L2TP/IPsec)`.
  - `TestMapExecError_IPSec800` and `TestMapExecError_IPSec809`: same assertions.
  - Add a test confirming a *different* network error (e.g. `timeout`) still maps to code `network` (regression guard for case ordering).
- `go test ./internal/vpn` passes.

### Risks / careful notes
- Must not break the existing `auth` (691) or `elevation` mappings — those cases precede and remain unchanged.
- Keep `sanitizeOutput` so no secrets leak into the mapped detail.

---

## Fix 6 — Empty-routes validation gate (confirm & keep)

### File(s) / function(s)
- **Confirm:** `internal/vpn/manager.go` lines 84-90 — blocks connect if `prefixes` is empty after parse.

### What currently happens (lines 84-90)
```go
if len(prefixes) == 0 {
    return newUserError("validation", "Tidak dapat menghubungkan", "Isi minimal satu IP atau CIDR untuk split tunnel.")
}
```
This is **intended**: split-tunnel requires ≥1 destination; connecting with zero routes would either fail route sync or produce an unintended full tunnel.

### Precise change required
- **Keep the gate.** No behavioral change required.
- Optionally clarify the message slightly to reinforce intent (still Indonesian, `UserError` style):
  `Isi minimal satu IP atau CIDR tujuan untuk split tunnel (mis. 10.0.0.0/24).`
- Add a unit test asserting `ConnectFull` (or the parse+gate block) returns a `validation` UserError when `Routes`/`RoutesText` is empty.

### Error / message text
As above (minor clarification only; current text is already clear).

### Testing / verification
- Add `TestConnectFull_EmptyRoutesBlocked`: `ConnectRequest` with empty `RoutesText`/`Routes` → `ConnectFull` returns a `UserError` with code `validation` and a message mentioning split tunnel.

### Risks / careful notes
- Do **not** remove this gate — removing it would regress split-tunnel safety (Fix 4 best-effort depends on having valid prefixes).
- No secrets involved.

---

## Implementation Plan

### Phase 1: Foundation (registry + test seams)
**Depends on:** Nothing
**Parallelizable:** Partially — Fix 1 and Fix 4's seam can be done together; Fix 5/6 are independent.

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/vpn/natt_windows.go` (new), `internal/vpn/stub_other.go` | Implement `EnsureNATRegistry` + `NATResult` using `golang.org/x/sys/windows/registry`; stub for non-Windows. |
| 1.2 | Backend Developer | `internal/vpn/manager.go` | Add `PhaseNATCheck`; call `EnsureNATRegistry`; handle `NATElevationRequired` (return `nat` UserError) and `NATSet` (log + warning). Add `syncRoutesFn` seam + extend return to `([]string, error)`. |
| 1.3 | Backend Developer | `internal/vpn/errors.go` | Add specific 789/800/809 case before `network`. |
| 1.4 | Backend Developer | `internal/vpn/dial_windows.go` | Extract `evaluateRasdialResult`; reorder so exit code is primary, text scan is fallback with ID markers. |
| 1.5 | Backend Developer | `internal/vpn/profile_windows.go` | Fix `EnforceSplitTunnel` to resolve interface via `Get-VpnConnection` (alias/index). |
| 1.6 | Backend Developer | `internal/vpn/manager.go` | Make `SyncRoutes` best-effort (warn + continue). |

### Phase 2: UI surfacing of warnings
**Depends on:** Phase 1 (return signature change)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Frontend Developer | `internal/ui/main_window.go` (controller `onConnect`) | Consume the new `warnings []string` return from `ConnectFull`; append each to the UI log area on success. Keep failure handling unchanged. |

### Phase 3: Tests + Review + Docs (always last)
**Depends on:** All implementation phases

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 3.1 | Backend Developer | Add tests: `TestEvaluateRasdialResult`, `TestNatValueOK`/`TestEnsureNATRegistryNoPanic`, `TestMapExecError_IPSec{789,800,809}` + network regression, `TestConnectFull_SyncRoutesErrorIsBestEffort`, `TestConnectFull_EmptyRoutesBlocked`, `EnforceSplitTunnel` script string test. |
| 3.2 | Debugger/Reviewer | Verify all acceptance criteria; run `go build ./cmd/vepeen` and `go test ./internal/vpn ./internal/route`; check no secret leakage; confirm non-Windows stubs compile (`GOOS=linux go build ./...`). |
| 3.3 | Security | Confirm PSK/password never logged; registry write path handles non-admin gracefully; no elevation attempted silently. |
| 3.4 | Documentation | Update `README.md` troubleshooting: NAT-T registry auto-set (admin needed), 789/800/809 guidance, split-tunnel enforcement, best-effort route sync. |

---

## Cross-cutting constraints (apply to every fix)

- **No console:** app is built `-H windowsgui`. All diagnostics go to `%AppData%\vepeen\vepeen.log` (already wired in `cmd/vepeen/main.go` via `log.SetOutput`) and the UI log area. Use `log.Printf` for warnings; never `fmt.Println`.
- **Secrets:** PSK/password must never appear in logs, error details, or warnings. Reuse `sanitizeOutput`/`sanitizeDetail`; the NAT/route paths touch no secrets, but keep the guards.
- **Indonesian UX:** all new `UserError` text follows the existing `newUserError(code, primary, detail)` style (code stable/machine-oriented, primary short, detail actionable).
- **Split-tunnel safety:** Fix 3 and Fix 6 together guarantee the server-pushed default route is removed and we never connect with zero routes.
- **Non-Windows:** every Windows-only addition needs a `stub_other.go` counterpart so `GOOS=linux go build ./...` stays green.
- **Registry elevation:** never assume admin; detect access-denied and return an actionable `nat` error instead of crashing or pretending the fix applied.

## Acceptance Criteria

- [ ] `EnsureNATRegistry` reads/set the PolicyAgent key; non-admin returns `NATElevationRequired` with an admin instruction; admin path sets value `2`.
- [ ] `Connect` treats rasdial exit code 0 as success; non-zero is success only with a known (EN/ID) success marker; localized successes no longer misreport as failure.
- [ ] `EnforceSplitTunnel` removes `0.0.0.0/0` using the resolved VPN interface (alias/index), not the bare profile name.
- [ ] A `SyncRoutes` error no longer aborts connect; it is logged + surfaced as a warning and connect proceeds.
- [ ] `MapExecError` returns a specific Indonesian message for 789/800/809 (preceding the generic network case).
- [ ] Empty-routes gate remains; connect is blocked with a clear split-tunnel message when no routes given.
- [ ] `go build ./cmd/vepeen` and `go test ./internal/vpn ./internal/route` pass; non-Windows build compiles.
- [ ] No PSK/password in any log, error, or warning.

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Registry write needs admin; app is non-elevated | NAT-T not applied → 789/809 | High | Detect access-denied → actionable `nat` error; document manual step |
| NAT-T value requires reboot to take effect | First connect after set still fails | Med | Warn user a reboot may be needed (`NATSet` warning) |
| Best-effort route sync hides a real route problem | Stale routes / unintended full tunnel | Med | `EnforceSplitTunnel` still runs post-dial; warning logged + shown |
| Localized rasdial uses unknown success word | False failure on new locale | Low | Marker list extensible; exit code remains primary signal |
| Changing `ConnectFull` return signature | UI call-site break | Med | Phase 2 updates `onConnect`; keep failure path identical |

## Rollback Strategy

All changes are additive/behavioral within existing files; no schema or API contract changes beyond the `ConnectFull` return signature (contained to `manager.go` + `onConnect`). To roll back, revert the six edits; `stub_other.go` and `natt_windows.go` can remain (harmless no-ops on non-Windows). No migration needed.

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-23 | Initial fix plan for NAT-T, localized success, split-tunnel alias, route-sync abort, 789/800/809 mapping |
