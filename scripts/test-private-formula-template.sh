#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d /tmp/njulogin-private-formula.XXXXXX)"
trap 'rm -rf "$tmp"' EXIT INT TERM

version=0.1.1
asset_id=123456789
sha256=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
formula_dir="$tmp/Formula"
formula="$formula_dir/njulogin.rb"
mkdir -p "$formula_dir"

"$root/scripts/render-private-formula.sh" \
  "$version" \
  "$asset_id" \
  "$sha256" \
  "$formula"

test "$(stat -c '%a' "$formula")" = 644
test "$(grep -c '@VERSION@\|@ASSET_ID@\|@SHA256@' "$formula" || true)" -eq 0
grep -q "releases/assets/$asset_id" "$formula"
grep -q 'class PrivateGitHubReleaseAssetDownloadStrategy' "$formula"
grep -q 'using: PrivateGitHubReleaseAssetDownloadStrategy' "$formula"
grep -q 'HOMEBREW_GITHUB_API_TOKEN' "$formula"
grep -q 'secrets:     \[token\]' "$formula"
grep -q 'version "0.1.1"' "$formula"
grep -q "sha256 \"$sha256\"" "$formula"
grep -q 'doc.install "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md", "third_party_licenses"' "$formula"
ruby -c "$formula" >/dev/null

if command -v brew >/dev/null 2>&1; then
  HOMEBREW_NO_AUTO_UPDATE=1 brew style "$formula"
fi

echo "private_formula_template=pass"
