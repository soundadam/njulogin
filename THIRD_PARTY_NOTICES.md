# Third-party notices

The compiled `njulogin` executable uses:

- [`github.com/skip2/go-qrcode`](https://github.com/skip2/go-qrcode), MIT
  License, for terminal QR rendering.
- [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term), BSD-3-Clause
  License, for no-echo terminal input and terminal detection.

The product's portal operation model was informed by
[`Do1e/NJUlogin`](https://github.com/Do1e/NJUlogin), MIT License. The upstream
Python package is not linked, downloaded, or vendored by the compiled product.

Exact dependency revisions are recorded in `go.mod` and `go.sum`. Distribution
must preserve the corresponding license notices.
