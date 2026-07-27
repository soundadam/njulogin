# njulogin

`njulogin` is a standalone command-line client for Nanjing University's
`p.nju.edu.cn` network authentication portal. It can inspect redacted online
state, log in or out, manage quick-login devices, render a CAS QR login, and
perform the same operations on a remote network path through SSH.

This repository was extracted from the former `adam-network-tools` workspace.
It is independent of SoundVPN, NJUConnect, EasyConnect, proxy selection, and
server desired state.

## Status

The source is now isolated as its own product under
`homebrew-upload/products/njulogin`. It has not been published, tagged, added
to the public tap, or connected to a GitHub remote.

## Build

Requirements:

- Go 1.25 or newer;
- macOS for Keychain-backed `setup` and default credential reuse;
- an SSH client for `remote` operations.

```sh
make check
make build
./bin/njulogin version
```

## First use

On macOS, configure the default account and store its password in Keychain:

```sh
njulogin setup --username <username>
```

The password is prompted without echo. It is passed to the `security` command
through standard input and is never placed in process arguments. The local
configuration file stores only the username and Keychain service.

## Commands

```text
njulogin
njulogin setup [--username USER] [--keychain-service SERVICE]
njulogin status
njulogin info
njulogin login [credential options] [--add-quick NAME]
njulogin qr
njulogin logout
njulogin kick --index N
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
njulogin info
njulogin login
njulogin qr

njulogin remote my-host status
njulogin remote my-host login
njulogin remote my-host qr
njulogin remote my-host info
njulogin remote my-host kick --index 1
```

Remote requests execute on the target's network path. `njulogin` uses the
normal SSH client and honors the operator's SSH configuration; managed hosts
must still be accessed through their owning infrastructure contract.

## Credential sources

Login accepts one credential source:

- the configured macOS Keychain item;
- `--ask` for a no-echo terminal prompt;
- `--password-stdin`;
- `--password-command CMD`, intended only for an operator-controlled secret
  helper.

Do not put a password directly in a shell command. See
[`docs/security.md`](docs/security.md) for the full boundary.

## Homebrew

The repository contains a source-build Formula template at
[`packaging/Formula/njulogin.rb.in`](packaging/Formula/njulogin.rb.in).
Publishing it requires an immutable source tag, archive checksum, clean CI,
offline Formula test, and a separate tap update. No public Formula is created
by this extraction.

Render a reviewed source archive into a candidate Formula with:

```sh
scripts/render-formula.sh VERSION URL SHA256 OUTPUT
```

## Documentation

- [Architecture and product boundary](docs/architecture.md)
- [Security model](docs/security.md)
- [Source provenance](docs/provenance.md)
- [Release readiness](docs/release-readiness.md)
- [Compatibility tools](tools/NJUlogin/README.md)

## License

The standalone implementation is licensed under the MIT License. It was
informed by the MIT-licensed
[Do1e/NJUlogin](https://github.com/Do1e/NJUlogin) project; see
[`NOTICE`](NOTICE) and [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
