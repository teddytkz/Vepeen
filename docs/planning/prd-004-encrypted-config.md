# PRD-004: Single Encrypted Config File (`vepeen.bin`)

**Version:** v0.1.0
**Status:** Draft
**Author:** Planner Agent
**Created:** 2026-07-23
**Updated:** 2026-07-23

---

## Overview

Replace the current two-store setup (plain `config.json` for non-secret settings + Windows Credential Manager for secrets) with a **single encrypted file** named `vepeen.bin` placed next to the executable. The file holds both non-secret settings (`selectedProfile`, `routes`, `rememberCredentials`) and per-profile secrets (`username`, `password`, `psk`), all encrypted via Windows DPAPI user-scope. This removes the CredMan dependency, keeps everything portable alongside `vepeen.exe`, and preserves the existing "no plaintext secrets on disk" guarantee.

## Problem Statement

Today secrets live in Windows Credential Manager (`internal/secrets`, `CRED_PERSIST_ENTERPRISE`) while settings live in `config.json`. This split has drawbacks:

- Two stores to migrate, debug, and reason about.
- CredMan entries are invisible to the user and not co-located with the app (hard to back up / move with `vepeen.exe`).
- The README claims PSK is stored in CredMan, but the code does **not** — `internal/secrets` only defines `KindUsername` and `KindPassword` (no `KindPSK`), and the UI has **no PSK entry field** (`userEntry`/`passEntry` only). PSK is currently neither collected nor persisted.
- CredMan is non-roaming anyway (`CRED_PERSIST_ENTERPRISE` is user-scoped, not roaming), so the portability benefit of DPAPI is equivalent.

The user explicitly chose the filename `vepeen.bin`.

## Goals

- [ ] A single `vepeen.bin` next to the executable contains both settings and secrets, encrypted with DPAPI user-scope.
- [ ] `config.Load()` / `config.Save()` transparently encrypt/decrypt; callers never see the blob.
- [ ] One-time migration merges legacy `config.json` + CredMan entries into `vepeen.bin`, then purges the old sources.
- [ ] No secret value is ever written to logs, status text, or error strings.
- [ ] `go build ./...` and `go test ./internal/config/...` pass; manual scenarios verified.

## Non-Goals

- **PSK collection UI is out of scope for this PRD's core.** This PRD establishes the *storage* for PSK (the `CredEntry.PSK` field exists and is migrated/encrypted), but wiring a PSK entry into the form + `ConnectRequest` + `EnsureProfile` is a separate follow-up task (see Risks/Open Questions). The plan notes the seam but does not implement PSK capture.
- Roaming / cross-device sync (DPAPI user-scope is intentionally non-roaming — same limitation as current CredMan).
- A passphrase-based encryption scheme (DPAPI is transparent, no prompt).
- macOS/Linux secret backends beyond a compile-only stub (VPN is Windows-only; `vepeen.bin` is Windows DPAPI).

---

## Feature Specification

### User Stories

- As a user, I want all my settings and saved credentials in one file next to `vepeen.exe`, so that I can back up / move the app as a unit.
- As a user, I want my saved username/password to survive app restarts, so that I don't re-type them.
- As a security-conscious user, I want secrets encrypted on disk and never printed in logs, so that casual file/process inspection doesn't leak them.
- As a user upgrading from the old version, I want my existing settings and saved credentials migrated automatically, so that nothing is lost.

### Acceptance Criteria

