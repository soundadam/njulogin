# Repository guidance

## Authority and workflow

- The only writable maintainer checkout is
  `/home/adam/projects/homebrew-upload/products/njulogin` on Elon. Do not edit
  product source or contracts from the former macOS checkout.
- Read `SPEC.md`, `docs/cli-contract.md`, and
  `docs/development-contract.md` before changing behavior. They are normative;
  README examples and current implementation do not override them.
- Start non-trivial work with the change brief defined in
  `docs/development-contract.md`.
- Cross-agent or API review must use the immutable packet protocol in
  `docs/review-handoff.md`. Reviewers report findings and do not edit or
  authorize publication.
- Identify in-scope and out-of-scope work before editing. Do not add commands,
  flags, dependencies, persistence mechanisms, remote runtimes, or platform
  promises inside an unrelated fix.
- Intentional contract changes update normative documents and executable
  evidence in the same change.
- Release-only work must not contain product behavior changes.
- Follow `docs/development-topology.md`: source work occurs on Elon; the local
  Mac installs a Homebrew candidate and returns sanitized acceptance evidence.

## Product invariants

- Keep the repository, Formula token, module suffix, and executable name
  `njulogin`; use `NJUlogin` only as the human-facing product name.
- This repository owns the standalone `p.nju.edu.cn` authentication CLI. It
  does not own SoundVPN, NJUConnect, EasyConnect, routes, VPN policy, SSH host
  inventory, or remote-server configuration.
- Local credentials are one plaintext username/password pair in `config.json`.
  On supported Unix-like systems, enforce directory mode `0700`, regular file
  mode `0600`, atomic replacement, and symlink rejection. Do not add OS keyring,
  Secret Service, KWallet, encryption-key management, or automatic migration.
- Passwords may be read from the saved file, a no-echo prompt, standard input,
  or an explicitly supplied trusted password command. They must never be
  direct command-line values.
- Keep remote operations local-protocol-first. The Go process owns HTTP, TLS,
  cookies, JSON, CAS QR state, QR rendering, redaction, and postcondition
  verification. SSH is only a direct-tcpip byte tunnel using `-W`.
- Do not run curl, Python, shell helpers, or `njulogin` on the remote host. Do
  not copy files to the remote host. Do not reintroduce remote proxy flags or
  environment variables.
- Keep remote operations host-agnostic. The binary may invoke the user's SSH
  client and configuration, but must not hard-code Elon or bypass an owning
  infrastructure repository's execution contract.
- Never put real, captured, or unsanitized passwords, cookies, QR URLs, tokens,
  complete portal responses, session identifiers, or device identifiers in
  Git, tests, fixtures, process arguments, logs, issue reports, review packets,
  or release artifacts. Synthetic tests may use only minimal reserved values.
- Keep output redacted and one-record-per-line. Untrusted portal fields must be
  normalized before output.
- Unit tests, CI, and Formula tests must be service-isolated. Real credential
  files, portal operations, QR scans, device mutations, and real SSH hosts are
  operator acceptance only.
- Homebrew installs only the compiled Go binary. Compatibility shell tools are
  source/developer aids and are not installed.
- Do not publish a repository, GitHub Release, tag, or tap Formula without a
  separate explicit owner request and release-readiness review.
