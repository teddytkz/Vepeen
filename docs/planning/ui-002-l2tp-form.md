# UI-002: L2TP VPN Client — Main Window Design Spec

**Version:** v1.0.0  
**Status:** Ready for Backend  
**Author:** Designer (UI/UX) Agent  
**Created:** 2026-07-22  
**Related:** PRD-002 (`docs/planning/prd-002-l2tp-split-tunnel.md`), research `docs/research/windows-l2tp-ipsec-split-tunnel.md`  
**Stack:** Go + Fyne v2 (desktop, Windows primary)  
**Scope:** Design only — Backend Developer implements all Go/Fyne code

---

## 1. Purpose

Replace the demo click-counter UI in `internal/ui/main_window.go` with a single-window L2TP/IPsec split-tunnel connection form. Users enter server, dual auth (PSK + username/password), and selective routes (IP/CIDR list), then connect/disconnect with clear status.

**Product principles for this screen**

1. **Content first** — form fields and status dominate; no decorative chrome.
2. **Forgiving** — validation before side effects; clear recovery from errors.
3. **Busy-safe** — no double-connect; primary action reflects current state.
4. **Secret-safe** — never show PSK/password in status, toasts, or logs.
5. **Practical Fyne** — default theme; standard widgets only for v1.

---

## 2. Window Shell

| Property | Spec |
| -------- | ---- |
| Window title | `Vepeen` |
| Default size | **480 × 640** (logical Fyne units) |
| Minimum size | **420 × 560** — prevent crushing multi-line routes + status |
| Maximum size | None (user may resize) |
| Initial position | `CenterOnScreen()` |
| Resizable | Yes |
| Multi-window | No (v1 single main window) |
| System tray | Not required |
| Theme | Fyne **default theme** (light/dark follows OS/Fyne settings) — no custom palette v1 |

**Resize behavior:** Form fields expand horizontally; multi-line routes and status grow vertically via scroll/border middle; action bar stays pinned bottom.

---

## 3. Layout Structure (Fyne containers)

### 3.1 Hierarchy (conceptual — not code)

```
Window content
└── Padded (outer margin ~ theme padding)
    └── Border
        ├── Top:    Header block (VBox)
        ├── Center: Scroll → Form body (VBox)
        └── Bottom: Footer block (VBox) — actions + status
```

**Why Border:** Keeps **Hubungkan / Putuskan / Simpan** and **Status** always visible while the form scrolls on short displays.

### 3.2 Header (Top)

`VBox` (tight spacing):

1. **Title label** — `Vepeen` — Bold, left-aligned (or center if preferred; left matches form labels better).
2. **Subtitle label** — `Klien VPN L2TP/IPsec (split tunnel)` — secondary/muted style if available (`widget.NewLabel` with normal style is fine); wrapping on.
3. Optional thin separator (`widget.NewSeparator`) under header.

### 3.3 Form body (Center, inside Scroll)

Prefer **`widget.Form`** for labeled rows (native Fyne form layout: label column + control column). Append items in this **fixed order** (focus order = visual order):

| # | Form item label | Widget | Notes |
| - | --------------- | ------ | ----- |
| 1 | Nama koneksi | `widget.Entry` | Single-line |
| 2 | Server | `widget.Entry` | Single-line |
| 3 | Pre-shared key (PSK) | `widget.Entry` password mode (`NewPasswordEntry`) | Masked |
| 4 | Username | `widget.Entry` | Single-line |
| 5 | Password | `widget.Entry` password mode | Masked |
| 6 | Daftar IP/CIDR | `widget.MultiLineEntry` | See routes field |

**Routes field extras (below or as form item helper):**

- Multi-line entry: `SetMinRowsVisible(5)` (target ~5–6 visible lines).
- Place a **helper label** immediately under the multi-line control (still inside scroll), not as a second form column:

  > Satu IP atau CIDR per baris. Contoh: `10.10.0.0/16` atau `203.0.113.50`. Baris kosong diabaikan. Awali dengan `#` untuk komentar.

