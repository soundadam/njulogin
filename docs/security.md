# Security model

## Secrets

`setup` stores the password in a macOS generic-password Keychain item and saves
only the username and Keychain service name in local configuration. The
configuration directory is mode `0700`; the file is mode `0600`.

When writing the Keychain item, `njulogin` invokes `security` with `-w` as the
final argument and supplies the password on standard input. The password is
not placed in the process argument list. Keychain reads are local and explicit.

`--password-command` executes an operator-supplied shell command. It is an
advanced integration surface and must point only to a trusted local secret
helper. The product never adds such a command automatically.

## Remote transport

Password material is sent only inside the encrypted SSH channel and on the
remote request's standard input. It is not placed in SSH arguments or written
to a remote file. Remote QR bootstrap state lives in one process and is not
persisted.

## Output

Normal reports mask accounts, names, and MAC addresses and omit IP addresses.
Online sessions are displayed by transient list index. QR URLs and complete
portal responses are not printed.

Backend device identifiers may appear only in an explicitly requested
quick-login device-management listing because deletion requires selecting the
target. They must never be included in CI fixtures or issue reports.

## Trust boundaries

The NJU portal, CAS service, local Keychain, local SSH client, remote SSH
server, and remote Python interpreter are separate trust boundaries. A
successful SSH connection does not prove portal login, and a successful portal
response does not prove arbitrary Internet destinations are reachable.

TLS verification is enabled by default. Insecure TLS and explicit proxy
overrides are diagnostic-only choices and must never become defaults.
