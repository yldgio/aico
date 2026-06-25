# install.ps1 — one-command installer for aico (Windows).
# Usage:
#   irm https://raw.githubusercontent.com/yldgio/aico/main/install.ps1 | iex
#   $env:INSTALL_DIR = "$HOME\bin"; irm ... | iex
#
# Downloads the latest release from GitHub and installs the binary.
# Default location: %USERPROFILE%\.local\bin (added to User PATH if missing).

$ErrorActionPreference = "Stop"

$Repo   = "yldgio/aico"
$Binary = "aico"

# ── Detect architecture ──────────────────────────────────────────────────────

$Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    "X64"   { "amd64" }
    "Arm64" { "arm64" }
    default { throw "Unsupported architecture: $_" }
}

# ── Determine install directory ──────────────────────────────────────────────

$Dir = if ($env:INSTALL_DIR) {
    $env:INSTALL_DIR
} else {
    Join-Path $env:USERPROFILE ".local\bin"
}

if (!(Test-Path $Dir)) {
    New-Item -ItemType Directory -Path $Dir -Force | Out-Null
}

# ── Fetch latest version ─────────────────────────────────────────────────────

Write-Host "> detecting latest release..."
$Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$Tag     = $Release.tag_name
$Version = $Tag -replace '^v', ''
Write-Host "  latest: $Tag"

# ── Download + extract ────────────────────────────────────────────────────────

$Archive = "${Binary}_${Version}_windows_${Arch}.zip"
$Url     = "https://github.com/$Repo/releases/download/$Tag/$Archive"
$TmpDir  = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "> downloading $Archive..."
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Archive) -UseBasicParsing

    Write-Host "> extracting..."
    Expand-Archive -Path (Join-Path $TmpDir $Archive) -DestinationPath $TmpDir -Force

    # ── Install ───────────────────────────────────────────────────────────────

    $BinName = "${Binary}.exe"
    Copy-Item (Join-Path $TmpDir $BinName) (Join-Path $Dir $BinName) -Force

    Write-Host "`n✓ installed $(Join-Path $Dir $BinName) ($Tag)" -ForegroundColor Green

    # ── PATH check ────────────────────────────────────────────────────────────

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$Dir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$Dir", "User")
        $env:Path = "$env:Path;$Dir"
        Write-Host "`n  Added $Dir to your User PATH." -ForegroundColor Yellow
        Write-Host "  Restart your terminal for the change to take effect."
    }
}
finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
