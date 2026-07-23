# UI-003: Simplified main form + activity log

**Version:** v1.1.0  
**Status:** Ready for Backend  
**Author:** Planner Agent → Designer (UI/UX)  
**Created:** 2026-07-22  
**Updated:** 2026-07-22  
**Related:** PRD-002 (`prd-002-l2tp-split-tunnel.md`), UI-002 (`ui-002-l2tp-form.md`), fix-001 (`fix-001-prd002-polish.md`)  
**Type:** UI simplification + log surface (addendum to PRD-002; no backend VPN rewrite)  
**Stack:** Go + Fyne v2 only (default theme; no custom widgets)  
**Supersedes (UI chrome only):** UI-002 field order/labels and window height — VPN behavior, enablement matrix, and secret rules from UI-002 / PRD-002 still apply unless this doc overrides.

---

## Overview

Simplify the main window so the primary path matches the user’s mental model: **IP, Key, Username, Password, Connect**, plus a **log text area** for connection history. Keep the existing L2TP/IPsec split-tunnel stack (`internal/vpn`, `route`, `config`, `secrets`) unchanged in behavior.

## Problem Statement

The current form (UI-002) is complete but dense: Nama koneksi, Server, PSK, Username, Password, Daftar IP/CIDR, Simpan, Putuskan, Hubungkan, and short status labels only. Users asked for a minimal connect surface and a place to **see logs**. Status labels alone do not retain phase/error history.

## Goals

- Primary visible inputs: **IP**, **Key**, **Username**, **Password**
- Primary action: **Connect** (Indonesian: **Hubungkan** or **Connect** — Designer picks; keep Indonesian product voice unless design says bilingual)
- Multi-line **log area** that appends timestamped, sanitized lines for validation, connect phases, success, and errors
- Preserve split-tunnel: routes still required for connect (de-emphasized, not removed)
- No secret leakage in log/status (PSK, password, full credential command lines)
- Backend VPN/route/secrets APIs remain the source of truth

## Non-Goals

- Full-tunnel mode or removing route requirement
- New VPN protocols, multi-profile UI, system tray
- File-based debug logging or remote telemetry
- Redesign of CredMan / PowerShell / rasdial pipeline
- Dark custom theme or non-Fyne widgets

---

## Feature Specification

### User Stories

- As a user, I want to enter **IP, key, username, password** and press **Connect**, so that I can dial without hunting through a long form.
- As a user, I want a **log text area**, so that I can see what happened during connect (phases and errors) without guessing from a one-line status.
- As a user, I still want selective routes, so that only listed IPs/CIDRs use the VPN (product remains split tunnel).
- As a security-conscious user, I never want PSK/password values to appear in the log.

### Acceptance Criteria

- [ ] Main form emphasizes four fields: IP (server), Key (PSK, masked), Username, Password (masked)
- [ ] Primary button is Connect / Hubungkan; Disconnect / Putuskan remains available when connected (secondary visual weight)
- [ ] Read-only multi-line log area is visible and scrolls; new events append with timestamps
- [ ] Log receives at least: load/save notes, validation failures, connect start, each `vpn.Phase` detail, connect success, connect/disconnect errors, disconnect success
- [ ] Log and status **never** contain PSK, password, or raw process argv with secrets
- [ ] Routes field remains usable and **still required** for connect (≥1 valid IPv4/CIDR); layout de-emphasizes it (secondary section / smaller help)
- [ ] Connection name defaults to `Vepeen` (config.DefaultConnectionName); not a primary field — compact optional field **or** hidden with default only (Designer chooses; Backend must still pass name to Manager)
- [ ] Save remains available as secondary **or** relies on existing quiet persist-on-connect; if Save button removed, connect path must still persist config/secrets (already `persistQuiet`)
- [ ] Short status labels may remain (current state) **in addition to** log history
- [ ] Busy/enablement rules from UI-002/fix-001 preserved (no double-connect; form locked while connecting)
- [ ] Window still usable at ~min size; log has a sensible minimum height
- [ ] README UI section updated to match simplified labels + log
- [ ] Security spot-check on log redaction; Debugger/Reviewer verifies acceptance criteria

---

## Technical Design

### Architecture Overview