- Optional: wrapping on multi-line (`Wrapping = fyne.TextWrapOff` preferred so CIDRs stay one-token-per-line; horizontal scroll OK inside entry).

**Do not** put Connect/Disconnect inside `widget.Form`’s OnSubmit — use explicit footer buttons for clearer dual-action UX (Connect vs Disconnect vs Save).

If Backend prefers pure containers over `widget.Form`: use `container.New(layout.NewFormLayout(), label, control, …)` with the same order. **Do not invent a third layout pattern.**

### 3.4 Footer (Bottom)

`VBox`:

1. **Primary actions row** — `HBox` (or `container.NewGridWithColumns(2)` for equal width buttons on narrow windows):

   - **Hubungkan** — primary (`widget.HighImportance`)
   - **Putuskan** — default/medium importance; destructive-adjacent but not delete — use normal importance (not Danger unless disconnect fails repeatedly)

2. **Secondary actions row** — `HBox`:

   - **Simpan pengaturan** — low/medium importance (`widget.MediumImportance` or default)
   - Optional spacer / stretch

   *Alternative acceptable layout:* single `HBox` with `Simpan | stretch | Putuskan | Hubungkan` (Save left, connect cluster right). Prefer this if vertical space is tight:

   ```
   [Simpan pengaturan]     [Putuskan] [Hubungkan]
   ```

   **Canonical v1:** Save left; Putuskan + Hubungkan right-aligned group.

3. **Status card** — `VBox` inside a light visual group:

   - Status heading label: `Status`
   - Status value label (wrapping, multi-line capable) — primary user feedback
   - Optional detail label (smaller / secondary) for last sanitized error or hint

   Implementation options (pick one, keep simple):

   - `widget.Card` with title `Status` and content = status labels, **or**
   - `VBox` + separator above status block (lighter)

4. Do **not** add progress bar unless Backend already has easy indeterminate progress; status text + disabled buttons is enough for v1. If used: only during Connecting/Disconnecting.

### 3.5 Outer padding

- `container.NewPadded` on root content.
- Avoid nested heavy padding that wastes vertical space on 1366×768 laptops.

### 3.6 What not to use (v1)

- No tabs, accordion, or multi-page wizard.
- No absolute positioning / fixed pixel coordinates for widgets.
- No custom canvas drawings for status.
- No system tray menu.
- No profile list / sidebar.
- No “advanced” collapsible IPsec cipher UI.

---

## 4. Copy & Labels (Indonesian + technical terms)

### 4.1 Static chrome

| Element | Text |
| ------- | ---- |
| Window title | `Vepeen` |
| Header title | `Vepeen` |
| Header subtitle | `Klien VPN L2TP/IPsec (split tunnel)` |
| Status section title | `Status` |

### 4.2 Field labels (Form)

| Field key | Label (UI) | Placeholder (optional) |
| --------- | ---------- | ---------------------- |
| connectionName | `Nama koneksi` | `Vepeen` |
| server | `Server` | `vpn.contoh.com atau IP` |
| psk | `Pre-shared key (PSK)` | (empty; do not put secrets in placeholder) |
| username | `Username` | `nama.pengguna` |
| password | `Password` | (empty) |
| routes | `Daftar IP/CIDR` | multi-line helper is preferred over placeholder |

### 4.3 Helper / hint text

| Location | Text |
| -------- | ---- |
| Under routes | `Satu IP atau CIDR per baris. Contoh: 10.10.0.0/16 atau 203.0.113.50. Baris kosong diabaikan. Awali dengan # untuk komentar.` |
| Under PSK (optional short) | `Kunci IPsec dari admin VPN. Tidak disimpan di file teks biasa.` |
| Under Password (optional short) | `Kata sandi akun VPN (MS-CHAPv2).` |
| Save tooltip / secondary note | `Menyimpan server, nama koneksi, username, dan daftar rute. PSK & password tidak ditulis ke config JSON.` |

