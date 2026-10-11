# Release readiness contract

Status: normative

This document defines the gates for each `njulogin` release.

## Release shape

A release is:

- one annotated semantic-version tag `v<version>` in the public repository
  `soundadam/njulogin`;
- one immutable GitHub Release for that tag carrying the archives that
  `scripts/package.sh` builds, named as the
  [appkit contract](https://github.com/soundadam/appkit#发布产物) requires; a
  version containing `-` is a prerelease;
- for a version without `-`, `Casks/njulogin.rb` in `soundadam/homebrew-tap`,
  rendered from [`packaging/homebrew/njulogin.rb`](../packaging/homebrew/njulogin.rb)
  with the checksums of those archives.

Pushing the tag publishes all three: the
[release workflow](../.github/workflows/release.yml) checks the tag, toolchain,
and changelog section, builds and publishes the archives, and pushes the Cask to
the tap's `main`. Approval to push a tag is therefore approval to publish that
release.

A release does not include an installer, daemon, background service,
telemetry, notarized macOS build, Windows build, or automatic credential
migration.

## Release closure requirements

The following remain mandatory release gates even after local implementation
evidence exists. Hosted CI, tap integration, and macOS acceptance must still
bind the exact release candidate:

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
9. The Cask template must pass `brew style`, and every archive the release ships must have its checksum in the rendered Cask (`make cask-check`).
10. `soundadam/homebrew-tap` CI must test `Casks/njulogin.rb`.
11. Product CI runs on Linux, and macOS acceptance uses a Cask-installed candidate only, as defined in `docs/development-topology.md`.
12. The repository has release immutability enabled and the
    `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret, a fine-grained token with
    Contents read and write on `soundadam/homebrew-tap` only.

No release task may bypass these items by weakening the contract.

## Gate A: release candidate and pre-tag acceptance

Before each tag:

- all release closure requirements are complete;
- `make release-check` passes locally on the exact commit proposed for the tag;
- CI passes on the exact clean main-branch commit proposed for the tag;
- dependency verification and vulnerability review pass;
- `CHANGELOG.md` has a `## <version> - <date>` section; it becomes the release
  notes and identifies the applicable contract version;
- a maintainer clone runs `scripts/package.sh <version>` on that exact commit
  and renders a candidate Cask with `scripts/render-cask.sh`, its `url`
  pointing at the local archive;
- in a temporary tap, the candidate Cask passes `brew style`, installs with
  `brew install --cask`, and the installed command passes version, help,
  non-terminal exit, and unknown-command exit checks;
- the Cask-installed candidate completes the operator acceptance checklist
  below;
- candidate evidence records the source commit, the archive and executable
  SHA-256 for the acceptance platform, platform, package version, and sanitized
  outcomes;
- no code or documentation changes remain uncommitted after candidate creation;
- the owner deliberately approves creation and push of the annotated tag.

The local candidate is acceptance evidence only. Its temporary tap and archives
MUST be removed after testing and MUST NOT be uploaded or published.

### Operator acceptance checklist

Acceptance uses the Cask-installed candidate on the maintainer's local macOS
machine. It does not run `go run`, `make build`, or `./bin/njulogin` from a
source checkout. It records no secret or raw response. The operator confirms:

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

## Gate B: tag and publication

After Gate A passes:

- create and push one annotated tag pointing to the accepted, CI-tested commit;
- verify the remote tag resolves to that exact commit;
- require the release workflow to succeed; when only its `homebrew` job fails,
  fix the cause and re-run that job, which renders the Cask from the published
  archives;
- verify the Release lists one archive per target that
  `scripts/package.sh --targets` prints, each with a SHA-256 digest;
- verify the executable in the published archive for the acceptance platform
  has the SHA-256 recorded for the candidate; when it differs, repeat the
  affected operator acceptance checks;
- for a version without `-`, verify the tap commit `njulogin <version>`, its
  CI, and `brew install --cask soundadam/tap/njulogin` reporting the version.

A failed post-tag verification does not authorize moving or replacing the tag
or the Release; both are immutable. Stop and publish a new version.

Homebrew tests are read-only and service-isolated. They must not attempt real
portal login, real credential-file access, or SSH.

## Release evidence

Each release keeps sanitized evidence in the release pull request, release
notes, or a private release record containing:

```text
Version:
Contract version:
Source commit:
Product main-branch CI:
Local release-check:
Candidate archive SHA-256 (acceptance platform):
Candidate executable SHA-256 (acceptance platform):
Candidate Cask style/install/smoke:
Pre-tag operator acceptance:
Tag creation approval:
Annotated tag:
Release workflow run:
Published executable SHA-256 (acceptance platform):
Final operator acceptance reconciliation:
Tap commit and CI:
Known limitations:
```

Evidence identifies the exact commit, artifacts, commands, and outcomes without
embedding credentials, tokens, QR URLs, raw portal responses, device
identifiers, MAC addresses, IP addresses, or terminal transcripts containing
operator data.

## Release procedure

1. Complete one release-only branch containing the version's changelog section
   and release documentation changes.
2. Run `make release-check` locally and require product pull-request CI to pass.
3. Merge the release-only branch, verify product main-branch CI, and record the
   exact clean main commit proposed for release.
4. From that exact commit, build the candidate archives and Cask, and on macOS
   run the Cask checks and the full operator acceptance checklist using only
   the installed candidate.
5. Remove the candidate package, temporary tap, and transferred candidate files;
   record sanitized pre-tag acceptance evidence.
6. Obtain explicit owner authorization to create and push the annotated tag.
7. Create and push the annotated tag; verify the remote tag target.
8. Follow the release workflow to completion and perform the Gate B checks.

## Abort conditions

Stop the release rather than working around any of these conditions:

- contract, implementation, tests, or packaging evidence disagree;
- a mutation can fail while exiting 0;
- a test, fixture, log, review packet, or evidence note contains real operator
  data or sensitive raw output;
- `make release-check`, product CI, Cask checks, or operator acceptance is
  skipped, unavailable, or fails on the relevant candidate;
- the candidate is not built from the exact clean, CI-tested commit;
- pre-tag acceptance uses a source binary or a macOS source checkout instead of
  the Cask-installed candidate;
- the annotated tag target differs from the accepted commit, the remote tag is
  missing, or any operation would move or replace an existing tag or Release;
- the published executable differs from the accepted candidate and the
  affected acceptance checks are not repeated successfully;
- approval for tag creation is absent or is reused for another version;
- tap CI fails on `Casks/njulogin.rb` and distribution is allowed to continue
  without a separately reviewed repair.
