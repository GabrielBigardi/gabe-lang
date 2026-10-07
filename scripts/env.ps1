# Dot-source this file: . .\scripts\env.ps1
# It changes only the current process environment and downloads nothing.
$gabeEnvironmentError = $ErrorActionPreference
try {
    $ErrorActionPreference = 'Stop'
    if ($env:GABE_CLANG) {
        Write-Host "Using configured Clang: $env:GABE_CLANG"
        return
    }
    if ($env:GABE_LLC) {
        Write-Host "Using configured LLVM llc: $env:GABE_LLC"
        return
    }
    $gabeClangCommand = Get-Command clang -ErrorAction SilentlyContinue
    if ($gabeClangCommand) {
        $env:GABE_CLANG = $gabeClangCommand.Source
        Write-Host "Using Clang: $env:GABE_CLANG"
        return
    }
    $gabeLocalClang = Join-Path $PSScriptRoot '..\.tools\llvm\bin\clang.exe'
    $gabeClangCandidates = @($gabeLocalClang)
    if ($env:ProgramFiles) {
        $gabeClangCandidates += Join-Path $env:ProgramFiles 'LLVM\bin\clang.exe'
    }
    foreach ($gabeCandidate in $gabeClangCandidates) {
        if (Test-Path -LiteralPath $gabeCandidate -PathType Leaf) {
            $env:GABE_CLANG = (Resolve-Path -LiteralPath $gabeCandidate).Path
            Write-Host "Using Clang: $env:GABE_CLANG"
            return
        }
    }
    if ($env:ProgramFiles) {
        $gabeAndroidPacks = Join-Path $env:ProgramFiles 'dotnet\packs\Microsoft.Android.Sdk.Windows'
        if (Test-Path -LiteralPath $gabeAndroidPacks) {
            $gabePackVersions = Get-ChildItem -LiteralPath $gabeAndroidPacks -Directory | Sort-Object Name -Descending
            foreach ($gabeVersion in $gabePackVersions) {
                $gabeLlcPath = Join-Path $gabeVersion.FullName 'tools\binutils\bin\llc.exe'
                if (Test-Path -LiteralPath $gabeLlcPath -PathType Leaf) {
                    $env:GABE_LLC = $gabeLlcPath
                    Write-Host "Using LLVM llc: $env:GABE_LLC"
                    return
                }
            }
        }
    }
    throw 'LLVM backend not found. Install Clang/LLVM 17+ or set GABE_CLANG or GABE_LLC to an existing executable. Frontend checks and --emit llvm work without LLVM.'
} finally {
    $ErrorActionPreference = $gabeEnvironmentError
}
