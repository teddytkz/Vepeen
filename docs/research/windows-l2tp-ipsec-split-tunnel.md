# Technology Evaluation: Windows L2TP/IPsec VPN Client (PSK + Split Tunnel)

**Date:** 2026-07-22  
**Researcher:** Learner Agent  
**Context:** Recommend how vepeen (Go 1.22 + Fyne v2.8 desktop) should implement L2TP/IPsec with pre-shared key and selective IP routing on Windows.  
**Existing Stack:** Greenfield starter only — `cmd/vepeen` + `internal/ui`; no VPN/network packages yet. Windows primary target; CGo already required for Fyne.

---

## Evaluation Criteria

| Criteria | Weight | Notes |
| -------- | ------ | ----- |
| Practicality for Go desktop app | High | Prefer OS-native APIs over shipping drivers/stacks |
| L2TP/IPsec + PSK support | High | Core user requirement |
| Selective routing (split tunnel) | High | Only specific IPs/CIDRs via VPN |
| Privilege / UX friction | High | Admin elevation is acceptable if clear; minimize always-admin |
| Maintainability | High | Small team; avoid fragile reverse-engineered stacks |
| Security of secrets | Medium | PSK + user/pass storage |
| Migration / dependency cost | Medium | Greenfield — low migration cost |
| Cross-platform future | Low | Windows-first; do not over-design |

---

## Candidates

| Criteria | A. Windows built-in VPN (VpnClient + rasdial/RAS) | B. rasapi32 pure Win32 from Go | C. Third-party stack (SoftEther / strongSwan / custom IPsec) |
| -------- | ------------------------------------------------ | ------------------------------ | ----------------------------------------------------------- |
| Maturity | High — OS-supported | High — same RAS stack | Medium–High (product-dependent) |
| L2TP + PSK | First-class (`-L2tpPsk`, `RASCM_PreSharedKey`) | First-class | Varies; SoftEther is server-oriented for L2TP |
| Split tunnel | First-class (`-SplitTunneling` + `Add-VpnConnectionRoute`) | Possible via routes/API | Manual routes / own stack |
| TypeScript/N/A | N/A (Go) | N/A | N/A |
| Bundle size | Zero (OS) | Zero | Large (drivers, services) |
| Community | Microsoft docs + widespread ops use | Win32 docs | Fragmented |
| Learning curve | Low–Med (PowerShell + process exec) | Med–High (structs, phonebook) | High |
| Fit for vepeen | **Best** | Good later optimization | Poor for v1 |

---

## Deep Dive: A — Windows Built-in VPN (Recommended)

### How it works

Windows already implements L2TP/IPsec client via RAS (Remote Access Service). Management surface:

1. **Profile create/update** — PowerShell `VpnClient` module:
   - `Add-VpnConnection` / `Set-VpnConnection`
   - `Remove-VpnConnection`
2. **Connect/disconnect** — `rasdial.exe` (or RAS API):
   - `rasdial <Name> <user> <pass>`
   - `rasdial <Name> /DISCONNECT`
3. **Selective routes** — `Add-VpnConnectionRoute` / `Remove-VpnConnectionRoute` (preferred) or post-connect `route` / `New-NetRoute`

### Create L2TP + PSK profile (canonical)

```powershell
Add-VpnConnection `
  -Name "Vepeen" `
  -ServerAddress "vpn.example.com" `
  -TunnelType L2tp `
  -L2tpPsk "YOUR_PSK" `
  -AuthenticationMethod MSChapv2 `
  -EncryptionLevel Required `
  -SplitTunneling `
  -Force `
  -RememberCredential:$false `
  -PassThru
```

Evidence (Microsoft Learn — `Add-VpnConnection`):

- `-TunnelType L2tp` supported
- `-L2tpPsk` sets IPsec pre-shared key; without it, certificate is used
- `-Force` acknowledges PSK over insecure channel (required when supplying PSK via cmdlet)
- `-SplitTunneling` prevents full-tunnel default route through VPN
- Default auth often **MS-CHAPv2** (user/password layer separate from IPsec PSK)

### Selective IP routing

With split tunneling enabled, Windows does **not** send all traffic through the VPN. Routes for specific destinations must be attached to the connection:

