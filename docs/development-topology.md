# Development and acceptance topology

Status: normative

Effective date: 2026-07-27

This document assigns one purpose to each maintainer environment. The split is
intended to prevent two writable source trees, machine-specific fixes, and
release evidence produced from an unreviewed checkout.

## Canonical source host

The canonical writable working tree is:

```text
host: Elon
path: /home/adam/projects/homebrew-upload/products/njulogin
platform: Linux
```

The canonical writable Homebrew tap checkout is:

```text
host: Elon
path: /home/adam/projects/homebrew-upload/homebrew-tap
```

All maintainer source iteration occurs in this working tree, including:

- product and contract edits;
- Git branches, commits, reviews, and release preparation;
- service-isolated unit and integration tests;
- formatting, linting, race, vulnerability, and secret checks;
- source builds and candidate Formula rendering;
- Formula and tap CI changes in the canonical tap checkout;
- independent-review packet generation and adjudication.

The Elon toolchain and product source CI MUST use the patched toolchain selected
by the `toolchain` directive in `go.mod`, not merely the language version in the
`go` directive. A change is not ready for review when it passes only on another
machine.

## macOS acceptance host

The maintainer's local macOS machine is an installation and acceptance node,
not a source-development node.

It MAY:

- update Homebrew and tap metadata;
- install or upgrade `njulogin` from a reviewed temporary or public tap;
- run `brew style`, `brew audit`, source installation, and `brew test` for the
  Formula under review;
- execute the installed `njulogin` binary for macOS, local credential-file, portal, QR, and
  SSH operator acceptance;
- return sanitized pass/fail evidence to the Elon change.

It MUST NOT:

- edit the product source or normative documents;
- edit the maintainer tap checkout or author Formula changes;
- create product commits or release branches;
- use a macOS source checkout as the basis for a fix;
- run an uninstalled source binary as release acceptance evidence;
- copy a local fix back into Elon outside Git review;
- publish a repository, tag, GitHub Release, or Formula without the separate
  owner authorization required by the release contract.

When acceptance finds a defect, the defect report returns to Elon. The fix,
tests, and documentation are made in the canonical source tree, and a new
Homebrew candidate is installed on macOS.

## Hosted CI roles

Product source CI runs on Linux and reproduces the service-isolated gate used
on Elon. It does not claim that Linux is a first-class installed product
platform.

Homebrew Formula CI runs on hosted macOS runners. It validates the immutable
source archive, Formula build, installed command, and read-only Formula tests.
Hosted macOS CI does not make the maintainer's local Mac a development host.

## Artifact flow

The normal flow is one-directional:

```text
Elon source change
  -> independent review and Linux source CI
  -> reviewed immutable source revision
  -> candidate Homebrew Formula
  -> Homebrew installation on macOS
  -> sanitized acceptance result returned to Elon
```

Source files do not flow from macOS back to Elon. Only review findings,
sanitized command outcomes, platform versions, package versions, and explicit
pass/fail observations return.

The Homebrew-managed tap under `/opt/homebrew/Library/Taps/` is package-manager
state used for installation and testing. It is not the maintainer's writable
tap source authority.

## Acceptance handoff

A macOS acceptance record identifies:

```text
Source commit:
Formula revision or temporary tap:
Installed version:
macOS version and architecture:
Homebrew install/test result:
Operator acceptance result:
Sanitized failures:
```

It MUST NOT contain credentials, cookies, QR URLs, raw portal responses,
session or device identifiers, MAC addresses, or IP addresses.

## Source-tree authority

After migration, any former macOS product checkout is an archive only. It is
not a mirror, fallback working tree, or synchronization peer. If source state
differs between Elon and a macOS archive, Elon is authoritative until a public
remote repository and reviewed branch establish a newer source revision.

## Migration completion

The environment migration is complete when:

- the Git history and current uncommitted documentation snapshot exist on
  Elon with matching content digests;
- the clean Homebrew tap Git checkout exists on Elon at the canonical tap
  path with its original remote intact;
- Apple-specific archive metadata and generated build artifacts are absent;
- source CI uses a Linux runner;
- the Elon toolchain can run the repository gate;
- the former macOS checkout is removed from the active product path or marked
  as a read-only migration archive;
- macOS acceptance uses only a Homebrew-installed candidate.
