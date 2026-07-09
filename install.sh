#!/usr/bin/env sh
set -eu

REPO="${REPO:-codestutis/cmdfreq}"
BINARY="${BINARY:-cmdfreq}"
VERSION="${VERSION:-latest}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"

need() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "error: $1 is required" >&2
		exit 1
	fi
}

detect_os() {
	case "$(uname -s)" in
		Darwin) echo "Darwin" ;;
		Linux) echo "Linux" ;;
		CYGWIN* | MINGW* | MSYS*)
			echo "error: native Windows is not supported; use WSL with bash/zsh history" >&2
			exit 1
			;;
		*)
			echo "error: unsupported OS: $(uname -s)" >&2
			exit 1
			;;
	esac
}

detect_arches() {
	case "$(uname -m)" in
		x86_64 | amd64) echo "x86_64 amd64" ;;
		arm64 | aarch64) echo "arm64" ;;
		*)
			echo "error: unsupported CPU architecture: $(uname -m)" >&2
			exit 1
			;;
	esac
}

download() {
	url="$1"
	out="$2"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$out"
	elif command -v wget >/dev/null 2>&1; then
		wget -q "$url" -O "$out"
	else
		echo "error: curl or wget is required" >&2
		exit 1
	fi
}

install_binary() {
	src="$1"
	dst="$BIN_DIR/$BINARY"

	if [ ! -d "$BIN_DIR" ]; then
		if ! mkdir -p "$BIN_DIR" 2>/dev/null; then
			if command -v sudo >/dev/null 2>&1; then
				sudo mkdir -p "$BIN_DIR"
			else
				echo "error: could not create $BIN_DIR and sudo is unavailable" >&2
				exit 1
			fi
		fi
	fi

	chmod +x "$src"
	if [ -w "$BIN_DIR" ]; then
		mv "$src" "$dst"
	elif command -v sudo >/dev/null 2>&1; then
		sudo mv "$src" "$dst"
	else
		echo "error: $BIN_DIR is not writable and sudo is unavailable" >&2
		exit 1
	fi
}

need uname
need tar
need mktemp
need find
need tr

os="$(detect_os)"
os_lower="$(printf '%s' "$os" | tr '[:upper:]' '[:lower:]')"
arches="$(detect_arches)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

if [ "$VERSION" = "latest" ]; then
	base_url="https://github.com/$REPO/releases/latest/download"
else
	base_url="https://github.com/$REPO/releases/download/$VERSION"
fi

archive="$tmp/$BINARY.tar.gz"
downloaded=""

for arch in $arches; do
	for name in \
		"${BINARY}_${os}_${arch}.tar.gz" \
		"${BINARY}_${os}_${arch}.tgz" \
		"${BINARY}_${os_lower}_${arch}.tar.gz" \
		"${BINARY}_${os_lower}_${arch}.tgz"
	do
		url="$base_url/$name"
		if download "$url" "$archive" >/dev/null 2>&1; then
			downloaded="$url"
			break 2
		fi
	done
done

if [ -z "$downloaded" ]; then
	echo "error: could not find a release archive for $os/$(uname -m)" >&2
	echo "looked under: $base_url" >&2
	exit 1
fi

tar -xzf "$archive" -C "$tmp"

bin_path="$(find "$tmp" -type f -name "$BINARY" | head -n 1)"
if [ -z "$bin_path" ]; then
	echo "error: archive did not contain $BINARY" >&2
	exit 1
fi

install_binary "$bin_path"

echo "installed $BINARY to $BIN_DIR"
