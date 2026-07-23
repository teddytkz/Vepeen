# Fix Plan: Optional credentials pass-through to rasdial (error 734)

**Related PRD:** PRD-002 (L2TP/IPsec split tunnel)
**Severity:** High
**Reported by:** User (manual `rasdial "XXX - xxx.xxx.xxx.xxx"` reproduces 734)
**Date:** 2026-07-23

## Bug Summary

After the pivot ("Vepeen only manages routing; profiles imported from Windows"), connecting via Vepeen fails with **Remote Access error 734 (PPP LCP terminated)**. The user confirmed the same 734 when running `rasdial "XXX - xxx.xxx.xxx.xxx"` manually in CMD. Connecting via the Windows Settings VPN UI works because it prompts for credentials interactively and caches them in Windows Credential Manager.

Root cause: `manager.go` `ConnectFull` dial step calls `m.connectFn(ConnectParams{Name: name})` with **no credentials** (the `Username`/`Password` fields were removed from `ConnectRequest` during the pivot). `dial_windows.go` `Connect` then runs `rasdial <name>` with no creds, relying on Windows Credential Manager — but no creds are stored there, so PPP auth is rejected → 734.

Secondary issue: `errors.go` `MapExecError` has **no case for `734`**. A 734 is currently mislabeled as `"Gagal autentikasi"` (code `auth`) because the rasdial output contains the substring `"username or password"`. The message is misleading for a PPP-negotiation failure.

## Root Cause Analysis

- `ConnectRequest` was reduced to `{Name, RoutesText, Routes}` during the pivot; the dial step dropped creds entirely.
- `dial_windows.go` already supports optional creds (`rasdial <name>` vs `rasdial <name> <user> <pass>`), but nothing upstream supplies them.
- `MapExecError` matches `734` output against the `auth` case (substring `"username or password"`) before any PPP-specific case, so 734 is miscategorized.

## Fix Strategy

### Option A: Minimal Fix (recommended)

- Re-add optional `Username`/`Password` to `ConnectRequest`; pass them through `ConnectFull` → `ConnectParams`.
- Add a dedicated `734` case in `MapExecError` (before the `auth` case) with a clear Indonesian PPP message.
- Re-add optional credential fields in the UI (`userEntry`, `passEntry`), pass-through only — **never persisted** to `config.json` (routing-only config preserved).
- No new exported funcs; `!windows` stubs unchanged.

**Risk:** Low — additive, optional fields; no behavior change when creds left blank.
**Effort:** S

### Option B: Thorough Fix

- Option A plus auto-caching creds into Windows Credential Manager when supplied.

**Risk:** Medium — reintroduces CredMan writes the user explicitly removed ("only routing" intent); violates the routing-only model.
**Effort:** M

**Recommended:** Option A — matches the user's stated intent (import profile from Windows, only manage routing; creds optional pass-through, not persisted).

## Implementation Tasks

| Task | Agent   | Files                              | Description                                                                                       |
| ---- | ------- | ---------------------------------- | ------------------------------------------------------------------------------------------------- |
| 1    | Backend | `internal/vpn/manager.go`          | Re-add `Username string`, `Password string` to `ConnectRequest`; pass to `ConnectParams` in dial. |
| 2    | Backend | `internal/vpn/errors.go`           | Add `734` case (code `ppp`) before the `auth` case; detect via substring `"734"`.                 |
| 3    | Frontend| `internal/ui/main_window.go`       | Add `userEntry`/`passEntry`; read in `onConnect`; keep optional in `validateConnect`; no persist. |
| 4    | Backend | `internal/vpn/manager_test.go`     | Confirm compiles (creds optional; existing literals fine). No change required.                     |

## Detailed Changes

### 1. `internal/vpn/manager.go`

**Struct edit** — add optional creds to `ConnectRequest` (after `Routes`):

```go
// ConnectRequest is the full connect input from the UI layer.
type ConnectRequest struct {
	Name string
	// RoutesText is multi-line IP/CIDR list from the form.
	RoutesText string
	// Routes optional pre-parsed prefixes; if empty, RoutesText is parsed.
	Routes []string
	// Username and Password are OPTIONAL. When both empty, rasdial uses the
	// Windows-saved credentials (CredMan). When either is set, both are passed
	// to rasdial. They are never persisted by Vepeen (routing-only config).
	Username string
	Password string
}
```

**Dial step edit** — in `ConnectFull`, change:

```go
	notify(PhaseDial)
	if err := m.connectFn(ConnectParams{Name: name}); err != nil {
		return nil, err
	}
```

to:

```go
	notify(PhaseDial)
	if err := m.connectFn(ConnectParams{
		Name:     name,
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		return nil, err
	}
```

Validation stays unchanged: creds remain optional; only `name` + ≥1 route are required.

### 2. `internal/vpn/errors.go`

**Add a `734` case BEFORE the `auth` case** in `MapExecError`'s `switch`. Insert after the `elevation` case and before the `auth` case:

```go
	case strings.Contains(lower, "734"):
		return newUserError("ppp",
			"Gagal negosiasi PPP (error 734)",
			"Server memutus koneksi saat verifikasi. Pastikan username/password benar dan tersimpan di Windows Credential Manager, atau isi kredensial di Vepeen. Periksa juga metode autentikasi (MS-CHAPv2) dan enkripsi di profil Windows.")
```

