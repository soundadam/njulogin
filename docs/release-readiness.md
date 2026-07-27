# Release readiness

The extraction is not a release.

## Required before the first tag

- `make check` passes from a clean checkout.
- The release binary reports the intended version injected at build time.
- Secret scanning and synthetic-fixture review pass.
- A local operator confirms status, password login, QR login, logout, and
  remote QR login without recording credentials or raw responses.
- The source archive is built from a reviewed immutable tag and its SHA-256 is
  recorded.
- The Formula is rendered from `packaging/Formula/njulogin.rb.in`.
- A clean local Homebrew source build passes the offline Formula test.
- `NOTICE`, dependency licenses, changelog, and security documentation match
  the tagged source.

## Separate owner actions

The following are not authorized by extraction:

- creating or publishing `soundadam/njulogin`;
- pushing source or tags;
- creating a GitHub Release;
- adding the Formula to `soundadam/homebrew-tap`;
- making the repository or Formula public.
