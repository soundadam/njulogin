# Compatibility tools

The installed product is the `njulogin` Go binary. This directory preserves
the former shell entrypoints for migration and diagnostics:

- `njulogin-remote.sh` forwards to an installed `njulogin remote` command, or
  to `go run ./cmd/njulogin remote` from a source checkout.
- `njulogin-portal.sh` is a dependency-light portal helper for remote hosts
  with Bash, curl, and Python.

Homebrew does not install these scripts. New functionality belongs in the Go
CLI so the repository has one canonical product interface.

Examples from a source checkout:

```sh
tools/NJUlogin/njulogin-remote.sh my-host status
tools/NJUlogin/njulogin-remote.sh my-host qr
```

Real portal and SSH operations are explicit operator acceptance steps. Do not
run them from tests or CI.
