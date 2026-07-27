# Development contract

Status: normative

This document controls how changes are planned, implemented, tested, and
reviewed. Its purpose is to prevent a narrow task from becoming an implicit
product redesign.

## Environment authority

The maintainer environment roles are normative in
[`docs/development-topology.md`](development-topology.md).

- All source and documentation changes occur in the canonical Elon working
  tree.
- `make check` and local pre-review evidence are produced on Elon.
- The local macOS machine installs a Homebrew candidate for acceptance and
  does not edit or build directly from a product source checkout.
- A macOS acceptance failure becomes a scoped defect report; the correction
  returns to the Elon work sequence below.

This separation does not prevent external contributors from building source.
It controls the maintainer's authoritative iteration and release evidence.

## Change brief

Every non-trivial change starts with a brief containing:

```text
Contract clauses:
Observable problem:
In scope:
Out of scope:
Required postcondition:
Security impact:
Release impact:
Evidence to produce:
```

The brief may be in an issue, pull request, commit plan, or review note. Work
must stop and the brief must be amended when implementation discovers a scope
change.

## Change classes

### Contract-preserving change

Examples include bug fixes, refactors, test additions, and portal compatibility
updates that keep documented behavior unchanged.

Requirements:

- cite the contract clause being preserved;
- add regression evidence;
- do not add flags, commands, persistence, dependencies, or side effects as a
  convenience.

### Contract-changing change

Examples include a new command, new credential source, changed output key,
changed exit semantics, broader platform support, or a weaker security rule.

Requirements:

- update `SPEC.md` or the relevant normative document first or in the same
  patch;
- explain compatibility and security consequences;
- state explicit non-goals;
- add complete executable evidence;
- receive deliberate owner review.

### Release-only change

Examples include version, changelog, tag, archive checksum, Formula, and release
notes. Release work MUST NOT include opportunistic product behavior changes.

## Required work sequence

For implementation tasks:

1. identify the governing contract clause;
2. write or update a failing synthetic test when practical;
3. make the smallest implementation change that satisfies the clause;
4. run the repository gate;
5. inspect the diff for unrelated changes and secret exposure;
6. update descriptive documentation only when observable behavior changed;
7. record release impact.

A refactor and a behavior change SHOULD be separate changes. A release change
and a feature change MUST be separate changes.

## Independent review handoff

Independent agent or API review follows
[`docs/review-handoff.md`](review-handoff.md). The reviewer receives an
immutable, secret-scanned snapshot and reports structured findings only. It
does not edit the worktree, broaden product scope, or authorize publication.

When files change after a review, a follow-up review MUST use a new packet and
snapshot digest rather than relying on the previous conversation context.

## Automated-test boundary

Automated tests and CI MUST NOT:

- contact `p.nju.edu.cn`, the NJU CAS service, or any real portal endpoint;
- read or write the operator's real credential file;
- connect to a real SSH host;
- use a real username, password, cookie, QR URL, device ID, MAC address, or IP
  address;
- persist portal responses or sensitive environment values.

Tests use only synthetic fixtures and local test servers. External commands
such as `ssh`, curl used by compatibility-only tools, and Python test relays
SHOULD be replaced through a temporary `PATH`, injected runner, or local
process harness.

The term **service-isolated** means the test does not contact real services or
operator infrastructure. It does not mean a clean machine can build without
downloading Go modules. Documentation MUST use **network-offline** only after a
clean empty-module-cache build has been demonstrated, or dependencies have
been vendored.

## Repository gate

`make check` is the single required local correctness gate and runs in the
canonical Elon working tree. Product source CI runs the corresponding gate on
Linux. Before the first public tag it must cover, directly or through scripts:

- formatting and whitespace checks;
- `go vet`;
- unit and synthetic integration tests;
- race detection;
- Staticcheck with no unsuppressed findings;
- Python syntax for repository test/check scripts;
- ShellCheck for tracked shell scripts;
- Actionlint for GitHub workflows;
- repository secret/invariant scanning;
- release build, version, help, and non-terminal smoke tests.

Security and dependency review must additionally run a pinned `govulncheck`
version before release and after dependency changes.

Tools used by CI MUST be pinned to a reviewed version or immutable revision.
Using `@latest` is acceptable for local investigation but not as a release
gate.

## Coverage contract

Coverage is evidence, not the product goal. Before the first public tag:

- overall Go statement coverage MUST be at least 60%;
- every public command has success, rejection, and malformed-response coverage
  where applicable;
- credential selection, local credential-file permissions and symlink
  rejection, exit status, redaction, SSH `-W` transport, and local QR state
  transitions have direct tests;
- no release-critical command function remains entirely uncovered.

Raising aggregate coverage with low-value helper tests does not substitute for
the command evidence required by `docs/cli-contract.md`.

## Dependency contract

A dependency change requires:

- a specific product need that cannot be met reasonably with the standard
  library or current dependencies;
- `go mod tidy -diff` and `go mod verify` passing;
- vulnerability review;
- license identification and notice updates;
- confirmation that release artifacts preserve required license texts.

Dependencies MUST NOT be added solely to avoid a small, well-contained
implementation task.

## Security review triggers

The following changes require explicit security review:

- credential input or persistence, subprocess, SSH tunneling, TLS, QR,
  logging, redaction, or destructive selection;
- any new environment variable or configuration field;
- any remote runtime, remote command, or remote file;
- any relaxation of timeouts or input validation;
- any artifact containing compiled code.

## Definition of done

A change is complete only when:

- its change brief remains accurate;
- normative docs and tests agree;
- `make check` passes from the canonical Elon checkout;
- no generated or sensitive files are tracked;
- the diff contains no unrelated feature work;
- user-visible failure paths return correct nonzero status;
- release impact is recorded as none, changelog-only, or release-blocking.

macOS installation acceptance is a separate release gate. It does not replace
the Elon source gate and must use a Homebrew-installed candidate.

Passing tests alone does not make a contract-changing task complete.

## Publication authority

Creating a remote repository, pushing branches or tags, publishing a GitHub
Release, changing repository visibility, or modifying the public Homebrew tap
requires a separate explicit owner instruction. Preparation and local
validation do not authorize publication.
