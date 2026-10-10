# Security model

## Local credential file

`setup` stores one username/password pair in `config.json`. The password is
plaintext. The product does not claim encryption and does not use an
operating-system keyring.

On supported Unix-like systems, the configuration directory is mode `0700` and
the file is a regular non-symlink file with mode `0600`. Writes use a temporary
file and atomic replacement. Reads reject permissive modes, symlinks, and
special files rather than silently accepting a weaker boundary.

The operating-system account, filesystem, backups, snapshots, and administrator
access are trusted. Anyone able to read the file can read the password. The
first release does not migrate, import, or delete credentials from an OS
keyring.

`--password-command` executes an operator-supplied local shell command. It is an
advanced integration surface, is never configured automatically, and is killed
if it does not complete within 15 seconds. Passwords may also be read from
standard input or a no-echo terminal prompt. They are never accepted as direct
command-line values.

## Remote transport

Remote operations do not run portal code on the remote host. The local process
performs HTTP, TLS, cookies, JSON parsing, CAS state transitions, QR generation,
redaction, and postcondition verification. The local SSH client opens a
`direct-tcpip` stdio channel with `ssh -W` to each required portal or CAS TCP
destination.

The SSH server therefore needs TCP-forwarding permission but no curl, Python,
helper script, installed `njulogin`, or interactive shell command. Portal
request bodies, cookies, and QR state flow as protocol bytes inside the SSH
channel and are not SSH arguments.

For normal HTTPS endpoints, TLS is established and verified locally through the
SSH tunnel. The SSH server can observe destination metadata and traffic timing,
but not the TLS plaintext. `--insecure` disables local TLS verification only for
that explicit invocation and emits a warning.

## Output and error handling

Normal reports mask accounts, names, and MAC addresses, omit IP addresses, and
normalize control characters into a single line. QR URLs, cookies, passwords,
raw tokens, complete portal responses, and token-bearing request URLs are not
printed.

Backend device identifiers may appear only where explicitly needed for
quick-login deletion. Real identifiers must not appear in tests, issue reports,
CI logs, review packets, or release evidence.

HTTP responses are size-bounded. Portal rejection, malformed data, oversized
responses, SSH failure, TLS failure, and unverified mutations return nonzero.
Raw SSH stderr and raw upstream response bodies are not copied into structured
errors.

## Trust boundaries

The separate trust boundaries are:

- the local operating-system account and credential file;
- the local `njulogin` process;
- the local SSH client and its configuration;
- the SSH server and its forwarding policy;
- the NJU portal and CAS services.

A successful SSH connection does not prove portal login. A successful portal
response does not prove the requested postcondition. Both local and remote
operations require the same response validation and read-back verification.
