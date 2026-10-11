# CLI contract

Status: normative

Applies to: contract version 1 and the first public release

This document defines observable command-line behavior. Tests must treat these
rules as executable requirements.

## Invocation rules

- `njulogin help`, `njulogin --help`, and `njulogin -h` are read-only.
- `njulogin version` and `njulogin --version` are read-only.
- `njulogin update --check` changes nothing but the update-check state;
  `njulogin update` is the only command that replaces the installed program.
- Running `njulogin` without a subcommand opens the interactive interface only
  when both standard input and standard output are terminals.
- Running without a subcommand in a non-terminal context prints usage to
  standard error and exits with status 2.
- Interactive mode is only a presentation layer over documented commands. It
  MUST NOT expose additional operations or weaker validation.

## Normative invocation surface

Flags for `remote` itself must appear before `<ssh-host>`. Operation flags
appear after the operation name. Unlisted flags, positional arguments, aliases,
and environment variables are not public compatibility surfaces.

| Invocation | Supported inputs |
| --- | --- |
| `njulogin` | No arguments; terminal-only interactive presentation. |
| `njulogin setup` | Optional `--username USER`; prompts for the credential without echo and writes the local credential file. |
| `njulogin status` | No arguments. |
| `njulogin info` | No arguments. |
| `njulogin login` | Optional `--username USER`; exactly zero or one of `--password-command CMD`, `--password-stdin`, `--ask`; optional `--add-quick NAME`. Zero explicit sources means use the saved local credential. |
| `njulogin qr` | Optional `--timeout DURATION`; duration must be positive. |
| `njulogin logout` | No arguments. |
| `njulogin kick` | Exactly one of `--id ID`, `--index N`; `N >= 1`. |
| `njulogin quick list` | No additional arguments. |
| `njulogin quick add` | Required `--name NAME`. |
| `njulogin quick delete` | Required `--id ID`. |
| `njulogin remote` | Before the host: optional `--base-url URL`, `--insecure[=BOOL]`, `--connect-timeout SECONDS`, `--max-time SECONDS`; connect timeout is `1..300` seconds and request timeout is `1..3600` seconds. Then `<ssh-host>` and one operation below. |
| `remote ... status` | No operation arguments. |
| `remote ... info` | No operation arguments. |
| `remote ... login` | Same credential and `--add-quick` inputs as local `login`. |
| `remote ... qr` | No operation arguments; uses the standard 60-second QR confirmation window. |
| `remote ... logout` | No operation arguments. |
| `remote ... kick` | Exactly one of `--id ID`, `--index N`. |
| `remote ... quick list` | No additional arguments. |
| `remote ... quick add` | Required `--name NAME`. |
| `remote ... quick delete` | Required `--id ID`. |
| `njulogin update` | Optional `--check`. |
| `njulogin version`, `njulogin --version` | No additional arguments. |
| `njulogin help`, `njulogin --help`, `njulogin -h` | No additional arguments. |

The supported environment inputs are:

| Variable | Contract |
| --- | --- |
| `NJULOGIN_CONFIG_DIR` | Overrides the directory containing `config.json`; intended for isolated testing and explicit operator-controlled configuration. |
| `NJU_PORTAL_BASE_URL` | Explicit diagnostic/test portal override and default remote portal URL. HTTPS is required except for a loopback HTTP test server. Embedded credentials, queries, and fragments are rejected. |

Portal, CAS, and SSH-routed traffic never uses a proxy. The standard proxy
variables (`https_proxy` and related) apply only to the update requests to
GitHub. No OS keyring, remote-helper, or remote-runtime environment variable is
a public input.

## Exit status

The exit status is the authoritative automation contract.

| Status | Meaning |
| --- | --- |
| `0` | The command completed and its documented postcondition holds. |
| `1` | Runtime, transport, upstream, dependency, parsing, persistence, or operation failure. |
| `2` | Invalid command syntax, conflicting options, or missing required input. |

`status` may return `state=not_authenticated` with exit status 0 because that is
a successfully observed state. A login, logout, device mutation, or QR command
MUST NOT exit 0 merely because an HTTP response was received.

The implementation MUST return nonzero when any of these occurs:

- an HTTP status is outside the accepted range;
- a response exceeds the bounded response-size limit;
- a required response is malformed or cannot be parsed;
- the portal returns a non-success operation code;
- a required postcondition is not observed before a bounded timeout;
- SSH, local credential persistence, or another required dependency fails;
- conflicting credential sources are supplied.

## Output streams

- Standard output contains command results only.
- Standard error contains prompts, warnings, usage errors, and diagnostics.
- Password prompts and interactive questions are written to standard error.
- Error messages MUST NOT echo a password, cookie, QR URL, token, complete
  portal response, full account, full MAC address, IP address, or token-bearing
  request URL.
- Untrusted fields are normalized to one line before they enter stable output.
- When a newer release is known and standard error is a terminal, one notice
  line follows the command's output on standard error.

## Stable text records

The first public release uses line-oriented text records. It does not promise a
JSON interface. Stable scalar records use `key=value`. Repeated redacted rows
use `session_N`, `online_N`, and `quick_N` keys. The value after the first `=`
is opaque human-readable text and MUST NOT be parsed as a secondary format.

Portal `reply_code` may be emitted as an optional diagnostic. Raw `reply_msg`
values MUST NOT be emitted.

