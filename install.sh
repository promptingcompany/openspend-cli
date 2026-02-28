#!/usr/bin/env sh
set -eu

REPO_URL="${OPENSPEND_REPO_URL:-https://github.com/promptingcompany/openspend-cli.git}"
INSTALL_BIN_DIR="${OPENSPEND_INSTALL_BIN_DIR:-$HOME/.local/bin}"
TMP_DIR="$(mktemp -d)"

cleanup() {
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

if ! command -v git >/dev/null 2>&1; then
	echo "error: git is required to install openspend" >&2
	exit 1
fi

if ! command -v go >/dev/null 2>&1; then
	echo "error: go is required to install openspend" >&2
	echo "install Go first: https://go.dev/doc/install" >&2
	exit 1
fi

echo "Cloning OpenSpend CLI from $REPO_URL..."
git clone --depth 1 "$REPO_URL" "$TMP_DIR/openspend-cli" >/dev/null 2>&1

echo "Building openspend CLI..."
mkdir -p "$INSTALL_BIN_DIR"
(
	cd "$TMP_DIR/openspend-cli"
	GOBIN="$INSTALL_BIN_DIR" go install .
)

echo "Installed openspend to $INSTALL_BIN_DIR/openspend"

case ":$PATH:" in
	*":$INSTALL_BIN_DIR:"*)
		echo "Done. Run: openspend --help"
		;;
	*)
		echo "Add this to your shell profile to use the command globally:"
		echo "  export PATH=\"$INSTALL_BIN_DIR:\$PATH\""
		;;
esac
