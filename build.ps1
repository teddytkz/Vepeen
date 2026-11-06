$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "1"

# ponytail: skipped go-winres check, .syso files already committed
# regenerate with: go-winres make --in winres/winres.json --out cmd/vepeen/rsrc

# Newer mingw-w64 (WinLibs GCC 16+) auto-links default-manifest.o, which collides with our
# .syso manifest ("multiple non-default manifests"). An empty stub found first via -B disables it.
$stub = Join-Path $PWD "bin/nodefmanifest"
New-Item -ItemType Directory -Force $stub | Out-Null
"" | gcc -c -x c - -o (Join-Path $stub "default-manifest.o")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$B = $stub -replace '\\', '/'

go build -ldflags="-H windowsgui -extldflags=-B$B" -o bin/vepeen.exe ./cmd/vepeen
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built vepeen.exe (GUI subsystem, no console, manifest embedded)."

# Plain alternative (keeps a console window for debugging):
#   go build -o vepeen.exe ./cmd/vepeen