```
┌─────────────────────────────────────────┐
│ Header (title + short subtitle)         │
├─────────────────────────────────────────┤
│ Primary form                            │
│   IP / Server                           │
│   Key (PSK)                             │
│   Username                              │
│   Password                              │
│ Secondary (de-emphasized)               │
│   Routes multi-line (required)          │
│   Optional: Nama koneksi (compact)      │
├─────────────────────────────────────────┤
│ [Simpan?]     [Putuskan] [Hubungkan]    │
│ Status (short primary + detail)         │
│ Log (MultiLineEntry read-only, scroll)  │
└─────────────────────────────────────────┘
```

No new packages required. Prefer extending `internal/ui/main_window.go`; optional small split (`log_view.go`) only if file becomes unwieldy.

### Codebase Context

| Area | Current fact |
| ---- | ------------ |
| UI | Single controller in `internal/ui/main_window.go` |
| Labels today | Nama koneksi, Server, PSK, Username, Password, Daftar IP/CIDR |
| Actions | Simpan pengaturan, Putuskan, Hubungkan |
| Feedback | `statusPri` + `statusDet` only — **no log widget** |
| Connect | `validateConnect` → `ConnectFull` + phase callback `vpn.PhaseDetail` |
| Persist on connect | `persistQuiet(req)` already |
| Secrets | Password entries; CredMan via `secrets.Store` |
| Sanitize | `sanitizeUIErr`, `formatVPNError`, `vpn.UserError` / `MapExecError` |

### Field mapping (product language → code)

| User-facing | Widget / config | Notes |
| ----------- | --------------- | ----- |
| IP (or IP / Server) | `serverEntry` → `ServerAddress` | Same as today’s Server |
| Key | `pskEntry` → secrets KindPSK | Masked; never log value |
| Username | `userEntry` | Trim for empty-check |
| Password | `passEntry` | Masked; never log value |
| Routes (secondary) | `routesEntry` → `Routes` / `RoutesText` | Still required on connect |
| Nama koneksi | default `Vepeen` | Compact or hidden |
| Connect | `btnConn` → `onConnect` | High importance |
| Disconnect | `btnDisc` → `onDisconnect` | Secondary |
| Log | **new** multi-line entry | Append-only UI buffer |

### Log behavior (implementation contract)

**Widget:** `widget.NewMultiLineEntry()` (or Entry with multi-line), `Disable()` / read-only pattern so user cannot edit history. Prefer disable over hiding selection if copy-paste is desired — Designer/Backend: if Fyne allows selectable disabled text, prefer selectable; otherwise disabled is OK for v1.

**API (suggested on controller):**

```text
appendLog(line string)          // timestamp + line; UI thread only
appendLogf(format string, ...)  // convenience
```

**Timestamp:** local time, short form e.g. `15:04:05` (or `2006-01-02 15:04:05` if Designer wants date). Prefix each line.

**Events to log (minimum):**

| Event | Example line (ID) |
| ----- | ----------------- |
| App load OK | `Pengaturan dimuat.` |
| Load fail | `Gagal memuat pengaturan; memakai default.` |
| Validation fail | `Validasi: …` (field-level message, no secrets) |
| Connect start | `Menghubungkan ke <server>…` (server host/IP OK; never PSK/password) |
| Phase updates | Reuse `vpn.PhaseDetail(phase)` text |
| Connect OK | `Terhubung. Split tunnel aktif.` |
| Connect err | Primary + detail from `formatVPNError` / `UserError` |
| Already connected | Same success-like copy as fix-001 |
| Disconnect start/OK/err | Matching Indonesian short lines |
| Save OK/err | Existing save messages |

**Redaction rules (mandatory):**

1. Never append `pskEntry.Text`, `passEntry.Text`, or raw secret store values.
2. Never append full PowerShell/`rasdial` command lines.
3. Prefer existing sanitizers (`sanitizeUIErr`, `vpn` user errors). If logging `err.Error()`, only after sanitize path.
4. Server address and connection name **may** appear in log (non-secret).
5. Cap log length (e.g. last ~200–500 lines or ~64–128 KiB of text): drop oldest lines to avoid unbounded memory. Exact cap Backend chooses; document in code comment.

**Threading:** All widget updates via `fyne.Do` from background workers (same as status today). `setStatus` should also call `appendLog` for primary/detail (or callers log explicitly — pick one pattern and stick to it to avoid double lines).

**Recommended pattern:** `setStatus` updates labels only; explicit `appendLog` at event sites **or** `setStatus` appends one combined line. Avoid duplicate spam on every phase if both status detail and log would repeat — phases: update status detail **and** append one log line per phase.