Keep optional under-field hints to **one line** each to avoid clutter. Routes helper is mandatory; PSK/password hints are nice-to-have.

### 4.4 Buttons

| Action | Label | Importance |
| ------ | ----- | ---------- |
| Connect | `Hubungkan` | `HighImportance` |
| Disconnect | `Putuskan` | Default |
| Save | `Simpan pengaturan` | Default / Medium |

**Dynamic button label (optional polish):** While connecting, primary may stay `Hubungkan` but **disabled** — do **not** rename to “Membuat profil…” on the button; put phase detail in status instead. Keeps layout stable.

### 4.5 Status messages (canonical examples)

All user-visible; **never** interpolate PSK, password, or full command lines.

| State / event | Status text (primary) | Detail (optional secondary) |
| ------------- | --------------------- | --------------------------- |
| Initial / idle disconnected | `Terputus` | `Siap terhubung.` |
| After load config | `Terputus` | `Pengaturan dimuat.` |
| Validation failed | `Tidak dapat menghubungkan` | Field-specific (see §6) |
| Connecting — ensure profile | `Menghubungkan…` | `Menyiapkan profil VPN…` |
| Connecting — sync routes | `Menghubungkan…` | `Menyelaraskan rute (split tunnel)…` |
| Connecting — rasdial | `Menghubungkan…` | `Menghubungi server…` |
| Connected | `Terhubung` | `Hanya IP/CIDR pada daftar yang melewati VPN.` |
| Disconnecting | `Memutuskan…` | `` |
| Disconnected after success | `Terputus` | `Koneksi ditutup.` |
| Error (generic) | `Gagal` | Sanitized reason |
| Auth failure | `Gagal autentikasi` | `Periksa username/password atau kebijakan server. PSK tidak ditampilkan.` |
| Profile / elevation | `Gagal menyiapkan profil` | `Mungkin diperlukan hak administrator. Coba jalankan sebagai user biasa dulu; profil per-user lebih disarankan.` |
| Network / timeout | `Gagal terhubung` | `Periksa server, jaringan, dan port UDP 500/4500 (L2TP/IPsec).` |
| Route sync failure | `Gagal menyelaraskan rute` | `Koneksi tidak dilanjutkan.` |
| Already connected | `Terhubung` | `Sudah terhubung.` |
| Save success | (keep connection state) | Toast/status flash: `Pengaturan disimpan.` |
| Save failure | (keep connection state) | `Gagal menyimpan pengaturan.` |

**Save feedback:** Prefer updating the status detail line for ~3s or until next state change; do not clear Connected state when save succeeds while connected.

### 4.6 Empty / first-run

- Pre-fill **Nama koneksi** = `Vepeen`.
- Other fields empty (or loaded from config).
- Status: `Terputus` / `Isi server, kredensial, dan minimal satu IP/CIDR, lalu Hubungkan.`

---

## 5. Component States

Connection lifecycle enum (UI + domain aligned with PRD):

`Disconnected` | `Connecting` | `Connected` | `Disconnecting` | `Error`

Treat **Error** as a sub-state of disconnected for control enablement (form editable again), with error text retained until next successful action or field edit (Backend may clear detail on next Connect attempt).

### 5.1 Enablement matrix

| Control | Disconnected | Connecting | Connected | Disconnecting | Error |
| ------- | :----------: | :--------: | :-------: | :-----------: | :---: |
| Nama koneksi | ✅ | ❌ | ❌ | ❌ | ✅ |
| Server | ✅ | ❌ | ❌ | ❌ | ✅ |
| PSK | ✅ | ❌ | ❌ | ❌ | ✅ |
| Username | ✅ | ❌ | ❌ | ❌ | ✅ |
| Password | ✅ | ❌ | ❌ | ❌ | ✅ |
| Daftar IP/CIDR | ✅ | ❌ | ❌ | ❌ | ✅ |
| Simpan pengaturan | ✅ | ❌ | ✅ | ❌ | ✅ |
| Hubungkan | ✅ | ❌ | ❌ | ❌ | ✅ |
| Putuskan | ❌ | ✅* | ✅ | ❌ | ❌** |