- [ ] On first run with no `vepeen.bin`, a fresh encrypted file is created on save (or on first persist) containing defaults + any entered credentials.
- [ ] `vepeen.bin` is an opaque DPAPI blob; opening it in a text editor shows no JSON/secret plaintext.
- [ ] Settings + saved credentials persist across app restart (fresh run reads `vepeen.bin` and pre-fills the form).
- [ ] Deleting `vepeen.bin` resets the app to defaults (no crash, no CredMan dependency).
- [ ] Legacy `config.json` (executable-adjacent or `%AppData%\vepeen\config.json`) is migrated into `vepeen.bin` and then removed.
- [ ] Existing CredMan entries (`vepeen/<name>/username`, `vepeen/<name>/password`) are migrated into `vepeen.bin` and then deleted from CredMan.
- [ ] Migration is idempotent: running it twice does not duplicate or lose data.
- [ ] No `password`, `psk`, `l2tppsk`, or credential value appears in `log.Printf`, `appendLog`, status labels, or error strings.
- [ ] `go build ./...` succeeds; `go test ./internal/config/...` passes (new encrypt/decrypt round-trip + migration tests).

---

## Technical Design

### Architecture Overview

`internal/config` becomes the single source of truth for persisted state. It owns a new DPAPI wrapper (build-tagged `windows`) that encrypts/decrypts the opaque blob. `internal/secrets` (CredMan) is deleted; `main_window.go` reads/writes credentials from the in-memory `Stored` struct instead of `secrets.Store`. `internal/vpn` is unchanged except for a log-sanitization confirmation.

### Codebase Context (verified by reading the actual files)

- `internal/config/config.go`: `Config{SelectedProfile, Routes, RememberCredentials}`; `Dir()` prefers `os.Executable()` dir else `%AppData%/vepeen`; `Path()` = dir + `config.json`; `Load()` migrates legacy `%AppData%\vepeen\config.json`; `Save()` atomic (tmp + rename); `parseConfig` handles new/old/legacy shapes. `config_test.go` covers `parseConfig` + `Default()`.
- `internal/secrets/secrets.go`: `Store` interface `Set/Get/Delete(connectionName, kind, value)`; `Kind` = `KindPassword | KindUsername` (**no `KindPSK`**); `Target()` = `vepeen/<name>/<kind>`.
- `internal/secrets/store_windows.go`: CredMan via `advapi32.dll` `CredWriteW/CredReadW/CredDeleteW/CredFree`; `credPersist = credPersistEnterprise` (user-scoped, non-roaming).
- `internal/secrets/store_other.go`: in-memory stub for non-Windows.
- `internal/ui/main_window.go`: `controller.store secrets.Store` (line 72), created via `secrets.NewStore()` (line 86). Credential flows: `loadCredentials()` (332) reads `KindUsername`/`KindPassword`; `persistCredentials()` (345) writes/deletes them; `onConnect()` (632) falls back to stored creds when form empty + remember checked; `persistQuiet()` (797) saves config only. `c.cfg config.Config` (56); `applyConfig(cfg config.Config)` (302); `onSave` sets `c.cfg.*` then `config.Save` (549-557); `loadInitial` calls `config.Load()` (250).
- `internal/vpn/manager.go`: `sanitizeDetail` (167) strips strings containing `l2tppsk`/`password`. `ConnectRequest{Name, Username, Password, RoutesText, Routes}` — **no PSK field**. `ConnectParams{Name, Username, Password}` — **no PSK field**. `EnsureProfile` (`profile_windows.go:132`) only sets `-SplitTunneling $true`; it does **not** set `-L2tpPsk`. So PSK is currently never applied to the Windows profile either.
- `golang.org/x/sys/windows v0.30.0` is already a dependency (used in `internal/ui/window_pos_windows.go` via `NewLazySystemDLL`/`NewProc`/`Call`). DPAPI (`Crypt32.dll` `CryptProtectData`/`CryptUnprotectData`) is reachable the same way.

### Data Model

New combined in-memory struct in `internal/config/config.go`:

```go
// Stored is the full persisted state (settings + per-profile secrets).
// It is JSON-marshaled, then DPAPI-encrypted into vepeen.bin.
type Stored struct {
	SelectedProfile     string              `json:"selectedProfile"`
	Routes              []string            `json:"routes"`
	RememberCredentials bool                `json:"rememberCredentials"`
	Credentials         map[string]CredEntry `json:"credentials"` // keyed by profile name
}

type CredEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PSK      string `json:"psk"`
}
```