### UI Changes

**Resolved by Designer — see § Design Spec (UI-003) below.** Summary of decisions:

| Topic | Decision |
| ----- | -------- |
| Primary labels | `IP`, `Key (PSK)`, `Username`, `Password` |
| Connect / Disconnect | `Hubungkan` (HighImportance) / `Putuskan` (default) |
| Save | Keep `Simpan` (short label), secondary left |
| Nama koneksi | **Hidden** — always `config.DefaultConnectionName` (`Vepeen`) unless loaded config has another name (still not shown as primary field; optional compact only if Backend needs multi-name later — **v1: hidden**) |
| Routes | Secondary section title `Rute (split tunnel)` + multi-line; still required |
| Log | Footer under status; title `Log`; read-only multi-line; optional `Bersihkan log` |
| Window | Default **480×720**, min **420×600** |

---

## Design Spec (UI-003) — Designer deliverable

**Type:** Modified Screen (main window simplification + log)  
**Widgets:** Fyne standard only (`widget.Form`, `Entry`, `PasswordEntry`, `MultiLineEntry`, `Button`, `Label`, `Separator`, `container.Border` / `VBox` / `HBox` / `VScroll`)  
**Theme:** Default Fyne (light/dark follows OS) — no custom palette, icons, or animation  
**Reference implementation today:** `internal/ui/main_window.go` (UI-002 layout)

---

### 1. Design intent

**Job of this screen:** Enter four credentials, confirm selective routes, connect, and **read what happened**.

**Hierarchy (top → bottom):**

1. Identity (compact header)
2. Primary credentials (IP / Key / Username / Password) — dominant
3. Secondary routes (required but quieter)
4. Actions (Connect primary)
5. Live status (one-line state)
6. Activity log (history; tallest flexible region)

**Aesthetic risk (deliberate, Fyne-safe):** Treat the **log as a console strip** under a thin separator — monospaced if Fyne theme allows via default entry font; otherwise plain multi-line is fine. Do **not** add fake terminal chrome, neon colors, or custom canvases. The “console” feel comes from **timestamped append-only lines + fixed min rows**, not decoration.

---

### 2. Window shell

| Property | Spec |
| -------- | ---- |
| Window title | `Vepeen` |
| Default size | **480 × 720** (taller than UI-002 480×640 to fit log) |
| Minimum size | **420 × 600** via existing `minSizeWrap` (or equivalent) |
| Maximum size | None |
| Position | `CenterOnScreen()` |
| Resizable | Yes |
| Theme | Fyne default only |

**Resize behavior:**

- Form fields expand horizontally.
- **Log** is the primary vertical flex consumer (grows when window grows).
- Header + primary form + actions + short status stay compact; routes keep a small fixed min rows (~3).
- On short displays, **scroll the form body only** if needed; keep actions + status + log chrome reachable (see layout).

---

### 3. Layout hierarchy (Fyne containers)

#### 3.1 Canonical tree

```
Window content
└── Padded (outer)
    └── Border
        ├── Top:    Header (VBox)
        ├── Center: Scroll → Form column (VBox)
        │             ├── Primary form (4 fields)
        │             ├── Separator (light)
        │             ├── Secondary: routes block
        │             └── (no buttons here)
        └── Bottom: Footer (VBox) — fixed chrome
                      ├── Actions row
                      ├── Status block (short)
                      ├── Separator
                      └── Log block (title + multi-line + optional clear)
```

**Why this split:**

- **Border Top/Center/Bottom** matches UI-002 pattern Backend already uses.
- Log lives in **Bottom** so history remains visible while user scrolls form fields on small heights.
- If Bottom becomes too tall on 600px min height: Backend may put **only log multi-line** in a nested `Border` with `SetMinRowsVisible` and let the window grow; do **not** put log inside the form scroll (users lose history while editing).

**Alternative acceptable (if footer too tall):**

```
Border
  Top: Header
  Center: Border
            Top: Scroll(form primary + routes)
            Bottom: Status (short only)
  Bottom: Actions + Log
```

Prefer **canonical** first; alternative only if min-size testing fails.

#### 3.2 Header (Top)

`VBox`:

1. **Title** — `Vepeen` — `TextStyle{Bold: true}`
2. **Subtitle** — `L2TP/IPsec · split tunnel` — wrapping on (shorter than UI-002; less dense)
3. `widget.NewSeparator()`

No connection-name field in header.

#### 3.3 Form body (Center, Scroll)

