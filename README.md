# njulogin

`njulogin` is a standalone command-line client for Nanjing University's
`p.nju.edu.cn` authentication portal. It can inspect redacted state, log in or
out, manage quick-login devices, render a CAS QR login, and run the same portal
protocol over a network path reached through SSH.

This repository was extracted from the former `adam-network-tools` workspace.
It is independent of SoundVPN, NJUConnect, EasyConnect, proxy selection, and
server desired state.

## Status

The canonical writable checkout is
`/home/adam/projects/homebrew-upload/products/njulogin` on the Elon Linux host.
The local macOS machine is reserved for Homebrew installation and installed
candidate acceptance.

The product repository and release assets are private. The public Homebrew tap
contains only Formula metadata; installing `njulogin` requires a GitHub token
that has read access to the private `soundadam/njulogin` repository.

## Build

Requirements:

- Go 1.25.12 or newer within the Go 1.25 line, as selected by `go.mod`;
- an OpenSSH-compatible client for `remote` operations;
- an SSH server that permits direct-tcpip forwarding to the NJU portal and CAS
  destinations.

```sh
make check
make build
./bin/njulogin version
```

## First use

Save the default username and password locally:

```sh
njulogin setup --username <username>
```

The password is prompted without echo. `setup` writes the username and password
as plaintext JSON in the local configuration directory. On macOS and Linux the
directory is restricted to mode `0700` and the file to mode `0600`.

This is not encrypted storage and does not use an OS keyring.
Protect the operating-system account, filesystem, backups, and snapshots that
contain the credential file.

## Commands

```text
njulogin
njulogin setup [--username USER]
njulogin status
njulogin info
njulogin login [credential options] [--add-quick NAME]
njulogin qr [--timeout DURATION]
njulogin logout
njulogin kick --id ID|--index N
njulogin quick list
njulogin quick add --name NAME
njulogin quick delete --id ID
njulogin remote [flags] <ssh-host> <operation>
njulogin version
```

Running `njulogin` in a terminal opens an interactive menu. Noninteractive use
requires an explicit subcommand.

Examples:

```sh
njulogin status
njulogin login
njulogin qr

njulogin remote my-host status
njulogin remote my-host login
njulogin remote my-host qr
njulogin remote my-host info
njulogin remote my-host kick --index 1
```

## Credential sources

Login uses one source:

- the saved local credential when no explicit source is selected;
- `--ask` for a no-echo terminal prompt;
- `--password-stdin`;
- `--password-command CMD`, intended only for a trusted local secret helper.

An explicit username cannot silently reuse a saved password belonging to a
different username. Do not put a password directly in a shell argument.

## Remote behavior

Remote operations do not execute portal tooling on the target host. The local
binary performs HTTP, TLS validation, cookies, JSON parsing, CAS QR polling, QR
rendering, redaction, and success verification. It uses the normal SSH client to
open direct-tcpip stdio tunnels equivalent to:

```text
ssh -T -W <portal-host>:<port> <ssh-host>
```

The remote host therefore does not need curl, Python, a helper script, or an
installed `njulogin`. It only needs an SSH service whose forwarding policy
allows access to the required NJU destinations. SSH aliases, authentication,
ProxyJump, and other connection policy remain in the operator's SSH
configuration.

Supported remote flags before `<ssh-host>` are:

```text
--base-url URL
--insecure[=BOOL]
--connect-timeout SECONDS
--max-time SECONDS
```

`--insecure` disables TLS verification in the local process for that invocation
and emits a warning.

## Homebrew

Private releases are installed from a checksum-pinned GitHub Release asset. Set
a GitHub token with read access to the private product repository, then install
from the public tap:

```sh
export HOMEBREW_GITHUB_API_TOKEN="$(gh auth token)"
brew tap soundadam/tap
brew install soundadam/tap/njulogin
```

The token is used only by Homebrew to request the private release asset through
the GitHub API. It is not embedded in the Formula or installed binary.

The repository keeps separate Formula templates for private authenticated
release assets and for a future public source archive:

```sh
scripts/render-private-formula.sh VERSION ASSET_ID SHA256 OUTPUT
scripts/render-formula.sh VERSION URL SHA256 OUTPUT
```

## Documentation

- [Normative product contract](SPEC.md)
- [CLI behavior and exit-status contract](docs/cli-contract.md)
- [Development and scope-control contract](docs/development-contract.md)
- [Development and macOS acceptance topology](docs/development-topology.md)
- [Independent review handoff contract](docs/review-handoff.md)
- [Architecture and product boundary](docs/architecture.md)
- [Service-isolated portal, CAS, and SSH testing](docs/testing-fixtures.md)
- [Security model](docs/security.md)
- [Source provenance](docs/provenance.md)
- [Release readiness](docs/release-readiness.md)
- [Compatibility tools](tools/NJUlogin/README.md)

## License

The standalone implementation is licensed under the MIT License. It was
informed by the MIT-licensed
[Do1e/NJUlogin](https://github.com/Do1e/NJUlogin) project; see
[`NOTICE`](NOTICE) and [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
