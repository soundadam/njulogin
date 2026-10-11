#!/usr/bin/env bash
# Builds the release archives into DIST (default dist/), named as the appkit
# contract requires: njulogin-<version>-<os>-<arch>.tar.gz, each holding
# njulogin-<version>-<os>-<arch>/njulogin and the license material.
#
#   scripts/package.sh VERSION [DIST]
#   scripts/package.sh --targets    print the GOOS/GOARCH pairs that ship
set -euo pipefail

targets=(darwin/amd64 darwin/arm64 linux/amd64 linux/arm64)

if [[ "${1:-}" == --targets ]]; then
  printf '%s\n' "${targets[@]}"
  exit 0
fi
if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 VERSION [DIST] | --targets" >&2
  exit 2
fi

version="$1"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || {
  echo "invalid version: $version" >&2
  exit 2
}

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
dist="$(mkdir -p "${2:-$root/dist}" && cd -- "${2:-$root/dist}" && pwd)"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

cd "$root"
for target in "${targets[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  case "$goos" in darwin) os=macos ;; *) os="$goos" ;; esac
  case "$goarch" in amd64) arch=x86_64 ;; arm64) arch=aarch64 ;; *) arch="$goarch" ;; esac
  name="njulogin-$version-$os-$arch"

  mkdir "$stage/$name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
    -ldflags "-s -w -X main.version=$version" \
    -o "$stage/$name/njulogin" ./cmd/njulogin
  cp README.md LICENSE NOTICE THIRD_PARTY_NOTICES.md "$stage/$name/"
  cp -R third_party_licenses "$stage/$name/"
  find "$stage/$name" -type d -exec chmod 0755 {} +
  find "$stage/$name" -type f -exec chmod 0644 {} +
  chmod 0755 "$stage/$name/njulogin"

  rm -f "$dist/$name.tar.gz"
  COPYFILE_DISABLE=1 tar -C "$stage" -czf "$dist/$name.tar.gz" "$name"
done

ls -l "$dist"