**A. Primary form** — `widget.NewForm` (or `FormLayout`) in this **exact order** (focus order):

| # | Label (UI) | Widget | Placeholder | Maps to |
| - | ---------- | ------ | ----------- | ------- |
| 1 | `IP` | `widget.Entry` | `vpn.contoh.com atau IP` | `serverEntry` / `ServerAddress` |
| 2 | `Key (PSK)` | `widget.PasswordEntry` | (empty) | `pskEntry` / CredMan PSK |
| 3 | `Username` | `widget.Entry` | `nama.pengguna` | `userEntry` |
| 4 | `Password` | `widget.PasswordEntry` | (empty) | `passEntry` / CredMan password |

**Do not** show long under-field essays for PSK/password in v1 (UI-002 optional hints removed to reduce density). Secrets policy remains in README, not form chrome.

**B. Secondary routes block** (below form, still in scroll):

```
Separator (optional)
Label bold: "Rute (split tunnel)"
Label muted/normal: "Wajib · satu IP/CIDR per baris. Hanya daftar ini lewat VPN."
MultiLineEntry  — SetMinRowsVisible(3)  // smaller than UI-002’s 5
Helper (wrap): "Contoh: 10.10.0.0/16 atau 203.0.113.50. Kosong diabaikan. # = komentar."
```

- Routes **remain required** for Connect (≥1 valid IPv4/CIDR).
- Visual de-emphasis: section title + shorter min rows + helper; **not** hidden, **not** accordion (no accordion in Fyne v1 scope).

**C. Connection name (hidden)**

- **Do not render** `Nama koneksi` entry in v1.
- Controller still uses `config.DefaultConnectionName` (`Vepeen`) for Manager / CredMan / config load-save.
- On load: if `config.json` has a non-default `ConnectionName`, **keep using it internally** (do not force-rename profiles) but still **do not show** a primary field. Optional later: compact read-only label `Profil: {name}` under subtitle — **not required for v1**.
- Backend must still pass `Name` into `ConnectRequest` / secrets keys.

#### 3.4 Footer (Bottom)

**A. Actions row** — `container.NewBorder` (same pattern as today):

```
[ Simpan ]                    [ Putuskan ] [ Hubungkan ]
```

| Button | Label | Importance | Role |
| ------ | ----- | ---------- | ---- |
| Save | `Simpan` | Default | Secondary; left. Short label (was `Simpan pengaturan`) to save width |
| Disconnect | `Putuskan` | Default | Secondary; right cluster |
| Connect | `Hubungkan` | **HighImportance** | Primary CTA; rightmost |

- Do **not** use DangerImportance on Putuskan.
- Do **not** merge into a single toggle button.
- Optional: no Save button only if product drops explicit save — **canonical v1: keep Simpan** (users expect it; async save already implemented).

**B. Status block** (keep short labels from UI-002):

```
Separator
Label bold: "Status"
statusPri  (wrapping)  — e.g. Terputus / Menghubungkan… / Terhubung / Gagal…
statusDet  (wrapping)  — one-line detail / phase
```

Status = **current state only**. History goes to Log.

**C. Log block**

```
Separator
HBox:  Label bold "Log"  |  stretch  |  optional Button "Bersihkan log" (low emphasis)
MultiLineEntry (read-only / disabled for edit)
```

| Property | Spec |
| -------- | ---- |
| Title | `Log` (not “Debug”; user-facing activity) |
| Widget | `widget.NewMultiLineEntry()` |
| Editable | **No** — `Disable()` after create **or** equivalent read-only; prefer still **selectable/copyable** if Fyne allows with disabled entry; if not, disabled is OK for v1 |
| Min rows | **`SetMinRowsVisible(8)`** target (6 minimum if space fights min window) |
| Wrapping | `fyne.TextWrapWord` or Off — prefer **Off** + horizontal scroll for long hostnames; word wrap acceptable |
| Scroll | Built-in entry scroll; auto-scroll to end on append |
| Clear | **Optional** button `Bersihkan log` — clears buffer + shows single line `Log dibersihkan.` with new timestamp; does **not** change VPN state. Importance: default/low. Place top-right of log header. **Canonical v1: include Clear** (cheap, high user value). |
| Empty state | On first paint, one placeholder line (see §5) |

---

### 4. Copy & labels (canonical)

#### 4.1 Chrome

