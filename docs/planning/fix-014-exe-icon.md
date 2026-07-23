# Fix Plan: Embed App Icon into vepeen.exe (Windows Explorer / Taskbar / Title Bar)

**Related PRD:** PRD-001 (Golang/Fyne starter)
**Severity:** Low (cosmetic — no functional regression possible)
**Reported by:** User observation
**Date:** 2026-07-23

---

## Bug Summary

`a.SetIcon(ui.PenelopeIcon)` only affects the Fyne in-process window icon and the
system-tray icon.  It does **not** embed an icon resource into the `.exe` binary itself,
so Windows Explorer, the taskbar button, and the title bar all show the generic Go
application icon.

---

## Root Cause Analysis

A Windows `.exe` icon is an `RT_ICON` / `RT_GROUP_ICON` resource compiled into the PE
binary.  Go's toolchain does not add one automatically.  The existing
`cmd/vepeen/rsrc.syso` (1 132 bytes) contains **only** a `RT_MANIFEST` (Per-Monitor v2
DPI), compiled via:

```
rsrc -manifest vepeen.exe.manifest -o cmd/vepeen/rsrc.syso
```

The `-ico` flag was never passed, so no icon resource was embedded.  Additionally,
`vepeen.exe.manifest` was not committed to the repo (only `rsrc.syso` was), so the
regeneration command documented in `build.ps1` would fail without it.

---

## Fix Strategy

### Option A — `fyne package` (recommended, one-liner)

`fyne package` converts `FyneApp.toml`'s `Icon = "docs/images/penelope.png"` to a
multi-size `.ico`, generates a manifest, and produces a fully-packaged `.exe` in one
step.  No source changes required.

```powershell
# From repo root — install fyne CLI once if needed
go install fyne.io/fyne/v2/cmd/fyne@v2.8.0

fyne package -os windows -name Vepeen -appID com.vepeen.app `
             -icon docs/images/penelope.png -release
# Produces: Vepeen.exe in the current directory (move to bin/ as desired)
```

**Limitation:** output lands in CWD as `Vepeen.exe` (not `bin/vepeen.exe`); rename
manually or wrap in `build.ps1`.  Does not honour `-ldflags="-H windowsgui"` from
`build.ps1` (it sets the GUI subsystem itself).

---

### Option B — Manual rsrc path (preserves existing `go build` workflow) ← **RECOMMENDED for this repo**

Four concrete steps:

1. **Recreate `vepeen.exe.manifest`** (was never committed).
2. **Produce `docs/images/penelope.ico`** from the existing PNG.
3. **Regenerate `cmd/vepeen/rsrc.syso`** with both manifest + ICO.
4. **Update `build.ps1`** regeneration command to pass `-ico`.

---

## Implementation Tasks (Option B)

### Task 1 — Recreate `vepeen.exe.manifest`

**Agent:** Frontend Developer  
**File:** `vepeen.exe.manifest` (repo root — `.gitignore` must NOT exclude it)

Create this file verbatim.  It is the standard Per-Monitor v2 DPI + UAC manifest for a
Windows GUI app:

```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity
    version="1.0.0.0"
    processorArchitecture="amd64"
    name="com.vepeen.app"
    type="win32"/>
  <description>Vepeen VPN Client</description>

  <!-- UAC: run as normal user, no elevation -->
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>

  <!-- Per-Monitor v2 DPI awareness (crisp on HiDPI, no virtualization) -->
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">
        PerMonitorV2, PerMonitor
      </dpiAwareness>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">
        true/PM
      </dpiAware>
    </windowsSettings>
  </application>

  <!-- Windows 10/11 compatibility -->
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/><!-- Win10/11 -->
      <supportedOS Id="{1f676c76-80e1-4239-95bb-83d0f6d0da78}"/><!-- Win8.1  -->
    </application>
  </compatibility>
</assembly>
```

> **Note:** This is functionally identical to the manifest that was originally used to
> produce the committed `rsrc.syso` — the DPI and UAC settings are unchanged.

---

### Task 2 — Produce `docs/images/penelope.ico`

**Agent:** Frontend Developer  
**File:** `docs/images/penelope.ico`

Use the `png2ico` Go tool (no external dependencies, cross-platform):

```powershell
# Install once
go install github.com/mat/besticon/ico/cmd/png2ico@latest

