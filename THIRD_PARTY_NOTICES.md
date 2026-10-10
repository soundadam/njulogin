# Third-party notices

The compiled `njulogin` executable uses:

- [`github.com/skip2/go-qrcode`](https://github.com/skip2/go-qrcode), MIT
  License, for terminal QR rendering. The exact license text is retained at
  [`third_party_licenses/github.com_skip2_go-qrcode_LICENSE`](third_party_licenses/github.com_skip2_go-qrcode_LICENSE).
- [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term), BSD-3-Clause
  License, for no-echo terminal input and terminal detection. The exact
  license text is retained at
  [`third_party_licenses/golang.org_x_term_LICENSE`](third_party_licenses/golang.org_x_term_LICENSE).
- [`golang.org/x/sys`](https://pkg.go.dev/golang.org/x/sys), BSD-3-Clause
  License, used indirectly by `golang.org/x/term`. The exact license text is
  retained at
  [`third_party_licenses/golang.org_x_sys_LICENSE`](third_party_licenses/golang.org_x_sys_LICENSE).
- [`github.com/spf13/cobra`](https://github.com/spf13/cobra), Apache License
  2.0, for command parsing and help. The exact license text is retained at
  [`third_party_licenses/github.com_spf13_cobra_LICENSE`](third_party_licenses/github.com_spf13_cobra_LICENSE).
- [`github.com/spf13/pflag`](https://github.com/spf13/pflag), BSD-3-Clause
  License, used by cobra for flag parsing. The exact license text is retained
  at
  [`third_party_licenses/github.com_spf13_pflag_LICENSE`](third_party_licenses/github.com_spf13_pflag_LICENSE).
- [`github.com/inconshreveable/mousetrap`](https://github.com/inconshreveable/mousetrap),
  Apache License 2.0, linked by cobra into Windows builds only. The exact
  license text is retained at
  [`third_party_licenses/github.com_inconshreveable_mousetrap_LICENSE`](third_party_licenses/github.com_inconshreveable_mousetrap_LICENSE).

`scripts/check-third-party-licenses.py` derives the linked modules from the
release platforms' build graphs and fails when a module lacks a retained
license, the retained text differs from the module's own, or this notice does
not reference it.

The product's portal operation model was informed by
[`Do1e/NJUlogin`](https://github.com/Do1e/NJUlogin), MIT License. The upstream
Python package is not linked, downloaded, or vendored by the compiled product.

Exact dependency revisions are recorded in `go.mod` and `go.sum`. Source
archives, Homebrew-built installations, and any future binary distribution
must preserve this notice and the corresponding files under
`third_party_licenses/`.
