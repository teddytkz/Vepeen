# Vepeen

Windows desktop client for **existing** OS VPN profiles (Go 1.22 + Fyne v2.8). Select a profile, optional username/password, global split-tunnel destinations **or Route All Traffic**, then connect/disconnect via `rasdial` + `VpnClient` PowerShell. Does **not** create L2TP profiles or set PSK.

**Last updated:** 2026-07-24

## What it does

1. Lists **existing** Windows VPN profiles (`Get-VpnConnection`) — you pick one.
2. Optional **username/password** for `rasdial` (Remember credentials).
3. Global destinations: IPv4 IP/CIDR **or domain names** — **or** check **Route All Traffic**.
4. Single CTA: **Connect → Cancel → Disconnect**. Disconnects other active Windows VPNs first.
5. Live stats when connected: **DOWN** / **UP** rates, **PING**, local VPN IP.
6. Persists settings + credentials in encrypted `vepeen.bin` (DPAPI, next to the exe).
7. Tray (X hides), single-instance, desktop shortcut (Apps menu), quit → disconnect dialog.

UI language is **English**. Theme accent: teal `#2dd4bf`.

## Dual authentication (L2TP/IPsec)

Typical L2TP/IPsec servers need **both** layers. Vepeen only passes optional PPP credentials at dial time.

| Layer | Secret | Who sets it |
| ----- | ------ | ----------- |
| IPsec (IKE) | Pre-shared key (PSK) | **OS VPN profile** (outside this app) |
| L2TP / PPP | Username + password (MS-CHAPv2) | Optional in Vepeen → `rasdial` |

Vepeen does **not** create/update the Windows profile and does **not** set PSK. Configure PSK (and the profile itself) in Windows Settings / PowerShell first. Wrong PSK often looks like a generic network/auth failure — check both layers.

## Routing modes

### Split tunnel (default)

- **Route All Traffic** unchecked.
- App enables split tunneling on the selected profile, syncs listed destinations (`Add-VpnConnectionRoute` / `Remove-VpnConnectionRoute`), then after dial best-effort removes a server-pushed `0.0.0.0/0` on the VPN interface.
- At least one valid destination is required.
- Route list changes need disconnect/reconnect to fully apply.

### Route All Traffic

- Checkbox **Route All Traffic**.
- App disables split tunneling on the profile; empty destination list is allowed.
- All traffic uses the VPN (subject to OS/server behavior).

### Destination list format

| Input | Result |
| ----- | ------ |
| `10.10.0.0/16` | Kept as that prefix |
| `203.0.113.50` | Treated as `/32` |
| `example.com` | Domain kept; **resolved to IPv4 /32s at connect** |
| Blank lines | Ignored |
| Lines starting with `#` | Comments (ignored) |
| IPv6 / garbage | Rejected: **Line N is invalid** |

Example:

```text
# office LAN
10.10.0.0/16
203.0.113.50
intranet.example.com
```

## Prerequisites (Windows)

| Requirement | Notes |
| ----------- | ----- |
| **Windows 10/11** | `VpnClient` module + `rasdial.exe` |
| **Existing VPN profile** | Create L2TP (or other) profile in Windows first |
| **Go 1.22+** | Module targets `1.22.0` |
| **C compiler (CGo)** | Fyne needs GCC on `PATH` (MSYS2 MinGW-w64 recommended) |
| **CGO enabled** | `go env CGO_ENABLED` → `1` |

```powershell
go version
go env CGO_ENABLED
gcc --version
Get-Command Get-VpnConnection, rasdial
```

## Setup / run / build

```powershell
go mod tidy
go run ./cmd/vepeen

# GUI binary (no console window):
.\build.ps1
.\bin\vepeen.exe
```

`build.ps1` sets `CGO_ENABLED=1`, embeds winres (icon + DPI manifest when `rsrc_*.syso` present), and builds with `-H windowsgui` → **`bin/vepeen.exe`**.

```powershell
$env:CGO_ENABLED = "1"
go build -ldflags="-H windowsgui" -o bin/vepeen.exe ./cmd/vepeen
```

Debug with a console: drop `-ldflags="-H windowsgui"`.

**Crash / startup log** (GUI hides stdout/stderr):

```text
%AppData%\vepeen\vepeen.log
```

Tests:

```powershell
go test ./internal/route ./internal/vpn ./internal/config
```

## Using the app

| Control | Purpose |
| ------- | ------- |
| **Connection profile** | Select an existing Windows VPN profile |
| **Username** / **Password** | Optional PPP credentials for `rasdial` (password masked) |
| **Remember credentials** | Reload stored user/pass from `vepeen.bin` |
| **Split tunnel routes** | Global multi-line destinations (IP/CIDR/domain) |
| **Route All Traffic** | Full tunnel via VPN; destinations optional |
| **Save Settings** | Persist settings + credentials to `vepeen.bin` |
| **Connect / Cancel / Disconnect** | Single CTA (hero ring + button) |
| **DOWN / UP / PING** | Live rates and gateway ping when connected |
| **Log** + **Clear** | Session activity (`HH:MM:SS`); never shows secrets |
| **Apps → Create Desktop Shortcut** | `Vepeen.lnk` on Desktop |
| **Apps → Quit** | Disconnect dialog, then exit |