| Element | Text |
| ------- | ---- |
| Window title | `Vepeen` |
| Header title | `Vepeen` |
| Header subtitle | `L2TP/IPsec · split tunnel` |
| Status title | `Status` |
| Log title | `Log` |
| Routes section title | `Rute (split tunnel)` |
| Routes duty line | `Wajib · satu IP/CIDR per baris. Hanya daftar ini lewat VPN.` |
| Routes helper | `Contoh: 10.10.0.0/16 atau 203.0.113.50. Kosong diabaikan. # = komentar.` |

#### 4.2 Fields

| Key | Label | Placeholder |
| --- | ----- | ----------- |
| server | `IP` | `vpn.contoh.com atau IP` |
| psk | `Key (PSK)` | — |
| username | `Username` | `nama.pengguna` |
| password | `Password` | — |
| routes | (section title above; form item label may be empty or `IP/CIDR`) | — |

If using `widget.Form` for routes as a fifth item, label = `IP/CIDR` and keep section title **or** skip form item label and use section title only — **prefer section title + bare multi-line** so routes feel secondary, not a 5th equal form row.

#### 4.3 Buttons

| Action | Label |
| ------ | ----- |
| Save | `Simpan` |
| Disconnect | `Putuskan` |
| Connect | `Hubungkan` |
| Clear log | `Bersihkan log` |

#### 4.4 Validation messages (update field names vs UI-002)

Reuse UI-002 catalog with renames:

| Condition | Message |
| --------- | ------- |
| Empty IP/server | `IP wajib diisi.` |
| Empty PSK | `Key (PSK) wajib diisi.` |
| Empty username | `Username wajib diisi.` |
| Empty password | `Password wajib diisi.` |
| No routes | `Isi minimal satu IP atau CIDR untuk split tunnel.` |
| Bad route line | same as UI-002 (`Baris {n} tidak valid…`) |
| Empty connection name (internal) | Should not happen if default applied; if empty after trim, set to `Vepeen` before connect |

Focus order on validation error: IP → Key → Username → Password → routes.

---

### 5. Log behavior (UX contract)

#### 5.1 Line format

```
HH:MM:SS  message
```

- Local time, 24h, **time only** (no date) for density — e.g. `15:04:05`.
- Single space or `  ` between timestamp and message.
- One event → one line (phases: one line per phase).

#### 5.2 Empty / first paint

```
--:--:--  Siap. Isi IP, Key, Username, Password, dan rute, lalu Hubungkan.
```

Or without fake timestamp:

```
(log kosong) Siap. Isi IP, Key, Username, Password, dan rute, lalu Hubungkan.
```

**Canonical:** use real timestamp at load for the first system line:

```
15:04:05  Siap. Isi IP, Key, Username, Password, dan rute, lalu Hubungkan.
```

Then append load result.

#### 5.3 What gets logged (minimum set)

| Event | Example log line (message part) | Also update status? |
| ----- | ------------------------------- | ------------------- |
| App ready / load OK | `Pengaturan dimuat.` | Yes — Terputus + detail |
| Load fail | `Gagal memuat pengaturan; memakai default.` | Yes |
| Validation fail | `Validasi: IP wajib diisi.` (prefix `Validasi: ` + message) | Yes — Tidak dapat menghubungkan |
| Connect start | `Menghubungkan ke {server}…` | Yes — Menghubungkan… |
| Phase EnsureProfile | `Menyiapkan profil VPN…` | statusDet only + log |
| Phase SyncRoutes | `Menyelaraskan rute (split tunnel)…` | statusDet + log |
| Phase Dial | `Menghubungi server…` | statusDet + log |
| Connect OK | `Terhubung. Split tunnel aktif.` | Yes |
| Already connected | `Sudah terhubung.` | Yes — Connected |
| Connect error | `{primary}: {detail}` sanitized | Yes — Error |
| Disconnect start | `Memutuskan…` | Yes |
| Disconnect OK | `Koneksi ditutup.` | Yes |
| Disconnect error | sanitized primary/detail | Yes |
| Save OK | `Pengaturan disimpan.` | status detail flash |
| Save fail | `Gagal menyimpan: {sanitized}` | Yes |
| Clear log | `Log dibersihkan.` | No status change |
| Secrets | **never** | — |

**Server host/IP and connection name may appear.** PSK, password, CredMan values, PowerShell/`rasdial` argv **must never** appear.

#### 5.4 Logging pattern (avoid double spam)

**Canonical pattern for Backend:**

