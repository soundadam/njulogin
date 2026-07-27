#!/usr/bin/env bash
set -euo pipefail
#
# Source model: Do1e/NJUlogin p.nju.edu.cn CLI
# https://github.com/Do1e/NJUlogin
#
# The upstream package is used for QRlogin/full CLI modes when it is available
# on the remote host. The local portal helper is used for dependency-light
# status and password login flows.

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
helper="$script_dir/njulogin-portal.sh"
remote_helper="${NJU_REMOTE_HELPER:-/tmp/njulogin-portal.sh}"
repo_root="$(cd -- "$script_dir/../.." && pwd)"

if command -v njulogin >/dev/null 2>&1; then
  exec njulogin remote "$@"
fi

if command -v go >/dev/null 2>&1 && [[ -f "$repo_root/go.mod" ]]; then
  cd "$repo_root"
  exec go run ./cmd/njulogin remote "$@"
fi

usage() {
  cat >&2 <<'EOF'
Usage:
  njulogin-remote.sh <ssh-host> [operation] [arg]

Dependency-light portal operations:
  status, probe, login, login-add DEVICE_NAME, logout, info, online, quick,
  add DEVICE_NAME, delete DEVICE_ID, kick ONLINE_ID, keychain-login USER [DEVICE_NAME]

Remote QR/upstream NJUlogin operations:
  qr-login, pwd-login, upstream-info, upstream-add DEVICE_NAME,
  upstream-delete DEVICE_ID, upstream-kick ONLINE_ID, upstream-logout

Examples:
  tools/NJUlogin/njulogin-remote.sh <ssh-host> status
  tools/NJUlogin/njulogin-remote.sh <ssh-host> qr-login
  tools/NJUlogin/njulogin-remote.sh <ssh-host> keychain-login <username> <device-name>
  tools/NJUlogin/njulogin-remote.sh <ssh-host> login-add <device-name>
  tools/NJUlogin/njulogin-remote.sh <ssh-host> upstream-info

QRlogin first uses a remote njulogin binary when present, then falls back to
upstream NJUlogin. The QR code shown in this SSH session authenticates the
remote network path.
EOF
}

quote() {
  printf '%q' "$1"
}

remote_exports() {
  printf 'export PNJU_CURL_PROXY=%q PNJU_INSECURE=%q PNJU_BASE_URL=%q PNJU_CONNECT_TIMEOUT=%q PNJU_MAX_TIME=%q; ' \
    "${PNJU_CURL_PROXY:-}" \
    "${PNJU_INSECURE:-0}" \
    "${PNJU_BASE_URL:-https://p.nju.edu.cn}" \
    "${PNJU_CONNECT_TIMEOUT:-5}" \
    "${PNJU_MAX_TIME:-15}"
}

install_helper() {
  local host="$1"
  ssh "$host" "install -m 700 /dev/stdin $(quote "$remote_helper")" < "$helper"
}

