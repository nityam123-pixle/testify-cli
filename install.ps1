# Testify Windows Installer
# Run with: iwr -useb https://raw.githubusercontent.com/nityam123-pixle/testify-cli/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$REPO = "nityam123-pixle/testify-cli"
$BINARY_NAME = "testify.exe"
$INSTALL_DIR = "$env:LOCALAPPDATA\Programs\testify"

# ── Detect Arch ───────────────────────────────────────────────────────────────
$ARCH = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Host "❌ 32-bit Windows is not supported."
    exit 1
}

# ── Fetch latest version ───────────────────────────────────────────────────────
Write-Host "🔍 Fetching latest Testify release..."
$release = Invoke-RestMethod "https://api.github.com/repos/$REPO/releases/latest"
$VERSION = $release.tag_name

if (-not $VERSION) {
    Write-Host "❌ Could not determine latest version."
    exit 1
}

$ASSET = "testify_windows_${ARCH}.zip"
$URL = "https://github.com/$REPO/releases/download/$VERSION/$ASSET"

Write-Host "📦 Downloading Testify $VERSION for windows/$ARCH..."
$TMP_DIR = Join-Path $env:TEMP "testify-install"
New-Item -ItemType Directory -Force -Path $TMP_DIR | Out-Null
$ZIP_PATH = Join-Path $TMP_DIR $ASSET

Invoke-WebRequest -Uri $URL -OutFile $ZIP_PATH -UseBasicParsing

Write-Host "📂 Extracting..."
Expand-Archive -Path $ZIP_PATH -DestinationPath $TMP_DIR -Force

# ── Install ───────────────────────────────────────────────────────────────────
New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
$BIN_SRC = Join-Path $TMP_DIR $BINARY_NAME
Copy-Item -Path $BIN_SRC -Destination (Join-Path $INSTALL_DIR $BINARY_NAME) -Force

# ── Add to PATH if not already there ─────────────────────────────────────────
$CURRENT_PATH = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($CURRENT_PATH -notlike "*$INSTALL_DIR*") {
    Write-Host "🔧 Adding Testify to your PATH..."
    [System.Environment]::SetEnvironmentVariable("PATH", "$CURRENT_PATH;$INSTALL_DIR", "User")
    $env:PATH += ";$INSTALL_DIR"
}

# ── Cleanup ───────────────────────────────────────────────────────────────────
Remove-Item -Recurse -Force $TMP_DIR

Write-Host ""
Write-Host "✅ Testify $VERSION installed successfully!"
Write-Host "   Restart your terminal, then run: testify version"
