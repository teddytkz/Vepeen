$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"
go build -ldflags="-H windowsgui" -o bin/vepeen.exe ./cmd/vepeen
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built vepeen.exe (GUI subsystem, no console)."

# Plain alternative (keeps a console window for debugging):
#   go build -o vepeen.exe ./cmd/vepeen