`Config` is retained as a convenience projection (settings only) so existing UI code (`c.cfg config.Config`, `applyConfig`, `onSave`) keeps working with minimal churn; `Stored` is the persisted type. `Default()` returns a `Stored` with empty `Credentials` map and `RememberCredentials: true`.

### API Changes

`internal/config/config.go`:

- Add `binFileName = "vepeen.bin"`; keep `configFileName = "config.json"` **only** for legacy migration.
- `BinPath() (string, error)` → `Dir()` + `vepeen.bin`.
- `Load() (Stored, error)` — replaces `Load() (Config, error)`. Reads `vepeen.bin`, DPAPI-decrypts, `json.Unmarshal`s into `Stored`. Missing file → `Default()` (no error). On decrypt failure → log a sanitized message, return `Default()` (graceful degradation; never panic on a corrupt blob).
- `Save(s Stored) error` — `json.Marshal` → DPAPI-encrypt → atomic write (tmp + rename) next to exe.
- Keep `Config` type + a helper `func (s Stored) Config() Config` returning the settings projection, and `func (c Config) withCreds(Credentials map[string]CredEntry) Stored` for the save path, so `main_window.go` changes stay small.
- Migration helpers (build-tagged or guarded): `migrateLegacy() (Stored, migrated bool, err error)` — see Migration Logic below. `Load()` calls it internally when `vepeen.bin` is absent but legacy sources exist.

`internal/secrets/*` — **DELETED** (Windows + other). All credential access moves to `Stored.Credentials`.

`internal/ui/main_window.go`:

- Remove `store secrets.Store` field and `secrets.NewStore()` init.
- `loadCredentials()`: read `c.stored.Credentials[name]` (Username/Password) instead of `c.store.Get`.
- `persistCredentials(name, user, pass)`: update `c.stored.Credentials[name]` (set or delete entry) and call `config.Save(c.stored)` (or stage for the next save).
- `onConnect()` fallback (632): read from `c.stored.Credentials[name]` instead of `c.store.Get`.
- `persistQuiet()` (797): build `Stored` from `req` + existing creds, `config.Save`.
- `loadInitial()` (250) and `onSave` (549-557): use `Stored`/`config.Load()`/`config.Save()`.

`internal/vpn/manager.go`: confirm `sanitizeDetail` already strips `l2tppsk`/`password`; no change required, but add a test assertion that a `Stored`/credential string is never passed through unsanitized (see tests).

### UI Changes

- No new visible fields in this PRD (PSK capture is a follow-up). The `rememberCheck` gate and credential pre-fill behavior are preserved, just sourced from `vepeen.bin` instead of CredMan.
- If PSK capture is later added, the `CredEntry.PSK` field is already present and encrypted — only the form + `ConnectRequest` + `EnsureProfile` wiring remains.

---

## DPAPI Wrapper Design

New file `internal/config/dpapi_windows.go` (`//go:build windows`):

```go
package config

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modCrypt32           = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
	procLocalFree          = windows.NewLazySystemDLL("kernel32.dll").NewProc("LocalFree")
)

const crYPTPROTECT_UI_FORBIDDEN = 0x1

// encryptDPAPI wraps CryptProtectData with user-scoped, no-prompt protection.
func encryptDPAPI(plain []byte) ([]byte, error) {
	var outData dataBlob
	in := dataBlob{Size: uint32(len(plain)), Data: uintptr(unsafe.Pointer(&plain[0]))}
	// dwPromptFlags = CRYPTPROTECT_UI_FORBIDDEN (no UI); no optional entropy;
	// no description; no reserved; no flags beyond UI_FORBIDDEN.
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // description (PWSTR, optional)
		0, // optional entropy (no additional secret)
		0, // reserved
		0, // prompt struct (nil)
		crYPTPROTECT_UI_FORBIDDEN,
		uintptr(unsafe.Pointer(&outData)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(outData.Data)
	return unsafe.Slice((*byte)(unsafe.Pointer(outData.Data)), outData.Size), nil
}

// decryptDPAPI wraps CryptUnprotectData (same user scope, no prompt).
func decryptDPAPI(blob []byte) ([]byte, error) { /* symmetric to encrypt */ }

type dataBlob struct {
	Size uint32
	Data uintptr
}
```