1. `setStatus(state, primary, detail)` → updates labels **only**.
2. Call sites also `appendLog(message)` with a **single** clear sentence.
3. On phase callback: update `statusDet` **and** `appendLog(PhaseDetail(phase))` once per phase.
4. Do **not** make `setStatus` auto-append both primary and detail as two lines every time (too noisy). Prefer one log line per user-visible event.

#### 5.5 Buffer cap

- Keep last **~300 lines** or **~100 KiB**, whichever Backend prefers; drop oldest.
- Document constant in code comment.

#### 5.6 Threading

- All `appendLog` / entry text updates on UI thread (`fyne.Do` from workers).
- Same as status updates today.

#### 5.7 Redaction (mandatory)

1. Never log `pskEntry.Text`, `passEntry.Text`, or store.Get values.
2. Never log full command lines.
3. Errors only via existing `sanitizeUIErr` / `formatVPNError` / `vpn.UserError` / `MapExecError` paths.
4. In-memory only — **no disk log file** in this plan.

---

### 6. Enable / disable rules (reuse PRD-002 / UI-002)

Connection states: `Disconnected` | `Connecting` | `Connected` | `Disconnecting` | `Error`

Treat **Error** like disconnected for form editability.

| Control | Disconnected | Connecting | Connected | Disconnecting | Error |
| ------- | :----------: | :--------: | :-------: | :-----------: | :---: |
| IP | ✅ | ❌ | ❌ | ❌ | ✅ |
| Key (PSK) | ✅ | ❌ | ❌ | ❌ | ✅ |
| Username | ✅ | ❌ | ❌ | ❌ | ✅ |
| Password | ✅ | ❌ | ❌ | ❌ | ✅ |
| Routes | ✅ | ❌ | ❌ | ❌ | ✅ |
| Simpan | ✅ | ❌ | ✅ | ❌ | ✅ |
| Hubungkan | ✅ | ❌ | ❌ | ❌ | ✅ |
| Putuskan | ❌ | ❌* | ✅ | ❌ | ❌ |
| Bersihkan log | ✅ | ✅ | ✅ | ✅ | ✅ |
| Log entry | always non-editable | | | | |

\* v1: no cancel API → Putuskan **disabled** while Connecting (same as UI-002 canonical).

**Busy flag:** In-flight Connect / Disconnect / Save sets `busy`; blocks re-entry; `applyEnablement` after every state change.

**Clear log:** Always enabled (even when busy) — pure UI buffer; must not touch `busy` or VPN.

**Connected freeze:** Form fields disabled while connected; user must Putuskan to edit credentials/routes. Save remains enabled to flush current field values (unchanged while frozen).

---

### 7. Component states (summary)

| State | Hubungkan | Putuskan | Form | Status primary | Log activity |
| ----- | --------- | -------- | ---- | -------------- | ------------ |
| Disconnected | On, High | Off | Editable | `Terputus` | Idle / last history |
| Connecting | Off | Off | Locked | `Menghubungkan…` | Phase lines append |
| Connected | Off | On | Locked | `Terhubung` | Success line |
| Disconnecting | Off | Off | Locked | `Memutuskan…` | Disconnect lines |
| Error | On, High | Off | Editable | `Gagal…` / validation | Error line |

**Loading:** No skeleton; disable buttons + status text is enough (desktop form).

**Empty log:** Placeholder readiness line (§5.2).

**Error:** Status + log both show sanitized text; focus first invalid field on validation.

---

### 8. Accessibility & usability

- Password fields: Fyne password entries (masked). No show-password toggle v1.
- Focus order: IP → Key → Username → Password → Routes → Simpan → Putuskan → Hubungkan → (Clear log).
- Enter: **no** implicit disconnect; Connect via button click (same as UI-002 safest rule).
- Touch targets: default Fyne button height OK (desktop-first).
- Meaning not color-only: always text state (`Terhubung` / `Gagal`).
- Screenshot-safe: status + log free of secrets.
- Motion: none added; no custom animation.

---

### 9. Visual constraints (Fyne)

| Topic | Decision |
| ----- | -------- |
| Theme | Default only |
| Primary CTA | `Hubungkan` HighImportance |
| Density | Primary 4-field form first; routes quieter; log tall |
| Clutter | No PSK/password essay hints; short subtitle |
| Icons | None v1 |
| Colors | No hardcoded hex |
| Custom widgets | Forbidden |

---

