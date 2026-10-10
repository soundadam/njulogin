# Development and acceptance topology

Status: normative

This document assigns one purpose to each maintainer environment. It keeps a
single source authority, keeps release evidence tied to reviewed commits, and
keeps release acceptance on the installed product rather than a source build.

## Source authority

The source authority is the `main` branch of the private GitHub repository
`soundadam/njulogin`. The Homebrew tap's authority is the `main` branch of
`soundadam/homebrew-tap`.

Any maintainer clone, on the Elon Linux host or on the maintainer's macOS
machine, MAY be used for source work. A clone is a working copy, not an
authority:

- work happens on a branch and reaches `main` only through a pull request;
- source moves between machines only through Git push and fetch, never by
  copying files between checkouts;
- `make check` passes in the clone that makes each commit;
- every gate run, local or CI, uses the patched toolchain selected by the
  `toolchain` directive in `go.mod`, not merely the language version in the
  `go` directive.

The Homebrew-managed tap under `/opt/homebrew/Library/Taps/` is package-manager
state used for installation and testing. It is not a writable tap source.

## Hosted CI roles

Product source CI runs `make release-check` on Linux and reproduces the
service-isolated gate. It does not claim that Linux is a first-class installed
product platform. The workflow asserts that `go env GOVERSION` equals the
`go.mod` toolchain directive.

Homebrew Formula CI runs on hosted macOS runners. It validates the immutable
source archive, Formula build, installed command, and read-only Formula tests.

## macOS acceptance

Release acceptance runs on the maintainer's macOS machine against a
Homebrew-installed candidate built from a reviewed commit. It MUST NOT use
`go run`, `make build`, or `./bin/njulogin` from a source checkout as release
acceptance evidence.

When acceptance finds a defect, the fix, tests, and documentation go through a
branch and pull request like any other change, and a new Homebrew candidate is
installed for acceptance.

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