**Params / semantics:**
- `CryptProtectData(DATA_BLOB* pDataIn, LPCWSTR szDataDescr, DATA_BLOB* pOptionalEntropy, PVOID pvReserved, CRYPTPROTECT_PROMPTSTRUCT* pPromptStruct, DWORD dwFlags, DATA_BLOB* pDataOut)`.
- `dwFlags = CRYPTPROTECT_UI_FORBIDDEN (0x1)` → no prompt; user-scope (current Windows account) by default.
- No optional entropy (`pOptionalEntropy = NULL`) → equivalent to current CredMan security; keeps it transparent (no passphrase).
- `pDataOut` must be freed with `LocalFree` (done via `procLocalFree.Call`).

**Error handling / graceful degradation:**
- If `encryptDPAPI`/`decryptDPAPI` returns a non-zero error (e.g., DPAPI unavailable, blob corrupt), `Save`/`Load` return a sanitized error and `Load` falls back to `Default()` rather than crashing. This mirrors the "CenterOnScreen-style graceful degradation" requirement: the app still runs, just without persisted secrets.
- `unsafe.Slice` requires Go 1.17+ (we are on Go 1.22 — fine). `runtime.KeepAlive` on Go-managed buffers during the native call, as `store_windows.go` already does for CredMan.

Non-Windows stub `internal/config/dpapi_other.go` (`//go:build !windows`): `encryptDPAPI`/`decryptDPAPI` return `errors.New("encrypted config requires Windows")` so the package compiles cross-platform (VPN features remain Windows-only; `vepeen.bin` is never produced off-Windows).

---

## Migration Logic (config.json + CredMan → vepeen.bin)

`migrateLegacy()` is invoked from `Load()` **only when `vepeen.bin` does not exist**. Ordering and idempotency:

1. **Detect sources.** Check for executable-adjacent `config.json`, legacy `%AppData%\vepeen\config.json`, and any CredMan entries under the `vepeen/` target prefix.
2. **Build `Stored`.** Start from `Default()`.
   - If either `config.json` exists: `parseConfig` (reuse existing migration for new/old/legacy shapes) → populate `SelectedProfile`, `Routes`, `RememberCredentials`.
   - If CredMan entries exist: for each profile name `N`, read `vepeen/N/username` and `vepeen/N/password` (via a **temporary, local** CredMan read inside `migrateLegacy`, imported only during migration — see note below) and populate `Stored.Credentials[N] = {Username, Password}`. PSK is not present in CredMan today, so `PSK` stays empty (no data loss).
3. **Write `vepeen.bin`.** `Save(stored)` (atomic). This is the idempotency anchor: once `vepeen.bin` exists, `Load()` never re-enters `migrateLegacy`.
4. **Purge old sources (only after successful write):**
   - Delete the `config.json` files (both locations).
   - Delete the migrated CredMan entries (`vepeen/N/username`, `vepeen/N/password`).
5. **Safe fallback.** If `Save` fails, do **not** purge old sources (keep CredMan + config.json so nothing is lost). If CredMan read fails, continue with config.json-only migration. If config.json read fails, continue with CredMan-only migration. Migration never aborts the app.

**Note on CredMan read during migration:** Because `internal/secrets` is deleted, `migrateLegacy` needs a one-shot CredMan reader. Two options:
- **(Recommended)** Keep a tiny `internal/secrets` read-only helper (`ReadLegacy(name, kind)`) used *only* by `migrateLegacy`, deleted in a later PR once no users remain on the old format. This avoids re-implementing `CredReadW` inline.
- **(Alt)** Inline a minimal `credReadW` in `dpapi_windows.go` for migration only.