run_remote() {
  local host="$1"
  shift
  local cmd
  cmd="$(remote_exports)$(quote "$remote_helper")"
  while (($#)); do
    cmd+=" $(quote "$1")"
    shift
  done
  ssh "$host" "$cmd"
}

run_upstream() {
  local host="$1"
  shift
  local args=""
  local arg
  for arg in "$@"; do
    args+=" $(quote "$arg")"
  done
  local cmd
  cmd="if command -v NJUlogin >/dev/null 2>&1; then exec NJUlogin$args; "
  cmd+="elif command -v uvx >/dev/null 2>&1; then exec uvx NJUlogin$args; "
  cmd+="elif command -v uv >/dev/null 2>&1; then exec uv tool run NJUlogin$args; "
  cmd+="else echo 'missing remote NJUlogin runner: install NJUlogin or uvx for upstream modes' >&2; exit 127; fi"
  ssh -tt "$host" "$cmd"
}

run_remote_qr() {
  local host="$1"
  local cmd
  cmd="if command -v njulogin >/dev/null 2>&1; then exec njulogin qr; "
  cmd+="elif command -v NJUlogin >/dev/null 2>&1; then exec NJUlogin -i; "
  cmd+="elif command -v uvx >/dev/null 2>&1; then exec uvx NJUlogin -i; "
  cmd+="elif command -v uv >/dev/null 2>&1; then exec uv tool run NJUlogin -i; "
  cmd+="else echo 'missing remote QR runner: install njulogin, NJUlogin, or uvx' >&2; exit 127; fi"
  ssh -tt "$host" "$cmd"
}

run_login() {
  local host="$1"
  local add_device="${2:-}"
  local cmd
  cmd="$(remote_exports)"
  cmd+="read -r -p 'NJU username: ' NJU_USERNAME; "
  cmd+="read -r -s -p 'NJU credential: ' NJU_PASSWORD; echo; "
  cmd+="export NJU_USERNAME NJU_PASSWORD; "
  cmd+="$(quote "$remote_helper") login"
  if [[ -n "$add_device" ]]; then
    cmd+=" && $(quote "$remote_helper") add-quick-login $(quote "$add_device")"
    cmd+=" && $(quote "$remote_helper") quick"
  fi
  ssh -tt "$host" "$cmd"
}

run_keychain_login() {
  local host="$1"
  local username="$2"
  local add_device="${3:-}"
  local service="${NJU_KEYCHAIN_SERVICE:-NJUlogin}"
  local secret
  secret="$(security find-generic-password -s "$service" -a "$username" -w)"
  local cmd
  cmd="$(remote_exports)"
  cmd+="read -r NJU_USERNAME; IFS= read -r NJU_PASSWORD; "
  cmd+="export NJU_USERNAME NJU_PASSWORD; "
  cmd+="$(quote "$remote_helper") login"
  if [[ -n "$add_device" ]]; then
    cmd+=" && $(quote "$remote_helper") add-quick-login $(quote "$add_device")"
    cmd+=" && $(quote "$remote_helper") quick"
  fi
  printf '%s\n%s\n' "$username" "$secret" | ssh "$host" "$cmd"
}

if [[ $# -lt 1 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 2
fi

if [[ ! -f "$helper" ]]; then
  echo "missing helper: $helper" >&2
  exit 1
fi

host="$1"
mode="${2:-status}"
shift 2 || true

install_helper "$host"

case "$mode" in
  status|getinfo|probe|headers|logout|info|online|quick|quick-login)
    run_remote "$host" "$mode" "$@"
    ;;
  add|add-quick-login)
    run_remote "$host" add-quick-login "${1:?missing DEVICE_NAME}"
    ;;
  delete|delete-quick-login)
    run_remote "$host" delete-quick-login "${1:?missing DEVICE_ID}"
    ;;
  kick)
    run_remote "$host" kick "${1:?missing ONLINE_ID}"
    ;;
  login)
    run_login "$host"
    ;;
  login-add)
    run_login "$host" "${1:?missing DEVICE_NAME}"
    ;;
  keychain-login)
    run_keychain_login "$host" "${1:?missing USER}" "${2:-}"
    ;;
  qr-login|login-qr)
    run_remote_qr "$host"
    ;;
  pwd-login|login-pwd)
    run_upstream "$host" -l pwdLogin -i
    ;;
  upstream-info)
    run_upstream "$host" -p
    ;;
  upstream-add)
    run_upstream "$host" -a "${1:?missing DEVICE_NAME}"
    ;;
  upstream-delete)
    run_upstream "$host" -d "${1:?missing DEVICE_ID}"
    ;;
  upstream-kick)
    run_upstream "$host" -k "${1:?missing ONLINE_ID}"
    ;;
  upstream-logout)
    run_upstream "$host" -o
    ;;
  *)
    usage
    exit 2
    ;;
esac
