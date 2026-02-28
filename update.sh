#!/usr/bin/env sh
set -eu

DEFAULT_INSTALL_DIR="$HOME/.local/bin"
INSTALL_SCRIPT_URL="${OPENSPEND_INSTALL_SCRIPT_URL:-https://raw.githubusercontent.com/promptingcompany/openspend-cli/main/install.sh}"

if [ -n "${OPENSPEND_INSTALL_BIN_DIR:-}" ]; then
	INSTALL_BIN_DIR="$OPENSPEND_INSTALL_BIN_DIR"
elif command -v openspend >/dev/null 2>&1; then
	INSTALL_BIN_DIR="$(dirname "$(command -v openspend)")"
else
	INSTALL_BIN_DIR="$DEFAULT_INSTALL_DIR"
fi

if [ -e "$INSTALL_BIN_DIR" ] && [ ! -w "$INSTALL_BIN_DIR" ]; then
	echo "error: install directory is not writable: $INSTALL_BIN_DIR" >&2
	echo "set OPENSPEND_INSTALL_BIN_DIR to a writable location or rerun with proper permissions" >&2
	exit 1
fi

CURRENT_VERSION="not-installed"
if command -v openspend >/dev/null 2>&1; then
	CURRENT_VERSION="$(openspend version 2>/dev/null || openspend --version 2>/dev/null || echo unknown)"
fi

echo "Updating OpenSpend CLI in $INSTALL_BIN_DIR"
echo "Current version: $CURRENT_VERSION"

tmp_script="$(mktemp)"
cleanup() {
	rm -f "$tmp_script"
}
trap cleanup EXIT INT TERM

curl -fsSL "$INSTALL_SCRIPT_URL" -o "$tmp_script"

OPENSPEND_INSTALL_BIN_DIR="$INSTALL_BIN_DIR" \
OPENSPEND_VERSION="${OPENSPEND_VERSION:-latest}" \
OPENSPEND_REPO_SLUG="${OPENSPEND_REPO_SLUG:-promptingcompany/openspend-cli}" \
sh "$tmp_script"

NEW_VERSION="$("$INSTALL_BIN_DIR/openspend" version 2>/dev/null || echo unknown)"
echo "Updated version: $NEW_VERSION"
