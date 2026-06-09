#!/bin/sh
# github-docs-cli installer (Linux / macOS)
#
#   curl -fsSL https://jhl-labs.github.io/github-docs-cli/install.sh | sh
#
# Downloads the latest release binary for your OS/arch into a bin directory.
# Override the target with: INSTALL_DIR=/path/to/bin sh install.sh
set -eu

REPO="jhl-labs/github-docs-cli"
BIN="github-docs-cli"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux)  goos="linux" ;;
  Darwin) goos="darwin" ;;
  *) echo "unsupported OS: $os (use the Windows binary from the releases page)" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

asset="${BIN}-${goos}-${goarch}"
url="https://github.com/${REPO}/releases/latest/download/${asset}"

tmp="$(mktemp)"
echo "Downloading ${asset} ..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "$url"
else
  echo "curl or wget is required" >&2; exit 1
fi

chmod +x "$tmp"

target="${INSTALL_DIR}/${BIN}"
if [ -w "$INSTALL_DIR" ]; then
  mv "$tmp" "$target"
else
  echo "Installing to ${target} (requires sudo) ..."
  sudo mv "$tmp" "$target"
fi

echo "Installed: $("$target" version 2>/dev/null || echo "$target")"
echo "Run '${BIN} help' to get started."
