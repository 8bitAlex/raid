#!/usr/bin/env bash
#
# Raid installer — Linux (with automatic Homebrew handoff on macOS).
#
# Raid is a declarative multi-repo development environment orchestrator:
# a cross-platform Go CLI that turns your team's commands, environments,
# and workflows into version-controlled YAML.
#
# Homepage: https://github.com/8bitalex/raid
# License:  GPL-3.0-only
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/8bitalex/raid/main/install.sh | bash
#
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

# ── Download & verify ────────────────────────────────────────────────────────
FILENAME="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

echo "Installing ${BINARY} v${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "${TMP}/${FILENAME}"
curl -fsSL "$CHECKSUMS_URL" -o "${TMP}/checksums.txt"

if ! command -v sha256sum >/dev/null 2>&1; then
  echo "Error: sha256sum is required to verify the downloaded archive."
  exit 1
fi

if ! grep "  ${FILENAME}\$" "${TMP}/checksums.txt" > "${TMP}/checksums_for_file.txt"; then
  echo "Error: checksum for ${FILENAME} not found in checksums.txt"
  exit 1
fi

(cd "$TMP" && sha256sum -c "${TMP}/checksums_for_file.txt")

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
