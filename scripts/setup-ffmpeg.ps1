[CmdletBinding()]
param(
    [switch]$Force
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repoRoot = Split-Path -Parent $PSScriptRoot
$targetDir = Join-Path $repoRoot "third_party/ffmpeg"
$ffmpegPath = Join-Path $targetDir "ffmpeg.exe"
$assetName = "ffmpeg-master-latest-win64-lgpl.zip"
$releaseApi = "https://api.github.com/repos/BtbN/FFmpeg-Builds/releases/tags/latest"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("aurora-ffmpeg-" + [guid]::NewGuid().ToString("N"))

function Write-Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

try {
    if ($env:OS -ne "Windows_NT") {
        throw "This setup script downloads the Windows x64 FFmpeg executable and must be run on Windows."
    }

    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        throw "Git was not found. Install Git for Windows, then run this script again."
    }

    $lfsVersion = & git -C $repoRoot lfs version 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Git LFS is required. Install Git LFS (https://git-lfs.com/), run 'git lfs install', then retry."
    }
    & git -C $repoRoot lfs install --local
    if ($LASTEXITCODE -ne 0) {
        throw "Could not initialize Git LFS for this repository."
    }
    Write-Host "Git LFS detected: $lfsVersion"

    if ((Test-Path $ffmpegPath) -and -not $Force) {
        throw "FFmpeg already exists at '$ffmpegPath'. Use -Force to replace it."
    }

    New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null

    Write-Step "Finding the current BtbN Windows x64 LGPL static build"
    $headers = @{
        "User-Agent" = "Aurora-Player-FFmpeg-Setup"
        "Accept"     = "application/vnd.github+json"
    }
    $release = Invoke-RestMethod -Uri $releaseApi -Headers $headers
    $archiveAsset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
    $checksumAsset = $release.assets | Where-Object { $_.name -eq "checksums.sha256" } | Select-Object -First 1

    if (-not $archiveAsset -or -not $checksumAsset) {
        throw "The latest BtbN release did not contain the expected LGPL archive and checksums.sha256. No executable was installed."
    }

    $archivePath = Join-Path $tempRoot $assetName
    $checksumsPath = Join-Path $tempRoot "checksums.sha256"
    Write-Host "Release: $($release.html_url)"
    Write-Host "Asset:   $($archiveAsset.name) ($([math]::Round($archiveAsset.size / 1MB, 1)) MiB)"

    Write-Step "Downloading the archive and its checksum list"
    Invoke-WebRequest -Uri $archiveAsset.browser_download_url -Headers $headers -OutFile $archivePath
    Invoke-WebRequest -Uri $checksumAsset.browser_download_url -Headers $headers -OutFile $checksumsPath

    $checksumText = Get-Content -LiteralPath $checksumsPath -Raw
    $expectedHash = $null
    foreach ($line in ($checksumText -split "\r?\n")) {
        if ($line -match '^\s*([0-9a-fA-F]{64})\s+\*?(.+?)\s*$') {
            $listedName = [System.IO.Path]::GetFileName($Matches[2].Trim())
            if ($listedName -eq $assetName) {
                $expectedHash = $Matches[1].ToLowerInvariant()
                break
            }
        }
    }
    if (-not $expectedHash) {
        throw "Could not find a SHA-256 entry for '$assetName' in the release checksum file."
    }

    Write-Step "Verifying the archive checksum"
    $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        throw "SHA-256 verification failed. Expected $expectedHash; received $actualHash. The archive will not be installed."
    }
    Write-Host "SHA-256 verified: $actualHash" -ForegroundColor Green

    Write-Step "Extracting FFmpeg"
    $extractDir = Join-Path $tempRoot "extracted"
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force
    $ffmpegSource = Get-ChildItem -LiteralPath $extractDir -Filter "ffmpeg.exe" -File -Recurse |
        Select-Object -First 1
    if (-not $ffmpegSource) {
        throw "The verified archive did not contain ffmpeg.exe."
    }

    $stagingExe = Join-Path $tempRoot "ffmpeg.exe"
    Copy-Item -LiteralPath $ffmpegSource.FullName -Destination $stagingExe -Force

    Write-Step "Checking the downloaded executable"
    $versionOutput = & $stagingExe -hide_banner -version 2>&1
    if ($LASTEXITCODE -ne 0 -or -not $versionOutput) {
        throw "The downloaded ffmpeg.exe did not pass its version check."
    }
    $versionOutput | Select-Object -First 2 | ForEach-Object { Write-Host $_ }

    Write-Step "Installing FFmpeg and retaining its license files"
    Copy-Item -LiteralPath $stagingExe -Destination $ffmpegPath -Force

    $licenseDir = Join-Path $targetDir "licenses"
    if (Test-Path $licenseDir) {
        Remove-Item -LiteralPath $licenseDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $licenseDir -Force | Out-Null

    $licenseFiles = Get-ChildItem -LiteralPath $extractDir -File -Recurse |
        Where-Object { $_.Name -match '^(LICENSE|COPYING|NOTICE|README)' }
    foreach ($file in $licenseFiles) {
        $relative = $file.FullName.Substring($extractDir.Length).TrimStart([char]'\', [char]'/')
        $destination = Join-Path $licenseDir $relative
        $destinationParent = Split-Path -Parent $destination
        New-Item -ItemType Directory -Path $destinationParent -Force | Out-Null
        Copy-Item -LiteralPath $file.FullName -Destination $destination -Force
    }

    @"
FFmpeg source: $($release.html_url)
Asset: $($archiveAsset.name)
SHA-256: $actualHash
Downloaded: $([DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ"))
Build variant: BtbN Windows x64 LGPL static

Review the included license files and the exact FFmpeg build configuration before redistribution.
The LGPL label alone is not a substitute for checking the applicable license obligations.
"@ | Set-Content -LiteralPath (Join-Path $targetDir "BUILD-INFO.txt") -Encoding UTF8

    Write-Step "Complete"
    Write-Host "Installed: $ffmpegPath" -ForegroundColor Green
    Write-Host "License files: $licenseDir"
    Write-Host ""
    Write-Host "The repository tracks ffmpeg.exe with Git LFS. To upload this resource:"
    Write-Host "  git add third_party/ffmpeg/ffmpeg.exe third_party/ffmpeg/licenses third_party/ffmpeg/BUILD-INFO.txt"
    Write-Host "  git commit -m 'Add FFmpeg runtime via Git LFS'"
    Write-Host "  git push"
}
finally {
    if (Test-Path $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