### Typical flow

1. Create/configure the Windows VPN profile (incl. PSK if required) **outside** Vepeen.
2. Open Vepeen → select the profile.
3. Optional username/password; check **Remember credentials** if desired.
4. Enter destinations **or** enable **Route All Traffic**.
5. **Save Settings** (optional; connect also persists).
6. **Connect** — other active Windows VPNs are disconnected first; watch Log/status.
7. **Disconnect** when finished (or **Cancel** while connecting).

### Status, stats & log

| Situation | UI |
| --------- | -- |
| Idle / after load | **Disconnected** |
| Connect in progress | **Connecting…** (CTA → **Cancel**) |
| Connected | **Connected**; local VPN IP when assigned; DOWN/UP/PING live |
| Already connected (OS) | Treated as **Connected** |
| Disconnect / quit | **Disconnecting…** then closed |

Log is in-memory for the session (~300 lines). Status/log never include password, PSK, or full credential-bearing command lines.

### Tray, single-instance, quit

- **X** hides to system tray; tray click shows the window.
- Second launch focuses the existing instance (mutex) instead of opening a blank window.
- **Quit** (tray or Apps menu) shows a progress dialog, best-effort disconnect (≈5s), then exits.

## Config & secrets

All settings and credentials live in **`vepeen.bin`** next to `vepeen.exe` (fallback: `%AppData%\vepeen\vepeen.bin`). Opaque **DPAPI** blob (current Windows user). No plaintext `config.json` and no Credential Manager at runtime.

| Field | Description |
| ----- | ----------- |
| `selectedProfile` | Last-selected Windows VPN profile name |
| `routes` | Global destination list (IP/CIDR/domain) |
| `routeAllTraffic` | `true` = full tunnel mode |
| `rememberCredentials` | Reload password from store on load |
| `credentials` | Per-profile `username` / `password` (`psk` reserved, unused by UI) |

**Policy:** secrets never plaintext on disk; bound to the Windows user (password reset / other account → blob unreadable → defaults). Delete `vepeen.bin` to reset.

**Migration (one-time):** if `vepeen.bin` is missing, app imports legacy `config.json` + CredMan entries, writes the bin, then removes old sources. Idempotent; on write failure old sources stay.

## Privileges

| Operation | Typical privilege |
| --------- | ----------------- |
| List profiles, `rasdial`, route sync | Standard user (per-user profile) |
| NAT-T registry (`AssumeUDPEncapsulationContextOnSendRule`) | May need **admin** once; non-fatal warning if denied |
| Profile **creation** | Not done by Vepeen — use Windows / elevated PowerShell yourself |

## Network requirements

Outbound:

- **UDP 500** (IKE)
- **UDP 4500** (NAT-T)
- ESP as required by the path

Corporate firewalls and some CGNAT setups break L2TP/IPsec; that is outside app control.

## Project layout

```text
vepeen/
├── cmd/vepeen/              # Entrypoint + winres .syso
├── bin/vepeen.exe           # build.ps1 output
├── winres/                  # Icon + DPI manifest source
├── internal/
│   ├── ui/                  # Fyne UI, tray, single-instance
│   ├── config/              # vepeen.bin DPAPI store
│   ├── secrets/             # Legacy CredMan (migration only)
│   ├── route/               # Parse + resolve + route sync
│   └── vpn/                 # Public facade (internal/vpn)
│       ├── shared/          # Types + error helpers (vpn/shared)
│       └── win/             # Windows VpnClient / rasdial (vpn/win)
├── docs/planning/           # PRDs / fix notes
├── docs/research/
├── build.ps1
├── FyneApp.toml
└── README.md
```

Facade: `internal/vpn` → `win` + `shared`. Non-Windows builds get stubs.

## Platform support

**Windows 10/11** is the primary target. **macOS** is partial (existing System Settings profiles only). **Linux** is compile stubs — not a usable VPN client.

| Capability | Windows | macOS | Linux |
| ---------- | ------- | ----- | ----- |
| UI (Fyne) | Full | Mostly | Basic window only |
| System tray | Yes | Yes | No |
| VPN connect (existing OS profiles) | Full (`rasdial` / VpnClient) | Partial (`scutil --nc` / `networksetup`) | No (stub) |
| Create VPN profile | No (app never creates) | No | No |
| UI username/password at dial | Yes (optional → rasdial) | Ignored (OS-saved only) | N/A |
| Split routes | Full | Partial (post-dial, admin prompt) | No |
| Secrets / encrypted config | `vepeen.bin` DPAPI | `vepeen.bin` AES + Keychain | Encrypt fails; no persistent secrets |
| Single-instance / desktop shortcut | Yes | No | No |
| Traffic counters | Yes | No | No |
| Packaging | `bin/vepeen.exe` | No `.app` recipe | None |

