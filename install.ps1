# Vecto Windows Installer
# Usage: irm https://raw.githubusercontent.com/Sehaan-1/vecto/master/install.ps1 | iex

$ErrorActionPreference = "Stop"

function Write-VectoInfo {
    param([string]$Message)
    Write-Host "[-] $Message" -ForegroundColor Cyan
}

function Write-VectoSuccess {
    param([string]$Message)
    Write-Host "[✓] $Message" -ForegroundColor Green
}

function Write-VectoError {
    param([string]$Message)
    Write-Host "[x] $Message" -ForegroundColor Red
}

try {
    # 1. Architecture detection
    $arch = if ([System.Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    } else {
        throw "Unsupported architecture. Vecto requires a 64-bit OS (amd64 or arm64)."
    }

    # 2. Determine Version
    $version = $env:VECTO_VERSION
    if (-not $version) {
        Write-VectoInfo "Fetching latest release information..."
        $releaseUrl = "https://api.github.com/repos/Sehaan-1/vecto/releases/latest"
        $release = Invoke-RestMethod -Uri $releaseUrl -Headers @{ "User-Agent" = "vecto-installer" }
        $version = $release.tag_name
    }

    # Normalize version: tag may be 'v0.2.0', binary archives use '0.2.0'
    $tag = if ($version.StartsWith("v")) { $version } else { "v$version" }
    $cleanVersion = if ($version.StartsWith("v")) { $version.Substring(1) } else { $version }

    Write-VectoInfo "Installing Vecto $tag for windows/$arch..."

    $assetName = "vecto_${cleanVersion}_windows_${arch}.zip"
    $downloadUrl = "https://github.com/Sehaan-1/vecto/releases/download/$tag/$assetName"
    $checksumsUrl = "https://github.com/Sehaan-1/vecto/releases/download/$tag/checksums.txt"

    $tempDir = Join-Path $env:TEMP "vecto-install-$([System.Guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    $zipPath = Join-Path $tempDir $assetName
    $checksumsPath = Join-Path $tempDir "checksums.txt"

    # 3. Download Archive and Checksums
    Write-VectoInfo "Downloading $downloadUrl..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

    # 4. Verify SHA-256 Checksum
    Write-VectoInfo "Verifying checksum..."
    $computedHash = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()
    $expectedHash = $null

    foreach ($line in Get-Content $checksumsPath) {
        $parts = -split $line
        if ($parts.Length -ge 2 -and $parts[1] -eq $assetName) {
            $expectedHash = $parts[0].ToLower()
            break
        }
    }

    if (-not $expectedHash) {
        throw "Could not find expected checksum for $assetName in checksums.txt"
    }

    if ($computedHash -ne $expectedHash) {
        throw "Checksum verification failed! Expected: $expectedHash, Computed: $computedHash"
    }
    Write-VectoSuccess "Checksum verified."

    # 5. Extract and Install
    $installDir = if ($env:VECTO_INSTALL_DIR) {
        $env:VECTO_INSTALL_DIR
    } else {
        Join-Path $env:LOCALAPPDATA "vecto\bin"
    }

    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
    $sourceExe = Join-Path $tempDir "vecto.exe"
    $targetExe = Join-Path $installDir "vecto.exe"

    Copy-Item -Path $sourceExe -Destination $targetExe -Force
    Write-VectoSuccess "Installed binary to $targetExe"

    # 6. Check and Update PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
    $pathEntries = $userPath -split ';' | Where-Object { $_ -ne "" }
    if ($pathEntries -notcontains $installDir) {
        $newUserPath = if ([string]::IsNullOrEmpty($userPath)) { $installDir } else { "$userPath;$installDir" }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, [EnvironmentVariableTarget]::User)
        $env:Path = "$env:Path;$installDir"
        Write-VectoSuccess "Added $installDir to User PATH."
    } else {
        Write-VectoSuccess "$installDir is already in User PATH."
    }

    # 7. Verification
    & $targetExe --version
    Write-Host ""
    Write-VectoSuccess "Vecto $tag successfully installed!"
    Write-Host "Run 'vecto --help' or 'vecto init' to get started." -ForegroundColor Cyan

} catch {
    Write-VectoError "Installation failed: $_"
    exit 1
} finally {
    if ($tempDir -and (Test-Path $tempDir)) {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
