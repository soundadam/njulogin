# Service-isolated portal, CAS, and SSH testing

This document records the synthetic infrastructure used to validate `njulogin`
without contacting the real NJU portal, CAS service, or an operator account.
It is descriptive evidence; the executable contract remains in
[`docs/development-contract.md`](development-contract.md) and
[`docs/cli-contract.md`](cli-contract.md).

## Objectives

The fixture suite proves that the installed CLI can exercise the complete local
and SSH-routed protocol while preserving these boundaries:

- no real account, password, Cookie, QR token, session ID, MAC address, or IP
  address is captured;
- HTTP, TLS, Cookie handling, JSON parsing, CAS polling, QR rendering, redaction,
  and postcondition verification run in the local `njulogin` process;
- the SSH destination provides only an OpenSSH `-W` direct-tcpip byte path;
- no portal helper, curl command, Python bootstrap, or installed `njulogin` runs
  on the SSH destination;
- all host mappings, packet-redirection rules, listeners, credentials, packages,
  and temporary taps are removed after the run.

## Synthetic portal

A loopback-only HTTP server implements the minimal response shapes needed for:

- unauthenticated and authenticated status;
- password login and logout;
- online-session listing and kick-by-index;
- quick-login list, add, and delete;
- mutation read-back verification.

The fixture stores only in-memory synthetic state. Requests use clearly
artificial values, and response output is checked for the stable `session_N`,
`online_N`, and `quick_N` records and for required redaction.

## Synthetic CAS and QR flow

A loopback TLS fixture presents a synthetic certificate for the CAS hostname and
implements:

- the CAS login form fields;
- QR token issuance;
- status transitions from waiting, to confirmed-on-phone, to accepted;
- the QR login form submission;
- the final authenticated portal read-back.

The first request is made with normal TLS verification and must reject the
synthetic certificate. A separate explicit `--insecure` invocation then proves
the positive QR path. Tests assert that the QR token, QR URL, CAS URL, and raw
responses never appear in stdout, stderr, SSH arguments, or evidence notes.

The hostname mapping and local packet-redirection rule are uniquely marked,
restricted to a reserved loopback address and the test user, and removed in the
fixture cleanup trap. The existing host HTTPS service is not stopped or
reconfigured.

## Real OpenSSH path

The strongest acceptance run installs a Homebrew-built candidate on macOS and
uses the operator's existing `elon` SSH alias. A local wrapper records arguments
and immediately executes `/usr/bin/ssh`, so authentication, SSH configuration,
and direct-tcpip forwarding are provided by the real OpenSSH client and Elon
sshd.

The installed CLI completes status, saved-credential login, info, quick-login
mutations, kick, logout, TLS rejection, and QR login through this path. The run
observed 28 SSH invocations, all using `-W`; no invocation contained a password,
QR token, QR URL, curl, Python, or remote helper command.

## Homebrew-installed acceptance

The macOS acceptance run also verifies:

- source installation and Formula tests;
- exact installed version reporting;
- local plaintext credential setup through a real PTY;
- configuration directory mode `0700` and file mode `0600`;
- exact password round-trip, including leading and trailing spaces;
- saved-credential reuse;
- `brew linkage --test`;
- package, temporary tap, transferred artifact, fixture, and credential cleanup.

Sanitized evidence is retained outside the repository. As of July 27, 2026, the
real Mac-to-Elon SSH acceptance evidence digest is:

```text
b18f591dc0e4471a4529bd9ac10d4d78f8f9da1f1ae86b82ea5fcc14d20193a7
```

The evidence intentionally contains outcomes and versions only, not terminal
transcripts or fixture secrets.

## What this does not prove

Synthetic acceptance does not prove current compatibility with the live NJU
portal or CAS service. Real-service acceptance remains an explicit operator
release step and must use the Homebrew-installed candidate without recording
sensitive data.