\* **Putuskan during Connecting:** Enabled so user can cancel/abort if Backend supports abort; if abort is not implemented in v1, keep **disabled** during Connecting and document in status (`Mohon tunggu…`). **Canonical v1 if no cancel API:** Putuskan **disabled** while Connecting; **enabled** only when Connected.  
\*\* Error: Putuskan disabled (not connected).

**Busy flag:** Any in-flight Connect/Disconnect/Save sets a UI `busy` that disables the triggering action and prevents re-entry (no double-connect).

### 5.2 State visuals

| State | Hubungkan | Putuskan | Form fields | Status tone |
| ----- | --------- | -------- | ----------- | ----------- |
| Disconnected | Enabled, HighImportance | Disabled | Editable | Neutral |
| Connecting | Disabled | Disabled (v1) | Read-only / disabled | Busy — `Menghubungkan…` |
| Connected | Disabled | Enabled | Disabled (settings frozen while up) | Success — `Terhubung` |
| Disconnecting | Disabled | Disabled | Disabled | Busy — `Memutuskan…` |
| Error | Enabled | Disabled | Editable | Error — `Gagal…` + detail |

**Connected + edit policy (v1):** Fields disabled while connected to avoid profile/route drift mid-session. User must **Putuskan** to change server/routes/credentials. **Simpan** remains enabled to persist non-secret edits only if fields were editable — under this freeze policy, Save while connected saves last known form values (still useful if secrets policy loads into memory). Simpler rule: **while Connected, form disabled; Save still allowed** to flush non-secrets currently in fields (values unchanged). Backend must not require re-enable for Save.

### 5.3 Button label stability

Do not swap Hubungkan ↔ Putuskan into a single toggle for v1. Two buttons reduce mis-clicks and match PRD.

---

## 6. Validation UX

### 6.1 When to validate

| Trigger | Behavior |
| ------- | -------- |
| **Hubungkan** click | Full validation; block connect on failure |
| **Simpan pengaturan** click | Validate non-secret shape lightly: connection name non-empty; routes if present must parse; server optional-on-save? **Canonical:** Save requires **Nama koneksi** non-empty; other fields saved as-is if parseable; invalid CIDR lines **block save** with message (do not persist garbage routes) |
| On typing | No aggressive red errors per keystroke v1 |
| On blur | Optional; not required v1 |

### 6.2 Required for Connect

1. Nama koneksi — non-empty (trim spaces)
2. Server — non-empty
3. PSK — non-empty
4. Username — non-empty
5. Password — non-empty
6. Daftar IP/CIDR — ≥1 valid IP or CIDR after parse

### 6.3 CIDR / IP rules (user-facing)

- Accept: `10.10.0.0/16`, `203.0.113.50/32`, bare IPv4 → treat as `/32`
- Ignore blank lines
- Optional: lines starting with `#` are comments
- Reject: non-IP garbage, bad prefix length, IPv6 if not supported (v1: **IPv4 only** unless Backend already supports IPv6 — if rejected, say `IPv6 belum didukung` / `Baris tidak valid`)

### 6.4 How errors show

**Primary pattern (Fyne-practical):**

1. Set status primary to `Tidak dapat menghubungkan` (or `Tidak dapat menyimpan`).
2. Set status detail to **specific** message.
3. Optionally `widget.ShowPopUp` / dialog only for unexpected fatal errors — prefer status area for validation to avoid modal fatigue.

**Field-level (if easy with Form validation):** mark invalid entry; not mandatory if status detail is precise.

### 6.5 Validation message catalog