# Convert — produces a multi-size ICO (16, 32, 48, 256 px all from penelope.png)
png2ico docs/images/penelope.ico docs/images/penelope.png
```

**Alternative — ImageMagick** (if already installed):

```powershell
magick docs/images/penelope.png `
       -define icon:auto-resize="256,48,32,16" `
       docs/images/penelope.ico
```

**Alternative — PowerShell + System.Drawing** (no extra tools):

```powershell
# Requires .NET Framework / .NET 5+ with System.Drawing
Add-Type -AssemblyName System.Drawing
$bmp = [System.Drawing.Bitmap]::new("docs\images\penelope.png")
# ... (manual multi-size ICO writing — skip in favour of png2ico)
```

> Prefer `png2ico` — it is a single `go install` with no native deps and produces
> correct multi-size ICO files that `rsrc` accepts.

---

### Task 3 — Regenerate `cmd/vepeen/rsrc.syso`

**Agent:** Frontend Developer  
**File:** `cmd/vepeen/rsrc.syso` (overwrite existing 1 132-byte manifest-only file)

```powershell
# Ensure rsrc is installed (already documented in build.ps1)
go install github.com/akavel/rsrc@latest

# From repo root — embed BOTH manifest AND icon
rsrc -manifest vepeen.exe.manifest `
     -ico docs/images/penelope.ico `
     -o cmd/vepeen/rsrc.syso
```

Expected result: `cmd/vepeen/rsrc.syso` grows from ~1 KB to ~200–300 KB (the icon
resource dominates, with four sizes totalling ~200 KB raw).

Commit the new `rsrc.syso`.

---

### Task 4 — Update `build.ps1` regeneration command

**Agent:** Frontend Developer  
**File:** `build.ps1`

Two changes:

1. The comment block (lines 13–15) documents the old regeneration command — update it to
   include the `-ico` flag.
2. The conditional `rsrc` invocation (line 21) also lacks `-ico` — update it so that
   when `rsrc.syso` is missing the tool regenerates it correctly (with icon).

**Before (comment, line ~14):**
```
#   rsrc -manifest vepeen.exe.manifest -o cmd/vepeen/rsrc.syso
```

**After:**
```
#   rsrc -manifest vepeen.exe.manifest -ico docs/images/penelope.ico -o cmd/vepeen/rsrc.syso
```

**Before (conditional, line ~21):**
```powershell
        rsrc -manifest vepeen.exe.manifest -o cmd/vepeen/rsrc.syso
```

**After:**
```powershell
        rsrc -manifest vepeen.exe.manifest -ico docs/images/penelope.ico -o cmd/vepeen/rsrc.syso
```

---

## Acceptance Criteria

- [ ] `docs/images/penelope.ico` exists and opens correctly in Windows Photo Viewer /
      Paint (shows teal face at multiple zoom levels).
- [ ] `vepeen.exe.manifest` exists in repo root and `rsrc` accepts it without error.
- [ ] `cmd/vepeen/rsrc.syso` is ≥ 50 KB (icon data present).
- [ ] `.\build.ps1` succeeds (`go build` exit 0, `bin/vepeen.exe` produced).
- [ ] `bin/vepeen.exe` shows the Penelope icon in Windows Explorer (Details view and
      Large Icons view).
- [ ] `bin/vepeen.exe` shows the Penelope icon in the taskbar while running.
- [ ] `bin/vepeen.exe` shows the Penelope icon in the title bar while running.
- [ ] No console window appears when launching `bin/vepeen.exe` (GUI subsystem
      unchanged).
- [ ] `go build ./...` continues to pass (no import/compilation errors).

---

## Regression Risk

Very low — this is a PE resource change only.  The Go compiler and linker treat `.syso`
files as opaque binary resource objects to merge into the final `.exe`; the Go source
code is unaffected.  The only realistic regressions are:

| Risk | Mitigation |
|------|------------|
| Corrupted `.ico` causes `rsrc` to fail | Verify `.ico` opens in Paint before running `rsrc` |
| Wrong manifest recreated (DPI settings differ) | Content above is identical to the original; verify with `manifest` resource viewer |
| `rsrc.syso` committed at old size → stale | CI / reviewer checks file size ≥ 50 KB |

---

## Rollback Strategy

Revert `cmd/vepeen/rsrc.syso` to the previous 1 132-byte commit:
```powershell
git checkout HEAD~1 -- cmd/vepeen/rsrc.syso
```
Delete `docs/images/penelope.ico` and `vepeen.exe.manifest` if not desired.
`go build` will continue to work (manifest-only embed = current behaviour).
