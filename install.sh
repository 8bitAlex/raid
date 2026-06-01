#!/usr/bin/env bash
#
# Raid installer — Linux, Windows (Git Bash), and WSL,
# with automatic Homebrew handoff on macOS.
#
# Raid is a declarative multi-repo development environment orchestrator:
# a cross-platform Go CLI that turns your team's commands, environments,
# and workflows into version-controlled YAML.
#
# On Windows, run this from a Git Bash shell. Inside WSL the script
# behaves like any other Linux install (it installs the Linux binary).
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

# ── OS check ────────────────────────────────────────────────────────────────
# WSL reports "linux" from uname, so it follows the Linux path automatically.
# Git Bash / MSYS2 / Cygwin report MINGW*/MSYS*/CYGWIN*, which map to Windows.
UNAME=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$UNAME" in
  darwin)
    if ! command -v brew &>/dev/null; then
      echo "Error: Homebrew is required on macOS. Install it from https://brew.sh"
      exit 1
    fi
    exec brew install 8bitalex/tap/raid
    ;;
  linux)
    OS="linux"
    EXT="tar.gz"
    ;;
  mingw*|msys*|cygwin*)
    OS="windows"
    EXT="zip"
    BINARY="raid.exe"
    ;;
  *)
    echo "Unsupported OS: $UNAME"
    exit 1
    ;;
esac

# ── Architecture ─────────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Windows releases are published for amd64 only.
if [[ "$OS" == "windows" && "$ARCH" != "amd64" ]]; then
  echo "Unsupported architecture for Windows: $ARCH (only amd64 is published)"
  exit 1
fi

# ── Install directory ──────────────────────────────────────────────────────
# Windows lacks a writable /usr/local/bin, so install into the user's
# ~/bin (created if missing) and warn if it is not on PATH.
if [[ "$OS" == "windows" ]]; then
  INSTALL_DIR="${HOME}/bin"
else
  INSTALL_DIR="/usr/local/bin"
fi

# ── Resolve version ──────────────────────────────────────────────────────────
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"v([^"]+)".*/\1/')

if [[ -z "$VERSION" ]]; then
  echo "Error: could not determine the latest version."
  exit 1
fi

# ── Download & verify ────────────────────────────────────────────────────────
FILENAME="${BINARY%.exe}_${VERSION}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

echo "Installing raid v${VERSION} (${OS}/${ARCH})..."

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

# ── Extract ────────────────────────────────────────────────────────────────
if [[ "$EXT" == "zip" ]]; then
  if ! command -v unzip >/dev/null 2>&1; then
    echo "Error: unzip is required to extract the Windows archive."
    exit 1
  fi
  unzip -o -q "${TMP}/${FILENAME}" -d "$TMP"
else
  tar -xzf "${TMP}/${FILENAME}" -C "$TMP"
fi

# ── Install ───────────────────────────────────────────────────────────────────
if [[ "$OS" == "windows" ]]; then
  mkdir -p "$INSTALL_DIR"
  mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
elif [[ -w "$INSTALL_DIR" ]]; then
  mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo ""
echo "Installed successfully:"
"${INSTALL_DIR}/${BINARY}" --version

# On Windows, ~/bin is rarely on PATH out of the box — nudge the user.
if [[ "$OS" == "windows" ]]; then
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      echo ""
      echo "Note: ${INSTALL_DIR} is not on your PATH."
      echo "Add it (e.g. in ~/.bashrc) so you can run 'raid' from anywhere:"
      echo "  export PATH=\"\$HOME/bin:\$PATH\""
      ;;
  esac
fi