| Condition | Message |
| --------- | ------- |
| Empty connection name | `Nama koneksi wajib diisi.` |
| Empty server | `Server wajib diisi.` |
| Empty PSK | `Pre-shared key (PSK) wajib diisi.` |
| Empty username | `Username wajib diisi.` |
| Empty password | `Password wajib diisi.` |
| No routes | `Isi minimal satu IP atau CIDR untuk split tunnel.` |
| Bad line N | `Baris {n} tidak valid: "{snippet}". Gunakan format IP atau CIDR (contoh 10.0.0.0/24).` |
| Snippet rules | Truncate to ~40 chars; **never** from PSK/password fields |

Multiple errors: show **first** error in focus order; optionally list up to 3 lines in status detail.

### 6.6 Focus on error

After failed validation, focus the first invalid field (connection name → … → routes).

---

## 7. Accessibility & Usability

### 7.1 Password masking

- PSK and Password use Fyne password entries (masked bullets).
- No “show password” toggle required in v1 (optional later).
- Copy-out of password fields follows OS/Fyne defaults; do not log pasted secrets.

### 7.2 Focus order (intent)

1. Nama koneksi  
2. Server  
3. PSK  
4. Username  
5. Password  
6. Daftar IP/CIDR  
7. Simpan pengaturan  
8. Putuskan  
9. Hubungkan  

Tab order must match visual order. Primary action is last among buttons so accidental Enter on form doesn’t disconnect.

### 7.3 Keyboard

- **Enter** in single-line fields: prefer move to next field or trigger Connect only if Backend wires Form OnSubmit — **canonical:** Enter does not disconnect; Connect is explicit button click (or Form submit mapped **only** to Connect when Disconnected and valid). Safest v1: **no implicit Enter-submit**; user clicks Hubungkan.
- **Escape:** not required to cancel connect unless cancel is implemented.

### 7.4 Busy feedback

- Disable Hubungkan immediately on click before async work.
- Status → `Menghubungkan…` on UI thread before worker starts.
- All VPN/PowerShell/rasdial work **off** UI thread; marshal status updates back to UI thread.
- Cursor: Fyne default; no custom wait cursor required.
- Prevent double-connect with `busy` mutex/flag in UI layer.

### 7.5 Touch / click targets

Desktop-first; Fyne default button height is acceptable. Avoid icon-only buttons without text.

### 7.6 Contrast / theme

Default Fyne theme; status error text should remain readable in light and dark. Prefer wording + state label over color-only meaning (`Terhubung` / `Gagal`, not just green/red).

### 7.7 Secrets & screen sharing

Status and any dialogs must be safe to screenshot: no PSK/password, no `rasdial` command lines with credentials.

---

## 8. Visual Constraints (Fyne)

| Topic | Decision |
| ----- | -------- |
| Theme | Default Fyne theme only |
| Primary CTA | `Hubungkan` → `Importance = HighImportance` |
| Danger | Do not use DangerImportance on Putuskan (not a destructive delete) |
| Icons | Optional small status icon later; v1 text-only OK |
| Fonts | Theme default; bold for title only |
| Colors | No hardcoded hex; no custom CSS |
| Density | Comfortable — one form, not dashboard cards grid |
| Clutter | Max one helper paragraph under routes; avoid duplicate instructions in header and status |
| Animation | None required; respect OS reduced-motion by not adding custom animation |
| Branding | Product name only; no marketing illustrations |

---

## 9. Data load / save UX

### 9.1 On startup

1. Show window with defaults (Nama koneksi `Vepeen`).
2. Load non-secret config async or sync before show if fast.
3. Populate: connection name, server, username (if remembered), routes (join with `\n`).
4. Load secrets from CredMan/DPAPI into PSK/password fields **if** implemented; if MVP in-memory only, leave secret fields empty and status detail may say: `PSK dan password perlu diisi setiap sesi (belum disimpan aman).` only when that tradeoff is active — otherwise stay quiet.
5. Query OS for existing connection state if cheap; if already connected under same name, set **Connected** and disable form per matrix.

### 9.2 Simpan pengaturan