### 10. Wireframe (ASCII)

```
┌──────────────────────────────────────────┐
│ Vepeen                                   │
│ L2TP/IPsec · split tunnel                │
│──────────────────────────────────────────│
│ IP            [ vpn.contoh.com atau IP ] │
│ Key (PSK)     [ •••••••••••••••••••••• ] │
│ Username      [ nama.pengguna          ] │
│ Password      [ •••••••••••••••••••••• ] │
│──────────────────────────────────────────│
│ Rute (split tunnel)                      │
│ Wajib · satu IP/CIDR per baris…          │
│ ┌──────────────────────────────────────┐ │
│ │ 10.10.0.0/16                         │ │
│ │ 203.0.113.50                         │ │
│ └──────────────────────────────────────┘ │
│ Contoh: 10.10.0.0/16 atau …              │
│          ▲ scroll form if needed         │
│──────────────────────────────────────────│
│ [Simpan]              [Putuskan][Hubungkan]│
│ Status                                   │
│ Terputus                                 │
│ Siap terhubung.                          │
│──────────────────────────────────────────│
│ Log                      [Bersihkan log] │
│ ┌──────────────────────────────────────┐ │
│ │ 15:04:05  Pengaturan dimuat.         │ │
│ │ 15:05:01  Menghubungkan ke 1.2.3.4…  │ │
│ │ 15:05:01  Menyiapkan profil VPN…     │ │
│ │ 15:05:02  Menyelaraskan rute…        │ │
│ │ 15:05:03  Menghubungi server…        │ │
│ │ 15:05:04  Terhubung. Split tunnel…   │ │
│ │                                      │ │
│ └──────────────────────────────────────┘ │
└──────────────────────────────────────────┘
   default ≈ 480×720     min ≈ 420×600
```

---

### 11. Frontend / Backend developer guidance

**Files:**

| Path | Action |
| ---- | ------ |
| `internal/ui/main_window.go` | Rebuild layout, labels, sizes, log widget, clear button; hide name field; keep controller wiring |
| `internal/ui/log.go` | Optional: `appendLog`, cap, timestamp helper |
| `README.md` | Field table + log note + routes still required |
| `internal/vpn/*`, `route/*`, `secrets/*`, `config/*` | **Do not change** unless tiny export needed (should not be) |

**Implementation checklist:**

1. Window resize default **480×720**, min wrap **420×600**.
2. Form order: IP, Key (PSK), Username, Password; routes secondary block.
3. Name: internal default `Vepeen`; no primary entry.
4. Actions: `Simpan` | stretch | `Putuskan` `Hubungkan` (HighImportance).
5. Keep `statusPri` / `statusDet`.
6. Add log multi-line + `appendLog` + cap; wire load/save/validate/phases/disconnect.
7. Optional `Bersihkan log`.
8. Preserve `applyEnablement`, async Save, `persistQuiet`, already-connected → Connected, validation focus, `PurgeOrphanScripts`.
9. Secret-safe log/status spot-check mentally at each `appendLog` call site.
10. Update README UI section.

**Sub-agent decomposition (single Backend session preferred):**

- Pass A: layout + labels + sizes + hide name  
- Pass B: log widget + append + event wiring  
- Pass C: enablement regression + README  

**Out of scope for Backend this plan:** VPN stack, CredMan schema, full-tunnel mode, disk logging, custom theme.

---

### 12. Acceptance mapping (Designer → AC)

| AC (plan) | Design coverage |
| --------- | --------------- |
| Four primary fields | §3.3 A labels IP / Key (PSK) / Username / Password |
| Connect primary, Disconnect secondary | §3.4 A |
| Read-only scrolling log + timestamps | §3.4 C, §5 |
| Log events minimum set | §5.3 |
| No secrets in log/status | §5.7 |
| Routes secondary but required | §3.3 B, §4.4, §6 |
| Name default Vepeen hidden | §3.3 C |
| Save secondary kept | §3.4 A |
| Short status kept | §3.4 B |
| Busy/enablement preserved | §6 |
| Taller window for log | §2 |

---

### Data Model / API Changes

**None** for config schema, secrets, or VPN manager public API.

Optional internal-only: if phase callback should always mirror to log, no VPN package change required — UI already receives phases.

### Security

- Log is an in-memory UI buffer only (not written to disk in this plan).
- Same secret rules as PRD-002 / UI-002 § secret-safe.
- Security agent: spot-check log call sites after Backend lands.

