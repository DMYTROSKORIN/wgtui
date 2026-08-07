#!/usr/bin/env bash
# Installs wgtui for the current user only - root is never required. Same
# idea as rustup/pipx/cargo: everything lands in the user's own XDG
# directories (~/.local/bin, ~/.local/share/...), never under /usr. The
# .deb/.rpm packages remain available for anyone who specifically wants a
# system-managed install instead; this is the default, no-sudo path.
#
# Downloads a release (latest by default, or `install.sh vX.Y.Z`) via the
# GitHub CLI - this repo is private, so a plain `curl` of the asset URL
# can't authenticate, but `gh` already carries that auth - and verifies
# checksums.txt before installing anything.
#
# Usage:
#   scripts/install.sh          # latest release
#   scripts/install.sh v0.1.0   # a specific release
set -euo pipefail

REPO="DMYTROSKORIN/wgtui"
VERSION="${1:-latest}"

if ! command -v gh >/dev/null 2>&1; then
	echo "install.sh: the GitHub CLI (\`gh\`) is required - this repo is private, so a plain download can't authenticate." >&2
	exit 1
fi
if ! gh auth status >/dev/null 2>&1; then
	echo "install.sh: \`gh\` is not authenticated - run \`gh auth login\` first." >&2
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
	tag="$(gh release view --repo "$REPO" --json tagName --jq .tagName)"
else
	tag="$VERSION"
fi

# Asked the release for its actual asset names instead of reconstructing
# "wgtui_${version#v}_${os}_${arch}.tar.gz" by hand - one less place that
# breaks if the archive naming ever changes.
archive_name="$(gh release view "$tag" --repo "$REPO" --json assets --jq '.assets[].name' | grep -E "_${os}_${arch}\.tar\.gz\$" | head -1)"
if [ -z "$archive_name" ]; then
	echo "install.sh: no $os/$arch archive found in release $tag" >&2
	exit 1
fi

echo "install.sh: installing wgtui $tag ($os/$arch)..."

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

gh release download "$tag" --repo "$REPO" -D "$workdir" -p "checksums.txt" -p "$archive_name" --clobber
(
	cd "$workdir"
	sha256sum --ignore-missing -c checksums.txt
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
