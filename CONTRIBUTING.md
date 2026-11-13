# Contributing

Read [`SPEC.md`](SPEC.md), the
[`CLI contract`](docs/cli-contract.md), and the
[`development contract`](docs/development-contract.md) before changing
behavior.

Work on a branch and propose changes through a pull request; environment roles
are defined in [`docs/development-topology.md`](docs/development-topology.md).

Start non-trivial work with the change brief defined in the development
contract. State the governing contract clause, in-scope behavior, explicit
non-goals, required postcondition, and evidence. Contract-changing work must
update the normative document and tests in the same pull request.

Run the complete local gate before proposing a change:

```sh
make check
```

Tests must be service-isolated and use synthetic fixtures. Do not record or
commit a real NJU account, password, Cookie, QR URL, portal response, session
ID, device ID, MAC address, or IP address.

Changes to portal endpoints or authentication behavior require a separate
operator acceptance run with the Homebrew-installed candidate after automated
tests. Never make a real login, logout, device bind, or remote SSH call from CI.

Use the pull-request template. Passing tests does not authorize repository,
tag, release, or Homebrew publication.