---

## Implementation Plan

### Phase 0: Design

**Depends on:** Nothing  
**Parallelizable:** N/A  
**Status:** ✅ Complete (2026-07-22) — see **Design Spec (UI-003)**  

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 0.1 | Designer | `docs/planning/ui-003-simplified-form-log.md` | Finalized layout hierarchy, labels, button weights, log chrome, window sizes, routes secondary, Save kept, name hidden. Status **Ready for Backend**. |

### Phase 1: UI implementation

**Depends on:** Phase 0  
**Parallelizable:** No — single primary file  

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/ui/main_window.go` | Rebuild form layout per Designer: primary IP/Key/User/Pass; secondary routes (+ optional compact name); actions; short status; **log multi-line**. Relabel Server→IP mapping; PSK→Key. Keep controller fields wired to same config/secrets/vpn types. |
| 1.2 | Backend Developer | `internal/ui/main_window.go` (+ optional `internal/ui/log.go`) | Implement `appendLog` with timestamp, max buffer, UI-thread safety; wire load/save/validate/connect phases/disconnect/error paths. Ensure `setStatus` + log do not leak secrets. |
| 1.3 | Backend Developer | `internal/ui/main_window.go` | Preserve enablement, async Save (if kept), `persistQuiet` on connect, already-connected → Connected, focus-on-validation, min-size wrapper. Adjust min/default size if Designer specified. |
| 1.4 | Backend Developer | `README.md` | Update UI field table and quick-start steps for simplified labels + log area; note routes still required; restate no secrets in log/status. |

**Sub-Agent Guidance:**

- Tasks 1.1–1.3 are one sequential Backend session on UI (avoid parallel edits to `main_window.go`).
- Task 1.4 can follow immediately in same session.
- Do **not** change `internal/vpn/*`, `route/*`, `secrets/*`, `config/*` unless a tiny export is required (should not be).

### Phase 2: Review & documentation

**Depends on:** Phase 1  

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 2.1 | Debugger/Reviewer | Verify acceptance criteria; manual checklist: connect phases appear in log; validation errors log; disconnect; already-connected; form lock while busy; routes still required |
| 2.2 | Security | Spot-check log/status paths for PSK/password leakage; confirm no new disk logging of secrets |
| 2.3 | Documentation | Confirm README matches shipped UI; update this plan Status → Done; changelog entry if not already complete |

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| User thinks routes optional after simplification | High (connect fails or wrong product expectation) | Med | Keep routes visible (secondary); validation message points to routes field; README states required |
| Log floods / memory growth | Low | Med | Cap lines/bytes; one line per phase |
| Double messages (status + log) | Low | Med | Single logging pattern; phases once each |
| Secret leak via `err.Error()` | High | Low | Only sanitized user errors; Security review |
| Hidden connection name breaks multi-profile later | Low | Low | Keep default constant; optional compact field |

## Rollback Strategy

Revert `internal/ui/main_window.go` (+ any new `internal/ui/log.go`) and README UI section to pre-UI-003. No config migration; no VPN profile format change. Users keep existing CredMan entries and `config.json`.

---

## Agent order (Orchestrator)

1. **Designer** — layout/labels/log chrome (Phase 0)  
2. **Backend Developer** — Fyne implementation + README (Phase 1)  
3. **Debugger/Reviewer** — acceptance (Phase 2.1)  
4. **Security** — log redaction spot-check (Phase 2.2)  
5. **Documentation** — close-out if README gaps remain (Phase 2.3)

---

## File list (expected touch set)

| Path | Role |
| ---- | ---- |
| `docs/planning/ui-003-simplified-form-log.md` | This plan / design target |
| `docs/planning/changelog.md` | Unreleased entry |
| `docs/planning/ui-002-l2tp-form.md` | Reference only (do not rewrite unless Designer notes supersession) |
| `internal/ui/main_window.go` | Primary implementation |
| `internal/ui/log.go` | Optional extract for log helper |
| `README.md` | UI section update |

**Do not modify (unless unexpected compile need):** `internal/vpn/*`, `internal/route/*`, `internal/secrets/*`, `internal/config/*`, `cmd/vepeen/main.go`

---

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v1.1.0 | 2026-07-22 | Designer: full Design Spec — layout, labels, log, sizes, enablement; Ready for Backend |
| v1.0.0 | 2026-07-22 | Initial plan: simplify primary fields + activity log; keep split-tunnel routes |
