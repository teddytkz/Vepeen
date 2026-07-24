# Vepeen

Windows desktop **L2TP/IPsec VPN client** (Go + Fyne v2) with **username/password** authentication and **selective IP/CIDR routing** (split tunnel only). The app orchestrates the built-in Windows VPN stack (`VpnClient` PowerShell + `rasdial`) instead of shipping a custom IPsec driver.

**Last updated:** 2026-07-24

> **Current status (2026-07-23):** PSK (pre-shared key) support is **reserved but not yet wired into the UI**. The encrypted store (`vepeen.bin`) reserves a `psk` field for future use, but the current build collects and stores only username/password. See [Dual authentication](#dual-authentication-important).

## UI / theme

The desktop UI uses a **custom dark theme** with a teal accent (`#0FB5AE`) defined in `internal/ui/theme.go` (wraps Fyne's built-in `DarkTheme` and overrides the primary color). The main window is a balanced two-column grid that fills the full window height — the **Rute** (left) and **Log** (right) cards expand to remove dead space at the bottom.

## What it does

1. Collects gateway **IP/host**, PPP **username/password** (MS-CHAPv2), and a single **global** multi-line **IPv4 IP/CIDR** route list shared by all profiles (connection name defaults to `Vepeen`). PSK is **not** collected by the current UI (the encrypted store reserves a `psk` field for future use).
2. Creates/updates a **per-user** Windows VPN profile: L2TP + MS-CHAPv2 + **SplitTunneling**. (PSK is not applied by the current build — the profile is created without a pre-shared key; set it on the Windows profile separately if the gateway requires one.)
3. Syncs profile routes with `Add-VpnConnectionRoute` / `Remove-VpnConnectionRoute` so **only listed destinations** use the tunnel (no full-tunnel default route).
4. Connects/disconnects with `rasdial`.
5. Persists **all** settings and credentials into a single encrypted file `vepeen.bin` next to the executable (DPAPI user-scope).
6. Stores **password** (and the reserved `psk` slot) inside `vepeen.bin` (DPAPI-encrypted, never plaintext) — no Windows Credential Manager dependency at runtime. The `psk` field is currently always empty; only username/password are collected.

## Dual authentication (important)

Typical L2TP/IPsec servers need **both**:

| Layer | Secret | Role |
| ----- | ------ | ---- |
| IPsec (IKE) | Pre-shared key (PSK) | Authenticates the tunnel to the gateway |
| L2TP / PPP | Username + password (MS-CHAPv2) | Authenticates the user session |

> **Note:** PSK entry is **not yet implemented in the UI**. The current build collects only username/password; the encrypted store reserves a `psk` field for future use, but it is never populated or applied to the Windows profile. To use a PSK-protected gateway today, configure the PSK on the Windows VPN profile outside Vepeen (e.g. via `Set-VpnConnectionIPsecConfiguration`).

Wrong PSK often surfaces as a generic network/auth failure — check **both** layers when connect fails.

## Split tunnel behavior

- The profile is always created with **`-SplitTunneling`** (no full-tunnel default route via VPN).
- Only destinations in **Rute (split tunnel)** are attached to the profile.
- **IPv4 only** in v1 (IPv6 is rejected).
- Split tunnel changes require a **disconnect/reconnect** to take effect. After connect, the app **enforces** split tunnel by removing any server-pushed `0.0.0.0/0` default route on the VPN interface, so only the listed destinations use the tunnel.

### IP/CIDR list format

| Input | Result |
| ----- | ------ |
| `10.10.0.0/16` | Kept as that prefix |
| `203.0.113.50` | Treated as `/32` |
| Blank lines | Ignored |
| Lines starting with `#` | Comments (ignored) |
| IPv6 / garbage | Rejected with **Baris N tidak valid** |

Example:

```text
# office LAN
10.10.0.0/16
203.0.113.50
```

## Prerequisites (Windows)

| Requirement | Notes |
| ----------- | ----- |
| **Windows 10/11** | `VpnClient` module + `rasdial.exe` |
| **Go 1.22+** | Module targets `1.22.0` |
| **C compiler (CGo)** | Fyne needs GCC on `PATH` (MSYS2 MinGW-w64 recommended) |
| **CGO enabled** | `go env CGO_ENABLED` → `1` |

Optional checks:

```powershell
go version
go env CGO_ENABLED
gcc --version
Get-Command Add-VpnConnection, rasdial
```

## Setup / run / build

```powershell
go mod tidy
go run ./cmd/vepeen
# or build the GUI .exe (no console window):
.\build.ps1
.\vepeen.exe
```

`build.ps1` builds with `CGO_ENABLED=1` and the linker flag `-H windowsgui`, which selects the **Windows GUI subsystem** instead of the console subsystem — so launching `vepeen.exe` from Explorer does **not** open a `cmd.exe` console window. The plain equivalent is:

```powershell
$env:CGO_ENABLED = "1"
go build -ldflags="-H windowsgui" -o vepeen.exe ./cmd/vepeen
```

> To keep a console for debugging, drop the `-ldflags="-H windowsgui"` part: `go build -o vepeen.exe ./cmd/vepeen`.

### Diagnosing hidden crashes

Because `-H windowsgui` hides `stdout`/`stderr`, runtime panics and startup errors are not visible in a terminal. The entrypoint logs them to a file instead:

```
%AppData%\vepeen\vepeen.log
```

Check that file when the GUI `.exe` fails to start or closes unexpectedly.

Unit tests (parser + error mapping):

```powershell
go test ./internal/route ./internal/vpn
```

## Using the app

Indonesian labels; technical terms (L2TP, PSK, CIDR) stay in English. Primary path: **IP → Username → Password → Hubungkan**, with a timestamped **Log** of connect phases and errors. (PSK/Key is not collected by the current UI — see Dual authentication.)

| Control | Purpose |
| ------- | ------- |
| **IP** | Host or IP of the L2TP gateway |
| **Key (PSK)** | _Reserved — not collected by the current UI_ (IPsec secret slot reserved in `vepeen.bin`; not yet wired) |
| **Username** / **Password** | PPP / MS-CHAPv2 (password masked) |
| **Rute (split tunnel)** | Single **global** split-tunnel route list (required for connect); shared by every profile |
| **Simpan** | Save non-secrets + secrets (async; UI stays responsive) |
| **Hubungkan** | Ensure profile → sync routes → `rasdial` |
| **Putuskan** | `rasdial <Name> /DISCONNECT` |
| **Status** | Current state + short detail (never shows PSK/password) |
| **Log** | Read-only activity history (`HH:MM:SS` lines); **Bersihkan log** clears it |
| **Bersihkan log** | Clears the log buffer (always available) |

Connection name is **hidden** (default Windows profile `Vepeen`, or the name already stored in `vepeen.bin`).

The top **Apps** menu (Fyne main menu bar) contains **Create Desktop Shortcut**, which writes a `Vepeen.lnk` on the current user's Desktop pointing at the running executable (skips silently if it already exists; result is reported in the Log).

### Typical flow

1. Fill **IP**, **Username**, **Password** (PSK is not collected by the current UI — see Dual authentication).
2. Enter at least one valid IPv4 IP/CIDR under **Rute (split tunnel)** — routes are still required for connect and apply to whichever profile is selected.
3. **Simpan** (optional) — everything (settings + password) → encrypted `vepeen.bin`. Connect also persists quietly. (The reserved `psk` slot is stored but always empty in the current build.)
4. **Hubungkan** — profile ensure → route sync → `rasdial`; watch **Log** for phases.
5. **Putuskan** when finished.

**Hubungkan otomatis memutus VPN Windows lain yang aktif sebelum menyambung.** Tombol **Batal** muncul saat menghubungkan untuk membatalkan.

### Status & log behavior

| Situation | UI |
| --------- | -- |
| Idle / after load | **Terputus**; log notes load result |
| Connect in progress | **Menghubungkan…** (form locked); log appends each phase |
| Connected | **Terhubung**; **Putuskan** enabled; log success line |
| OS already connected (e.g. rasdial “already connected”) | Treated as **Terhubung** (not an error); **Putuskan** enabled |
| Disconnect in progress | **Memutuskan…** |
| Validation / OS failure | Error status + sanitized log line |

On startup the app also queries OS status for the named profile and shows **Terhubung** if a session is already up.

**Log retention:** in-memory for the current app session only (not written to disk). Cap is ~300 lines; oldest lines are dropped. **Bersihkan log** clears the buffer. Status and log never include PSK, password, or full command lines with credentials.

## Config & secrets

All settings and credentials live in a single encrypted file, `vepeen.bin`, located **next to `vepeen.exe`** (with a `%AppData%\vepeen\vepeen.bin` fallback if the executable path cannot be resolved). There is no plaintext `config.json` and no Windows Credential Manager dependency at runtime.

| Data | Location |
| ---- | -------- |
| Settings + credentials (name, server, username, routes, password; `psk` reserved) | `vepeen.bin` (DPAPI-encrypted blob, next to the executable) |

**Policy:**

- `vepeen.bin` is an **opaque DPAPI-encrypted blob** — not plaintext JSON. It is encrypted with the **current Windows user account** (user-scope, no passphrase prompt).
- Password (and the reserved `psk` slot) are **never** written in plaintext; they are encrypted inside `vepeen.bin`. The `psk` field is currently always empty because the UI does not collect it.
- Deleting `vepeen.bin` resets the app to defaults (no CredMan dependency remains at runtime; `internal/secrets` is now migration-only).

### Storage

- **Location:** `vepeen.bin` next to `vepeen.exe` (or `%AppData%\vepeen\vepeen.bin` fallback).
- **Format:** DPAPI-encrypted opaque blob (JSON payload, then `CryptProtectData`). Not human-readable; not editable by hand.
- **Contents:** `selectedProfile`, global `routes`, `rememberCredentials`, and per-profile credentials (`username` / `password`; the `psk` field is reserved for future use).
- **Non-roaming caveat:** encryption is bound to the **current Windows user account** (user-scope, equivalent to the prior Credential Manager scope). If the user **resets their Windows password** or **switches Windows accounts**, the blob **cannot be decrypted** and the app falls back to defaults — the same limitation as before.

### Migration (upgrading users)

On **first launch** after upgrading, the app automatically migrates existing data into `vepeen.bin` and then removes the old sources:

- Legacy `config.json` (both the executable-adjacent copy and `%AppData%\vepeen\config.json`).
- Windows Credential Manager entries (`vepeen/<name>/username`, `vepeen/<name>/password`).

Migration is **one-time and idempotent** — once `vepeen.bin` exists, the old sources are never read again. If migration fails, the old sources are left in place (safe fallback, no data loss).

> **Note:** `config.json` and Windows Credential Manager are now **legacy migration sources only** — they are read once, then deleted. They are not used at runtime.

### Config shape (inside `vepeen.bin`)

Routes are **global** — one shared split-tunnel list that applies to whichever Windows VPN profile is selected. There is no per-profile `routes` anymore; switching the profile dropdown does **not** change the routes.

| Field | Type | Description |
| ----- | ---- | ----------- |
| `selectedProfile` | string | Last-selected Windows VPN profile name |
| `routes` | string[] | Global split-tunnel destinations (IPv4 IP/CIDR or hostnames); required for connect |
| `rememberCredentials` | bool | Reuse stored password from `vepeen.bin` on load (PSK is not collected) |
| `credentials` | map | Per-profile `username` / `password` (and reserved `psk`) |

**Migration from the old per-profile shape:** on first load, an old `config.json` that stored a `profiles` map is migrated automatically. The selected profile's routes become the global `routes` list; if no `selectedProfile` matches, the **union** of all profiles' routes is used. Non-selected profiles' unique routes are dropped by design — the global model keeps a single shared list. After the next save/connect the data is written to `vepeen.bin` without the `profiles` key.

> **User-facing implication:** because routes are now global, a route that was previously tied to one profile (e.g. `git-rbi.xxx.xxx`) now applies to **all** selected profiles, not just `XXX - xxx.xxx.xxx.xxx`.

## Privileges

| Operation | Typical privilege |
| --------- | ----------------- |
| Per-user profile + routes (no `-AllUserConnection`) | **Preferred** — standard user often works |
| Some profile writes | May prompt for elevation / fail with access denied |
| Day-to-day `rasdial` connect | Standard user for per-user profile |

Prefer **per-user** profiles over all-user. If profile setup fails with access denied, try once elevated, then continue as a normal user.

## Network requirements

Outbound must allow:

- **UDP 500** (IKE)
- **UDP 4500** (NAT-T)
- ESP as required by the path

Corporate firewalls and some CGNAT setups break L2TP/IPsec; that is outside app control.

## Project layout

```
vepeen/
├── cmd/vepeen/main.go           # Thin entrypoint
├── internal/ui/                 # Fyne form + status
├── internal/config/             # Encrypted single-file store (vepeen.bin, DPAPI)
├── internal/secrets/            # Legacy Credential Manager read (migration-only)
├── internal/route/              # CIDR parse + route sync
├── internal/vpn/                # Profile, rasdial, status, temp scripts
├── docs/planning/               # PRD + UI design + polish notes
├── docs/research/               # Stack evaluation
├── FyneApp.toml                 # Fyne metadata + fyne.Do migration opt-in
├── go.mod
└── README.md
```

Windows-specific orchestration uses `//go:build windows`. Non-Windows builds get stubs that return a clear error.

## Security notes & residual risks

- L2TP/IPsec with PSK is weaker than certificate IPsec or modern WireGuard — acceptable for this product scope. When PSK support is wired in, treat PSK as sensitive. The current build does not collect or store a PSK.
- **Brief temp script (reserved):** When PSK support is added, profile ensure may write the PSK into a short-lived PowerShell script under `%TEMP%\vepeen\vpn-*.ps1` with a restricted ACL, then delete it. In the current build no PSK is written, so this path is dormant. Residual risk (once active): local process/file inspection during that window.
- **Orphan purge:** On app start and before writing a new script, leftover `vpn-*.ps1` files under `%TEMP%\vepeen\` are best-effort deleted (crash/kill mid-ensure can leave orphans).
- **`rasdial` argv password:** Username/password are passed as process arguments (OS limitation). Vepeen does not log them; other local tools might still observe argv.
- Do not screenshot status expecting secrets — the UI is designed not to show them.

## Manual test checklist (Windows)

1. `go test ./internal/route ./internal/vpn` — CIDR parser + error mapping.
2. `go build -o vepeen.exe ./cmd/vepeen` — binary builds.
3. Launch app; confirm Indonesian labels and default name `Vepeen`.
4. Save with sample server/routes; confirm `vepeen.bin` exists next to `vepeen.exe` and is **not** plaintext (no readable `password`/`psk` fields in a text editor).
5. Confirm credentials survive a restart (read back from the encrypted `vepeen.bin`, not Credential Manager).
6. Against a real L2TP/IPsec server: Hubungkan → status **Terhubung**; only listed prefixes should route via VPN (`Get-VpnConnection`, `route print`).
7. Hubungkan again while already connected → still **Terhubung** (not error); **Putuskan** enabled.
8. Putuskan → **Terputus**.
9. Invalid CIDR line → status shows **Baris N tidak valid** without connecting.
10. Kill the app mid-connect once, restart → no leftover `vpn-*.ps1` under `%TEMP%\vepeen\` (or they clear on next start/ensure).

Remove profile manually if needed:

```powershell
rasdial Vepeen /DISCONNECT
Remove-VpnConnection -Name Vepeen -Force
```

## Troubleshooting

| Symptom | What to try |
| ------- | ----------- |
| Fyne `fyne.Do` migration warning on launch | Resolved by root `FyneApp.toml` `[Migrations] fyneDo = true` (app already marshals UI updates with `fyne.Do`) |
| `cgo: C compiler "gcc" not found` | Install MSYS2 MinGW-w64; add `mingw64\bin` to `PATH` |
| gopls / IDE: `build constraints exclude all Go files` in `go-gl` with `[darwin]` (or similar) | **IDE analysis noise**, not a failed Windows build. gopls is analyzing with wrong `GOOS` and/or `CGO_ENABLED=0`. Workspace `.vscode/settings.json` sets `go.toolsEnvVars` (`CGO_ENABLED=1`, `GOOS=windows`, `GOARCH=amd64`). Verify real env: `go env CGO_ENABLED GOOS` → `1` and `windows`. Real check: `go build -o vepeen.exe ./cmd/vepeen` (needs `gcc` on `PATH`). After a form/layout fix, rebuild and run the new binary — the IDE error alone does not mean the form is broken. |
| Gagal menyiapkan profil / access denied | Prefer per-user profile; try elevated once for create; avoid all-user |
| Gagal autentikasi | Check username/password. If the gateway requires a PSK, configure it on the Windows profile outside Vepeen (PSK entry is not yet in the UI). PSK errors often look like auth/network failures. |
| Gagal terhubung / timeout | Server reachability; **UDP 500/4500**; firewall/NAT |
| Gagal menyelaraskan rute | Profile must exist; routes are read via `Get-VpnConnection` (`.Routes`) |
| Baris N tidak valid | Fix that line: IPv4 or `x.x.x.x/nn` only; remove IPv6 |
| All traffic goes through VPN (not just listed IPs) | Ensure SplitTunneling is on (app enforces it); the app removes a pushed `0.0.0.0/0` route after connect. Check the Log 'Diagnostik' line. If still full-tunnel, the server may force it — verify with `Get-NetRoute -InterfaceAlias Vepeen`. |
| Secrets empty after restart | `vepeen.bin` write/decrypt failed (or blob bound to a different Windows user); re-enter and **Simpan** |
| Status error when already online | Should not happen after polish — already-connected maps to **Terhubung**; if stuck, Putuskan then Hubungkan |

## Non-goals (v1)

Full tunnel, IKEv2/WireGuard/OpenVPN, cert-based IPsec UI, macOS/Linux clients, kill switch, multi-profile manager, CI/Docker/installers.

## License

Use and extend as needed for your project.