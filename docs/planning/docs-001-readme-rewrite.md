# Docs-001: Root README rewrite (accuracy pass)

**Version:** v1.0.0  
**Status:** Approved for implementation  
**Author:** Planner Agent  
**Created:** 2026-07-24  
**Updated:** 2026-07-24  
**Type:** Documentation only (no code)

---

## Decision

**Major documentation rewrite** — not a product PRD.

| Option | Why / why not |
| ------ | ------------- |
| Full feature PRD | No — no code, schema, or API work |
| Changelog-only one-liner | No — README is largely wrong; needs a structured rewrite brief |
| **Docs plan + changelog** | **Yes** — single-file rewrite with clear section map and acceptance criteria |

**File scope:** `README.md` only (+ this plan + `docs/planning/changelog.md` note).

**Agent:** Documentation  
**Review:** Debugger/Reviewer (accuracy vs code; no secrets invented)

---

## Problem

Root `README.md` describes an older product model:

- Creates/updates L2TP profiles (IP + PSK UI)
- Indonesian labels
- Split tunnel only; full tunnel listed as non-goal
- Routes IPv4 only; build output at repo root; flat `internal/vpn`

Current product (verified 2026-07-24):

- Manages **existing** OS VPN profiles (`ListProfiles` / rasdial) — does **not** create L2TP profiles
- English UI; single CTA Connect → Cancel → Disconnect
- Optional username/password; **Route All Traffic** checkbox
- Global routes: IPv4 IP/CIDR **or domain names** (resolved at connect)
- Live stats: DOWN/UP, PING, local VPN IP when connected
- Tray (hide on X), single-instance, desktop shortcut, quit disconnect dialog
- Storage: `vepeen.bin` DPAPI next to exe
- Build: `.\build.ps1` → `bin/vepeen.exe`
- Layout: `internal/vpn` facade + `win/` + `shared/`
- Theme accent: `#2dd4bf`

---

## Goals

- README matches shipped behavior; no invented features
- Concise (lazy senior): accurate sections, no essays
- English UI strings in checklists / troubleshooting
- Document existing-profile model + Route All Traffic + domain routes
- Update layout, build path, non-goals, last-updated date **2026-07-24**

## Non-Goals

- Code changes, screenshots, i18n, new features
- Rewriting PRDs under `docs/planning/` (optional “Further reading” links only)
- Documenting macOS as supported (stubs exist; product is Windows-first)
- Claiming PSK UI or profile creation

---

## Section map (rewrite / keep / delete)

| Current section | Action | Notes |
| --------------- | ------ | ----- |
| Title + one-liner | **Rewrite** | Existing-profile client; split **or** full tunnel via Route All Traffic |
| Status callout (PSK reserved) | **Rewrite / shrink** | Keep brief: PSK not in UI; configure on OS profile if needed. Drop “creates profile without PSK” |
| UI / theme | **Rewrite** | English UI; accent `#2dd4bf` (not `#0FB5AE`); drop Indonesian “Rute/Log” |
| What it does | **Rewrite** | Select existing profile → optional creds → routes or Route All Traffic → connect |
| Dual authentication | **Keep, tighten** | Still true for L2TP/IPsec; clarify app does not set PSK |
| Split tunnel behavior | **Rewrite** | Default split path; add **Route All Traffic** subsection |
| IP/CIDR list format | **Rewrite** | Add domain names; English parse errors (`Line N is invalid`) |
| Prerequisites | **Keep** | Windows 10/11, Go 1.22+, CGo/GCC |
| Setup / run / build | **Rewrite** | `.\build.ps1` → `bin/vepeen.exe`; debug console variant; log path |
| Using the app | **Rewrite** | English controls table; single CTA; stats; tray/quit/shortcut |
| Typical flow | **Rewrite** | Profile select first; no IP/PSK fields |
| Status & log | **Rewrite** | English states; local IP; rates/ping |
| Config & secrets | **Keep core, fix details** | `vepeen.bin` DPAPI; fields: `selectedProfile`, `routes`, `routeAllTraffic`, `rememberCredentials`, `credentials` |
| Migration | **Keep short** | Legacy config.json / CredMan → bin; one-time |
| Privileges | **Rewrite** | No profile create; rasdial + route sync; NAT-T registry may need admin |
| Network requirements | **Keep** | UDP 500/4500 |
| Project layout | **Rewrite** | `bin/`, `winres/`, `vpn/win`, `vpn/shared` |
| Security notes | **Keep, trim** | rasdial argv; DPAPI user-bound; dormant PSK script path if still accurate — do not invent |
| Manual test checklist | **Rewrite** | English; existing profile; Route All Traffic; domains optional |
| Troubleshooting | **Rewrite** | English strings; Route All Traffic vs split enforce |
| Non-goals | **Rewrite** | **Remove** “Full tunnel”; keep IKEv2/WG/OpenVPN, cert UI, kill switch, multi-profile manager, CI/installers, etc. |
| License | **Keep** | One line |
| Further reading | **Add** | Optional links to PRD-002…006, prd-vpn-win-package, research note |

**Delete / do not restore:** Indonesian-only control tables; “creates/updates L2TP profile”; IP + Key (PSK) form fields; “split tunnel only” as product definition; root `vepeen.exe` as primary build output; flat vpn package tree.

---

## Target README outline (ordered)

