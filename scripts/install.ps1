# Install sse from a release directory (same layout as scripts/dist-archives.sh).
# Requires PowerShell 5.1+ and tar (Windows 10+).
#
# Remote:
#   .\install.ps1 -BaseUrl "https://downloads.example.com/sse/v1.0.0" -Version "1.0.0"
#
# Local folder (e.g. after VERSION=1.0.0 make dist-archives):
#   .\install.ps1 -LocalDist ".\dist" -Version "1.0.0"
#
param(
    [Parameter(ParameterSetName = "Remote", Mandatory = $true)]
    [string]$BaseUrl,
    [Parameter(ParameterSetName = "Local", Mandatory = $true)]
    [string]$LocalDist,
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$InstallDir = $(Join-Path $env:USERPROFILE ".local\bin")
)

$ErrorActionPreference = "Stop"

$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    ([System.Runtime.InteropServices.Architecture]::X64) { "amd64" }
    ([System.Runtime.InteropServices.Architecture]::Arm64) { "arm64" }
    default { throw "Unsupported CPU: $_" }
}

$archive = "sse_${Version}_windows_${arch}.tar.gz"
$sumfile = "SHA256SUMS"

$tmp = Join-Path $env:TEMP ("sse-install-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    if ($PSCmdlet.ParameterSetName -eq "Local") {
        $local = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($LocalDist)
        Copy-Item (Join-Path $local $archive) (Join-Path $tmp $archive)
        Copy-Item (Join-Path $local $sumfile) (Join-Path $tmp $sumfile)
    }
    else {
        $base = $BaseUrl.TrimEnd("/")
        Invoke-WebRequest -Uri "$base/$archive" -OutFile (Join-Path $tmp $archive) -UseBasicParsing
        Invoke-WebRequest -Uri "$base/$sumfile" -OutFile (Join-Path $tmp $sumfile) -UseBasicParsing
    }

    $sumPath = Join-Path $tmp $sumfile
    $line = Get-Content $sumPath | Where-Object { $_ -match [regex]::Escape($archive) } | Select-Object -First 1
    if (-not $line) { throw "$archive not found in $sumfile" }
    $want = ($line -split '\s+', 2)[0].ToLowerInvariant()
    $got = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $tmp $archive)).Hash.ToLowerInvariant()
    if ($got -ne $want) { throw "Checksum mismatch for $archive" }

    Push-Location $tmp
    try {
        & tar -xzf $archive
    }
    finally {
        Pop-Location
    }

    $src = Join-Path $tmp "sse.exe"
    if (-not (Test-Path $src)) { throw "Archive did not contain sse.exe" }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $dest = Join-Path $InstallDir "sse.exe"
    Copy-Item -Force $src $dest

    Write-Host "Installed sse to $dest"
    if ($env:PATH -notlike "*$InstallDir*") {
        Write-Host "Add to PATH, e.g.: `$env:PATH = '$InstallDir;' + `$env:PATH"
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
