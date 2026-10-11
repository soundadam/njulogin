# NJUlogin product contract

Status: normative for the first public release

Contract version: 1

This document defines what `njulogin` is allowed to become. Implementation,
tests, packaging, and documentation must remain consistent with this contract.
The words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are
normative.

## Authority

When repository materials disagree, use this order:

1. `SPEC.md` and the normative documents it links;
2. executable tests that demonstrate those contracts;
3. architecture and security explanations;
4. README examples and compatibility notes;
5. current implementation behavior.

Current behavior is not automatically the intended contract. A bug does not
become a feature because it already exists.

The normative documents are:

- [`docs/cli-contract.md`](docs/cli-contract.md);
- [`docs/security.md`](docs/security.md);
- [`docs/development-contract.md`](docs/development-contract.md);
- [`docs/development-topology.md`](docs/development-topology.md);
- [`docs/review-handoff.md`](docs/review-handoff.md);
- [`docs/release-readiness.md`](docs/release-readiness.md).

## Product promise

`njulogin` is a focused command-line client for the Nanjing University
`p.nju.edu.cn` authentication portal. It provides explicit, operator-invoked
local and SSH-routed operations for:

- inspecting redacted authentication and device state;
- password or CAS QR login;
- logout;
- online-session removal;
- quick-login device listing, binding, and removal.

It is not a daemon, control plane, network manager, credential synchronizer, or
remote software deployment system.

The repository, Go module suffix, Cask token, release archive prefix, and
executable are named `njulogin`; `NJUlogin` is only the human-facing product
name.

## Supported surface

The first public release supports:

- the compiled `njulogin` executable on macOS 13 or newer and on Linux, for
  x86_64 and aarch64;
