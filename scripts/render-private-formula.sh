#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: $0 VERSION ASSET_ID SHA256 OUTPUT" >&2
  exit 2
fi

version="$1"
asset_id="$2"
sha256="$3"
output="$4"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd -- "$script_dir/.." && pwd)"
template="$root/packaging/Formula/njulogin-private.rb.in"

[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || {
  echo "invalid version: $version" >&2
  exit 2
}
[[ "$asset_id" =~ ^[1-9][0-9]*$ ]] || {
  echo "ASSET_ID must be a positive decimal GitHub release asset ID" >&2
  exit 2
}
[[ "$sha256" =~ ^[0-9a-f]{64}$ ]] || {
  echo "SHA256 must contain exactly 64 lowercase hexadecimal characters" >&2
  exit 2
}

sed \
  -e "s|@VERSION@|$version|g" \
  -e "s|@ASSET_ID@|$asset_id|g" \
  -e "s|@SHA256@|$sha256|g" \
  "$template" >"$output"

chmod 0644 "$output"
