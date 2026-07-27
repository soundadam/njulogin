#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: $0 VERSION URL SHA256 OUTPUT" >&2
  exit 2
fi

version="$1"
url="$2"
sha256="$3"
output="$4"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd -- "$script_dir/.." && pwd)"
template="$root/packaging/Formula/njulogin.rb.in"

[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || {
  echo "invalid version: $version" >&2
  exit 2
}
[[ "$url" =~ ^(https|file):// ]] || {
  echo "URL must use https:// or file://" >&2
  exit 2
}
[[ "$sha256" =~ ^[0-9a-f]{64}$ ]] || {
  echo "SHA256 must contain exactly 64 lowercase hexadecimal characters" >&2
  exit 2
}

escaped_url="${url//\\/\\\\}"
escaped_url="${escaped_url//&/\\&}"
escaped_url="${escaped_url//|/\\|}"

sed \
  -e "s|@VERSION@|$version|g" \
  -e "s|@URL@|$escaped_url|g" \
  -e "s|@SHA256@|$sha256|g" \
  "$template" >"$output"
