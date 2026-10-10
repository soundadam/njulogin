# Architecture and product boundary

This document explains the implementation shape. Public behavior and scope are
governed by [`SPEC.md`](../SPEC.md) and
[`docs/cli-contract.md`](cli-contract.md).

## Components

- `cmd/njulogin` is the entry point; it links the version and calls
  `internal/cli`.
- `internal/cli` owns cobra command parsing, credential-source selection, and
  output records. Every command runs once against a `Target`:
  `Local` reaches the portal directly, `SSH` through a tunnel. Local and remote
  operations differ only in the target they receive.
- `internal/portal` speaks the portal JSON API behind the `portal.API`
  interface and verifies postconditions with `portal.Poll`.
- `internal/cas` drives the CAS QR login; a `cas.Renderer` draws the QR code
  locally.
- `internal/credential` persists the local credential behind `credential.Store`
  and reads passwords from `credential.Source` implementations: password
  command, standard input, or no-echo terminal prompt.
- `internal/transport` builds the HTTP clients. A `transport.Dialer` decides
  where TCP connections originate; HTTP, TLS, and cookies stay in-process.
- `internal/sshtunnel` is the `Dialer` that adapts OpenSSH `-W` direct-tcpip
  stdio channels into `net.Conn` values.
- `internal/proc` provides bounded subprocess cancellation, including
  process-group termination on Unix-like systems.
- `internal/redact` normalizes and masks untrusted portal fields.
- `internal/tui` is the bubbletea interface opened by bare `njulogin`. It
  reaches operations only through the `tui.Backend` interface, which
  `internal/cli` implements with the same App operations the commands run.
  Text inputs can paste from the system clipboard, which runs the local
  platform paste tool (`pbpaste`, `xclip`, `xsel`, or `wl-paste`).
- `tools/NJUlogin/` contains source-level compatibility and diagnostic wrappers.
  Homebrew does not install them.

The Go binary owns the product interface. No Python source or remote helper is
part of the installed product path.

## Development and acceptance environments

The canonical source tree is on the Elon Linux host. Source edits, tests,
linting, review, and release preparation occur there. Product source CI runs the
same service-isolated gate on Linux.

The local macOS machine is an installed-product acceptance environment. It
receives a candidate through Homebrew and exercises the installed binary. It
does not act as a second source tree. The normative workflow is defined in
[`docs/development-topology.md`](development-topology.md).

## Local credential storage

`setup` writes one username/password pair to the local `config.json`. On
supported Unix-like systems the parent directory is `0700` and the regular file
is `0600`. Writes are atomic. The password is plaintext and the product does not
integrate with an operating-system keyring.

## Local operations

Local status, login, logout, device management, and QR operations contact the
NJU portal directly. The HTTP client keeps cookies in memory for the duration of
the command and does not persist portal responses or cookies.

## SSH-routed operations

Remote operations use a local bootstrap transport, not remote application code.
For each required TCP connection, `njulogin` starts the local SSH client with a
fixed `-W target-host:target-port` direct-tcpip request. Go's local HTTP client
then reads and writes the SSH process's stdio as a network connection.

Consequences:

- HTTP, TLS, CookieJar, JSON, CAS forms, QR polling, QR rendering, redaction, and
  postcondition verification remain local;
- the remote host requires only SSH forwarding permission;
- no curl, Python, copied helper, installed CLI, or remote shell command is
  required;
- normal SSH configuration, aliases, ProxyJump, and authentication remain owned
  by the operator;
- portal TLS is verified locally end-to-end through the tunnel.

The product does not own SSH aliases, keys, jump hosts, VPNs, routes, proxies,
or remote server configuration. An SSH server that disables direct-tcpip
forwarding is an unsupported path and produces a transport failure.

## Out of scope

- SoundVPN, NJUConnect, EasyConnect, and proxy selector management;
- background login, monitoring, launchd, or a network daemon;
- server inventory, desired state, or remote helper deployment;
- OS-keyring integration, credential synchronization, account recovery, or
  password rotation;
- encryption-key management for the local credential file;
- analytics, telemetry, or persistence of portal responses.