```powershell
Add-VpnConnectionRoute -ConnectionName "Vepeen" -DestinationPrefix "10.10.0.0/16"
Add-VpnConnectionRoute -ConnectionName "Vepeen" -DestinationPrefix "203.0.113.50/32"
```

These routes are bound to the VPN profile and applied when connected — better than ad-hoc `route add` after connect (survives reconnects, cleaned with profile).

Fallback if needed after connect:

```powershell
# Identify VPN interface, then:
New-NetRoute -DestinationPrefix "10.10.0.0/16" -InterfaceIndex <ifIndex>
# or: route add 10.10.0.0 mask 255.255.0.0 <gateway>
```

### Connect / disconnect

```text
rasdial Vepeen username password
rasdial Vepeen /DISCONNECT
rasdial   # list status
```

Status can also be polled via `Get-VpnConnection -Name Vepeen` (`ConnectionStatus`).

### Username/password vs PSK

**Yes — typically both are required.**

| Layer | Secret | Role |
| ----- | ------ | ---- |
| IPsec (IKE) | Pre-shared key (PSK) | Authenticates the **machine/tunnel** to the VPN gateway |
| L2TP / PPP | Username + password (usually MS-CHAPv2) | Authenticates the **user** session |

Most commercial/home L2TP/IPsec servers (MikroTik, SoftEther L2TP, Windows RRAS, many VPS panels) expect:

1. PSK for IPsec
2. User credentials for PPP

Some misconfigured servers allow empty/blank PPP auth — treat as optional only if server docs say so; **UI should collect user + password by default**.

### Privilege model

| Operation | Typical privilege |
| --------- | ----------------- |
| `Add-VpnConnection` (per-user phonebook, no `-AllUserConnection`) | Standard user often works |
| `Add-VpnConnection -AllUserConnection` | **Administrator** |
| Setting PSK (`-L2tpPsk` / `RASCM_PreSharedKey`) | May require elevation depending on phonebook scope; all-user PSK needs admin (`ERROR_ACCESS_DENIED` documented for non-admin all-user PSK) |
| `Add-VpnConnectionRoute` | Usually same as profile owner; all-user needs admin |
| `rasdial` connect | Standard user if profile is per-user and credentials available |
| Manual `route add` / some `New-NetRoute` cases | Often **Administrator** |

**Recommendation:** Use **per-user** VPN profile (omit `-AllUserConnection`) to reduce elevation. If PSK write fails without admin, elevate once for profile create/update (UAC), then connect as normal user.

### Go integration pattern (no implementation here)

Preferred v1 approach: **orchestrate OS tools from Go**, not reimplement IPsec.

```
internal/vpn/          # profile ensure, connect, disconnect, status
internal/route/        # ensure CIDR list on profile (Add-VpnConnectionRoute)
internal/config/       # server, routes, non-secret prefs
internal/secrets/      # optional: DPAPI / Credential Manager for PSK & password
internal/ui/           # Fyne forms + connect button + status
```

Execution options (in order of pragmatism):

1. **PowerShell for profile + routes** (`powershell -NoProfile -Command ...`) — matches Microsoft docs 1:1
2. **`rasdial` for connect/disconnect** — simple, reliable
3. Later: optional `golang.org/x/sys/windows` + `rasapi32` for cleaner status/credentials without shell

Avoid embedding full IPsec stacks in v1.

### Security notes

- **Do not store PSK/password in plain JSON.** Prefer:
  - Windows Credential Manager / DPAPI (`CredWrite` / `CryptProtectData`) for secrets
  - Config file only for non-secrets (server, name, CIDR list)
- Passing PSK on PowerShell command line can appear in process listings — mitigate by:
  - Writing a short-lived `.ps1` with restricted ACL, or
  - Using RAS API `RasSetCredentials` with `RASCM_PreSharedKey` (better long-term)
- `-Force` is required for `-L2tpPsk` but acknowledges insecure channel for the *management* call, not the tunnel itself
- Clear secrets from memory where practical; never log PSK/password
- L2TP/IPsec with PSK is weaker than cert-based IPsec / modern WireGuard — acceptable for user request, document risk

### Windows version caveats

