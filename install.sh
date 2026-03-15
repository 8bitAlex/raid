#!/usr/bin/env bash
set -euo pipefail

REPO="8bitalex/raid"
BINARY="raid"
INSTALL_DIR="/usr/local/bin"

# ── OS check ────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [[ "$OS" == "darwin" ]]; then
  if ! command -v brew &>/dev/null; then
    echo "Error: Homebrew is required on macOS. Install it from https://brew.sh"
    exit 1
  fi
  exec brew install 8bitalex/tap/raid
fi

if [[ "$OS" != "linux" ]]; then
  echo "Unsupported OS: $OS"
  exit 1
fi

# ── Architecture ─────────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# ── Resolve version ──────────────────────────────────────────────────────────
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"v([^"]+)".*/\1/')

if [[ -z "$VERSION" ]]; then
  echo "Error: could not determine the latest version."
  exit 1
fi

# ── Download & extract ───────────────────────────────────────────────────────
FILENAME="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

echo "Installing ${BINARY} v${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "${TMP}/${FILENAME}"
tar -xzf "${TMP}/${FILENAME}" -C "$TMP"

# ── Install ───────────────────────────────────────────────────────────────────
if [[ -w "$INSTALL_DIR" ]]; then
  mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo ""
echo "Installed successfully:"
"${INSTALL_DIR}/${BINARY}" --version
