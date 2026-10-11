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
- [`github.com/soundadam/appkit/go`](https://github.com/soundadam/appkit),
  available under MIT or Apache-2.0 and used here under the MIT License, for
  release lookup, self-update, and the `update` and `version` commands. The
  exact license text is retained at
  [`third_party_licenses/github.com_soundadam_appkit_go_LICENSE-MIT`](third_party_licenses/github.com_soundadam_appkit_go_LICENSE-MIT);
  the module's Apache-2.0 text is retained at
  [`third_party_licenses/github.com_soundadam_appkit_go_LICENSE-APACHE`](third_party_licenses/github.com_soundadam_appkit_go_LICENSE-APACHE).
- [`github.com/spf13/cobra`](https://github.com/spf13/cobra), Apache-2.0
  License, for command dispatch. The exact license text is retained at
  [`third_party_licenses/github.com_spf13_cobra_LICENSE.txt`](third_party_licenses/github.com_spf13_cobra_LICENSE.txt).
- [`github.com/spf13/pflag`](https://github.com/spf13/pflag), BSD-3-Clause
  License, used by `cobra`. The exact license text is retained at
  [`third_party_licenses/github.com_spf13_pflag_LICENSE`](third_party_licenses/github.com_spf13_pflag_LICENSE).
- [`golang.org/x/mod`](https://pkg.go.dev/golang.org/x/mod), BSD-3-Clause
  License, used by `appkit` for version comparison. The exact license text is
  retained at
  [`third_party_licenses/golang.org_x_mod_LICENSE`](third_party_licenses/golang.org_x_mod_LICENSE).

The product's portal operation model was informed by
[`Do1e/NJUlogin`](https://github.com/Do1e/NJUlogin), MIT License. The upstream
Python package is not linked, downloaded, or vendored by the compiled product.

Exact dependency revisions are recorded in `go.mod` and `go.sum`. Every release
archive carries this notice, `NOTICE`, `LICENSE`, and the files under
`third_party_licenses/`; `scripts/check-third-party-licenses.py` checks that
they cover every module linked into a shipped target.
