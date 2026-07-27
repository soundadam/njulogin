# Compatibility tools

These source-tree scripts are migration and diagnostic aids. Homebrew does not
install them, and they are not a second public interface.

- `njulogin-portal.sh` is a local compatibility helper for direct portal
  diagnostics. It may use local Bash, curl, and Python and must remain
  service-isolated in automated tests.
- `njulogin-remote.sh` is only a thin wrapper around the local command:

  ```text
  njulogin remote [flags] <ssh-host> <operation> [args]
  ```

  It does not copy a helper, execute curl or Python remotely, read an OS
  keyring, or install `njulogin` on the remote host.

The compiled Go CLI owns credential storage, SSH tunneling, HTTP, TLS, CAS QR,
redaction, and success semantics. Compatibility scripts must not diverge from
that contract or acquire new product behavior.
