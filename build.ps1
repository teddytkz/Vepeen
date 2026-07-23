$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"

# --- Windows application manifest (Per-Monitor v2 DPI) -----------------------
# The manifest is embedded as a compiled resource (rsrc.syso) so the linker
# merges an RT_MANIFEST into bin/vepeen.exe. This declares dpiAwareness =
# "PerMonitorV2, PerMonitor" (crisp on HiDPI, no DPI-virtualization flash) and
# uiAccess=false, while keeping the GUI subsystem (no console).
#
# One-time tool install (already done on this machine):
#   go install github.com/akavel/rsrc@latest
#
# To regenerate the .syso after editing vepeen.exe.manifest:
#   rsrc -manifest vepeen.exe.manifest -o cmd/vepeen/rsrc.syso
#
# The committed cmd/vepeen/rsrc.syso is picked up automatically by `go build`,
# so no extra step is required for a normal build.
if (-not (Test-Path cmd/vepeen/rsrc.syso)) {
    if (Get-Command rsrc -ErrorAction SilentlyContinue) {
        rsrc -manifest vepeen.exe.manifest -o cmd/vepeen/rsrc.syso
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } else {
        Write-Warning "rsrc not found and cmd/vepeen/rsrc.syso is missing; manifest will not be embedded. Install with: go install github.com/akavel/rsrc@latest"
    }
}

go build -ldflags="-H windowsgui" -o bin/vepeen.exe ./cmd/vepeen
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built vepeen.exe (GUI subsystem, no console, manifest embedded)."

# Plain alternative (keeps a console window for debugging):
#   go build -o vepeen.exe ./cmd/vepeen
