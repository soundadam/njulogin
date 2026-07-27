#!/usr/bin/env python3
from __future__ import annotations

import hashlib
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
EXPECTED = {
    "third_party_licenses/github.com_skip2_go-qrcode_LICENSE": (
        "4cdab772de606bdf53e174b2f5d6a2c398156380ca1f763122198cece531d02c"
    ),
    "third_party_licenses/golang.org_x_term_LICENSE": (
        "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"
    ),
    "third_party_licenses/golang.org_x_sys_LICENSE": (
        "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"
    ),
}
REQUIRED_MODULES = {
    "github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e",
    "golang.org/x/term v0.44.0",
    "golang.org/x/sys v0.46.0",
}


def main() -> None:
    go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
    missing_modules = sorted(value for value in REQUIRED_MODULES if value not in go_mod)
    if missing_modules:
        raise SystemExit(
            "dependency revisions changed without updating the license gate: "
            + ", ".join(missing_modules)
        )

    for relative, expected in EXPECTED.items():
        path = ROOT / relative
        if not path.is_file():
            raise SystemExit(f"missing third-party license: {relative}")
        actual = hashlib.sha256(path.read_bytes()).hexdigest()
        if actual != expected:
            raise SystemExit(
                f"third-party license digest changed for {relative}: {actual} != {expected}"
            )

    notices = (ROOT / "THIRD_PARTY_NOTICES.md").read_text(encoding="utf-8")
    for relative in EXPECTED:
        if relative not in notices:
            raise SystemExit(f"THIRD_PARTY_NOTICES.md does not reference {relative}")

    print(f"third_party_licenses=pass files={len(EXPECTED)}")


if __name__ == "__main__":
    main()
