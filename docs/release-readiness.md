# Release readiness contract

Status: normative

This document defines the gates for each `njulogin` release.

## Release shape

A release is:

- one annotated semantic-version tag in the public repository
  `soundadam/njulogin`;
- one GitHub source release for that tag;
- one Homebrew Formula in `soundadam/homebrew-tap` that builds from the
  immutable tag archive.

A release does not include a prebuilt macOS binary, installer, daemon,
background service, telemetry, or automatic credential migration.

## Release closure requirements

The following remain mandatory release gates even after local implementation
evidence exists. Hosted CI, independent review, tap integration, and macOS
acceptance must still bind the exact release candidate:

1. Portal rejection, malformed operation responses, and failed postcondition
   verification must return nonzero exit status.
2. Explicit credential sources must be mutually exclusive.
3. Stable row output must use `session_N`, `online_N`, and `quick_N` keys.
4. The command evidence and 60% coverage floor in
   `docs/development-contract.md` must be met.
5. Staticcheck, ShellCheck, Actionlint, race testing, secret scanning, and
   pinned `govulncheck` must pass without unexplained findings.
6. OS-keyring integration and all remote curl/Python/helper execution paths must be absent.
7. Remote command and QR tests must prove that only local `ssh -W` byte tunnels are used.
8. Documentation must say **service-isolated**, not **network-offline**, unless a clean empty-module-cache build is proven.
9. The Formula must omit a redundant explicit `version` when the tag URL already provides it.
10. `soundadam/homebrew-tap` CI must watch `Formula/njulogin.rb` and run a dedicated audit, source-install, and Formula-test job.
11. Product CI runs on Linux, and macOS acceptance uses a Homebrew-installed candidate only, as defined in `docs/development-topology.md`.

No release task may bypass these items by weakening the contract.

## Gate A: release candidate and pre-tag acceptance

Before each tag:

- all release closure requirements are complete;
- `make release-check` passes locally on the exact commit proposed for the tag;
- product source CI passes on its Linux runner;
- dependency verification and vulnerability review pass;
- changelog, intended release notes, and intended tag version agree, and the
  release notes identify the applicable contract version;
- CI passes on the exact clean main-branch commit proposed for the tag;
- a maintainer clone creates a local-only, release-shaped source archive from that exact
  commit with the intended release version and normalized file modes;
- a candidate Formula using the local archive passes `brew style`, strict
  audit, source installation with Homebrew API installation disabled, and
  `brew test`;
- the Homebrew-installed candidate reports the intended version and completes
  the operator acceptance checklist below;
- candidate evidence records the source commit, source aggregate, archive
  SHA-256, Formula SHA-256, platform, package version, and sanitized outcomes;
- no code or documentation changes remain uncommitted after candidate creation;
- the owner deliberately approves creation and push of the annotated tag.

The local candidate is acceptance evidence only. It MUST use a temporary tap,
MUST be removed after testing, and MUST NOT be uploaded, published, or treated
as the final GitHub archive.

### Operator acceptance checklist

Acceptance uses the Homebrew-installed candidate on the maintainer's local
macOS machine. It does not run `go run`, `make build`, or `./bin/njulogin` from
a source checkout. It records no secret or raw response. The operator confirms:

- local unauthenticated and authenticated status;
- local credential-file setup, permissions, and default credential reuse;
- password login success and intentional failure;
- QR login success, expiry, and timeout behavior;
- logout and verified unauthenticated state;
- online-session listing and one explicit kick;
- quick-login list, add, and delete;
- remote status, password login, QR login, and logout through one owned SSH
  destination;
- redaction of account, name, MAC, and IP data;
- nonzero exits for rejected operations and unavailable remote dependencies.

Acceptance notes contain only pass/fail, source commit, package version,
platform version, and sanitized observations.

## Gate B: immutable source tag and GitHub release

After Gate A passes:

- create and push one annotated tag pointing to the accepted, CI-tested commit;
- verify the remote tag resolves to that exact commit;
- treat tag approval as authorization for the tag only;
- obtain a separate explicit owner instruction before creating or publishing a
  GitHub Release;
- publish the source release and release notes for that tag;
- download the published tag archive and calculate its SHA-256;
- verify the archive represents the accepted tag tree and contains no generated
  binaries, private fixtures, or unexpected paths;
- render the final Formula from the reviewed template using the immutable HTTPS
  tag URL and exact checksum;
- omit a redundant explicit Formula version;
- repeat `brew style`, strict audit, source installation, `brew test`, installed
  version, help, non-terminal exit, and unknown-command exit checks against the
  final tag archive;
- repeat operator acceptance or abort when the final tag build differs
  materially from the accepted pre-tag candidate.

A failed post-tag verification does not authorize moving or replacing the tag.
Stop the release and use a new version.

## Gate C: Homebrew tap publication

After Gate B passes:

- update `Formula/njulogin.rb` and the dedicated tap CI job in one tap branch;
- obtain explicit owner authorization to push the tap proposal branch and open
  its pull request; this does not authorize merge or publication;