- Writes non-secrets only.
- Does not connect.
- Success: `Pengaturan disimpan.`
- Failure: show sanitized error.
- Does not clear password fields on success.

### 9.3 Auto-save

Not required v1. Explicit **Simpan pengaturan** is enough. Optional: auto-save non-secrets on successful Connect (nice-to-have; if implemented, still allow manual Save).

---

## 10. Async flows (UI responsibility)

### 10.1 Connect (happy path)

```
User clicks Hubungkan
  → validate
  → busy=true; state=Connecting; disable form + Hubungkan
  → status Menghubungkan… / Menyiapkan profil…
  → background: EnsureProfile → SyncRoutes → rasdial
  → UI updates detail phases if callbacks available
  → success: state=Connected; status Terhubung + split-tunnel note
  → failure: state=Error; re-enable form; sanitized error
  → busy=false
```

### 10.2 Disconnect

```
User clicks Putuskan
  → busy=true; state=Disconnecting
  → background: rasdial /DISCONNECT
  → success: Disconnected
  → failure: Error or stay Connected with error detail; allow retry Putuskan
  → busy=false
```

### 10.3 Idempotency

- Connect while already Connected: no-op; status `Sudah terhubung.`
- Disconnect while Disconnected: no-op; status `Sudah terputus.`

---

## 11. Empty / Error / Success messaging (sanitized examples)

### Empty routes on connect

- Primary: `Tidak dapat menghubungkan`
- Detail: `Isi minimal satu IP atau CIDR untuk split tunnel.`

### Invalid CIDR

- Primary: `Tidak dapat menghubungkan`
- Detail: `Baris 2 tidak valid: "abc". Gunakan format IP atau CIDR (contoh 10.0.0.0/24).`

### Auth failure (rasdial)

- Primary: `Gagal autentikasi`
- Detail: `Username/password ditolak atau server menolak autentikasi. Pastikan PSK juga benar (tidak ditampilkan di sini).`

### Success connected

- Primary: `Terhubung`
- Detail: `Hanya IP/CIDR pada daftar yang melewati VPN.`

### Success save

- Detail flash: `Pengaturan disimpan.`  
- Never: `Disimpan password=...`

### Forbidden examples (do not implement)

- `PSK=supersecret123`
- `rasdial Vepeen alice MyP@ss`
- Full PowerShell dumps with `-L2tpPsk '...'`

---

## 12. Wireframe (text)

```
┌─────────────────────────────────────────────┐
│ Vepeen                                      │  ← window title
├─────────────────────────────────────────────┤
│ Vepeen                                      │
│ Klien VPN L2TP/IPsec (split tunnel)         │
│ ─────────────────────────────────────────── │
│ ┌ scroll ─────────────────────────────────┐ │
│ │ Nama koneksi     [ Vepeen             ] │ │
│ │ Server           [                    ] │ │
│ │ Pre-shared key   [ ••••••••••         ] │ │
│ │ Username         [                    ] │ │
│ │ Password         [ ••••••••••         ] │ │
│ │ Daftar IP/CIDR   [ 10.10.0.0/16      ] │ │
│ │                  [ 203.0.113.50       ] │ │
│ │                  [                    ] │ │
│ │ Satu IP atau CIDR per baris. Contoh: …  │ │
│ └─────────────────────────────────────────┘ │
│ [Simpan pengaturan]     [Putuskan][Hubungkan]│
│ Status                                      │
│ Terputus                                    │
│ Siap terhubung.                             │
└─────────────────────────────────────────────┘
```

---

## 13. Frontend / Backend Developer Guidance

### 13.1 Files

| File | Action |
| ---- | ------ |
| `internal/ui/main_window.go` | Replace demo UI with this form |
| Optional `internal/ui/status.go` / `form.go` | Split only if file grows large |
| `cmd/vepeen/main.go` | Keep thin; title/app ID only if needed |

### 13.2 Widgets to use (Fyne std)

