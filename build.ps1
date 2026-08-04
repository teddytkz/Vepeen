$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"

# --- Windows resources: icon + Per-Monitor v2 DPI manifest -------------------
# Resources are embedded as compiled .syso files so the linker merges an
# RT_GROUP_ICON and RT_MANIFEST into bin/vepeen.exe:
#   - Icon:     winres/penelope.png  → all standard sizes (256…16 px)
#   - Manifest: dpiAwareness = "per monitor v2", uiAccess = false
#
# One-time tool install:
#   go install github.com/tc-hib/go-winres@latest
#
# To regenerate after changing winres/winres.json or winres/penelope.png:
#   go-winres make --in winres/winres.json --out cmd/vepeen/rsrc
# This produces cmd/vepeen/rsrc_windows_amd64.syso (and _386.syso), which
# go build picks up automatically — no extra step for a normal build.
if (-not (Test-Path cmd/vepeen/rsrc_windows_amd64.syso)) {
    if (Get-Command go-winres -ErrorAction SilentlyContinue) {
        go-winres make --in winres/winres.json --out cmd/vepeen/rsrc
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } else {
        Write-Warning "go-winres not found and rsrc_windows_amd64.syso is missing; icon + manifest will not be embedded. Install with: go install github.com/tc-hib/go-winres@latest"
    }
}

go build -ldflags="-H windowsgui" -o bin/vepeen.exe ./cmd/vepeen
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built vepeen.exe (GUI subsystem, no console, manifest embedded)."

# Plain alternative (keeps a console window for debugging):
#   go build -o vepeen.exe ./cmd/vepeen
