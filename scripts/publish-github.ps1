# Creates the owner's requested private repository after GitHub authentication.
# Run from any directory: .\scripts\publish-github.ps1
param([string]$Owner = '')

$ErrorActionPreference = 'Stop'
$gabeProjectRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw 'GitHub CLI (gh) is not installed. Install it from https://cli.github.com and run gh auth login, then rerun this script.'
}
Push-Location -LiteralPath $gabeProjectRoot
try {
    gh auth status
    if ($LASTEXITCODE -ne 0) { throw 'Authenticate first with gh auth login, then rerun this script.' }
    git rev-parse --verify HEAD | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'The local repository needs an initial commit before publishing.' }
    $gabeGitChanges = git status --porcelain
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read Git status.' }
    if ($gabeGitChanges) { throw 'Commit the current project changes before publishing.' }
    $gabeRemotes = @(git remote)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read Git remotes.' }
    if ($gabeRemotes -contains 'origin') {
        $gabeExistingOrigin = git remote get-url origin
        throw "An origin already exists ($gabeExistingOrigin). Review it and use git push; this script will not replace it."
    }
    if (-not $Owner) {
        $Owner = gh api user --jq '.login'
        if ($LASTEXITCODE -ne 0 -or -not $Owner) { throw 'Cannot determine the signed-in GitHub account.' }
    }
    if ($Owner -notmatch '^[A-Za-z0-9][A-Za-z0-9-]*$') { throw 'Owner must be a GitHub account or organization name.' }
    gh repo create "$Owner/gabe-lang" --private --source $gabeProjectRoot --remote origin --push --description 'Gabe: a native LLVM-backed language with C#-inspired syntax and .gb source files'
    if ($LASTEXITCODE -ne 0) { throw 'GitHub repository creation/push failed. Review the error before retrying.' }
    gh repo view "$Owner/gabe-lang" --json nameWithOwner,url,visibility
    if ($LASTEXITCODE -ne 0) { throw 'Repository was created, but the final GitHub verification failed.' }
} finally {
    Pop-Location
}
