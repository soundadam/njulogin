#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d /tmp/njulogin-formula.XXXXXX)"
trap 'rm -rf "$tmp"' EXIT INT TERM

version=0.1.0
archive="$tmp/njulogin-$version.tar.gz"
formula_dir="$tmp/Formula"
formula="$formula_dir/njulogin.rb"
mkdir -p "$formula_dir"

git -C "$root" archive \
  --format=tar.gz \
  --prefix="njulogin-$version/" \
  --output="$archive" \
  HEAD
sha256="$(sha256sum "$archive" | awk '{print $1}')"

"$root/scripts/render-formula.sh" \
  "$version" \
  "https://github.com/soundadam/njulogin/archive/refs/tags/v$version.tar.gz" \
  "$sha256" \
  "$formula"

test -n "$(find "$formula" -perm 644)"
test "$(grep -c '@VERSION@\|@URL@\|@SHA256@' "$formula" || true)" -eq 0
if grep -q '^  version ' "$formula"; then
  echo "Formula contains a redundant explicit version" >&2
  exit 1
fi
grep -q 'archive/refs/tags/v0.1.0.tar.gz' "$formula"
grep -q "sha256 \"$sha256\"" "$formula"
grep -q 'doc.install "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md", "third_party_licenses"' "$formula"
grep -q 'assert_path_exists doc/"third_party_licenses/golang.org_x_sys_LICENSE"' "$formula"
ruby -c "$formula" >/dev/null

if command -v brew >/dev/null 2>&1; then
  HOMEBREW_NO_AUTO_UPDATE=1 brew style "$formula"
fi

echo "formula_template=pass"