- `VpnClient` module: Windows 8 / Server 2012+ client SKUs; solid on **Windows 10/11**
- Target **Windows 10 1809+ / Windows 11** for vepeen
- UDP 500, UDP 4500 (NAT-T), and ESP must be allowed outbound; corporate firewalls often block L2TP/IPsec
- Some ISPs/CGNAT break IPsec; not app-fixable
- Connection name must be unique; avoid colliding with existing user VPN profiles
- If server requires specific IPsec crypto, optional `Set-VpnConnectionIPsecConfiguration` (cipher/DH/PFS) — advanced setting, not v1 default

---

## Deep Dive: B — Pure rasapi32 from Go

- Same underlying stack as A
- Better control: `RasSetCredentials` (PSK via `RASCM_PreSharedKey`, user/pass), `RasDial`/`RasHangUp`, phonebook APIs
- Higher implementation cost (structs, Unicode, error codes)
- **Recommendation:** Phase 2 optimization after PowerShell/`rasdial` path works end-to-end

---

## Deep Dive: C — Third-party stacks

| Option | Why rejected for v1 |
| ------ | ------------------- |
| SoftEther client | Separate product/service; not a clean embeddable library for Fyne app |
| strongSwan / libreswan | Linux-centric; Windows support poor for app embedding |
| Custom Go IPsec + L2TP | Extremely high effort; crypto/NAT-T/kernel offload; security risk |
| tun2socks / Wintun | Great for proxy/TUN apps, **not** L2TP/IPsec protocol |

---

## Recommendation

**Recommended:** **Windows built-in VPN** via PowerShell `VpnClient` + `rasdial`, with selective routes via `Add-VpnConnectionRoute`.

**Why:**

1. Official, maintained L2TP/IPsec + PSK path (`-L2tpPsk`)
2. Native split tunneling (`-SplitTunneling`) + per-destination routes (`Add-VpnConnectionRoute`)
3. Zero extra drivers/binaries; fits single-exe Go desktop model
4. Matches real-world Windows admin practice; easy to debug with OS tools
5. Lowest maintainability risk for a greenfield Fyne app

**Tradeoffs accepted:**

- Shell/process orchestration is less “pure” than RAS API — acceptable for v1; can harden later
- Depends on Windows VPN stack quirks and network path (UDP 500/4500)
- PSK on cmdline is imperfect — mitigate with DPAPI storage and eventual RAS API

**Risks:**

| Risk | Mitigation |
| ---- | ---------- |
| Admin required for some profile ops | Prefer per-user profile; elevate only for create/update if needed |
| Server needs user/pass + PSK | Collect both in UI; document dual-auth model |
| Routes not applied | Always set `-SplitTunneling` + explicit `Add-VpnConnectionRoute` for each CIDR |
| Secret leakage | Credential Manager/DPAPI; never log secrets; avoid long-lived temp scripts |
| Firewall/NAT blocks L2TP | Surface clear connection errors; document ports |

---

## Required User Inputs

| Input | Required | Notes |
| ----- | -------- | ----- |
| Connection name | Optional (default e.g. `Vepeen`) | Windows phonebook entry name |
| Server address | **Yes** | Host or IP |
| Pre-shared key (PSK) | **Yes** | IPsec secret |
| Username | **Yes** (typical) | PPP / MS-CHAPv2 |
| Password | **Yes** (typical) | PPP / MS-CHAPv2 |
| IP/CIDR list | **Yes** for selective routing | e.g. `10.0.0.0/8`, `203.0.113.10/32` |
| Remember secrets | Optional | Store via Credential Manager |
| Advanced IPsec crypto | Out of scope v1 | Optional later |

---

## High-Level Architecture (for Planner)

```
┌─────────────────────────────────────────────┐
│  Fyne UI (internal/ui)                      │
│  form: server, PSK, user, pass, CIDRs       │
│  actions: Connect / Disconnect / Save       │
│  status: Connected / Connecting / Error     │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│  VPN Manager (internal/vpn)                 │
│  EnsureProfile()  → Add/Set-VpnConnection   │
│  Connect()        → rasdial                 │
│  Disconnect()     → rasdial /DISCONNECT     │
│  Status()         → Get-VpnConnection       │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│  Route Manager (internal/route)             │
│  SyncRoutes(cidrs) → Add/Remove-VpnConnectionRoute │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│  Config + Secrets                           │
│  config: server, name, CIDRs (file)         │
│  secrets: PSK, password (DPAPI / CredMan)   │
└─────────────────────────────────────────────┘
```