This must precede the `auth` case so that `734` (whose rasdial text also contains `"username or password"`) is matched as PPP, not auth.

### 3. `internal/ui/main_window.go`

**Struct edit** — add fields to `controller` (near `routesEntry`):

```go
	profileSelect *widget.Select
	routesEntry   *widget.Entry
	userEntry     *widget.Entry
	passEntry     *widget.PasswordEntry
	logEntry      *widget.Entry
```

**build() edit** — add credential entries under the profile selector inside `cardKoneksi`. After `c.profileSelect` is created and before/within the `cardKoneksi` VBox:

```go
	c.userEntry = widget.NewEntry()
	c.userEntry.PlaceHolder = "Username (opsional)"
	c.passEntry = widget.NewPasswordEntry()
	c.passEntry.PlaceHolder = "Password (opsional — kosongkan untuk pakai kredensial Windows)"

	credNote := widget.NewLabel("Kosongkan username & password untuk menggunakan kredensial Windows (Credential Manager).")
	credNote.Wrapping = fyne.TextWrapWord

	cardKoneksi := widget.NewCard("Koneksi VPN", "",
		container.NewVBox(
			widget.NewLabel("Pilih profil VPN yang sudah ada di Windows."),
			c.profileSelect,
			widget.NewSeparator(),
			widget.NewLabel("Kredensial (opsional)"),
			c.userEntry,
			c.passEntry,
			credNote,
		),
	)
```

**onConnect edit** — build request with creds (trim username; password passed as-is):

```go
	req := vpn.ConnectRequest{
		Name:       name,
		Username:   strings.TrimSpace(c.userEntry.Text),
		Password:   c.passEntry.Text,
		RoutesText: c.routesEntry.Text,
	}
```

**validateConnect edit** — NO new requirement for creds. Keep existing checks (selected profile + ≥1 route). Optionally add a soft warning only if exactly one of user/pass is filled (the dial layer already returns a clear `validation` error, so this is optional; leave validation as-is to stay minimal).

**applyEnablement edit** — enable/disable the new entries with the form. In the `setEntry(c.routesEntry, formEnabled)` area, add:

```go
	setEntry(c.userEntry, formEnabled)
	setEntry(c.passEntry, formEnabled)
```

**persistQuiet / onSave** — UNCHANGED. Do NOT write `Username`/`Password` to `config.json`. Config stays routing-only (`SelectedProfile` + per-profile `Routes`). Do NOT re-add the `secrets` import or CredMan writes.

### 4. `internal/vpn/manager_test.go`

No change required. `ConnectRequest` literals without `Username`/`Password` still compile (fields optional). `TestConnectFull_SyncRoutesErrorIsBestEffort` and `TestConnectFull_EmptyRoutesBlocked` remain valid. Confirm `go test ./internal/vpn/...` compiles and passes.

## Connect Flow (post-fix)

```
onConnect (UI)
  └─ validateConnect: profile selected + ≥1 route (creds optional)
  └─ ConnectFull(ctx, req{Name, Username, Password, RoutesText})
       1. PhaseNATCheck      → EnsureNATRegistry (best-effort, warn)
       2. parse routes       → ≥1 required (else validation error)
       3. PhaseDisconnectOthers → DisconnectAllExcept(name) (best-effort)
       4. PhaseSyncRoutes    → SyncRoutes (best-effort, warn, continue)
       5. PhaseDial          → Connect(ConnectParams{Name, Username, Password})
            • both empty  → rasdial <name>            (uses Windows CredMan)
            • either set  → rasdial <name> <user> <pass>
       6. PhaseSplitEnforce  → EnforceSplitTunnel(name) (best-effort)
       7. PhaseDone          → warnings returned to UI
  └─ MapExecError maps 734 → code "ppp" (clear Indonesian PPP message)
```

## Acceptance Criteria

- [ ] `ConnectRequest` has optional `Username`/`Password`; `ConnectFull` passes them to `ConnectParams`.
- [ ] When creds are blank, connect runs `rasdial <name>` (CredMan fallback) — unchanged behavior.
- [ ] When creds are filled, connect runs `rasdial <name> <user> <pass>` and succeeds where CredMan was empty (734 resolved).
- [ ] `MapExecError` returns code `ppp` / "Gagal negosiasi PPP (error 734)" for a 734 (not `auth`).
- [ ] UI has optional `userEntry`/`passEntry` under the profile selector, clearly marked optional; they are enabled/disabled with the form.
- [ ] Creds are NOT persisted to `config.json`; `secrets` import not re-added; config remains routing-only.
- [ ] `go build ./...` (Windows, CGO) succeeds; `go vet ./internal/ui/...` clean; `go test ./internal/vpn/...` passes.

## Regression Risk

- `!windows` stubs (`stub_other.go`) unchanged — no new exported symbols, so cross-compile unaffected.
- `dial_windows.go` already handles the both-empty vs either-set branches; no change there.
- `MapExecError` ordering: the new `734` case is inserted before `auth` so existing 691/auth behavior is preserved.
- `persistQuiet`/`onSave` untouched → no secret leakage into config.

## Rollback Strategy

Revert the three file edits (`manager.go`, `errors.go`, `main_window.go`) via git. No schema/config migration; `config.json` shape unchanged.
