# PRD-002: L2TP/IPsec VPN Client with Split Tunnel (Selective Routes)

**Version:** v0.1.0  
**Status:** Draft  
**Author:** Planner Agent  
**Created:** 2026-07-22  
**Updated:** 2026-07-22  
**Related:** PRD-001 (starter scaffold), `docs/research/windows-l2tp-ipsec-split-tunnel.md`

---

## Overview

Turn **vepeen** from a Go + Fyne demo into a Windows desktop client that connects to an **L2TP/IPsec VPN using a pre-shared key (PSK)** and routes **only selected IPs/CIDRs** through the tunnel (split tunnel). The app orchestrates the **Windows built-in VPN stack** (PowerShell `VpnClient` + `rasdial`) rather than embedding a third-party VPN driver or reimplementing IPsec.

## Problem Statement

Users need a simple GUI to:

1. Connect to L2TP/IPsec servers that require a **secret key (PSK)** plus typical **username/password** (MS-CHAPv2).
2. Avoid full-tunnel behavior so **only specific destinations** (e.g. office subnets, a few hosts) use the VPN; all other traffic stays on the normal default route.

Today vepeen is a starter UI only (`internal/ui` demo). There is no VPN, config, secrets, or routing code. Manual PowerShell/`rasdial` is error-prone for non-admin users and does not persist a friendly form for server, credentials, and CIDR lists.

## Goals

- Provide a Fyne main window to enter connection settings, secrets, and a multi-line IP/CIDR list, then **Connect** / **Disconnect**.
- Create/update a **per-user** Windows VPN profile: L2TP + PSK + MS-CHAPv2 + **SplitTunneling**.
- Attach **only** user-specified destinations via `Add-VpnConnectionRoute` (and remove stale routes on sync).
- Connect/disconnect via `rasdial` and surface clear status/errors in the UI.
- Persist **non-secret** settings (server, connection name, CIDRs, username optional) to a local config file under the user config directory.
- Prefer **not** writing PSK/password to plain JSON; use Windows Credential Manager / DPAPI when feasible for v1, with documented fallback if CredMan is deferred.
- Keep a single Go ownership path for packages + Fyne UI after Designer specs (Backend Developer implements all Go).

## Non-Goals

- Full-tunnel VPN (default route via VPN / kill-all-local-routing).
- Other VPN types: IKEv2, SSTP, WireGuard, OpenVPN, SoftEther client embed.
- Certificate-based IPsec (machine/user cert) as primary auth.
- Always On VPN, MDM ProfileXML, enterprise CSP.
- Custom IPsec cipher suite UI (`Set-VpnConnectionIPsecConfiguration`) unless forced by a later bugfix.
- macOS / Linux clients (Windows primary; non-Windows may build-tag stub with clear error).
- Shipping third-party drivers, TUN adapters, or custom L2TP/IPsec stacks.
- Kill switch / firewall lockdown / multi-hop / anti-censorship.
- Multi-profile manager UI (v1: one primary profile, default name `Vepeen`).
- Auto-connect on boot, system tray-only mode, or background service (optional later).
- DevOps: CI, Docker, installers, code signing (out of scope).
- Pure `rasapi32` first implementation (Phase 2 optimization only).

---

## Feature Specification

### User Stories

- As a Windows user, I want to enter VPN server, PSK, username, and password, so that I can authenticate to a typical L2TP/IPsec gateway.
- As a Windows user, I want to list only the IPs/CIDRs that should use the VPN, so that the rest of my traffic does not go through the tunnel.
- As a Windows user, I want Connect and Disconnect buttons with visible status, so that I know whether the tunnel is up or why it failed.
- As a Windows user, I want my non-secret settings remembered between launches, so that I do not retype server and routes every time.
- As a Windows user, I want PSK and password stored more safely than plain text in a config file, so that casual file inspection does not leak secrets.
- As a Windows user, I want Indonesian (or bilingual) labels on the form, so that the app is easy to use without hiding technical terms (L2TP, PSK, CIDR).

