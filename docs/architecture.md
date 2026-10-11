# Architecture and product boundary

This document explains the implementation shape. Public behavior and scope are
governed by [`SPEC.md`](../SPEC.md) and
[`docs/cli-contract.md`](cli-contract.md).

## Components

- `cmd/njulogin/main.go` owns CLI parsing, local credentials, portal operations,
  CAS QR state, output, and postcondition verification. A cobra root command
  dispatches by name; each command parses its own flags.
- [appkit](https://github.com/soundadam/appkit) supplies `update`, `version`,
  `--version`, and the update notice.
- `cmd/njulogin/ssh_tunnel.go` adapts OpenSSH `-W` direct-tcpip stdio channels
  into local `net.Conn` values used by Go's HTTP transport.
- `cmd/njulogin/subprocess_unix.go` and `subprocess_other.go` provide bounded
  subprocess cancellation, including process-group termination on Unix-like
  systems.
- `tools/NJUlogin/` contains source-level compatibility and diagnostic wrappers.
  Release archives do not include them.

The Go binary owns the product interface. No Python source or remote helper is
part of the installed product path.

## Development and acceptance environments

Source work happens on a branch in any maintainer clone and reaches `main`
through a pull request. Product source CI runs the same service-isolated gate
on Linux.

The maintainer's macOS machine is also the installed-product acceptance
environment: it receives a candidate through the Homebrew Cask and exercises the
installed binary. The normative workflow is defined in
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
