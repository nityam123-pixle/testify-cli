#!/bin/bash
set -e

REPO="nityam123-pixle/testify-cli"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="testify"

# ── Detect OS ────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux"  ;;
  *)
    echo "❌ Unsupported OS: $OS"
    echo "   Please download manually from: https://github.com/$REPO/releases"
    exit 1
    ;;
esac

# ── Detect Arch ───────────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  arm64 | aarch64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported architecture: $ARCH"
    echo "   Please download manually from: https://github.com/$REPO/releases"
    exit 1
    ;;
esac

# ── Fetch latest version tag ──────────────────────────────────────────────────
echo "🔍 Fetching latest Testify release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "❌ Could not determine latest version."
  exit 1
fi

ASSET="testify_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"

echo "📦 Downloading Testify $VERSION for ${OS}/${ARCH}..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$URL" -o "$TMP_DIR/$ASSET"

echo "📂 Extracting..."
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"

# ── Install ───────────────────────────────────────────────────────────────────
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
  echo "🔐 Requesting sudo to install to $INSTALL_DIR..."
  sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo ""
echo "✅ Testify $VERSION installed successfully!"
echo "   Run: testify version"
