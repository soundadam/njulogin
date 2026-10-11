#!/usr/bin/env python3
"""Check that the release archives carry a license for every linked module.

The modules are those `go list -deps` links into ./cmd/njulogin for each
target that scripts/package.sh ships. Each needs at least one file named
third_party_licenses/<module path with / as _>_<license file name>, referenced
from THIRD_PARTY_NOTICES.md. Every LICENSE*, COPYING*, or NOTICE* file at the
module root in the module cache must be copied there byte for byte, and no
license file may remain for a module that is no longer linked.
"""
from __future__ import annotations

import os
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LICENSES = ROOT / "third_party_licenses"
LICENSE_PREFIXES = ("LICENSE", "LICENCE", "COPYING", "NOTICE")


def go(*args: str, env: dict[str, str] | None = None) -> str:
    return subprocess.run(
        ["go", *args], cwd=ROOT, env=env, check=True, capture_output=True, text=True
    ).stdout


def linked_modules() -> dict[str, Path]:
    targets = subprocess.run(
        ["bash", str(ROOT / "scripts/package.sh"), "--targets"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split()
    modules: dict[str, Path] = {}
    for target in targets:
        goos, goarch = target.split("/")
        env = {**os.environ, "GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0"}
        output = go(
            "list",
            "-deps",
            "-f",
            "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Dir}}{{end}}{{end}}",
            "./cmd/njulogin",
            env=env,
        )
        for line in output.splitlines():
            if line:
                path, directory = line.split("\t")
                modules[path] = Path(directory)
    return modules


def main() -> None:
    modules = linked_modules()
    notices = (ROOT / "THIRD_PARTY_NOTICES.md").read_text(encoding="utf-8")
    present = {path.name for path in LICENSES.iterdir() if path.is_file()}
    claimed: set[str] = set()
    problems: list[str] = []

    for module, directory in sorted(modules.items()):
        prefix = module.replace("/", "_") + "_"
        files = sorted(name for name in present if name.startswith(prefix))
        claimed.update(files)
        if not files:
            problems.append(f"{module}: no license under third_party_licenses/{prefix}*")
        for name in files:
            if f"third_party_licenses/{name}" not in notices:
                problems.append(f"THIRD_PARTY_NOTICES.md does not reference third_party_licenses/{name}")
        for upstream in sorted(directory.iterdir()):
            if not upstream.is_file() or not upstream.name.upper().startswith(LICENSE_PREFIXES):
                continue
            copy = LICENSES / (prefix + upstream.name)
            if not copy.is_file():
                problems.append(f"{module}: {upstream.name} is not copied to {copy.relative_to(ROOT)}")
            elif copy.read_bytes() != upstream.read_bytes():
                problems.append(f"{copy.relative_to(ROOT)} differs from {module}'s {upstream.name}")

    for name in sorted(present - claimed):
        problems.append(f"third_party_licenses/{name} belongs to no linked module")

    if problems:
        raise SystemExit("\n".join(problems))
    print(f"third_party_licenses=pass modules={len(modules)} files={len(claimed)}")


if __name__ == "__main__":
    main()
