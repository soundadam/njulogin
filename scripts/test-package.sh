#!/usr/bin/env bash
# Builds the release archives and checks their names, layout, and the version
# built into the binary for this host. That run's standard error goes to a
# file, so it makes no update check, and a temporary HOME and XDG_CONFIG_HOME
# keep the real configuration untouched.
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

version=0.0.0-check
"$root/scripts/package.sh" "$version" "$tmp/dist" >/dev/null

host="$(go env GOOS)/$(go env GOARCH)"
count=0
while read -r target; do
  goos="${target%/*}"
  goarch="${target#*/}"
  case "$goos" in darwin) os=macos ;; *) os="$goos" ;; esac
  case "$goarch" in amd64) arch=x86_64 ;; arm64) arch=aarch64 ;; *) arch="$goarch" ;; esac
  name="njulogin-$version-$os-$arch"
  archive="$tmp/dist/$name.tar.gz"
  listing="$(tar -tzf "$archive")"
  for member in njulogin README.md LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
    grep -qx "$name/$member" <<<"$listing" || {
      echo "$name.tar.gz lacks $name/$member" >&2
      exit 1
    }
  done
  for license in "$root"/third_party_licenses/*; do
    grep -qx "$name/third_party_licenses/$(basename "$license")" <<<"$listing" || {
      echo "$name.tar.gz lacks $(basename "$license")" >&2
      exit 1
    }
  done
  if [[ "$target" == "$host" ]]; then
    tar -xzf "$archive" -C "$tmp"
    got="$(HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" "$tmp/$name/njulogin" version 2>"$tmp/version-err")"
    [[ "$got" == "njulogin $version" ]] || {
      echo "$name reports $got" >&2
      cat "$tmp/version-err" >&2
      exit 1
    }
  fi
  count=$((count + 1))
done < <("$root/scripts/package.sh" --targets)

[[ "$(find "$tmp/dist" -name '*.tar.gz' | wc -l | tr -d ' ')" -eq "$count" ]]
echo "release_archives=pass archives=$count"
