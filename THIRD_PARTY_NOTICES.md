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
  License, for command parsing and help. The exact license text is retained at
  [`third_party_licenses/github.com_spf13_cobra_LICENSE.txt`](third_party_licenses/github.com_spf13_cobra_LICENSE.txt).
- [`github.com/spf13/pflag`](https://github.com/spf13/pflag), BSD-3-Clause
  License, used by `cobra`. The exact license text is retained at
  [`third_party_licenses/github.com_spf13_pflag_LICENSE`](third_party_licenses/github.com_spf13_pflag_LICENSE).
- [`golang.org/x/mod`](https://pkg.go.dev/golang.org/x/mod), BSD-3-Clause
  License, used by `appkit` for version comparison. The exact license text is
  retained at
  [`third_party_licenses/golang.org_x_mod_LICENSE`](third_party_licenses/golang.org_x_mod_LICENSE).

The interactive interface links these modules. Each license text is retained
under `third_party_licenses/`:

| Module | License | Purpose | Retained text |
| --- | --- | --- | --- |
| [`github.com/charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) | MIT | interactive terminal interface | [`github.com_charmbracelet_bubbletea_LICENSE`](third_party_licenses/github.com_charmbracelet_bubbletea_LICENSE) |
| [`github.com/charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) | MIT | text inputs and spinner | [`github.com_charmbracelet_bubbles_LICENSE`](third_party_licenses/github.com_charmbracelet_bubbles_LICENSE) |
| [`github.com/charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) | MIT | terminal styling | [`github.com_charmbracelet_lipgloss_LICENSE`](third_party_licenses/github.com_charmbracelet_lipgloss_LICENSE) |
| [`github.com/charmbracelet/colorprofile`](https://github.com/charmbracelet/colorprofile) | MIT | terminal color detection | [`github.com_charmbracelet_colorprofile_LICENSE`](third_party_licenses/github.com_charmbracelet_colorprofile_LICENSE) |
| [`github.com/charmbracelet/x/ansi`](https://github.com/charmbracelet/x/ansi) | MIT | ANSI sequence handling | [`github.com_charmbracelet_x_ansi_LICENSE`](third_party_licenses/github.com_charmbracelet_x_ansi_LICENSE) |
| [`github.com/charmbracelet/x/cellbuf`](https://github.com/charmbracelet/x/cellbuf) | MIT | screen cell buffer | [`github.com_charmbracelet_x_cellbuf_LICENSE`](third_party_licenses/github.com_charmbracelet_x_cellbuf_LICENSE) |
| [`github.com/charmbracelet/x/term`](https://github.com/charmbracelet/x/term) | MIT | terminal state | [`github.com_charmbracelet_x_term_LICENSE`](third_party_licenses/github.com_charmbracelet_x_term_LICENSE) |
| [`github.com/aymanbagabas/go-osc52/v2`](https://github.com/aymanbagabas/go-osc52) | MIT | OSC 52 clipboard sequences | [`github.com_aymanbagabas_go-osc52_v2_LICENSE`](third_party_licenses/github.com_aymanbagabas_go-osc52_v2_LICENSE) |
| [`github.com/atotto/clipboard`](https://github.com/atotto/clipboard) | BSD-3-Clause | paste into text inputs | [`github.com_atotto_clipboard_LICENSE`](third_party_licenses/github.com_atotto_clipboard_LICENSE) |
| [`github.com/clipperhouse/displaywidth`](https://github.com/clipperhouse/displaywidth) | MIT | display width | [`github.com_clipperhouse_displaywidth_LICENSE`](third_party_licenses/github.com_clipperhouse_displaywidth_LICENSE) |
| [`github.com/clipperhouse/uax29/v2`](https://github.com/clipperhouse/uax29) | MIT | grapheme segmentation | [`github.com_clipperhouse_uax29_v2_LICENSE`](third_party_licenses/github.com_clipperhouse_uax29_v2_LICENSE) |
| [`github.com/lucasb-eyer/go-colorful`](https://github.com/lucasb-eyer/go-colorful) | MIT | color conversion | [`github.com_lucasb-eyer_go-colorful_LICENSE`](third_party_licenses/github.com_lucasb-eyer_go-colorful_LICENSE) |
| [`github.com/mattn/go-isatty`](https://github.com/mattn/go-isatty) | MIT | terminal detection | [`github.com_mattn_go-isatty_LICENSE`](third_party_licenses/github.com_mattn_go-isatty_LICENSE) |
| [`github.com/mattn/go-runewidth`](https://github.com/mattn/go-runewidth) | MIT | rune width | [`github.com_mattn_go-runewidth_LICENSE`](third_party_licenses/github.com_mattn_go-runewidth_LICENSE) |
| [`github.com/muesli/ansi`](https://github.com/muesli/ansi) | MIT | ANSI-aware string width | [`github.com_muesli_ansi_LICENSE`](third_party_licenses/github.com_muesli_ansi_LICENSE) |
| [`github.com/muesli/cancelreader`](https://github.com/muesli/cancelreader) | MIT | cancelable terminal input | [`github.com_muesli_cancelreader_LICENSE`](third_party_licenses/github.com_muesli_cancelreader_LICENSE) |
| [`github.com/muesli/termenv`](https://github.com/muesli/termenv) | MIT | terminal capabilities | [`github.com_muesli_termenv_LICENSE`](third_party_licenses/github.com_muesli_termenv_LICENSE) |
| [`github.com/rivo/uniseg`](https://github.com/rivo/uniseg) | MIT | Unicode segmentation | [`github.com_rivo_uniseg_LICENSE.txt`](third_party_licenses/github.com_rivo_uniseg_LICENSE.txt) |
| [`github.com/xo/terminfo`](https://github.com/xo/terminfo) | MIT | terminfo lookup | [`github.com_xo_terminfo_LICENSE`](third_party_licenses/github.com_xo_terminfo_LICENSE) |

`scripts/check-third-party-licenses.py` derives the linked modules from the
build graph of every target `scripts/package.sh` ships and fails when a module
lacks a retained license, a retained text differs from the module's own, or
this notice does not reference it.

The product's portal operation model was informed by
[`Do1e/NJUlogin`](https://github.com/Do1e/NJUlogin), MIT License. The upstream
Python package is not linked, downloaded, or vendored by the compiled product.

Exact dependency revisions are recorded in `go.mod` and `go.sum`. Every release
archive carries this notice, `NOTICE`, `LICENSE`, and the files under
`third_party_licenses/`; `scripts/check-third-party-licenses.py` checks that
they cover every module linked into a shipped target.