## Minimum successful output

| Command | Required successful record |
| --- | --- |
| `setup` | `state=configured`, `credential_store=local_file` |
| `status` | `state=authenticated` or `state=not_authenticated` |
| `info` | `state=...`, `online_devices=N`, `quick_login_devices=N` |
| `login` | `state=authenticated` |
| `qr` | `state=authenticated` |
| `logout` | `state=not_authenticated` |
| `kick` | `result=success` |
| `quick list` | `quick_login_devices=N` |
| `quick add` | `result=success` |
| `quick delete` | `result=success` |
| `update` | the [appkit](https://github.com/soundadam/appkit#命令行) report of the installed and latest versions |
| `version` | exactly `njulogin <version>` |

Remote operations follow the same result contract as their local equivalent.

## Credential selection and storage

A login invocation has three explicit credential sources:

- `--ask`;
- `--password-stdin`;
- `--password-command CMD`.

Rules:

1. At most one explicit source may be selected.
2. If no explicit source is selected, `njulogin` reads the saved username and
   password from its local credential file.
3. An explicit username that differs from the saved username cannot silently
   reuse the saved password; an explicit credential source or a new `setup` is
   required.
4. Conflicting sources are rejected with exit status 2 before network or SSH
   activity.
5. `--password-command` is an advanced operator-controlled integration and is
   terminated if it does not complete within 15 seconds.
6. A password is never accepted as a direct command-line value.
7. Empty username or empty password input is rejected before a login request.

The local credential file contains the username and password in plaintext JSON.
This is an explicit product choice, not encryption. On supported Unix-like
systems:

- the parent directory MUST be mode `0700`;
- the file MUST be a regular non-symlink file with mode `0600`;
- writes MUST use a temporary file and atomic replacement;
- permissive, symlink, or special-file configurations MUST be rejected;
- `setup` is the only command that writes or replaces the saved credential.

The operator is responsible for protecting the local operating-system account,
backups, and filesystem. The first release does not integrate with an OS keyring and does not migrate credentials
from them.

## Command postconditions

Read-only commands succeed only after valid response parsing. Every mutation
requires portal acceptance and the following bounded read-back:

- `login` and `qr`: authenticated state is observed;
- `logout`: unauthenticated state is observed;
- `kick`: the selected online-session target is absent;
- `quick add`: the requested binding is observable;
- `quick delete`: the selected binding is absent.

Retries MUST have a fixed deadline and MUST NOT repeat the mutation itself
unless it is documented as idempotent.

### Setup

`setup` is supported on macOS and Linux source/install environments. It succeeds
only after the credential directory and file meet the permission and file-type
requirements above and the file contains a nonempty username and password.

## Destructive selection

- `kick` accepts either an explicit backend ID or a 1-based displayed index,
  but not both.
- Public examples and interactive flows SHOULD prefer the displayed index.
- Deleting a quick-login binding requires an explicit target.
- No destructive command selects a default target silently.

## Remote execution

Remote operations keep the complete portal implementation local. The local
`njulogin` process performs HTTP, TLS verification, cookies, JSON parsing, CAS
state transitions, QR generation, redaction, and postcondition checks.

For each required TCP connection, the local process starts the operator's SSH
client with an OpenSSH direct-tcpip stdio channel equivalent to:

```text
ssh -T -W <target-host>:<target-port> <ssh-host>
```

The following rules apply:

- the SSH destination is an opaque operator-owned alias or destination;
- destinations beginning with `-`, whitespace, or control characters are
  rejected;
- the product does not edit SSH configuration or provision keys;
- the SSH server must permit direct-tcpip forwarding to the NJU portal and CAS
  destinations;
- the remote host does not need a shell command, curl, Python, a copied helper,
  or an installed `njulogin`;
- HTTP request bodies, cookies, and QR state remain local protocol data flowing
  inside the SSH byte tunnel and are never SSH command arguments;
- TLS is established and verified by the local process end-to-end through the
  tunnel; `--insecure` disables that local verification only for the explicit
  invocation and emits a warning;
- QR URLs are rendered locally and MUST NOT be printed or placed in SSH
  arguments;
- SSH subprocesses and HTTP requests are bounded by the configured timeouts.

An SSH connection succeeding does not establish portal authentication. The same
postcondition rules apply to local and remote operations.

## Redaction contract

Normal output masks account identifiers, personal names, and MAC addresses;
omits IP addresses; normalizes control characters; and never prints passwords,
cookies, QR URLs, raw tokens, or complete responses. Backend device identifiers
may appear only where explicitly required for quick-login deletion and remain
prohibited in real test fixtures, issue reports, CI logs, and release evidence.

## Executable evidence

Before the first public tag, tests must demonstrate at minimum:

- every public command's successful exit and minimum output;
- portal rejection and malformed/oversized responses produce nonzero exit;
- unauthenticated `status` exits 0;
- every conflicting credential-source pair exits 2 without a request;
- local credential file permissions, atomic replacement, and symlink rejection;
- secrets remain absent from process arguments and captured output;
- every remote operation uses SSH `-W` and does not execute curl, Python, or a
  helper remotely;
- remote QR is parsed and rendered by the local process;
- each mutation verifies or times out on its postcondition;
- interactive mode and non-terminal invocation follow the same command rules.