```markdown
# Vepeen

{1–2 sentence product definition — Windows desktop client for existing OS VPN profiles;
 optional user/pass; global split-tunnel routes or Route All Traffic; Go + Fyne;
 VpnClient + rasdial. Last updated: 2026-07-24}

## What it does
1. Lists existing Windows VPN profiles
2. Optional username/password (Remember credentials)
3. Global destinations: IPv4 IP/CIDR or domain names — OR Route All Traffic
4. Connect/disconnect (single CTA); disconnects other active Windows VPNs first
5. Live DOWN/UP, PING, local VPN IP when connected
6. Persists settings/creds in vepeen.bin (DPAPI next to exe)
7. Tray hide-on-X, single-instance, desktop shortcut, quit → disconnect dialog

## Dual authentication (L2TP/IPsec)
{short table IPsec PSK vs PPP user/pass; app does not create profile or set PSK}

## Routing modes
### Split tunnel (default)
### Route All Traffic
### Destination list format
{IP, CIDR, domain, # comments, blank lines; IPv6 rejected; domains resolved at connect}

## Prerequisites (Windows)
## Setup / run / build
{go run; .\build.ps1 → bin/vepeen.exe; windowsgui; vepeen.log; go test paths}

## Using the app
{English control table: profile select, user, pass, remember, routes, Route All Traffic,
 Save, Connect/Cancel/Disconnect CTA, status, log, stats, Apps menu}
### Typical flow
### Status, stats & log
### Tray, single-instance, quit

## Config & secrets
{vepeen.bin location, DPAPI, field table incl. routeAllTraffic, migration one-liner}

## Privileges
## Network requirements
## Project layout
{cmd/vepeen, internal/{ui,config,secrets,route,vpn,vpn/win,vpn/shared}, bin/, winres/, docs/}

## Security notes & residual risks
## Manual test checklist (Windows)
## Troubleshooting
## Non-goals
## Further reading (optional)
## License
```

---

## Implementation tasks

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1 | Documentation | `README.md` | Full rewrite per outline; English only; match code facts above |
| 2 | Documentation | `docs/planning/changelog.md` | Already planned; ensure Unreleased entry present after write |
| 3 | Debugger/Reviewer | `README.md` | Spot-check vs `main_window.go`, `manager.go`, `parse.go`, `build.ps1`, `theme.go` |

**Parallelizable:** No — single file rewrite.

---

## Acceptance criteria

- [ ] No claim that the app creates/updates L2TP profiles or collects IP/PSK in UI
- [ ] English UI labels only in user-facing tables/checklists
- [ ] Route All Traffic documented (default off; empty routes OK when checked)
- [ ] Routes: IPv4 IP/CIDR **or** domain names; domains resolved at connect
- [ ] Build: `.\build.ps1` → `bin/vepeen.exe`
- [ ] Layout includes `internal/vpn/win`, `internal/vpn/shared`, `bin/`, `winres/`
- [ ] Non-goals do **not** list full tunnel as out of scope
- [ ] Troubleshooting/manual tests use English status strings
- [ ] Config documents `routeAllTraffic` + `vepeen.bin` DPAPI next to exe
- [ ] Theme accent `#2dd4bf` if theme mentioned
- [ ] Last updated **2026-07-24**
- [ ] No invented features (no kill switch, no multi-profile manager, no PSK UI)

---

## Handoff notes (Documentation agent)

1. **Source of truth:** code + Explorer brief, not old README prose. Prefer deleting wrong paragraphs over patching Indonesian leftovers.
2. **Product one-liner:** “Windows desktop client that connects **existing** OS VPN profiles with optional credentials and either selective routes or full tunnel (Route All Traffic).”
3. **Manager comment:** `Manager orchestrates route sync and dial for an existing Windows VPN profile` — lead with that model.
4. **UI strings to use:** `Select VPN profile…`, `Username`, `Password`, `Remember credentials`, `SPLIT TUNNEL ROUTES`, `Route All Traffic`, `Connect` / `Cancel` / `Disconnect`, `Disconnected` / `Connecting…` / `Connected`, `Create Desktop Shortcut`, `Quit`, `Quitting Vepeen` / `Disconnecting VPN…`.
5. **Parse errors:** `Line N is invalid: …` (not `Baris N tidak valid`).
6. **PSK:** mention only as OS-profile / dual-auth context; reserved `psk` in store is optional one-liner max — do not center the README on it.
7. **Security:** keep rasdial argv + DPAPI user-scope caveats; only mention temp PSK scripts if still present in code paths (dormant is fine as one line).
8. **Length budget:** aim shorter than current README; drop repeated migration essays if one short subsection covers it.
9. **Further reading (optional, relative links):**
   - `docs/planning/prd-002-l2tp-split-tunnel.md` (historical; profile-create model superseded)
   - `docs/planning/prd-003-global-routes.md`
   - `docs/planning/prd-004-encrypted-config.md`
   - `docs/planning/prd-005-local-ip-status.md`
   - `docs/planning/prd-006-route-all-traffic.md`
   - `docs/planning/prd-vpn-win-package.md`
   - `docs/research/windows-l2tp-ipsec-split-tunnel.md`
10. **Do not** update repo memory identity file unless asked; README is the user-facing fix.

---

## Risks

| Risk | Mitigation |
| ---- | ---------- |
| Copying stale PRD-002 “creates profile” language | Prefer manager.go + UI; mark PRD-002 as historical in Further reading |
| Over-documenting macOS | Windows-first; non-Windows stubs “not supported” one line max |
| Re-introducing Indonesian | Grep README for Indonesian tokens before finish |

## Rollback

`git checkout -- README.md`

---

## Version history

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.0.0 | 2026-07-24 | Initial docs rewrite plan |
