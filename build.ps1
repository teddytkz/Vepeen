$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"

# ponytail: skipped go-winres check, .syso files already committed
# regenerate with: go-winres make --in winres/winres.json --out cmd/vepeen/rsrc

go build -ldflags="-H windowsgui" -o bin/vepeen.exe ./cmd/vepeen
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built vepeen.exe (GUI subsystem, no console, manifest embedded)."

# Plain alternative (keeps a console window for debugging):
#   go build -o vepeen.exe ./cmd/vepeen
