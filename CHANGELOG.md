# Changelog

## 0.1.1 - 2026-07-27

- Replace the private Release asset Formula headers with a dedicated Homebrew
  download strategy that reads `HOMEBREW_GITHUB_API_TOKEN` only at fetch time,
  masks it as a secret, and passes strict online audit.
- Keep `v0.1.0` immutable after its post-tag Formula audit exposed the issue;
  publish the corrected Homebrew distribution from `v0.1.1`.

## 0.1.0 - 2026-07-27

- Extract the `p.nju.edu.cn` client from the former `adam-network-tools`
  workspace into an independent product repository.
- Preserve local and remote status, password login, CAS QR login, logout,
  online-session, and quick-login operations.
- Add source-build Homebrew packaging, service-isolated CI, provenance, and
  security boundaries.
- Move the canonical maintainer checkout and source iteration to the Elon
  Linux host; reserve the local macOS machine for Homebrew installation and
  installed-product acceptance.
- Select the patched Go 1.25.12 toolchain for source and CI builds.
- Make portal rejection, malformed responses, and failed mutation read-back
  return nonzero status for local and remote commands.
- Enforce mutually exclusive credential sources before network or SSH work.
- Standardize repeated output as `session_N`, `online_N`, and `quick_N`
  records while keeping portal reply messages and sensitive subprocess output
  out of diagnostics.
- Add bounded synthetic coverage for local, remote, QR, compatibility-helper,
  and Formula behavior; bind race, 60% coverage, Staticcheck, ShellCheck,
  Actionlint, repository invariants, and vulnerability scanning into the
  source release gate.
- Replace OS-keyring storage with an explicit local plaintext credential file
  protected by Unix permissions and atomic writes.
- Replace remote curl/Python/helper execution with local HTTP, TLS, CAS, and QR handling over SSH `-W` direct-tcpip tunnels.
- Keep saved passwords and short-lived QR URLs out of process arguments and terminal output.
- Publish the product as a private GitHub repository and private release asset,
  with authenticated installation metadata in `soundadam/homebrew-tap`.
- Record the service-isolated synthetic portal/CAS fixtures and the real
  Mac-to-Elon OpenSSH `-W` acceptance path.
