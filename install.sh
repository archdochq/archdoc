#!/bin/sh
# Installs archdoc on Linux or macOS.
#
#   curl -fsSL https://github.com/archdochq/archdoc/releases/latest/download/install.sh | sh
#
# The copy attached to a release installs that release. This copy, in the
# repository, sets none, so it needs ARCHDOC_VERSION.
#
#   ARCHDOC_VERSION       the release to install, as a tag such as v0.6.0
#   ARCHDOC_INSTALL_DIR   the directory to install into
#   ARCHDOC_DOWNLOAD_URL  the base URL of the release's assets
set -eu

# Set to the tag in the copy a release attaches; docs/build.go writes it.
release=

fail() {
	echo "install.sh: $*" >&2
	exit 1
}

tmp=
partial=
cleanup() {
	[ -n "$tmp" ] && rm -rf "$tmp"
	[ -n "$partial" ] && rm -f "$partial"
	return 0
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

version=${ARCHDOC_VERSION:-$release}
[ -n "$version" ] || fail "no release is set in this copy; set ARCHDOC_VERSION to a tag such as v0.6.0"

os=$(uname -s)
case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) fail "$os is not supported; download a release archive from https://github.com/archdochq/archdoc/releases" ;;
esac
arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "$arch is not supported; download a release archive from https://github.com/archdochq/archdoc/releases" ;;
esac

if command -v curl >/dev/null 2>&1; then
	download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	download() { wget -q -O "$2" "$1"; }
else
	fail "neither curl nor wget is installed"
fi
if command -v sha256sum >/dev/null 2>&1; then
	digest() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
	digest() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
	fail "neither sha256sum nor shasum is installed"
fi

base=${ARCHDOC_DOWNLOAD_URL:-https://github.com/archdochq/archdoc/releases/download/$version}
archive="archdoc_${version#v}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
download "$base/$archive" "$tmp/$archive" || fail "could not download $base/$archive"
download "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download $base/checksums.txt"

want=$(awk -v name="$archive" '$2 == name || $2 == "*" name { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || fail "checksums.txt has no line for $archive"
[ "$(digest "$tmp/$archive")" = "$want" ] || fail "$archive does not match checksums.txt"
tar -xzf "$tmp/$archive" -C "$tmp" archdoc || fail "$archive holds no archdoc"

if [ -n "${ARCHDOC_INSTALL_DIR:-}" ]; then
	dir=$ARCHDOC_INSTALL_DIR
elif [ -w /usr/local/bin ]; then
	dir=/usr/local/bin
elif [ -n "${HOME:-}" ]; then
	dir=$HOME/.local/bin
else
	fail "/usr/local/bin is not writable and HOME is unset; set ARCHDOC_INSTALL_DIR"
fi
mkdir -p "$dir" 2>/dev/null || fail "could not create $dir; set ARCHDOC_INSTALL_DIR"
[ -L "$dir/archdoc" ] && fail "$dir/archdoc is a symbolic link, as a package manager installs it; update it there, or set ARCHDOC_INSTALL_DIR"
[ -d "$dir/archdoc" ] && fail "$dir/archdoc is a directory; remove it, or set ARCHDOC_INSTALL_DIR"

# Copied to a temporary name beside the target and renamed over it, so a binary
# already there is replaced whole or not at all. mktemp creates the name, so
# nothing planted at a guessable one is written through.
partial=$(mktemp "$dir/.archdoc.XXXXXX" 2>/dev/null) || fail "could not write to $dir; set ARCHDOC_INSTALL_DIR"
cp "$tmp/archdoc" "$partial" || fail "could not write to $dir; set ARCHDOC_INSTALL_DIR"
chmod 0755 "$partial"
mv -f "$partial" "$dir/archdoc" || fail "could not write to $dir; set ARCHDOC_INSTALL_DIR"
partial=

echo "Installed archdoc $version at $dir/archdoc"
case ":${PATH:-}:" in
*":$dir:"*) ;;
*) echo "warning: $dir is not on PATH" >&2 ;;
esac