- the release archives on GitHub Releases and the Homebrew Cask in
  `soundadam/homebrew-tap`, named and installed as the
  [appkit contract](https://github.com/soundadam/appkit) defines;
- source builds and local credential storage on maintained macOS and Linux
  environments using the Go toolchain declared by `go.mod`;
- SSH destinations configured and owned by the operator;
- remote operations through an OpenSSH-compatible direct-tcpip stdio channel
  (`ssh -W`) permitted by the SSH server.

The remote host does not require curl, Python, a copied helper, an installed
`njulogin`, or a usable interactive shell. HTTP, TLS, cookies, JSON, CAS QR
state, QR rendering, and result validation remain in the local Go process.

The files under `tools/NJUlogin/` are migration and diagnostic aids. They are
not a second public product interface and MUST NOT acquire functionality absent
from the Go CLI.

Maintainer development and macOS acceptance use the separated environment roles
defined in [`docs/development-topology.md`](docs/development-topology.md).

## Product invariants

Every change MUST preserve these invariants unless the contract is changed
explicitly first:

1. **Explicit operation.** No login, logout, device mutation, SSH connection,
   credential write, self-update, or persistent state change occurs without an
   explicit command or interactive selection. The one exception is the
   automatic update check: at most once a day it reads the latest release from
   the GitHub API and records the result in `update.json`, creating its
   directory if it is missing (see
   [`docs/security.md`](docs/security.md#updates)).
2. **Truthful success.** Exit status zero means the documented postcondition was
   satisfied. Portal rejection, malformed or oversized responses, transport
   failure, and unverified mutation are failures.
3. **One credential source.** A login selects at most one explicit source or
   falls back to the one saved local credential. Ambiguous combinations are
   rejected before network or SSH activity.
4. **Explicit local secret persistence.** `setup` stores one username/password
   pair in a local plaintext JSON file protected by directory mode `0700` and
   file mode `0600` on supported Unix-like systems. The product does not claim
   encryption, does not use an OS keyring, and does not migrate keyring data.
5. **Secret containment.** Outside the explicit local credential file, real,
   captured, or unsanitized passwords, cookies, QR URLs, tokens, and complete
   portal responses are never written to logs, tests, Git history, release
   artifacts, process arguments, or evidence. Tests MAY use minimal clearly
   artificial values under reserved domains.
6. **Redacted observation.** Normal status output masks account, name, and MAC
   fields, omits IP addresses, and normalizes control characters. Destructive
   commands prefer transient displayed indexes over stable backend identifiers.
7. **Local protocol ownership.** The Go process owns HTTP, TLS, cookies, portal
   parsing, CAS QR state, QR rendering, redaction, and postcondition checks for
   both local and SSH-routed operations.
8. **Host-agnostic SSH transport.** The product uses the operator's normal SSH
   client and configuration only as a byte tunnel. It does not own host
   inventory, aliases, routes, VPNs, jump hosts, keys, server desired state, or
   remote runtime installation.
9. **Bounded execution.** Network requests, SSH processes, password commands,
   QR polling, and postcondition verification have explicit bounds. No command
   retries forever or becomes a background service.
10. **One canonical interface.** The Go binary owns public behavior. Shell tools
    are source/developer aids only; no Python runtime is part of the installed
    or remote product path.
11. **Service-isolated automation.** Automated tests and CI never contact the
    real NJU portal, CAS service, an operator credential file, or a real SSH
    host.
12. **Immutable release artifacts.** A release is an immutable tag and an
    immutable GitHub Release whose archives are built by the release workflow
    from that tag. macOS archives are not notarized; the Cask and the
    self-update install them without the quarantine attribute.
13. **Separated iteration and acceptance.** Source changes reach `main` only
    through pull requests. Release acceptance validates a Homebrew-installed
    candidate, never a binary built from a source checkout.

## Public command set

The first public command surface is limited to:

```text
njulogin
njulogin setup
njulogin status
njulogin info
njulogin login
njulogin qr
njulogin logout
njulogin kick
njulogin quick list
njulogin quick add
njulogin quick delete
njulogin remote <ssh-host> status
njulogin remote <ssh-host> info
njulogin remote <ssh-host> login
njulogin remote <ssh-host> qr
njulogin remote <ssh-host> logout
njulogin remote <ssh-host> kick
njulogin remote <ssh-host> quick list
njulogin remote <ssh-host> quick add
njulogin remote <ssh-host> quick delete
njulogin update
njulogin version
njulogin help
```

Undocumented aliases, probes, OS-keyring variants, proxy controls, and
compatibility entrypoints are not part of the public promise. The exact flags,
positional arguments, environment inputs, and platform restrictions are closed
by [`docs/cli-contract.md`](docs/cli-contract.md).

## Out of scope

The repository MUST NOT expand into:

- SoundVPN, NJUConnect, EasyConnect, proxy selection, route, DNS, or firewall
  management;
- SSH inventory, key provisioning, jump-host policy, remote server setup, or
  remote helper deployment;
- background login, monitoring, launchd, cron, agents, or daemons;
- credential synchronization, account recovery, password rotation, OS keyring,
  Secret Service, KWallet, or other OS-keyring integration;
- encryption-key management for the local credential file;
- analytics, telemetry, portal-response persistence, or cloud services other
  than reading GitHub Releases for updates;
- a general-purpose NJU account or self-service client.

Requests in these areas require a separate product or explicit contract change,
not an incremental extension hidden inside another task.

## Compatibility policy

Before the first public tag, contracts may change deliberately while the change
is documented and reviewed. After the first public tag:

- command names, documented flags, exit-status meaning, credential-selection
  and storage rules, redaction guarantees, SSH transport semantics, and stable
  output keys are compatibility surfaces;
- adding an optional output key is backward-compatible;
- removing or renaming a command, flag, or stable key is breaking;
- weakening a security invariant or changing success semantics is breaking;
- portal-specific implementation changes are not breaking when observable
  behavior remains within contract.

## Contract change protocol

An intentional product change MUST update the relevant contract before or in
the same change as implementation. The change description must state:

- the contract clause being changed;
- the user-visible reason;
- what remains explicitly out of scope;
- compatibility and security impact;
- executable evidence for the new behavior.

Implementation work MUST NOT silently broaden scope. When a requested change
does not fit this contract, stop at a design proposal rather than folding it
into an unrelated task.
