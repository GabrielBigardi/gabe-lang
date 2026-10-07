# Finish initial Git setup in a terminal with write access to .git.
# Add -Publish to create/push the requested private GitHub repository using gh.
param([switch]$Publish, [string]$Owner = '')

$ErrorActionPreference = 'Stop'
$gabeProjectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $gabeProjectRoot
try {
    if (-not (Test-Path -LiteralPath (Join-Path $gabeProjectRoot '.git'))) {
        git init -b main
        if ($LASTEXITCODE -ne 0) { throw 'Cannot initialize the Git repository.' }
    }
    $gabeGitRoot = git rev-parse --show-toplevel
    if ($LASTEXITCODE -ne 0 -or (Resolve-Path -LiteralPath $gabeGitRoot).Path -ne $gabeProjectRoot) {
        throw 'Git repository root does not match the Gabe project directory.'
    }
    git rev-parse --verify --quiet HEAD | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Host 'An initial commit already exists; no new commit was made.'
    } else {
        git add --all
        if ($LASTEXITCODE -ne 0) { throw 'Cannot stage files. Run this script in a terminal with write access to .git.' }
        git update-index --chmod=+x scripts/check.sh
        if ($LASTEXITCODE -ne 0) { throw 'Cannot set the validation script executable bit.' }
        git diff --cached --check
        if ($LASTEXITCODE -ne 0) { throw 'Staged files failed the whitespace check.' }
        git commit -m 'Bootstrap Gabe with a typed frontend and LLVM native backend'
        if ($LASTEXITCODE -ne 0) { throw 'Initial commit failed. Check Git write permissions and user.name/user.email configuration.' }
    }
    if ($Publish) {
        & (Join-Path $PSScriptRoot 'publish-github.ps1') -Owner $Owner
    }
} finally {
    Pop-Location
}