### Suggested connect flow

1. Validate inputs (server, PSK, user/pass, parse CIDRs)
2. `EnsureProfile` with L2TP + PSK + MS-CHAPv2 + SplitTunneling
3. `SyncRoutes` for configured CIDRs
4. `rasdial` with username/password
5. Poll status; show errors from stderr/exit code
6. On disconnect: `rasdial /DISCONNECT` (routes stay on profile for next connect)

### Privilege model for product

- **Default:** run as normal user; create **per-user** VPN entry
- **If profile/PSK write fails:** prompt to relaunch elevated **once** for setup, or run elevated helper only for that operation
- Do **not** require permanent admin for day-to-day connect if avoidable

---

## Out of Scope / Non-Goals (v1)

- Full-tunnel VPN (default route via VPN)
- Certificate-based IPsec (machine cert)
- IKEv2 / SSTP / WireGuard / OpenVPN
- Always On VPN / MDM ProfileXML
- Custom IPsec cipher UI (unless server incompatibility forces it)
- macOS/Linux clients
- Shipping third-party VPN drivers or SoftEther
- Kill switch / firewall lockdown
- Multi-hop, obfuscation, or anti-censorship features

---

## Alternatives Rejected (brief)

1. **Full custom L2TP/IPsec in Go** — unjustified crypto/network complexity  
2. **SoftEther embedding** — heavy external stack; wrong product boundary  
3. **Always On / VPNv2 CSP** — enterprise MDM path; overkill for desktop app  
4. **Only `route add` without split-tunnel profile** — racey, full-tunnel risk if default route appears  
5. **Pure rasapi32 first** — correct long-term, slower to ship; use after PoC with PowerShell

---

## Implementation Notes for Planner

- **Estimated integration effort:** **M** (medium) for MVP connect + routes + basic UI; **L** if Credential Manager + elevation helper + robust error mapping
- **Dependencies to add:** none required for v1 (stdlib `os/exec`); optional later `golang.org/x/sys/windows`
- **Packages to create:**
  - `internal/vpn` — Windows profile + dial
  - `internal/route` — CIDR sync on VPN profile
  - `internal/config` — non-secret settings persistence
  - `internal/secrets` — DPAPI/Credential Manager (can be phase 1.5)
  - extend `internal/ui` — replace demo with VPN form
- **Build tags:** `//go:build windows` on VPN/route packages; stub or clear error on non-Windows
- **Testing strategy:** manual against real L2TP server; unit-test CIDR parsing and command construction with fakes
- **PRD acceptance sketch:**
  - User can save server + PSK + credentials + CIDR list
  - Connect establishes L2TP/IPsec; only listed CIDRs route via VPN
  - Disconnect tears down tunnel; UI shows status/errors
- **PoC suggestion (optional before full UI):** 20-line PowerShell script create → route → rasdial → ping one CIDR host → disconnect

---

## Sources

- Microsoft Learn: [Add-VpnConnection](https://learn.microsoft.com/en-us/powershell/module/vpnclient/add-vpnconnection) (`-L2tpPsk`, `-SplitTunneling`, `-TunnelType L2tp`, `-AuthenticationMethod`)
- Microsoft Learn: [Add-VpnConnectionRoute](https://learn.microsoft.com/en-us/powershell/module/vpnclient/add-vpnconnectionroute) / [Remove-VpnConnectionRoute](https://learn.microsoft.com/en-us/powershell/module/vpnclient/remove-vpnconnectionroute)
- Microsoft Learn: [Set-VpnConnectionIPsecConfiguration](https://learn.microsoft.com/en-us/powershell/module/vpnclient/set-vpnconnectionipsecconfiguration) (optional crypto)
- Microsoft Learn: [RasDial](https://learn.microsoft.com/en-us/windows/win32/api/ras/nf-ras-rasdiala), [RasSetCredentials](https://learn.microsoft.com/en-us/windows/win32/api/ras/nf-ras-rassetcredentialsa) (`RASCM_PreSharedKey`, admin notes)
- Local machine: `VpnClient` module present (`Add-VpnConnection`, `Add-VpnConnectionRoute`, `rasdial.exe`)
- Project context: vepeen Go+Fyne starter; no existing VPN code (`docs/planning/prd-001-...`)
