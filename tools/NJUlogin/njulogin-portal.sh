#!/usr/bin/env bash
set -euo pipefail
#
# Source model: Do1e/NJUlogin p.nju.edu.cn CLI
# https://github.com/Do1e/NJUlogin
#
# This helper keeps a dependency-light portal path for remote hosts where the
# upstream Python package is not installed. Use njulogin-remote.sh QR modes when
# QRlogin or full upstream behavior is required.

mode="${1:-status}"
shift || true

base_url="${PNJU_BASE_URL:-https://p.nju.edu.cn}"
connect_timeout="${PNJU_CONNECT_TIMEOUT:-5}"
max_time="${PNJU_MAX_TIME:-15}"

curl_args=(
  curl
  -sS
  --connect-timeout "$connect_timeout"
  --max-time "$max_time"
)

if [[ -n "${PNJU_CURL_PROXY:-}" ]]; then
  curl_args+=(-x "$PNJU_CURL_PROXY")
else
  curl_args+=(--noproxy '*')
fi

if [[ "${PNJU_INSECURE:-0}" == "1" ]]; then
  curl_args+=(-k)
fi

usage() {
  cat <<'EOF'
Usage:
  njulogin-portal.sh status
  njulogin-portal.sh probe
  njulogin-portal.sh login
  njulogin-portal.sh logout
  njulogin-portal.sh info
  njulogin-portal.sh online
  njulogin-portal.sh quick
  njulogin-portal.sh add-quick-login DEVICE_NAME
  njulogin-portal.sh delete-quick-login DEVICE_ID
  njulogin-portal.sh kick ONLINE_ID

Environment:
  PNJU_BASE_URL=https://p.nju.edu.cn
  PNJU_CURL_PROXY=http://127.0.0.1:7700   # optional; direct by default
  PNJU_INSECURE=1                         # optional curl -k
  NJU_USERNAME=... NJU_PASSWORD=...       # login only
EOF
}

now_ms() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

request_get() {
  local path="$1"
  "${curl_args[@]}" "$base_url$path"
}

request_post_json() {
  local path="$1"
  local body="$2"
  "${curl_args[@]}" \
    -H 'Content-Type: application/json' \
    -X POST \
    --data "$body" \
    "$base_url$path"
}

login_body() {
  python3 - <<'PY'
import json
import os

print(json.dumps({
    "username": os.environ["NJU_USERNAME"],
    "password": os.environ["NJU_PASSWORD"],
}, ensure_ascii=False))
PY
}

device_body() {
  local device_name="$1"
  PNJU_DEVICE_NAME="$device_name" python3 - <<'PY'
import json
import os

print(json.dumps({"device": os.environ["PNJU_DEVICE_NAME"]}, ensure_ascii=False))
PY
}