### macOS — what works / what's missing

**Works:** build + Fyne UI + tray; connect/disconnect **existing** Network profiles (`scutil --nc`, `networksetup`); post-dial split routes (admin `osascript`); Keychain secrets + AES-GCM `vepeen.bin`.

**Missing:**

- No L2TP/PSK profile creation (set System Settings first)
- Username/password fields ignored at dial
- Single-instance, show-signal, desktop shortcut unsupported
- Traffic counters / active connections unavailable
- Log path still Windows-oriented (`%AppData%` fallback)
- No `.app` packaging; not Windows feature-parity

### Linux — what's missing

- No `internal/vpn/linux` (or any `*linux*` backend) — stubs only
- VPN + routes: unsupported
- Config encrypt fails; secrets in-memory only
- UI: basic window, no tray
- Compile-only, not a usable VPN client

### Needed for real Linux

- `internal/vpn/linux` — nmcli/NetworkManager or strongSwan
- `internal/route` Linux — `ip route` / NM
- Secrets + encrypted config (Secret Service / keyring)
- Tray, correct log path, deps docs (CGo, OpenGL, NM L2TP plugins)

## Security notes & residual risks

- L2TP/IPsec + PSK is weaker than cert IPsec or WireGuard — product scope accepts that. PSK is configured on the OS profile, not in this UI.
- **`rasdial` argv password:** optional password is process arguments (OS limitation). Vepeen does not log it; local tools might still observe argv.
- **Temp scripts:** `PurgeOrphanScripts` cleans leftover `%TEMP%\vepeen\vpn-*.ps1` on start. PSK-in-script path is dormant (no profile create / no PSK UI).
- DPAPI is user-bound; not a multi-user portable secret store.
- Do not screenshot expecting secrets — UI is designed not to show them.

## Manual test checklist (Windows)

1. `go test ./internal/route ./internal/vpn ./internal/config`
2. `.\build.ps1` → `bin\vepeen.exe` runs (no console).
3. English UI; accent teal; no IP/PSK form fields.
4. Create a Windows VPN profile outside the app; it appears in the profile list.
5. Save settings → `vepeen.bin` next to exe is **not** plaintext.
6. Credentials survive restart when **Remember credentials** is on.
7. Split mode: destinations only via VPN; status **Connected** + local IP when assigned.
8. **Route All Traffic**: empty routes allowed; split tunneling disabled on profile.
9. Domain line resolves at connect (or clear error if NXDOMAIN / no IPv4).
10. Invalid line → **Line N is invalid** / **Cannot connect**.
11. Connect while already connected → **Connected** (not error).
12. **Cancel** during connect; **Disconnect**; quit shows disconnect dialog.
13. Second instance focuses the first; X hides to tray.

Remove a profile manually if needed:

```powershell
rasdial "YourProfile" /DISCONNECT
Remove-VpnConnection -Name "YourProfile" -Force
```

## Troubleshooting

| Symptom | What to try |
| ------- | ----------- |
| `cgo: C compiler "gcc" not found` | Install MSYS2 MinGW-w64; add `mingw64\bin` to `PATH` |
| gopls excludes `go-gl` / wrong GOOS | IDE noise. Workspace sets `CGO_ENABLED=1`, `GOOS=windows`. Real check: `.\build.ps1` |
| No profiles in dropdown | Create a Windows VPN connection first |
| Cannot connect / auth failure | Username/password; if gateway needs PSK, set it on the **OS profile**. Check UDP **500/4500** |
| Line N is invalid | IPv4, `x.x.x.x/nn`, or domain; no IPv6 |
| Domain resolve error | DNS must return IPv4 at connect time |
| Split routes ignored / all traffic via VPN | Uncheck **Route All Traffic**; reconnect; check Log warnings; `Get-VpnConnection` / `Get-NetRoute` |
| Route All Traffic not applied | Profile may reject split-tunnel toggle; try elevated once for profile property change |
| NAT-T warning | Run elevated once so registry can be set; reboot may be required |
| Secrets empty after restart | `vepeen.bin` decrypt failed (other Windows user / password reset); re-enter and **Save Settings** |
| GUI exits with no window | Read `%AppData%\vepeen\vepeen.log` |
| Second window blank | Single-instance should focus the first; kill stray processes if mutex stuck |

## Non-goals

WireGuard / IKEv2 / OpenVPN stacks, certificate IPsec UI, kill switch, multi-profile manager (beyond selecting one existing OS profile), CI pipelines, installers/MSI, shipping a custom IPsec driver.

## Further reading

- `docs/planning/prd-002-l2tp-split-tunnel.md`
- `docs/planning/prd-003-global-routes.md`
- `docs/planning/prd-004-encrypted-config.md`
- `docs/planning/prd-005-local-ip-status.md`
- `docs/planning/prd-006-route-all-traffic.md`
- `docs/planning/prd-vpn-win-package.md`
- `docs/research/windows-l2tp-ipsec-split-tunnel.md`

## License

Use and extend as needed for your project.