The plan recommends the first option (keep `internal/secrets` as a **migration-only, read-only fallback** for one release, then delete in a follow-up). This satisfies "keep a safe fallback" from the request.

---

## Implementation Plan

### Phase 1: Encrypted config core (Windows DPAPI + Stored struct)

**Depends on:** Nothing
**Parallelizable:** No — foundational; everything else builds on it

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1.1 | Backend Developer | `internal/config/config.go` | Add `Stored`/`CredEntry` structs, `binFileName`, `BinPath()`, keep `Config` + `Stored.Config()`/`Config.withCreds()` projection helpers, `Default()` returns `Stored`. |
| 1.2 | Backend Developer | `internal/config/dpapi_windows.go` (new, `//go:build windows`) | Implement `encryptDPAPI`/`decryptDPAPI` via `crypt32.dll` `CryptProtectData`/`CryptUnprotectData` + `LocalFree`; `CRYPTPROTECT_UI_FORBIDDEN`; `runtime.KeepAlive`; error handling. |
| 1.3 | Backend Developer | `internal/config/dpapi_other.go` (new, `//go:build !windows`) | Stub returning "encrypted config requires Windows" so package compiles cross-platform. |
| 1.4 | Backend Developer | `internal/config/config.go` | Rewrite `Load() (Stored, error)` and `Save(Stored) error`: read/write `vepeen.bin` (atomic tmp+rename), DPAPI encrypt/decrypt, graceful fallback to `Default()` on missing/corrupt blob. |

