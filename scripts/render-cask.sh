#!/usr/bin/env bash
# Fills packaging/homebrew/njulogin.rb with VERSION and the SHA-256 of each
# archive that scripts/package.sh wrote to DIST.
#
#   scripts/render-cask.sh VERSION DIST OUTPUT
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 VERSION DIST OUTPUT" >&2
  exit 2
fi

version="$1"
dist="$2"
output="$3"
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

sha() {
  local archive="$dist/njulogin-$version-$1.tar.gz"
  [[ -f "$archive" ]] || {
    echo "missing archive: $archive" >&2
    exit 1
  }
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$archive" | cut -d' ' -f1
  else
    shasum -a 256 "$archive" | cut -d' ' -f1
  fi
}

macos_aarch64="$(sha macos-aarch64)"
macos_x86_64="$(sha macos-x86_64)"
linux_aarch64="$(sha linux-aarch64)"
linux_x86_64="$(sha linux-x86_64)"

sed \
  -e "s/@VERSION@/$version/g" \
  -e "s/@MACOS_AARCH64_SHA256@/$macos_aarch64/" \
  -e "s/@MACOS_X86_64_SHA256@/$macos_x86_64/" \
  -e "s/@LINUX_AARCH64_SHA256@/$linux_aarch64/" \
  -e "s/@LINUX_X86_64_SHA256@/$linux_x86_64/" \
  "$root/packaging/homebrew/njulogin.rb" >"$output"

if grep -n '@[A-Z0-9_]*@' "$output"; then
  echo "unfilled placeholder in $output" >&2
  exit 1
fi
