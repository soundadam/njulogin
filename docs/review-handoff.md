# Review handoff contract

Status: normative for repository review workflows

This document defines how one agent hands repository material to another agent
for independent review. Its purpose is to preserve scope, make findings
reproducible, and prevent a reviewer from silently becoming an implementer or
product designer.

## Roles

Every handoff has three logical roles, even when one person performs more than
one role:

- **Producer:** prepares the review packet and states the governing contract.
- **Reviewer:** inspects the packet and reports findings without editing files.
- **Adjudicator:** accepts, rejects, or defers each finding and authorizes any
  follow-up change.

The reviewer MUST NOT act as the adjudicator. A review result is evidence, not
authorization to change product scope or publish artifacts.

## Review packet

The producer gives the reviewer one immutable packet containing:

```text
Review objective:
Repository and snapshot:
Governing documents:
Files in scope:
Files out of scope:
Known implementation gaps:
Questions to answer:
Forbidden actions:
Required output format:
```

The packet SHOULD identify the Git commit and whether uncommitted changes are
included. When the review includes uncommitted work, the producer SHOULD record
the output of `git status --short` and a SHA-256 digest of the supplied bundle.

The reviewer evaluates only the supplied snapshot. It MUST NOT assume that a
newer branch, remote repository, issue, or undocumented implementation is
authoritative.

## Authority and scope

For NJUlogin documentation review, the governing authority is:

1. `SPEC.md` and its linked normative documents;
2. the explicit review objective and in-scope file list;
3. descriptive documents such as `README.md` and architecture notes;
4. implementation behavior only when the packet explicitly includes code as
   evidence.

The reviewer MUST NOT:

- propose unrelated features, commands, services, platforms, or dependencies;
- treat current implementation bugs as intended behavior;
- weaken a security invariant to make documents easier to reconcile;
- edit files, execute publication actions, or make network requests unless the
  packet explicitly authorizes them;
- report stylistic preferences as defects unless they create ambiguity,
  contradiction, or unusable instructions.

When the reviewer notices a valuable idea outside scope, it may be reported
only as `deferred`, with a direct explanation of why it is outside the current
contract.

## Review questions

A documentation review answers these questions in order:

1. Do normative documents contradict one another?
2. Do descriptive documents contradict or overpromise beyond the normative
   contract?
3. Are requirements testable, bounded, and stated at the correct authority
   level?
4. Are exit status, postconditions, security boundaries, platform support, and
   release authority internally consistent?
5. Does any wording permit task drift or an unauthorized publication action?
6. Are terms used consistently, especially **service-isolated** versus
   **network-offline**?
7. Are release gates measurable, and does each gate identify the evidence that
   satisfies it?

The reviewer SHOULD prefer a small number of high-confidence findings over a
large list of speculative improvements.

## Severity

Findings use these levels:

| Level | Meaning |
| --- | --- |
| `blocker` | The documents authorize unsafe behavior, contradict a core invariant, or cannot govern implementation reliably. |
| `major` | A material contradiction, untestable requirement, incorrect compatibility promise, or missing release authority boundary. |
| `minor` | A localized ambiguity or omission that could mislead an implementer but does not invalidate the contract. |
| `note` | Non-blocking clarification or explicitly deferred idea. |

Severity is based on contract impact, not editing effort.

## Required finding format

Each finding contains:

```text
id: stable short identifier
severity: blocker | major | minor | note
status: open | deferred
files: exact paths
clauses: headings or line references
summary: one-sentence defect statement
evidence: the conflicting or insufficient requirements
impact: what an implementer, tester, or release operator could do incorrectly
recommended_resolution: the smallest contract-level correction
```

The reviewer MUST distinguish a documented contradiction from an
implementation gap. A documentation-only review reports implementation gaps
only when they are already declared in the packet or when code was explicitly
included as review evidence.

The review response also contains:

- `verdict`: `pass`, `pass_with_minor_findings`, or `changes_required`;
- `scope_confirmation`: confirmation that only supplied files were reviewed;
- `finding_count` by severity;
- `unresolved_questions` that cannot be answered from the packet;
- `no_findings_statement` when no defect is found.

## Reviewer prompt

The following prompt is the default handoff instruction:

```text
You are the independent contract reviewer for the supplied NJUlogin
documentation snapshot.

Review only the supplied packet. Do not edit files, invoke tools, browse the
web, infer authority from current implementation behavior, or propose product
features outside the stated scope.

Use this authority order:
1. SPEC.md and linked normative documents;
2. the review objective and explicit scope;
3. descriptive documents;
4. implementation evidence only when explicitly supplied.

Check for contradictions, overpromises, untestable requirements, ambiguous
success semantics, security-boundary gaps, platform-support mismatches,
release-authority gaps, and wording that permits task drift.

Return only the required structured review result. Every finding must cite
exact files and clauses, explain concrete impact, and recommend the smallest
contract-level correction. Mark unrelated ideas as deferred rather than
expanding scope. If there are no findings, state that explicitly.
```

## Handoff lifecycle

1. The producer validates that the packet contains no secrets.
2. The producer freezes and hashes the packet.
3. The reviewer returns findings without modifying the repository.
4. The adjudicator records a decision for every finding.
5. Approved corrections are implemented as a separate bounded change.
6. The repository gate runs.
7. A second review uses a new snapshot rather than continuing against stale
   context.

Reviewer conversation history MUST NOT replace a new packet after files have
changed. Each review is tied to one explicit snapshot.
