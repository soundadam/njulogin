// Package fakessh installs a local stand-in for ssh that honors -W by relaying
// bytes to the target, so tunnel tests never contact a real SSH host.
package fakessh

import (
	"os"
	"path/filepath"
	"testing"
)

const relaySource = `import os
import socket
import sys
import threading

host, port = sys.argv[1].rsplit(":", 1)
sock = socket.create_connection((host.strip("[]"), int(port)), timeout=5)

def upload():
    try:
        while True:
            data = os.read(sys.stdin.fileno(), 65536)
            if not data:
                break
            sock.sendall(data)
    finally:
        try:
            sock.shutdown(socket.SHUT_WR)
        except OSError:
            pass

thread = threading.Thread(target=upload, daemon=True)
thread.start()
while True:
    data = sock.recv(65536)
    if not data:
        break
    sys.stdout.buffer.write(data)
    sys.stdout.buffer.flush()
sock.close()
thread.join(timeout=1)
`

const script = `#!/bin/sh
set -eu
printf '%s\n' "$@" >> "$SSH_ARGS_LOG"
target=
previous=
for argument in "$@"; do
  if [ "$previous" = "-W" ]; then
    target=$argument
    break
  fi
  previous=$argument
done
if [ -z "$target" ]; then
  echo 'missing -W target' >&2
  exit 64
fi
exec python3 "$SSH_RELAY_SCRIPT" "$target"
`

// Install puts a fake ssh first on PATH and returns the file that records
// every argument it receives, one per line.
func Install(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "args.log")
	relay := filepath.Join(dir, "relay.py")
	if err := os.WriteFile(relay, []byte(relaySource), 0o600); err != nil {
		t.Fatalf("WriteFile relay: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SSH_ARGS_LOG", argsLog)
	t.Setenv("SSH_RELAY_SCRIPT", relay)
	return argsLog
}
