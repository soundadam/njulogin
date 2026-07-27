# Architecture and product boundary

## Components

- `cmd/njulogin/main.go` is the installed CLI.
- `cmd/njulogin/remote_qr_bootstrap.py` is embedded into the Go binary and
  streamed over SSH for remote QR login. It uses only Python's standard
  library on the remote host.
- `tools/NJUlogin/` contains compatibility shell entrypoints retained for
  migration and source-level diagnostics. Homebrew does not install them.

The Go binary owns the daily interface. Compatibility scripts must not become
a second product surface.

## Local operations

Local status, login, logout, device management, and QR operations contact the
NJU portal directly from the current machine. The HTTP client keeps cookies in
memory for the duration of the command and does not persist them.

## Remote operations

Remote operations invoke the local SSH client. Portal requests therefore leave
from the remote machine's network path. Password login sends a JSON request
body on SSH standard input. QR login streams the embedded bootstrap to a remote
Python interpreter and renders the returned short-lived QR value locally
without printing it.

The product does not own SSH aliases, keys, jump hosts, VPNs, routes, proxies,
or remote service configuration. Managed infrastructure may require a
repository-owned execution wrapper instead of the generic SSH path.

## Out of scope

- SoundVPN, NJUConnect, EasyConnect, and proxy selector management;
- background login, monitoring, launchd, or a network daemon;
- server inventory or desired state;
- credential synchronization, account recovery, or password rotation;
- analytics, telemetry, or persistence of portal responses.
