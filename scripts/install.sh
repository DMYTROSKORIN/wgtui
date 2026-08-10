#!/usr/bin/env bash
# Installs wgtui for the current user only - root is never required. Same
# idea as rustup/pipx/cargo: everything lands in the user's own XDG
# directories (~/.local/bin, ~/.local/share/...), never under /usr. The
# .deb/.rpm packages remain available for anyone who specifically wants a
# system-managed install instead; this is the default, no-sudo path.
#
# Downloads a release (latest by default, or a pinned version) straight
# from GitHub Releases and verifies checksums.txt before installing
# anything.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/DMYTROSKORIN/wgtui/master/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/DMYTROSKORIN/wgtui/master/scripts/install.sh | WGTUI_VERSION=v0.5.9 bash
#   scripts/install.sh          # from a local clone: latest release
#   scripts/install.sh v0.5.9   # from a local clone: a specific release
set -euo pipefail

REPO="DMYTROSKORIN/wgtui"
VERSION="${WGTUI_VERSION:-${1:-latest}}"

if ! command -v curl >/dev/null 2>&1; then
	echo "install.sh: curl is required but not installed" >&2
	exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
	checksum_cmd() { sha256sum --ignore-missing -c checksums.txt; }
elif command -v shasum >/dev/null 2>&1; then
	checksum_cmd() { shasum -a 256 --ignore-missing -c checksums.txt; }
else
	echo "install.sh: neither sha256sum nor shasum found - can't verify the download" >&2
	exit 1
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
linux | darwin) ;;
*)
	echo "install.sh: unsupported OS '$os' (wgtui ships linux and darwin builds only)" >&2
	exit 1
	;;
esac

arch="$(uname -m)"
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*)
	echo "install.sh: unsupported architecture '$arch' (wgtui ships amd64 and arm64 builds only)" >&2
	exit 1
	;;
esac

if [ "$VERSION" = "latest" ]; then
	release_url="https://api.github.com/repos/${REPO}/releases/latest"
else
	release_url="https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
fi
release_json="$(curl -fsSL "$release_url")"

tag="$(printf '%s' "$release_json" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
[ -n "$tag" ] || {
	echo "install.sh: could not resolve release '$VERSION' - check the version/tag exists" >&2
	exit 1
}

# Asked the release for its actual asset name instead of reconstructing
# "wgtui_${version#v}_${os}_${arch}.tar.gz" by hand - one less place that
# breaks if the archive naming ever changes.
archive_name="$(printf '%s' "$release_json" | grep '"name"' | sed 's/.*"name": *"\([^"]*\)".*/\1/' | grep -E "_${os}_${arch}\.tar\.gz\$" | head -1)"
[ -n "$archive_name" ] || {
	echo "install.sh: no $os/$arch archive found in release $tag" >&2
	exit 1
}
download_base="https://github.com/${REPO}/releases/download/${tag}"

echo "install.sh: installing wgtui $tag ($os/$arch)..."

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

curl -fsSL --progress-bar "${download_base}/${archive_name}" -o "$workdir/${archive_name}"
curl -fsSL "${download_base}/checksums.txt" -o "$workdir/checksums.txt"
(
	cd "$workdir"
	checksum_cmd
	tar xzf "$archive_name"
)

bin_dir="$HOME/.local/bin"
man_dir="$HOME/.local/share/man/man1"
bash_dir="$HOME/.local/share/bash-completion/completions"
zsh_dir="$HOME/.local/share/zsh/site-functions"
fish_dir="$HOME/.config/fish/completions"
mkdir -p "$bin_dir" "$man_dir" "$bash_dir" "$zsh_dir" "$fish_dir"

install -m 0755 "$workdir/wgtui" "$bin_dir/wgtui"
install -m 0644 "$workdir/docs/man/wgtui.1" "$man_dir/wgtui.1"
install -m 0644 "$workdir/completions/wgtui.bash" "$bash_dir/wgtui"
install -m 0644 "$workdir/completions/_wgtui" "$zsh_dir/_wgtui"
install -m 0644 "$workdir/completions/wgtui.fish" "$fish_dir/wgtui.fish"

echo "install.sh: installed $("$bin_dir/wgtui" version | head -1) to $bin_dir/wgtui"
case ":$PATH:" in
*":$bin_dir:"*) ;;
*) echo "install.sh: NOTE - $bin_dir is not on your PATH; add it to your shell profile." ;;
esac
