# Fix Plan: COM Apartment Refcount Imbalance in CreateDesktopShortcut

**Related PRD:** PRD-001 (Golang/Fyne starter)
**Severity:** Medium (can cause downstream `RPC_E_DISCONNECTED` / `CO_E_NOTINITIALIZED` on Fyne's UI thread)
**Reported by:** Code review of `internal/ui/desktop_shortcut_windows.go`
**Date:** 2026-07-24

---

## Bug Summary

In `CreateDesktopShortcut()` → `createShellLink()`, `CoInitializeEx` is called and
`CoUninitialize` is **deferred unconditionally**. `CoInitializeEx` can return
`S_FALSE` (value `1`) meaning "COM was already initialized on this thread by another
caller." On that success path the current code still defers `CoUninitialize`, which
decrements the apartment refcount and can tear down COM for the actual owner (e.g.
Fyne's UI thread), causing downstream `RPC_E_DISCONNECTED` / `CO_E_NOTINITIALIZED`
failures.

Current code (lines ~130–136 of `internal/ui/desktop_shortcut_windows.go`):

```go
	// S_OK=0 (initialized), S_FALSE=1 (already initialized on this thread).
	// Any other value is a fatal failure — do not balance with CoUninitialize.
	if r != 0 && r != 1 {
		return fmt.Errorf("CoInitializeEx failed (hresult 0x%x)", uint32(r))
	}
	defer procCoUninitialize.Call()
```

The `defer procCoUninitialize.Call()` runs on **both** the `S_OK` (0) and `S_FALSE` (1)
paths. The `S_FALSE` path must NOT uninitialize.

---

## Root Cause Analysis

COM uses a per-thread apartment reference count. `CoInitializeEx` returns:
- `S_OK` (0) — this caller initialized the apartment; it owns the matching `CoUninitialize`.
- `S_FALSE` (1) — the apartment was already initialized by another caller on this thread;
  this caller must NOT call `CoUninitialize` (doing so decrements the owner's refcount).
- Any other HRESULT — fatal failure; COM is not available.

The current code only skips the error return on `S_FALSE` but still defers the
uninitialize, breaking the refcount balance whenever COM was already initialized
(typical when invoked from Fyne's UI thread, which already initialized COM).

---

## Fix Strategy

### Option A: Minimal Fix (recommended)

- File: `internal/ui/desktop_shortcut_windows.go`
- Risk: Low — narrow, well-understood change; preserves existing behavior on `S_OK` and error paths.
- Effort: S

Replace the current logic with refcount-correct balancing: only defer
`CoUninitialize` when the caller actually initialized COM (`S_OK` = 0). On `S_FALSE`
(1), proceed without deferring. On any other non-zero HRESULT, return an error.

**Exact change** (replace the block at lines ~130–136):

```go
	r, _, _ := procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED|COINIT_DISABLE_OLE1DDE)
	if r == 0 {
		defer procCoUninitialize.Call()
	} else if r != 1 { // S_FALSE = already initialized, do not uninitialize
		return fmt.Errorf("CoInitializeEx failed (hresult 0x%x)", uint32(r))
	}
```

Note: the local constants in this file are `coinitApartmentThreaded` and
`coinitDisableOle1Dde` (not `COINIT_APARTMENTTHREADED` / `COINIT_DISABLE_OLE1DDE`).
Apply the fix using the file's existing identifiers:

```go
	r, _, _ := procCoInitializeEx.Call(0, uintptr(coinitApartmentThreaded|coinitDisableOle1Dde))
	if r == 0 {
		defer procCoUninitialize.Call()
	} else if r != 1 { // S_FALSE = already initialized, do not uninitialize
		return fmt.Errorf("CoInitializeEx failed (hresult 0x%x)", uint32(r))
	}
```

**Recommended:** Option A — minimal, correct, no behavioral change on the `S_OK`/error paths.

---

## Implementation Tasks

| Task | Agent   | Files                                | Description                                                                 |
| ---- | ------- | ------------------------------------ | --------------------------------------------------------------------------- |
| 1    | Frontend Developer | `internal/ui/desktop_shortcut_windows.go` | Replace the unconditional `defer procCoUninitialize.Call()` block with the refcount-balanced version above (using the file's `coinitApartmentThreaded`/`coinitDisableOle1Dde` constants). |

---

## Acceptance Criteria

- [ ] `CoUninitialize` is deferred **only** when `CoInitializeEx` returned `S_OK` (0).
- [ ] On `S_FALSE` (1), no `CoUninitialize` is deferred, but shortcut creation proceeds normally.
- [ ] On any other non-zero HRESULT, `createShellLink` returns the formatted error.
- [ ] `go build ./...` passes.
- [ ] `go vet ./internal/ui/...` passes.

## Regression Risk

Low. The change only affects the COM teardown path. The `S_OK` success path and the
error path are functionally unchanged. The only behavioral change is that on the
`S_FALSE` path COM is no longer incorrectly torn down — which fixes, rather than
risks, downstream COM usage on the calling thread.
