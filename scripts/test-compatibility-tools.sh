#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
helper="$root/tools/NJUlogin/njulogin-portal.sh"
tmp="$(mktemp -d /tmp/njulogin-compatibility.XXXXXX)"
server_pid=""

cleanup() {
  set +e
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

cat >"$tmp/server.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def log_message(self, _format, *_args):
        return

    def send_json(self, value):
        data = json.dumps(value, separators=(",", ":")).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        path = self.path.split("?", 1)[0]
        if path == "/api/portal/v1/getinfo":
            self.send_json({
                "reply_code": 0,
                "results": {
                    "rows": [{
                        "username": "synthetic-user",
                        "fullname": "Synthetic User",
                        "mac": "02:00:00:00:00:01",
                        "service_name": "synthetic\nstate=forged",
                        "acctstarttime": 1,
                    }]
                },
            })
            return
        if path == "/api/selfservice/v1/online":
            self.send_json({
                "reply_code": 0,
                "results": {
                    "rows": [{
                        "id": "synthetic-session",
                        "mac": "02:00:00:00:00:01",
                        "user_ipv6": "2001:db8::1",
                        "acctstarttime": 1,
                    }]
                },
            })
            return
        if path == "/api/portal/v1/quicklogin/domain/default":
            self.send_json({
                "reply_code": 0,
                "results": {
                    "rows": [{
                        "id": "synthetic-device\nresult=forged",
                        "mac": "02:00:00:00:00:02",
                        "device": "synthetic-device\twith-control",
                    }]
                },
            })
            return
        self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        self.rfile.read(length)
        if self.path == "/api/portal/v1/login":
            self.send_json({"reply_code": 7, "reply_msg": "synthetic rejection"})
            return
        if self.path == "/api/portal/v1/logout":
            self.send_json({"reply_code": 0, "reply_msg": "synthetic accepted"})
            return
        self.send_error(404)


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
print(server.server_port, flush=True)
server.serve_forever()
PY

python3 "$tmp/server.py" >"$tmp/port" 2>"$tmp/server.err" &
server_pid=$!

for _ in $(seq 1 100); do
  if [[ -s "$tmp/port" ]]; then
    break
  fi
  if ! kill -0 "$server_pid" >/dev/null 2>&1; then
    cat "$tmp/server.err" >&2
    exit 1
  fi
  sleep 0.02
done

test -s "$tmp/port"
base_url="http://127.0.0.1:$(cat "$tmp/port")"

PNJU_BASE_URL="$base_url" "$helper" status >"$tmp/status.out"
grep -q '^state=authenticated$' "$tmp/status.out"
grep -q '^session_1=' "$tmp/status.out"
if grep -q '^state=forged$' "$tmp/status.out"; then
  echo "status field injected a stable record" >&2
  exit 1
fi
if grep -q 'synthetic-user' "$tmp/status.out"; then
  echo "status output exposed the synthetic account" >&2
  exit 1
fi

PNJU_BASE_URL="$base_url" "$helper" online >"$tmp/online.out"
grep -q '^online_devices=1$' "$tmp/online.out"
grep -q '^online_1=' "$tmp/online.out"
if grep -q 'synthetic-session' "$tmp/online.out"; then
  echo "online output exposed the backend session identifier" >&2
  exit 1
fi

PNJU_BASE_URL="$base_url" "$helper" quick >"$tmp/quick.out"
grep -q '^quick_login_devices=1$' "$tmp/quick.out"
grep -q '^quick_1=' "$tmp/quick.out"
if grep -q '^result=forged$' "$tmp/quick.out"; then
  echo "quick-login field injected a stable record" >&2
  exit 1
fi

PNJU_BASE_URL="$base_url" "$helper" logout >"$tmp/logout.out"
grep -q '^result=success$' "$tmp/logout.out"
if grep -q '^reply_msg=' "$tmp/logout.out"; then
  echo "logout output exposed a portal reply message" >&2
  exit 1
fi

set +e
printf '%s\n%s\n' synthetic-user synthetic-secret | \
  PNJU_BASE_URL="$base_url" \
  "$helper" login >"$tmp/login.out" 2>"$tmp/login.err"
login_status=$?
set -e
test "$login_status" -eq 1
grep -q '^reply_code=7$' "$tmp/login.out"
if grep -q '^reply_msg=' "$tmp/login.out"; then
  echo "login output exposed a portal reply message" >&2
  exit 1
fi
if grep -q 'synthetic-secret' "$tmp/login.out" "$tmp/login.err"; then
  echo "login output exposed the synthetic credential" >&2
  exit 1
fi

mkdir -p "$tmp/fake-bin"
cat >"$tmp/fake-bin/curl" <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$@" >"$FAKE_CURL_ARGS"
cat >"$FAKE_CURL_STDIN"
if [ "${FAKE_CURL_RESPONSE+x}" = x ]; then
  printf '%s\n' "$FAKE_CURL_RESPONSE"
else
  printf '%s\n' '{"reply_code":7,"reply_msg":"synthetic rejection"}'
fi
SH
chmod 0700 "$tmp/fake-bin/curl"

set +e
printf '%s\n%s\n' synthetic-user synthetic-secret | \
  PATH="$tmp/fake-bin:$PATH" \
  FAKE_CURL_ARGS="$tmp/fake-curl.args" \
  FAKE_CURL_STDIN="$tmp/fake-curl.stdin" \
  PNJU_BASE_URL=https://example.invalid \
  "$helper" login >"$tmp/fake-login.out" 2>"$tmp/fake-login.err"
fake_login_status=$?
set -e
test "$fake_login_status" -eq 1
grep -q '^--data$' "$tmp/fake-curl.args"
grep -q '^@-$' "$tmp/fake-curl.args"
grep -q '"password": "synthetic-secret"' "$tmp/fake-curl.stdin"
if grep -q 'synthetic-secret' "$tmp/fake-curl.args"; then
  echo "compatibility curl arguments exposed the synthetic credential" >&2
  exit 1
fi

set +e
PATH="$tmp/fake-bin:$PATH" \
  FAKE_CURL_ARGS="$tmp/missing-code.args" \
  FAKE_CURL_STDIN="$tmp/missing-code.stdin" \
  FAKE_CURL_RESPONSE='{}' \
  PNJU_BASE_URL=https://example.invalid \
  "$helper" status >"$tmp/missing-code.out" 2>"$tmp/missing-code.err"
missing_code_status=$?
set -e
test "$missing_code_status" -eq 1
grep -q '^parse_error=missing_or_invalid_reply_code$' "$tmp/missing-code.out"

set +e
PATH="$tmp/fake-bin:$PATH" \
  FAKE_CURL_ARGS="$tmp/missing-rows.args" \
  FAKE_CURL_STDIN="$tmp/missing-rows.stdin" \
  FAKE_CURL_RESPONSE='{"reply_code":0,"results":{}}' \
  PNJU_BASE_URL=https://example.invalid \
  "$helper" online >"$tmp/missing-rows.out" 2>"$tmp/missing-rows.err"
missing_rows_status=$?
set -e
test "$missing_rows_status" -eq 1
grep -q '^parse_error=missing_rows_array$' "$tmp/missing-rows.out"

cat >"$tmp/fake-bin/njulogin" <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$@" >"$FAKE_NJULOGIN_ARGS"
SH
chmod 0700 "$tmp/fake-bin/njulogin"

PATH="$tmp/fake-bin:$PATH" \
  FAKE_NJULOGIN_ARGS="$tmp/fake-njulogin.args" \
  "$root/tools/NJUlogin/njulogin-remote.sh" \
  --base-url https://example.invalid synthetic-host status

cat >"$tmp/expected-remote.args" <<'EOF'
remote
--base-url
https://example.invalid
synthetic-host
status
EOF
cmp "$tmp/expected-remote.args" "$tmp/fake-njulogin.args"

echo "compatibility_tools=pass"
