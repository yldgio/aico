#!/bin/sh
# install.sh — one-command installer for aico.
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/yldgio/aico/main/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/yldgio/aico/main/install.sh | INSTALL_DIR=~/bin sh
#
# Detects OS + arch, downloads the latest release from GitHub, and installs the
# binary. Defaults to /usr/local/bin if writable, else ~/.local/bin.
set -e

REPO="yldgio/aico"
BINARY="aico"

# ── Detect platform ──────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "error: unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# ── Determine install directory ──────────────────────────────────────────────

if [ -n "${INSTALL_DIR:-}" ]; then
  DIR="$INSTALL_DIR"
elif [ -w /usr/local/bin ]; then
  DIR="/usr/local/bin"
else
  DIR="$HOME/.local/bin"
fi

mkdir -p "$DIR"

# ── Fetch latest version ─────────────────────────────────────────────────────

echo "› detecting latest release..."

if command -v curl >/dev/null 2>&1; then
  TAG=$(curl -sSf "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
elif command -v wget >/dev/null 2>&1; then
  TAG=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
else
  echo "error: curl or wget is required"; exit 1
fi

if [ -z "$TAG" ]; then
  echo "error: could not determine latest release"; exit 1
fi

VERSION="${TAG#v}"
echo "  latest: $TAG"

# ── Download + extract ────────────────────────────────────────────────────────

if [ "$OS" = "windows" ]; then
  EXT="zip"
else
  EXT="tar.gz"
fi

ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/$REPO/releases/download/$TAG/$ARCHIVE"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "› downloading $ARCHIVE..."
if command -v curl >/dev/null 2>&1; then
  curl -sSfL "$URL" -o "$TMPDIR/$ARCHIVE"
else
  wget -q "$URL" -O "$TMPDIR/$ARCHIVE"
fi

echo "› extracting..."
case "$EXT" in
  tar.gz) tar xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR" ;;
  zip)    unzip -qo "$TMPDIR/$ARCHIVE" -d "$TMPDIR" ;;
esac

# ── Install ───────────────────────────────────────────────────────────────────

BIN_NAME="$BINARY"
[ "$OS" = "windows" ] && BIN_NAME="${BINARY}.exe"

mv "$TMPDIR/$BIN_NAME" "$DIR/$BIN_NAME"
chmod +x "$DIR/$BIN_NAME"

echo "✓ installed $DIR/$BIN_NAME ($TAG)"

# ── PATH check ────────────────────────────────────────────────────────────────

case ":${PATH}:" in
  *":${DIR}:"*) ;;
  *)
    echo ""
    echo "⚠  $DIR is not in your PATH."
    echo "   Add it to your shell profile:"
    echo "     export PATH=\"$DIR:\$PATH\""
    ;;
esac
