$ErrorActionPreference = 'Stop'
$gabeProjectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $gabeProjectRoot
try {
    . (Join-Path $PSScriptRoot 'env.ps1')
    cargo fmt --all -- --check
    if ($LASTEXITCODE -ne 0) { throw 'Rust formatting check failed.' }
    cargo clippy --workspace --all-targets --locked -- -D warnings
    if ($LASTEXITCODE -ne 0) { throw 'Clippy failed.' }
    $env:GABE_REQUIRE_NATIVE_TESTS = '1'
    cargo test --workspace --locked
    if ($LASTEXITCODE -ne 0) { throw 'Compiler or native tests failed.' }
    Write-Host 'Formatting, lint, frontend tests, and required native tests passed.'
} finally {
    Pop-Location
}
