$ErrorActionPreference = 'Stop'
$gabeProjectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $gabeProjectRoot
try {
    . (Join-Path $PSScriptRoot 'env.ps1')
    go run ./tools/checkfmt
    if ($LASTEXITCODE -ne 0) { throw 'Go formatting check failed.' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    $env:GABE_REQUIRE_NATIVE_TESTS = '1'
    go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Compiler or native tests failed.' }
    go build -trimpath -o bin/ ./cmd/gabe
    if ($LASTEXITCODE -ne 0) { throw 'Compiler build failed.' }
    Write-Host 'Formatting, lint, frontend tests, and required native tests passed.'
} finally {
    Pop-Location
}
