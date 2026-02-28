#!/usr/bin/env sh
set -eu

REPO_SLUG="${OPENSPEND_REPO_SLUG:-promptingcompany/openspend-cli}"
REPO_URL="${OPENSPEND_REPO_URL:-https://github.com/${REPO_SLUG}.git}"
INSTALL_BIN_DIR="${OPENSPEND_INSTALL_BIN_DIR:-$HOME/.local/bin}"
REQUESTED_VERSION="${OPENSPEND_VERSION:-latest}"
TMP_DIR="$(mktemp -d)"

cleanup() {
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

detect_os() {
	case "$(uname -s)" in
		Linux) echo "linux" ;;
		Darwin) echo "darwin" ;;
		*)
			echo "error: unsupported OS: $(uname -s)" >&2
			return 1
			;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64|amd64) echo "amd64" ;;
		arm64|aarch64) echo "arm64" ;;
		*)
			echo "error: unsupported architecture: $(uname -m)" >&2
			return 1
			;;
	esac
}

resolve_tag() {
	if [ "$REQUESTED_VERSION" = "latest" ]; then
		curl -fsSL "https://api.github.com/repos/${REPO_SLUG}/releases/latest" \
			| sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
			| head -n 1
	else
		case "$REQUESTED_VERSION" in
			v*) echo "$REQUESTED_VERSION" ;;
			*) echo "v$REQUESTED_VERSION" ;;
		esac
	fi
}

install_from_release() {
	os="$(detect_os)"
	arch="$(detect_arch)"
	tag="$(resolve_tag)"
	if [ -z "$tag" ]; then
		echo "error: failed to resolve release tag" >&2
		return 1
	fi

	version="${tag#v}"
	archive="openspend_${version}_${os}_${arch}.tar.gz"
	download_url="https://github.com/${REPO_SLUG}/releases/download/${tag}/${archive}"

	echo "Downloading OpenSpend CLI ${tag} (${os}/${arch})..."
	curl -fLsS "$download_url" -o "$TMP_DIR/$archive" || return 1

	tar -xzf "$TMP_DIR/$archive" -C "$TMP_DIR"
	binary_path="$(find "$TMP_DIR" -type f -name openspend | head -n 1)"
	if [ -z "$binary_path" ]; then
		echo "error: downloaded archive did not contain openspend binary" >&2
		return 1
	fi

	mkdir -p "$INSTALL_BIN_DIR"
	cp "$binary_path" "$INSTALL_BIN_DIR/openspend"
	chmod +x "$INSTALL_BIN_DIR/openspend"
	echo "Installed openspend ${tag} to $INSTALL_BIN_DIR/openspend"
	return 0
}

install_from_source() {
	if ! command -v git >/dev/null 2>&1; then
		echo "error: git is required for source fallback install" >&2
		return 1
	fi
	if ! command -v go >/dev/null 2>&1; then
		echo "error: go is required for source fallback install" >&2
		echo "install Go first: https://go.dev/doc/install" >&2
		return 1
	fi

	echo "Falling back to source build from $REPO_URL..."
	tag="$(resolve_tag)"
	git clone --depth 1 --branch "$tag" "$REPO_URL" "$TMP_DIR/openspend-cli" >/dev/null 2>&1 \
		|| git clone --depth 1 "$REPO_URL" "$TMP_DIR/openspend-cli" >/dev/null 2>&1
	mkdir -p "$INSTALL_BIN_DIR"
	(
		cd "$TMP_DIR/openspend-cli"
		go build -o "$INSTALL_BIN_DIR/openspend" .
	)
	chmod +x "$INSTALL_BIN_DIR/openspend"
	echo "Installed openspend (source build) to $INSTALL_BIN_DIR/openspend"
}

if ! install_from_release; then
	install_from_source
fi

case ":$PATH:" in
	*":$INSTALL_BIN_DIR:"*)
		echo "Done. Run: openspend --help"
		;;
	*)
		echo "Add this to your shell profile to use the command globally:"
		echo "  export PATH=\"$INSTALL_BIN_DIR:\$PATH\""
		;;
esac
