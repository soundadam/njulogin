#!/usr/bin/env bash
# Renders the Cask from synthetic archives and checks that every archive
# scripts/package.sh ships gets its own checksum, then runs brew style on it.
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
tap=""
cleanup() {
  [[ -z "$tap" ]] || HOMEBREW_NO_AUTO_UPDATE=1 brew untap "$tap" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

version=1.2.3-check
mkdir "$tmp/dist"
while read -r target; do
  goos="${target%/*}"
  goarch="${target#*/}"
  case "$goos" in darwin) os=macos ;; *) os="$goos" ;; esac
  case "$goarch" in amd64) arch=x86_64 ;; arm64) arch=aarch64 ;; *) arch="$goarch" ;; esac
  printf '%s\n' "$target" >"$tmp/dist/njulogin-$version-$os-$arch.tar.gz"
done < <("$root/scripts/package.sh" --targets)

cask="$tmp/njulogin.rb"
"$root/scripts/render-cask.sh" "$version" "$tmp/dist" "$cask"

grep -q "version \"$version\"" "$cask"
for archive in "$tmp"/dist/*; do
  sum="$(sha256sum "$archive" 2>/dev/null || shasum -a 256 "$archive")"
  grep -q "\"${sum%% *}\"" "$cask" || {
    echo "Cask lacks the checksum of $(basename "$archive")" >&2
    exit 1
  }
done
if "$root/scripts/render-cask.sh" "$version" "$tmp/missing" "$tmp/broken.rb" 2>/dev/null; then
  echo "render-cask.sh accepted a directory without archives" >&2
  exit 1
fi
ruby -c "$cask" >/dev/null

if command -v brew >/dev/null 2>&1; then
  # brew style applies the Cask cops only to a file inside a tap
  tap="njulogin/cask-check-$$"
  HOMEBREW_NO_AUTO_UPDATE=1 brew tap-new --no-git "$tap" >/dev/null
  mkdir -p "$(brew --repository "$tap")/Casks"
  cp "$cask" "$(brew --repository "$tap")/Casks/njulogin.rb"
  HOMEBREW_NO_AUTO_UPDATE=1 brew style "$tap/njulogin"
fi

echo "cask_template=pass"