**Sub-Agent Guidance:**
- 1.1–1.4 are sequential within the phase (1.2/1.3 can be written in parallel since they're separate build tags, but 1.4 depends on 1.1+1.2).

### Phase 2: Migration (config.json + CredMan → vepeen.bin)

**Depends on:** Phase 1

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 2.1 | Backend Developer | `internal/config/config.go` | Add `migrateLegacy() (Stored, bool, error)`: detect config.json (both locations) + CredMan; build `Stored`; call `Save`; purge old sources only on success; safe fallbacks. |
| 2.2 | Backend Developer | `internal/secrets/*` | Convert `internal/secrets` to **migration-only read-only** helper (`ReadLegacy(name, kind)` using existing `CredReadW`); remove `Set`/`Delete`/write paths. Document it is deleted in a follow-up release. |
| 2.3 | Backend Developer | `internal/config/config.go` | Wire `migrateLegacy` into `Load()` (invoked only when `vepeen.bin` absent). |

### Phase 3: UI credential flows → vepeen.bin

**Depends on:** Phase 1 (Phase 2 optional but recommended before shipping)

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 3.1 | Frontend Developer | `internal/ui/main_window.go` | Remove `store secrets.Store` field + `secrets.NewStore()`; add `stored config.Stored` to controller; load it in `loadInitial` via `config.Load()`. |
| 3.2 | Frontend Developer | `internal/ui/main_window.go` | `loadCredentials()`: read `c.stored.Credentials[name]` instead of `c.store.Get`. |
| 3.3 | Frontend Developer | `internal/ui/main_window.go` | `persistCredentials()`: update `c.stored.Credentials[name]` (set/delete) + `config.Save(c.stored)`. |
| 3.4 | Frontend Developer | `internal/ui/main_window.go` | `onConnect()` fallback (632): read from `c.stored.Credentials[name]`. |
| 3.5 | Frontend Developer | `internal/ui/main_window.go` | `persistQuiet()` (797) + `onSave` (549-557) + `applyConfig` (302): use `Stored`/`config.Save`. |
| 3.6 | Frontend Developer | `internal/ui/main_window.go` | Remove `import "vepeen/internal/secrets"` once no references remain. |

### Phase 4: Tests + log-sanitization confirmation

**Depends on:** Phases 1–3

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 4.1 | Backend Developer | `internal/config/config_test.go` | Add DPAPI round-trip test (`Save`→`Load`→equal `Stored`, including `Credentials` map); missing-file → `Default()`; corrupt-blob → `Default()` no panic; `BinPath()` ends with `vepeen.bin`. |
| 4.2 | Backend Developer | `internal/config/config_test.go` | Add migration test: seed a temp `config.json` + (mock/real) CredMan entries, call `Load`, assert `vepeen.bin` created, settings + creds migrated, old `config.json` removed; idempotent on second `Load`. |
| 4.3 | Backend Developer | `internal/vpn/manager_test.go` (or `errors_test.go`) | Assert `sanitizeDetail` strips `l2tppsk`/`password`; add a guard that a `CredEntry` JSON is never logged in plaintext (document the rule; no secret string reaches `log.Printf`). |

### Phase 5: Review & Documentation (Always Last)

**Depends on:** All implementation phases

| Task | Agent | Description |
| ---- | ----- | ----------- |
| 5.1 | Debugger/Reviewer | Verify all acceptance criteria; run `go build ./...` + `go test ./internal/config/...`; check no `secrets.` references remain in `main_window.go`. |
| 5.2 | Security | Confirm no secret (password/psk/username) is written to logs/status/errors; confirm `vepeen.bin` is opaque (no plaintext JSON); confirm CredMan purge after migration. |
| 5.3 | Documentation | Update `README.md` (storage section: `vepeen.bin` next to exe, DPAPI user-scope, non-roaming caveat, migration note); update `docs/planning/changelog.md`. |

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
| ---- | ----- | ----- | ----- |
| DPAPI is non-roaming (blob bound to current Windows user account) | High (secrets lost if user resets password / switches account / moves machine) | Medium | Same limitation as current `CRED_PERSIST_ENTERPRISE`. Document clearly in README. No roaming in scope. |
| AV / EDR flags `vepeen.bin` as suspicious (unknown binary next to exe) | Medium | Low–Medium | `.bin` is opaque but not executable; if flagged, note user can add an exclusion. Consider documenting the blob format. |
| Reverse-engineering the blob | Medium | Medium | Equivalent to current CredMan (also user-scoped, readable by the same user). DPAPI is the OS-standard store. No additional exposure. |
| Log/status leakage of decrypted secrets | High | Low | `sanitizeDetail` already strips `l2tppsk`/`password`; `Stored` is never passed to `log.Printf`; add test guard (4.3). |
| Atomic write of encrypted blob interrupted (power loss mid-rename) | Medium | Low | Reuse existing atomic tmp+rename pattern from `Save`. `Load` falls back to `Default()` on corrupt blob (no crash). |
| Migration leaves duplicate CredMan entries if purge fails | Low | Low | Purge only after successful `Save`; if purge fails, next run re-migrates idempotently (CredMan read is safe to repeat). Keep `internal/secrets` read-only fallback for one release. |
| PSK currently unstored (README claims otherwise) | Medium | High (existing gap) | `CredEntry.PSK` field added + encrypted now; actual PSK capture UI + `ConnectRequest`/`EnsureProfile` wiring is a separate follow-up (see Open Questions). No data loss either way. |

---

## Open Questions / Follow-ups (not in this PRD)

1. **PSK capture:** Add a PSK entry to the form, thread it through `ConnectRequest` → `ConnectParams` → `EnsureProfile` (`-L2tpPsk`), and persist via `CredEntry.PSK`. Currently absent in code despite README. This PRD only stores the field.
2. **Delete `internal/secrets` entirely:** After one release with no users on the old format, remove the migration-only read helper.
3. **Backup/export:** Optional future feature (export `vepeen.bin` with a passphrase) — out of scope.

---

## Rollback Strategy

- `vepeen.bin` is self-contained; to roll back the feature, revert the commits. Legacy `config.json` is purged only after successful migration, so a pre-migration backup of `%AppData%\vepeen/` + the exe dir is sufficient to restore the old state.
- Because migration is idempotent and CredMan purge is best-effort, reverting the binary and re-running will re-migrate from any surviving CredMan entries (if the read-only helper is still present).

---

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v0.1.0 | 2026-07-23 | Initial draft |
