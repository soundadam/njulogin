# njulogin

`njulogin` is a standalone command-line client for Nanjing University's
`p.nju.edu.cn` authentication portal. It can inspect redacted state, log in or
out, manage quick-login devices, render a CAS QR login, and run the same portal
protocol over a network path reached through SSH.

This repository was extracted from the former `adam-network-tools` workspace.
It is independent of SoundVPN, NJUConnect, EasyConnect, proxy selection, and
server desired state.

## Install

On macOS and Linux (x86_64 and aarch64), install the Homebrew Cask:

```sh
brew install --cask soundadam/tap/njulogin
```

Or download `njulogin-<version>-<os>-<arch>.tar.gz` from
[Releases](https://github.com/soundadam/njulogin/releases) and put the
`njulogin` executable from it on `PATH`. The archive also carries the license
material. There are no Windows builds: the credential file protection below
relies on Unix file modes.

macOS builds are not notarized. A Cask install and a self-update are not
blocked by Gatekeeper; an archive downloaded with a browser is quarantined, and
macOS refuses the first run. Allow it under System Settings → Privacy &
Security, or remove the attribute:

```sh
xattr -d com.apple.quarantine <path to the extracted njulogin>
```

Build from source with the Go toolchain selected by the `toolchain` directive
in `go.mod`:

```sh
make check
make build
./bin/njulogin version
```

Remote operations need an OpenSSH-compatible client and an SSH server that
permits direct-tcpip forwarding to the NJU portal and CAS destinations.

## Updates

```sh
njulogin update              # install the latest release
njulogin update --check      # only report whether a newer release exists
njulogin update --auto off   # turn the automatic check off (on turns it back on)
```

A Cask install upgrades through `brew`; a downloaded executable replaces itself
after checking the archive against the SHA-256 digest GitHub publishes for it.
Source builds report version `dev` and do not update.

At most once a day, a command whose standard error is a terminal checks GitHub
for a newer release in the background; when one is known, one line after the
command's output says so. The update contract, including how the
installation is detected and which requests are made, is
[appkit's](https://github.com/soundadam/appkit#更新).

The 0.1.x Homebrew Formula builds have no `update` command. Move to the Cask
with:

```sh
brew uninstall njulogin
brew install --cask soundadam/tap/njulogin
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
njulogin update [--check]
njulogin update --auto on|off
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

## Releases

Artifact names, the Cask, and prerelease handling follow
[appkit](https://github.com/soundadam/appkit#发布产物). Pushing a `v<version>`
tag runs the [release workflow](.github/workflows/release.yml): it takes the
release notes from the `## <version>` section of [`CHANGELOG.md`](CHANGELOG.md),
builds the archives with `scripts/package.sh`, publishes the GitHub Release,
and pushes [`packaging/homebrew/njulogin.rb`](packaging/homebrew/njulogin.rb),
filled in by `scripts/render-cask.sh`, to `soundadam/homebrew-tap`. Tags
containing `-` publish a prerelease and leave the tap alone. The gates before a
tag are in [release readiness](docs/release-readiness.md).

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
