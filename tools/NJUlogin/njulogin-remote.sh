#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
  cat >&2 <<'EOF'
Usage:
  tools/NJUlogin/njulogin-remote.sh [remote flags] <ssh-host> <operation> [args]

This is a source-tree compatibility wrapper for:
  njulogin remote [remote flags] <ssh-host> <operation> [args]

It does not install a remote helper and does not execute curl, Python, or
njulogin on the remote host.
EOF
}

if [[ $# -lt 2 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 2
fi

if command -v njulogin >/dev/null 2>&1; then
  exec njulogin remote "$@"
fi

if [[ -x "$root/bin/njulogin" ]]; then
  exec "$root/bin/njulogin" remote "$@"
fi

echo "missing local njulogin executable: install it or run make build" >&2
exit 1
