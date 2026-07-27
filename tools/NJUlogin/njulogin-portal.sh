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
  login reads username and credential as two lines from standard input
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
  printf '%s' "$body" | "${curl_args[@]}" \
    -H 'Content-Type: application/json' \
    -X POST \
    --data '@-' \
    "$base_url$path"
}

login_body() {
  python3 -c '
import json
import sys

username = sys.stdin.readline().rstrip("\n")
password = sys.stdin.readline().rstrip("\n")
if not username or not password:
    raise SystemExit("login requires nonempty username and credential on stdin")
print(json.dumps({"username": username, "password": password}, ensure_ascii=False))
'
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

def normalize_field(value):
    value = "" if value is None else str(value)
    return " ".join(value.split())

def output_field(value, fallback="unknown"):
    value = normalize_field(value)
    return value if value and value != "<nil>" else fallback

def mask_text(value):
    value = normalize_field(value)
    if not value:
        return "unknown"
    if len(value) <= 2:
        return value[0] + "*" if len(value) == 2 else "*"
    return value[0] + ("*" * max(1, len(value) - 2)) + value[-1]

def mask_account(value):
    value = normalize_field(value)
    if len(value) <= 6:
        return mask_text(value)
    return value[:3] + "***" + value[-3:]

def mask_mac(value):
    value = normalize_field(value)
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

if not isinstance(data, dict):
    print("parse_error=top_level_not_object")
    sys.exit(1)

reply_code = data.get("reply_code")
if type(reply_code) is not int:
    print("parse_error=missing_or_invalid_reply_code")
    sys.exit(1)
print(f"reply_code={reply_code}")

mutation_kinds = {"login", "logout", "add", "delete", "kick"}
if kind in mutation_kinds:
    if reply_code != 0:
        sys.exit(1)
    print("result=success")
    sys.exit(0)

if kind == "getinfo" and reply_code != 0:
    print("state=not_authenticated")
    sys.exit(0)

if kind == "quick" and reply_code == 404:
    print("quick_login_devices=0")
    sys.exit(0)

results = data.get("results")
if not isinstance(results, dict):
    print("parse_error=missing_results_object")
    sys.exit(1)

rows = results.get("rows")
if kind == "getinfo":
    if not isinstance(rows, list):
        print("parse_error=missing_rows_array")
        sys.exit(1)
    if rows:
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
                    output_field(row.get("service_name")),
                    mask_mac(row.get("mac")),
                )
            )
    else:
        print("state=not_authenticated")
        print("sessions=0")
    sys.exit(0)

if kind == "online":
    if reply_code != 0:
        sys.exit(1)
    if not isinstance(rows, list):
        print("parse_error=missing_rows_array")
        sys.exit(1)
    online_rows = rows
    print(f"online_devices={len(online_rows)}")
    for index, row in enumerate(online_rows, start=1):
        print(
            "online_%d=start=%s mac=%s ipv4=redacted ipv6=%s"
            % (
                index,
                fmt_time(row.get("acctstarttime")),
                mask_mac(row.get("mac")),
                "present" if row.get("user_ipv6") else "none",
            )
        )
    sys.exit(0)

if kind == "quick":
    if reply_code != 0:
        sys.exit(1)
    if not isinstance(rows, list):
        print("parse_error=missing_rows_array")
        sys.exit(1)
    quick_rows = rows
    print(f"quick_login_devices={len(quick_rows)}")
    for index, row in enumerate(quick_rows, start=1):
        print(
            "quick_%d=device_id=%s mac=%s name=%s"
            % (
                index,
                output_field(row.get("id")),
                mask_mac(row.get("mac")),
                output_field(row.get("device")),
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
