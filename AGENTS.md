# Repository guidance

- Keep the canonical repository, Formula token, module suffix, and executable
  name `njulogin`; use `NJUlogin` only as the human-facing product name.
- This repository owns the standalone `p.nju.edu.cn` authentication CLI. It
  does not own SoundVPN, NJUConnect, EasyConnect, campus routing, VPN policy,
  SSH host aliases, or remote-server configuration.
- Keep remote operations host-agnostic. The binary may invoke the user's SSH
  client and configuration, but must not hard-code Elon or bypass an owning
  infrastructure repository's execution contract.
- Never put usernames, passwords, cookies, QR URLs, tokens, complete portal
  responses, session identifiers, device MAC addresses, or IP addresses in
  Git, tests, fixtures, command arguments, logs, or release artifacts.
- Passwords may be read interactively, from standard input, or from an
  explicitly selected macOS Keychain item. Keychain writes are allowed only
  through the explicit `setup` command and must keep the password out of the
  process argument list.
- Local configuration stores only the default username and Keychain service,
  using directory mode `0700` and file mode `0600`.
- Keep status and informational output redacted. Destructive session or device
  operations require an explicit command and should prefer a displayed index
  over exposing stable backend identifiers.
- Unit tests, CI, and Homebrew Formula tests must be offline. Real login,
  logout, QR, device binding, and remote SSH are explicit operator acceptance
  steps only.
- Homebrew installs the compiled `njulogin` binary. Compatibility shell tools
  remain source/developer aids and are not installed by the Formula.
- Do not publish a repository, GitHub Release, tag, or tap Formula without a
  separate explicit owner request and release-readiness review.
