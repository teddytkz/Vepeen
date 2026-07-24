# Docs-002: Platform support notes (macOS / Linux gaps)

**Version:** v1.0.0  
**Status:** Approved for implementation  
**Author:** Planner Agent  
**Created:** 2026-07-24  
**Type:** Documentation only (no code)  
**Scope:** Minor — changelog + README section

---

## Decision

**Changelog + one README section.** No product PRD.

| Option | Why |
| ------ | --- |
| Full PRD | No — no code/API |
| **Changelog + short docs plan** | **Yes** — insert compact Platform support notes |
| Changelog one-liner only | Too thin for matrix + gap lists |

**Files:** `README.md`, `docs/planning/changelog.md`  
**Agent:** Documentation → Debugger/Reviewer

---

## Where to insert

**After** `## Project layout` (including the facade line: *Non-Windows builds get stubs.*)  
**Before** `## Security notes & residual risks`

Rationale: layout already mentions stubs; Non-goals stays product-scope; Prerequisites stays Windows-only.

Do **not** change the title one-liner (Windows-first). Optional one-word cross-link under Prerequisites is unnecessary.

---

## Section outline (write this)

### `## Platform support`

Lead (1–2 sentences):

- Primary target: **Windows 10/11**.
- macOS: partial / experimental. Linux: compile stubs only — **not** a usable VPN client.

### Support matrix

| Area | Windows | macOS | Linux |
| ---- | ------- | ----- | ----- |
| Build / Fyne UI | Yes | Yes | Basic window only |
| System tray | Yes | Yes | No |
| VPN dial | Yes (`rasdial` / VpnClient) | Partial (`scutil --nc` + `networksetup`; existing profiles only) | Unsupported stub |
| Create L2TP/PSK profile | No (OS outside app) | No (user creates in System Settings) | No |
| UI username/password at dial | Yes (optional → rasdial) | Ignored (OS-saved creds only) | N/A |
| Split-tunnel routes | Yes | Partial (post-dial `ApplySplitTunnel`, admin `osascript`) | Unsupported stub |
| Secrets / config | DPAPI `vepeen.bin` | Keychain + AES-GCM → `vepeen.bin` | Encrypt fails; secrets in-memory only |
| Single-instance / show-signal | Yes | No-op | No-op |
| Desktop shortcut | Yes | Unsupported | Unsupported |
| Traffic counters / active conns | Yes | Unavailable / empty | N/A |
| Packaging | `build.ps1` → `bin/vepeen.exe` | No `.app` story | None |

### macOS gaps (bullets — keep short)

- Does not create/configure L2TP/PSK profiles (System Settings first).
- UI username/password ignored at dial (OS-saved credentials only).
- Single-instance, show-signal, desktop shortcut: no-ops / unsupported.
- Traffic counters unavailable; active connections empty.
- Log path still Windows-ish (`%AppData%` → fallback).
- No `.app` packaging; not feature-parity with Windows.

### Linux gaps (bullets)

- No `*linux*` backend files; VPN/route unsupported stubs.
- Config encrypt fails; secrets in-memory only.
- UI: basic window, no tray.
- Compile-only — not a usable VPN client.

### Needed for real Linux (one short list)

- `internal/vpn` Linux backend (`nmcli`/NetworkManager or strongSwan)
- `internal/route` Linux
- Secrets + encrypted config (Secret Service)
- Tray, log path, deps docs

**Tone:** state gaps only; do not claim macOS/Linux fully supported. Use Explorer facts only (2026-07-24).

---

## Implementation tasks

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1 | Documentation | `README.md` | Insert `## Platform support` per outline; keep rest of README structure |
| 2 | Documentation | `docs/planning/changelog.md` | Ensure Unreleased entry (Planner may pre-add) |
| 3 | Debugger/Reviewer | `README.md` | Verify matrix/gaps match code stubs; no overclaim |

---

## Acceptance criteria

- [ ] README has a compact **Platform support** section (matrix + macOS gaps + Linux gaps + Linux needs)
- [ ] Windows remains the documented primary platform
- [ ] macOS described as partial; Linux as stub/compile-only
- [ ] No invented features; no claim of full macOS/Linux support
- [ ] Existing sections unchanged except clean insert + optional `Last updated` stay `2026-07-24`
- [ ] Changelog Unreleased notes this docs change

## Non-goals

- Implementing Linux/macOS backends
- Rewriting Prerequisites / Non-goals / full README
- Screenshots, installers, CI

## Rollback

Revert the Platform support section in `README.md` and the changelog bullet.
