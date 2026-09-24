#!/bin/sh
# Installs isoshelf on Linux, for the person running it - no sudo:
#
#   curl -fsSL https://raw.githubusercontent.com/ZachCurry13/isoshelf/main/install.sh | sh
#
# It's short, and worth reading first. It finds the newest release, downloads
# the program for this machine, checks it against the release's SHA256SUMS,
# and puts it in ~/.local/bin as "isoshelf" (set ISOSHELF_DIR to choose
# another folder). Nothing else is touched. From then on isoshelf updates
# itself, checking this project's signature before it installs anything.
set -eu

repo=ZachCurry13/isoshelf
dir=${ISOSHELF_DIR:-"$HOME/.local/bin"}

say() { printf '%s\n' "$*"; }
fail() { printf 'isoshelf install: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) ;;
  *) fail "this installs isoshelf on Linux. On Windows, download it from https://github.com/$repo/releases; on a Mac, use the container (docs/docker.md)." ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "there is no isoshelf for $(uname -m) yet. Ask in https://github.com/$repo/discussions." ;;
esac
for tool in curl sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || fail "needs $tool, which isn't installed."
done

# The newest release. Read from GitHub's answer without jq, which not every
# machine has: the one line that names the tag.
tag=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
  sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
[ -n "$tag" ] || fail "couldn't find the newest release on GitHub. Try again in a minute."

name="isoshelf-$tag-linux-$arch"
base="https://github.com/$repo/releases/download/$tag"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading isoshelf $tag for $arch..."
curl -fL --progress-bar -o "$tmp/$name" "$base/$name"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS"

# Only the line for this file, checked by sha256sum itself.
grep " $name\$" "$tmp/SHA256SUMS" > "$tmp/check" || fail "the release's SHA256SUMS doesn't list $name."
(cd "$tmp" && sha256sum -c --status check) ||
  fail "the download doesn't match its published checksum, so nothing was installed."

mkdir -p "$dir"
cp "$tmp/$name" "$dir/isoshelf.new"
chmod 0755 "$dir/isoshelf.new"
mv "$dir/isoshelf.new" "$dir/isoshelf"

say ""
say "isoshelf $tag is installed in $dir."
case ":$PATH:" in
  *":$dir:"*) say "Start it with:  isoshelf" ;;
  *)
    say "Start it with:  $dir/isoshelf"
    say "($dir isn't on your PATH. Add it, and plain \"isoshelf\" works too.)"
    ;;
esac
say "It opens in your browser. Press Update now in its top bar when a new version is out."
