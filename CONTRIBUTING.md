# Contributing

Run the complete local gate before proposing a change:

```sh
make check
```

Tests must be offline and use synthetic fixtures. Do not record or commit a
real NJU account, password, Cookie, QR URL, portal response, session ID, MAC
address, or IP address.

Changes to portal endpoints or authentication behavior require a separate
operator acceptance run after automated tests. Never make a real login,
logout, device bind, or remote SSH call from CI.