- `widget.Label`, `widget.Entry`, `widget.NewPasswordEntry`, `widget.NewMultiLineEntry`
- `widget.Button` + `Importance`
- `widget.Form` **or** `layout.NewFormLayout`
- `widget.Separator`, optional `widget.Card`
- `container.NewBorder`, `NewVBox`, `NewHBox`, `NewPadded`, `NewScroll`
- `fyne.NewSize(480, 640)` default resize

### 13.3 Must not invent (Backend constraints from Design)

1. **No multi-profile manager** — single form, default name `Vepeen`.
2. **No system tray** requirement.
3. **No full-tunnel toggle** in UI — split tunnel is always the product behavior.
4. **No “remember password” checkbox** unless secrets backend is real; if MVP memory-only, omit checkbox rather than lying.
5. **No plaintext secret display** in status/logs/dialogs.
6. **No dual-theme custom branding** or third-party UI kits.
7. **Do not** change field set without PRD update: connection name, server, PSK, username, password, routes, save, connect, disconnect, status.
8. **Do not** enable form editing while Connected (v1 freeze policy).
9. **Do not** use a single toggle button instead of Hubungkan + Putuskan.
10. **Do not** block the UI thread on PowerShell/rasdial.
11. **IPv4-first** route list messaging; don’t silently drop lines without error.
12. **Window title** stays `Vepeen` (not “Settings”, not server hostname).

### 13.4 Suggested UI state struct (conceptual)

```
ConnectionUIState:
  Phase: Disconnected | Connecting | Connected | Disconnecting | Error
  Busy: bool
  StatusTitle: string
  StatusDetail: string
  LastErrorSanitized: string
```

Map domain errors → catalog in §4.5 / §6.5 before SetText on labels.

### 13.5 Sub-agent decomposition (if split)

- **A — Shell & layout:** window size, Border/Scroll/header/footer structure  
- **B — Form fields & validation messages:** widgets, enablement matrix, validate-on-connect  
- **C — Actions wiring:** Save / Connect / Disconnect async + status updates (depends on `internal/vpn|route|config|secrets`)

Design ownership ends at this document; all Go is Backend Developer.

---

## 14. Acceptance checklist (UI)

- [ ] Demo counter removed; VPN form visible on launch  
- [ ] All PRD fields present with Indonesian labels  
- [ ] Default size ~480×640; usable at min ~420×560  
- [ ] Password fields masked  
- [ ] Hubungkan is HighImportance  
- [ ] State matrix respected (no double-connect)  
- [ ] Validation messages in Indonesian; CIDR line errors cite line number  
- [ ] Status never contains secrets  
- [ ] Connected freezes settings; Putuskan enabled  
- [ ] Scroll works when window is short  
- [ ] Save persists non-secrets only (behavior + copy aligned)

---

## 15. Out of scope (explicit)

- Multi-language i18n framework (hardcoded Indonesian strings OK v1)
- System tray, auto-connect, start with Windows
- Profile import/export UI
- QR setup, deep links
- Custom IPsec proposal editor
- Accessibility audit tooling beyond practical Fyne defaults

---

## 16. Return summary (for Orchestrator)

| Item | Value |
| ---- | ----- |
| Design doc path | `docs/planning/ui-002-l2tp-form.md` |
| Layout | Padded → Border(top header, center Scroll+Form, bottom actions+status) |
| Default window | 480×640 (min ~420×560), title `Vepeen`, default Fyne theme |
| Primary CTA | `Hubungkan` (HighImportance); `Putuskan` + `Simpan pengaturan` secondary |
| Key state rule | Form editable only when Disconnected/Error; frozen when Connected/Connecting/Disconnecting; busy prevents double-connect |
| Backend must not invent | Multi-profile, tray, full-tunnel toggle, secret-in-status, single toggle button, UI-thread VPN calls, connected-field editing |

**Next agent:** Backend Developer — implement packages + replace `internal/ui/main_window.go` per this spec and PRD-002.