- require `brew audit --strict --new --formula`, source installation,
  `brew test`, version/help/exit smoke, and tap pull-request CI to pass on both
  supported macOS runners;
- obtain a separate explicit owner authorization to merge the tap pull request
  and publish the Formula;
- merge the reviewed Formula;
- verify tap main-branch CI after merge; if it fails, stop distribution and
  revert or repair through a separately reviewed change.

Homebrew tests are read-only and service-isolated. They must not attempt real
portal login, real credential-file access, or SSH.

## Release evidence

Each release keeps sanitized evidence in the product release pull request,
release notes, or a private release record containing:

```text
Version:
Contract version:
Source commit:
Product pull-request CI:
Product main-branch CI:
Local release-check:
Pre-tag candidate source aggregate SHA-256:
Pre-tag candidate archive SHA-256:
Pre-tag candidate Formula SHA-256:
Pre-tag Homebrew style/audit/install/test:
Pre-tag operator acceptance:
Tag creation approval:
Annotated tag:
GitHub Release publication approval:
Published tag archive SHA-256:
Final Formula style/audit/install/test:
Final operator acceptance reconciliation:
Tap proposal approval:
Tap pull request and CI:
Tap publication approval:
Tap main-branch CI:
Known limitations:
```

Evidence identifies the exact commit, artifacts, commands, and outcomes without
embedding credentials, tokens, QR URLs, raw portal responses, device
identifiers, MAC addresses, IP addresses, or terminal transcripts containing
operator data. A local candidate checksum is not the final GitHub archive
checksum and the two fields MUST remain distinct.

## Release procedure

1. Complete one release-only branch containing version, changelog,
   intended release notes, and release documentation changes.
2. Run `make release-check` locally and require product pull-request CI to pass.
3. Merge the release-only branch, verify product main-branch CI, and record the
   exact clean main commit proposed for release.
4. From that exact commit, create a local-only release-shaped source archive and
   candidate Formula using the intended version and normalized file modes.
5. On macOS, verify the candidate digests, run Homebrew style, strict audit,
   source installation, `brew test`, version/help/exit smoke, and complete the
   full operator acceptance checklist using only the installed candidate.
6. Remove the candidate package, temporary tap, and transferred candidate files;
   record sanitized pre-tag acceptance evidence.
7. Obtain explicit owner authorization to create and push the annotated tag.
8. Create and push one annotated tag pointing to the accepted exact commit, then
   verify the remote tag target. Do not move or replace the tag.
9. Obtain a separate explicit owner instruction to create and publish the
   GitHub Release and its release notes.
10. Publish the GitHub source release, download the immutable tag archive,
    calculate its SHA-256, verify its tree and path boundary, render the final
    Formula, and repeat the packaging and installed-command checks against that
    final archive.
11. Reconcile the final build with the pre-tag accepted candidate. Repeat the
    affected operator acceptance checks when there is any material difference;
    abort and use a new version when the final candidate cannot be accepted.
12. Prepare the tap branch containing `Formula/njulogin.rb` and its dedicated CI
    wiring. Obtain explicit owner authorization to push that proposal branch and
    open a pull request; this approval does not authorize merge.
13. Push the tap proposal, open the pull request, and require audit,
    source-install, Formula test, version/help/exit smoke, and pull-request CI to
    pass on both supported macOS runners.
14. Obtain a separate explicit owner authorization to merge the tap pull request
    and publish the Formula.
15. Merge the tap pull request and verify tap main-branch CI. On failure, stop
    distribution and revert or repair through a separately reviewed and
    authorized change.

## Abort conditions

Stop the release rather than working around any of these conditions:

- contract, implementation, tests, or packaging evidence disagree;
- a mutation can fail while exiting 0;
- a test, fixture, log, review packet, or evidence note contains real operator
  data or sensitive raw output;
- `make release-check`, product CI, Formula checks, operator acceptance, or tap
  CI is skipped, unavailable, or fails on the relevant candidate;
- the pre-tag candidate is not created from the exact clean, CI-tested commit;
- pre-tag acceptance uses a source binary or a macOS source checkout instead of
  the Homebrew-installed candidate;
- the local candidate or final tag Formula fails style, strict audit, source
  installation, `brew test`, installed-version, help, or exit-status checks;
- the annotated tag target differs from the accepted commit, the remote tag is
  missing, or any operation would move or replace an existing tag;
- the published archive checksum is calculated from a different artifact or the
  archive tree/path boundary differs unexpectedly from the accepted source;
- the final tag build differs materially from the accepted pre-tag candidate and
  the affected acceptance checks are not repeated successfully;
- approval for tag creation, GitHub Release publication, tap proposal push, or
  tap merge/publication is absent or is incorrectly reused for another stage;
- the tap Formula is merged before pull-request CI passes on both supported
  macOS runners;
- tap main-branch CI fails after merge and distribution is allowed to continue
  without a separately reviewed revert or repair.
