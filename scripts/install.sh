#!/bin/sh
set -eu

repo="${OPENEXIT_REPO:-RamazanKara/openexit}"
version="${OPENEXIT_VERSION:-latest}"
base_url="${OPENEXIT_BASE_URL:-}"
bin_dir="${BIN_DIR:-${OPENEXIT_INSTALL_DIR:-}}"

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'openexit install: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

download_stdout() {
	url="$1"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$url"
	else
		fail "missing curl or wget"
	fi
}

fetch_asset() {
	asset="$1"
	dest="$2"
	if [ -n "$base_url" ] && [ -d "$base_url" ]; then
		cp "$base_url/$asset" "$dest" || fail "could not copy $asset from $base_url"
		return
	fi
	url="${base_url%/}/$asset"
	if [ -z "$base_url" ]; then
		url="https://github.com/$repo/releases/download/$version/$asset"
	fi
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$dest" || fail "download failed: $url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$dest" "$url" || fail "download failed: $url"
	else
		fail "missing curl or wget"
	fi
}

detect_platform() {
	case "$(uname -s)" in
		Linux) os="linux" ;;
		Darwin) os="darwin" ;;
		*) fail "unsupported OS: $(uname -s)" ;;
	esac
	case "$(uname -m)" in
		x86_64 | amd64) arch="amd64" ;;
		aarch64 | arm64) arch="arm64" ;;
		*) fail "unsupported architecture: $(uname -m)" ;;
	esac
}

resolve_version() {
	if [ "$version" != "latest" ]; then
		return
	fi
	if [ -n "$base_url" ] && [ -d "$base_url" ]; then
		manifest="$base_url/RELEASE_MANIFEST.json"
		[ -f "$manifest" ] || fail "OPENEXIT_VERSION=latest requires RELEASE_MANIFEST.json in $base_url"
		version="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" | head -n 1)"
	else
		api="https://api.github.com/repos/$repo/releases/latest"
		version="$(download_stdout "$api" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
	fi
	[ -n "$version" ] || fail "could not resolve latest release version"
}

verify_checksum() {
	asset="$1"
	awk -v want="$asset" '$2 == want { print }' "$tmp/SHA256SUMS" > "$tmp/SHA256SUMS.selected"
	[ -s "$tmp/SHA256SUMS.selected" ] || fail "SHA256SUMS has no entry for $asset"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$tmp" && sha256sum -c SHA256SUMS.selected >/dev/null) || fail "checksum verification failed for $asset"
	elif command -v shasum >/dev/null 2>&1; then
		(cd "$tmp" && shasum -a 256 -c SHA256SUMS.selected >/dev/null) || fail "checksum verification failed for $asset"
	else
		fail "missing sha256sum or shasum"
	fi
}

choose_bin_dir() {
	if [ -n "$bin_dir" ]; then
		return
	fi
	if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		bin_dir="/usr/local/bin"
		return
	fi
	[ -n "${HOME:-}" ] || fail "HOME is not set; set BIN_DIR explicitly"
	bin_dir="$HOME/.local/bin"
}

install_binary() {
	src="$1"
	mkdir -p "$bin_dir"
	if command -v install >/dev/null 2>&1; then
		install -m 0755 "$src" "$bin_dir/openexit"
	else
		cp "$src" "$bin_dir/openexit"
		chmod 0755 "$bin_dir/openexit"
	fi
}

need awk
need chmod
need cp
need mkdir
need sed
need uname

detect_platform
resolve_version
choose_bin_dir

tmp="${TMPDIR:-/tmp}/openexit-install.$$"
rm -rf "$tmp"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT INT TERM

artifact="openexit_${version}_${os}_${arch}"
fetch_asset "RELEASE_MANIFEST.json" "$tmp/RELEASE_MANIFEST.json"
fetch_asset "SHA256SUMS" "$tmp/SHA256SUMS"
fetch_asset "$artifact" "$tmp/$artifact"
chmod 0755 "$tmp/$artifact"

verify_checksum "$artifact"
"$tmp/$artifact" verify-release "$tmp/RELEASE_MANIFEST.json" --dist "$tmp" --require-checksums --artifact "$artifact" >/dev/null
install_binary "$tmp/$artifact"

log "installed openexit $version to $bin_dir/openexit"
"$bin_dir/openexit" version