### Acceptance Criteria

- [ ] Demo click-counter UI is replaced by a VPN connection form in the main window.
- [ ] Form fields exist for: **Nama koneksi** (default `Vepeen`), **Server**, **PSK**, **Username**, **Password**, **Daftar IP/CIDR** (multi-line, one per line), **Connect**, **Disconnect**, and a **status** area.
- [ ] Empty/invalid required fields are blocked before connect with a clear message (server, PSK, username, password, at least one valid CIDR/IP for selective routing).
- [ ] CIDR/IP parser accepts lines such as `10.10.0.0/16`, `203.0.113.50/32`, and bare IPv4 treated as `/32`; rejects garbage with a user-visible error.
- [ ] On Connect, app ensures a **per-user** Windows VPN profile with `TunnelType L2tp`, PSK, MS-CHAPv2, encryption required, and **SplitTunneling** enabled (via PowerShell `Add-VpnConnection` / `Set-VpnConnection` as appropriate).
- [ ] On Connect (or profile ensure), configured destinations are synced with `Add-VpnConnectionRoute` / `Remove-VpnConnectionRoute` so the profile’s routes match the form list.
- [ ] Connect uses `rasdial <Name> <user> <pass>`; Disconnect uses `rasdial <Name> /DISCONNECT`.
- [ ] Status shows at least: Disconnected, Connecting, Connected, Disconnecting, Error (with short reason; never include PSK/password in logs or status text).
- [ ] After successful connect, only listed prefixes are intended to route via VPN (split tunnel); app does not add a full default route through the VPN.
- [ ] Non-secret config persists across restarts (JSON under user config dir, e.g. `%AppData%\vepeen\config.json` or equivalent).
- [ ] PSK and password are **not** written to the plain config JSON. Secrets use Credential Manager/DPAPI when implemented; if MVP temporarily keeps secrets in memory only, README/PRD tradeoff is documented and “remember password” is disabled or clearly limited.
- [ ] Prefer per-user profile (no `-AllUserConnection` by default). If elevation is required for profile/PSK write, user gets a clear Indonesian/English message; day-to-day connect should not require permanent admin when avoidable.
- [ ] Packages exist: `internal/vpn`, `internal/route`, `internal/config`, `internal/secrets`, and updated `internal/ui`.
- [ ] VPN/route Windows orchestration is isolated (`//go:build windows` where appropriate); non-Windows builds fail gracefully or are not claimed as supported.
- [ ] README documents: L2TP dual-auth model, split-tunnel behavior, ports (UDP 500/4500), privileges, secrets handling, and run/build steps.
- [ ] Security review completed for secrets, process invocation, and logging.
- [ ] No CI/Docker/DevOps artifacts introduced for this feature.

### UI Fields (v1)

| Field | Control | Required | Default / notes |
| ----- | ------- | -------- | --------------- |
| Nama koneksi | Entry | Yes | `Vepeen` |
| Server | Entry | Yes | host or IP |
| Pre-shared key (PSK) | Password entry | Yes | IPsec secret; never plain-log |
| Username | Entry | Yes (typical) | PPP / MS-CHAPv2 |
| Password | Password entry | Yes (typical) | PPP; never plain-log |
| Daftar IP/CIDR | Multi-line entry | Yes (≥1 valid) | One IP or CIDR per line; `#` comments optional if easy |
| Simpan pengaturan | Button or auto-save | — | Saves non-secrets; secrets per secrets policy |
| Hubungkan (Connect) | Button | — | Disabled while connecting/connected as appropriate |
| Putuskan (Disconnect) | Button | — | Enabled when connected/connecting per UX rules |
| Status | Label / multi-line | — | State + last error (sanitized) |

**Language:** Prefer clear **Indonesian labels** with English technical terms allowed (L2TP, PSK, CIDR, VPN).

**Layout (Designer owns visual spec):** Single main window; form top/middle; actions + status bottom; readable default size; password fields masked; no absolute positioning — Fyne containers.

---

## Technical Design

### Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│  Fyne UI (internal/ui)                                   │
│  Form + Connect/Disconnect + status                      │
│  Validates input; calls managers on background workers   │
└────────────────────────────┬─────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
┌───────────────┐   ┌────────────────┐   ┌─────────────────┐
│ internal/     │   │ internal/vpn   │   │ internal/route  │
│ config        │   │ EnsureProfile  │   │ SyncRoutes      │
│ Load/Save     │   │ Connect        │   │ (VpnConnection  │
│ non-secrets   │   │ Disconnect     │   │  Route cmdlets) │
└───────┬───────┘   │ Status         │   └────────┬────────┘
        │           └───────┬────────┘            │
        │                   │                     │
        ▼                   ▼                     ▼
┌───────────────┐   ┌──────────────────────────────────────┐
│ internal/     │   │ Windows OS                           │
│ secrets       │   │ PowerShell VpnClient + rasdial.exe   │
│ CredMan/DPAPI │   │ Per-user phonebook + profile routes  │
└───────────────┘   └──────────────────────────────────────┘
```

**Connect flow (canonical):**

1. Validate UI inputs (server, PSK, user, pass, parse CIDRs).
2. Load/merge config; resolve secrets from store or form.
3. `EnsureProfile` — create or update L2TP + PSK + MSChapv2 + SplitTunneling (per-user).
4. `SyncRoutes` — make profile routes match the CIDR list.
5. `rasdial <Name> <user> <pass>`.
6. Poll/query status (`Get-VpnConnection` and/or rasdial list); update UI.
7. Disconnect: `rasdial <Name> /DISCONNECT` (routes remain on profile for next connect).

**Research basis:** `docs/research/windows-l2tp-ipsec-split-tunnel.md` (Learner recommendation: Option A — Windows built-in VPN).

### Codebase Context

| Item | Current state |
| ---- | ------------- |
| Module | `vepeen`, Go 1.22.0 |
| UI | Fyne v2.8.0; `cmd/vepeen/main.go` thin entry; `internal/ui/main_window.go` **demo only** — replace |
| VPN code | None |
| Platform | Windows primary; CGo already required for Fyne |
| Patterns | VBox + Center + Padded; no tests/CI; no persistence yet |
| PRD-001 | Starter complete; networking was explicit non-goal — this PRD supersedes that non-goal for VPN |

**Reuse:** Keep thin `main`; grow `internal/ui`; add new `internal/*` packages; no new frameworks beyond stdlib `os/exec` for v1 (optional later `golang.org/x/sys/windows`).

### Data Model

**Config file (non-secret), example shape:**

```json
{
  "connectionName": "Vepeen",
  "serverAddress": "vpn.example.com",
  "username": "alice",
  "routes": [
    "10.10.0.0/16",
    "203.0.113.50/32"
  ],
  "rememberUsername": true
}
```

- **Do not** store `password` or `psk` in this file.
- Path: user config directory for the app (e.g. `%AppData%\vepeen\config.json` on Windows). Implementer chooses exact path via standard user-config conventions.

**Secrets store (preferred v1):**

| Secret | Target store | Keying idea |
| ------ | ------------ | ----------- |
| PSK | Windows Credential Manager or DPAPI blob | e.g. target `vepeen/<connectionName>/psk` |
| Password | Same | e.g. `vepeen/<connectionName>/password` |

**MVP tradeoff (if CredMan proves heavy mid-implementation):**

- Secrets remain **in-memory** from the form for the session only.
- Optional “remember” disabled or limited to username only.
- Document in README; follow-up task to complete CredMan/DPAPI before calling secrets “done.”
- **Still forbidden:** writing PSK/password into plain JSON.

**In-memory runtime state:** connection status enum, last error (sanitized), busy flags for UI.

### API Changes

No HTTP/RPC API. Process boundaries only:

| Operation | Mechanism | Notes |
| --------- | --------- | ----- |
| Create/update profile | `powershell -NoProfile -Command` / scripted cmdlets | `Add-VpnConnection` / `Set-VpnConnection`; `-L2tpPsk`, `-SplitTunneling`, `-Force` as required by MS docs |
| Sync routes | `Add-VpnConnectionRoute` / `Remove-VpnConnectionRoute` | Per destination prefix |
| Connect | `rasdial.exe Name user pass` | Capture exit code + output |
| Disconnect | `rasdial.exe Name /DISCONNECT` | |
| Status | `Get-VpnConnection -Name` and/or `rasdial` | Map to UI states |

**Security for process invocation:**

- Avoid logging full command lines that include PSK/password.
- Prefer short-lived scripts with restricted ACL or stdin patterns where practical; document residual risk of process listing for cmdline secrets.
- Never echo secrets into UI status or debug files by default.

### Package Responsibilities

| Package | Responsibility |
| ------- | -------------- |
| `internal/config` | Load/save non-secret settings; defaults (`connectionName=Vepeen`); path resolution |
| `internal/secrets` | Store/retrieve/delete PSK and password; no plain-file fallback for secrets |
| `internal/vpn` | Ensure profile, connect, disconnect, status; wraps PowerShell + rasdial; maps errors to typed/user messages |
| `internal/route` | Parse/normalize CIDRs; sync profile routes to desired set |
| `internal/ui` | Fyne form, validation, async actions, status display; wires managers |
| `cmd/vepeen` | App lifecycle only; may pass platform check |

### UI Changes

- **Replace** demo content in `internal/ui/main_window.go` (may split into additional files under `internal/ui/` if Designer/Backend prefer: e.g. form widgets vs window shell — avoid file thrash; keep ownership clear).
- Window title remains **Vepeen** (or “Vepeen — VPN” if Designer prefers).
- Designer produces layout/copy/state rules **before** Backend implements Fyne.
- Backend Developer implements all Fyne Go code (single ownership for Go GUI + packages).

### Error Handling & Status Messaging

| Situation | User-facing guidance (intent) |
| --------- | ----------------------------- |
| Validation failure | Point to missing/invalid field (e.g. CIDR baris ke-N) |
| Profile create denied | Explain possible need for elevation / per-user profile |
| rasdial auth failure | Wrong user/pass or server auth policy — no secret echo |
| Timeout / no response | Check server, network, UDP 500/4500, firewall |
| Route cmdlet failure | Status error; do not claim Connected if ensure failed critically |
| Already connected | Idempotent connect or clear “sudah terhubung” message |
| Disconnect failure | Show error; allow retry |

Status enum (suggested): `Disconnected` | `Connecting` | `Connected` | `Disconnecting` | `Error`.

### Security Constraints

1. **Never** persist PSK/password in plain config JSON or world-readable temp files long-term.
2. **Never** log PSK/password (stdout, files, status labels).
3. Prefer **per-user** VPN profile; elevate only when necessary for profile/PSK write.
4. Clear sensitive strings from memory where practical (Go limitations OK to document).
5. Treat L2TP/IPsec+PSK as weaker than cert/WireGuard — document risk in README.
6. Command construction must not allow injection via connection name/server/CIDR (validate/escape arguments; prefer structured args over string-concat shell).
7. Security agent review required before Done.

### Privilege Model

| Operation | Expected privilege |
| --------- | ------------------ |
| Per-user `Add-VpnConnection` / routes | Standard user often OK |
| All-user profile / some PSK writes | Administrator — **avoid by default** |
| `rasdial` connect | Standard user for per-user profile |
| Manual `route add` fallback | Often admin — **prefer profile routes**, not ad-hoc route add |

---

## Implementation Plan

### Phase 0: UI/UX Design

**Depends on:** Nothing  
**Parallelizable:** No — blocks Fyne implementation of the main screen  

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 0.1 | Designer | `docs/planning/` design notes and/or UI sketch section appended to this PRD / short `docs/planning/ui-002-l2tp-form.md` | Specify layout, Indonesian labels, field order, button enablement rules, status copy, empty/error/connected states, window size guidance for Fyne. |

**Sub-Agent Guidance:**

- Designer does **not** write production Go unless explicitly asked; deliver specs Backend can implement.
- No Backend UI coding until 0.1 is available.

### Phase 1: Core Domain Packages (Windows orchestration + config/secrets)

**Depends on:** Nothing for pure packages; can start in parallel with Phase 0 if UI not touched  
**Parallelizable:** Yes — config/secrets vs vpn/route if files do not overlap  

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/config/config.go` (and helpers as needed) | Define config struct; default connection name `Vepeen`; load/save JSON under user config dir; no secrets fields on disk. |
| 1.2 | Backend Developer | `internal/secrets/secrets.go` (+ windows-specific file if needed) | API to set/get/delete PSK and password; Credential Manager or DPAPI preferred; document MVP in-memory limitation if incomplete. |
| 1.3 | Backend Developer | `internal/route/parse.go`, `internal/route/sync.go` (names flexible) | Parse multi-line IP/CIDR list; normalize; `SyncRoutes` via VpnConnectionRoute cmdlets for a connection name. |
| 1.4 | Backend Developer | `internal/vpn/profile.go`, `internal/vpn/dial.go`, `internal/vpn/status.go` (names flexible) | Ensure L2TP+PSK+MSChapv2+SplitTunneling profile; connect/disconnect via rasdial; status query; sanitized errors. |

**Sub-Agent Guidance:**

- Tasks 1.1 and 1.2 can proceed in parallel.
- Tasks 1.3 and 1.4 can proceed in parallel after interfaces for connection name / CIDR list are clear; 1.4 may call into 1.3 or UI may call both — prefer `vpn` orchestrating `route` **or** a thin facade used by UI; avoid circular imports (`vpn` → `route` OK; not reverse).
- Use `//go:build windows` on OS-specific files.
- Unit-test pure parsing and any command-argument builders with fakes where practical; full tunnel tests are manual.

### Phase 2: Fyne UI Integration

**Depends on:** Phase 0 (Designer), Phase 1 (packages usable)  
**Parallelizable:** No single-file conflict — one Backend owner for UI  

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Backend Developer | `internal/ui/main_window.go` (+ optional `internal/ui/*.go`) | Replace demo UI with Designer-specified VPN form; wire validation, save, connect, disconnect, status updates on UI thread after background work. |
| 2.2 | Backend Developer | `cmd/vepeen/main.go` | Keep thin; only adjust if app ID, lifecycle, or platform guard needed. |

**Sub-Agent Guidance:**

- All Go including Fyne: **Backend Developer** (avoid Frontend/Backend split ownership).
- Long-running PowerShell/rasdial must not block the Fyne UI thread.
- Load config + secrets into form on startup when available.

### Phase 3: Documentation

**Depends on:** Phase 2 behavior stable enough to describe  
**Parallelizable:** Can draft earlier; finalize after implementation  

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 3.1 | Documentation | `README.md` | Document VPN usage, dual-auth (PSK + user/pass), split tunnel, config path, secrets policy, privileges, ports UDP 500/4500, troubleshooting, non-goals. |

### Phase 4: Review (Always Last)

**Depends on:** Phases 1–3  

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 4.1 | Debugger/Reviewer | Verify acceptance criteria; manual connect path if environment allows; check no plain-secret config; UI states; package boundaries. |
| 4.2 | Security | Review secrets storage, process invocation, logging, injection risks, elevation messaging. |
| 4.3 | Documentation | Confirm README matches shipped behavior and security notes. |

**No DevOps agent.**

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Elevation required for PSK/profile write | Medium | Medium | Per-user profile; clear UAC/elevation message; avoid all-user |
| Secrets on process command line | High | Medium | Minimize duration; prefer CredMan + future RAS API; never log |
| Server requires both PSK and user/pass | High if UI omits one | High (typical) | Collect both by default |
| Full tunnel accidental | High | Low if SplitTunneling always set | Always set `-SplitTunneling`; only Add-VpnConnectionRoute for list |
| Firewall/NAT blocks L2TP (UDP 500/4500) | High (user cannot connect) | Medium | Document ports; surface rasdial errors |
| PowerShell execution policy / path issues | Medium | Low–Med | `-NoProfile`; absolute powershell path if needed; clear errors |
| Connection name collision | Medium | Low | Default `Vepeen`; allow rename; document |
| CredMan scope slips past MVP | Medium | Medium | Explicit tradeoff; block plain JSON secrets always |
| Fyne UI thread blocking | Medium | Medium | Background workers + UI callbacks |

## Rollback Strategy

- Feature is additive packages + UI replacement. Rollback options:
  1. Revert commits introducing `internal/vpn|route|config|secrets` and restore demo `internal/ui`.
  2. Leave packages but hide Connect behind build tag / feature flag only if already shipped mid-way (prefer clean revert for v1).
- Windows side: user may remove VPN profile manually: `Remove-VpnConnection -Name Vepeen -Force` (document in README).
- Config/secrets files under user AppData can be deleted without affecting other apps.
- No DB migrations.

---

## Open Questions

None **blocking**. Defaults locked for implementation:

| Topic | Decision |
| ----- | -------- |
| Connection name default | `Vepeen` |
| CIDR input | Multi-line text, one IP/CIDR per line |
| UI language | Indonesian labels + English technical terms |
| Profile scope | Per-user |
| Orchestration | PowerShell VpnClient + rasdial |
| Secrets | No plain JSON; CredMan/DPAPI preferred; in-memory MVP only with docs if needed |
| Go UI owner | Backend Developer after Designer |
| Platform | Windows primary |

Non-blocking follow-ups (do not stop v1):

- Exact CredMan target naming scheme.
- Whether username is persisted always or behind checkbox.
- Optional advanced IPsec crypto later.

---

## Implementation Summary (for Orchestrator)

**PRD path:** `docs/planning/prd-002-l2tp-split-tunnel.md`  
**Scope:** Major — L2TP/IPsec PSK client with selective IP/CIDR routing (split tunnel) on Windows via built-in VPN  
**Research:** `docs/research/windows-l2tp-ipsec-split-tunnel.md`

**Ordered agent pipeline:**

1. **Designer** — VPN form layout, Indonesian copy, states (Phase 0)  
2. **Backend Developer** — `internal/config`, `internal/secrets`, `internal/route`, `internal/vpn`, then Fyne `internal/ui` + thin `main` (Phases 1–2)  
3. **Documentation** — README VPN usage & security notes (Phase 3)  
4. **Debugger/Reviewer** — acceptance verification (Phase 4.1)  
5. **Security** — secrets, exec, logging review (Phase 4.2)  
6. **Documentation** — final README accuracy pass (Phase 4.3)

**Primary implementer after design:** Backend Developer (all Go, including Fyne).  
**Not used:** DevOps, Frontend Developer (Fyne owned by Backend).

**Files to create / modify:**

| File | Action | Purpose |
| ---- | ------ | ------- |
| `docs/planning/ui-002-l2tp-form.md` (or design section) | Create (Designer) | UI/UX spec |
| `internal/config/*.go` | Create | Non-secret persistence |
| `internal/secrets/*.go` | Create | PSK/password secure storage |
| `internal/route/*.go` | Create | CIDR parse + route sync |
| `internal/vpn/*.go` | Create | Profile ensure, dial, status |
| `internal/ui/main_window.go` (+ optional splits) | Modify/replace | VPN form UI |
| `cmd/vepeen/main.go` | Modify if needed | Lifecycle only |
| `README.md` | Modify | Product usage + security |
| `go.mod` / `go.sum` | Modify only if new deps | Prefer stdlib-only v1 |

**Planner-owned files this turn:**

| File | Purpose |
| ---- | ------- |
| `docs/planning/prd-002-l2tp-split-tunnel.md` | This PRD |
| `docs/planning/changelog.md` | Changelog entry |

---

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v0.1.0 | 2026-07-22 | Initial draft: Windows L2TP/IPsec PSK + split tunnel via VpnClient/rasdial |
