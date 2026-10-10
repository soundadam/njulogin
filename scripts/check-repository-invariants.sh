#!/usr/bin/env bash
set -euo pipefail

git diff --check

secret_pattern='-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}'
if rg -n --hidden -e "$secret_pattern" . \
  -g '!.git/**' -g '!bin/**' -g '!.build/**' -g '!dist/**' -g '!.tools/**'; then
  echo "Potential credential material found in tracked files." >&2
  exit 1
fi

# Match literal shell variable references in tracked source; expansion here
# would defeat the repository invariant.
# shellcheck disable=SC2016
argument_pattern='"-w",[[:space:]]*password|--data[[:space:]]+"\$(body|secret|password)"|--data[[:space:]]+"\$NJU_PASSWORD"'
if rg -n -e "$argument_pattern" cmd tools scripts; then
  echo "Credential material must not be placed in process arguments or production literals." >&2
  exit 1
fi

if rg -ni 'keychain-service|remote_qr_bootstrap|NJU_PROXY_URL|PNJU_INSECURE' \
  cmd README.md SPEC.md SECURITY.md docs tools/NJUlogin/README.md \
  tools/NJUlogin/njulogin-remote.sh; then
  echo "Deleted OS-keyring, remote-bootstrap, and remote-proxy surfaces must not return." >&2
  exit 1
fi

if rg -n '"(curl|python|python3)"|sh -c|remote helper' \
  cmd/njulogin/main.go cmd/njulogin/ssh_tunnel.go \
  cmd/njulogin/subprocess_unix.go cmd/njulogin/subprocess_other.go; then
  echo "The installed Go product must not execute portal tooling on the remote host." >&2
  exit 1
fi

if ! grep -q '"-W"' cmd/njulogin/ssh_tunnel.go; then
  echo "The SSH transport must remain an OpenSSH -W direct-tcpip tunnel." >&2
  exit 1
fi

if git ls-files | grep -E '(^|/)(bin|\.build|__pycache__)(/|$)|\.pyc$'; then
  echo "Generated build or Python cache files are tracked." >&2
  exit 1
fi

generated_outside_build="$({
  find . \
    -path './.git' -prune -o \
    -path './.build' -prune -o \
    -path './bin' -prune -o \
    -path './dist' -prune -o \
    -path './.tools' -prune -o \
    -type f \( -name '*.exe' -o -name '*.test' -o -name 'njulogin' \) -print
} | sed -n '1,20p')"
if [[ -n "$generated_outside_build" ]]; then
  printf '%s\n' "$generated_outside_build" >&2
  echo "Generated executables must remain under .build/, bin/, dist/, or .tools/." >&2
  exit 1
fi

echo "repository_invariants=pass"