summarize_json() {
  local kind="$1"
  local raw="$2"
  PNJU_JSON="$raw" python3 - "$kind" <<'PY'
import datetime as dt
import json
import os
import sys

kind = sys.argv[1]
raw = os.environ.get("PNJU_JSON", "")

def mask_text(value):
    value = "" if value is None else str(value)
    if not value:
        return "unknown"
    if len(value) <= 2:
        return value[0] + "*" if len(value) == 2 else "*"
    return value[0] + ("*" * max(1, len(value) - 2)) + value[-1]

def mask_account(value):
    value = "" if value is None else str(value)
    if len(value) <= 6:
        return mask_text(value)
    return value[:3] + "***" + value[-3:]

def mask_mac(value):
    value = "" if value is None else str(value)
    parts = value.split(":")
    if len(parts) == 6:
        return ":".join(parts[:2] + ["xx", "xx", "xx"] + parts[-1:])
    return "redacted" if value else "unknown"

def fmt_time(value):
    try:
        return dt.datetime.fromtimestamp(int(value)).strftime("%Y-%m-%d %H:%M:%S")
    except Exception:
        return "unknown"

try:
    data = json.loads(raw)
except Exception as exc:
    print(f"parse_error={exc}")
    print("raw_available=true")
    sys.exit(1)

reply_code = data.get("reply_code")
reply_msg = data.get("reply_msg")
if reply_code is not None:
    print(f"reply_code={reply_code}")
if reply_msg:
    print(f"reply_msg={reply_msg}")

results = data.get("results")
if not isinstance(results, dict):
    if kind in {"login", "logout", "add", "delete", "kick"}:
        print("operation_response=received")
    sys.exit(0)

rows = results.get("rows")
if kind == "getinfo":
    if reply_code == 0 and isinstance(rows, list) and rows:
        print("state=authenticated")
        print(f"sessions={len(rows)}")
        for index, row in enumerate(rows, start=1):
            print(
                "session_%d=%s %s start=%s service=%s mac=%s ip=redacted"
                % (
                    index,
                    mask_account(row.get("username")),
                    mask_text(row.get("fullname")),
                    fmt_time(row.get("acctstarttime")),
                    row.get("service_name") or "unknown",
                    mask_mac(row.get("mac")),
                )
            )
    else:
        print("state=not_authenticated")
        if isinstance(rows, list):
            print(f"sessions={len(rows)}")
    sys.exit(0)

if kind == "online":
    online_rows = rows if isinstance(rows, list) else []
    print(f"online_devices={len(online_rows)}")
    for row in online_rows:
        print(
            "online id=%s start=%s mac=%s ipv4=redacted ipv6=%s"
            % (
                row.get("id", "unknown"),
                fmt_time(row.get("acctstarttime")),
                mask_mac(row.get("mac")),
                "present" if row.get("user_ipv6") else "none",
            )
        )
    sys.exit(0)

if kind == "quick":
    quick_rows = rows if isinstance(rows, list) else []
    if reply_code == 404:
        print("quick_login_devices=0")
        sys.exit(0)
    print(f"quick_login_devices={len(quick_rows)}")
    for row in quick_rows:
        print(
            "quick device_id=%s mac=%s name=%s"
            % (
                row.get("id", "unknown"),
                mask_mac(row.get("mac")),
                row.get("device") or "unknown",
            )
        )
PY
}

status() {
  local raw
  raw="$(request_get "/api/portal/v1/getinfo?_=$(now_ms)")"
  summarize_json getinfo "$raw"
}

online() {
  local raw
  raw="$(request_get "/api/selfservice/v1/online?page=1&limit=10000&_=$(now_ms)")"
  summarize_json online "$raw"
}

quick() {
  local raw
  raw="$(request_get "/api/portal/v1/quicklogin/domain/default?_=$(now_ms)")"
  summarize_json quick "$raw"
}

case "$mode" in
  -h|--help|help)
    usage
    ;;
  status|getinfo)
    status
    ;;
  headers)
    "${curl_args[@]}" -I "$base_url"
    ;;
  probe)
    printf '== portal headers ==\n'
    "${curl_args[@]}" -I "$base_url" || true
    printf '\n== status ==\n'
    status || true
    ;;
  login)
    : "${NJU_USERNAME:?set NJU_USERNAME for login}"
    : "${NJU_PASSWORD:?set NJU_PASSWORD for login}"
    raw="$(request_post_json "/api/portal/v1/login" "$(login_body)")"
    summarize_json login "$raw"
    printf '\n== status ==\n'
    status
    ;;
  logout)
    raw="$(request_post_json "/api/portal/v1/logout" '{"domain":"default"}')"
    summarize_json logout "$raw"
    ;;
  info)
    printf '== status ==\n'
    status
    printf '\n== online ==\n'
    online
    printf '\n== quick-login ==\n'
    quick
    ;;
  online)
    online
    ;;
  quick|quick-login)
    quick
    ;;
  add|add-quick-login)
    device_name="${1:?missing DEVICE_NAME}"
    raw="$(request_post_json "/api/portal/v1/quicklogin/bind/default" "$(device_body "$device_name")")"
    summarize_json add "$raw"
    ;;
  delete|delete-quick-login)
    device_id="${1:?missing DEVICE_ID}"
    raw="$(request_post_json "/api/portal/v1/quicklogin/unbind/id/$device_id" '{}')"
    summarize_json delete "$raw"
    ;;
  kick)
    online_id="${1:?missing ONLINE_ID}"
    raw="$(request_post_json "/api/selfservice/v1/online/kick/$online_id" '{}')"
    summarize_json kick "$raw"
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
