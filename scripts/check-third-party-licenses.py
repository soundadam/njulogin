#!/usr/bin/env python3
"""Require a retained license for every module linked into a release binary.

The module list comes from `go list -deps` for each release platform, and each
retained file must match the license shipped inside the module verified by
go.sum. `--write` copies missing or changed license files; notices stay
hand-written so a reviewer reads every new license.
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LICENSE_DIR = ROOT / "third_party_licenses"
PLATFORMS = [("darwin", "arm64"), ("darwin", "amd64"), ("linux", "amd64"), ("linux", "arm64"), ("windows", "amd64")]
LICENSE_NAMES = ["LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "COPYING"]


def linked_modules() -> dict[str, str]:
    modules: dict[str, str] = {}
    for goos, goarch in PLATFORMS:
        env = {**os.environ, "GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0"}
        output = subprocess.run(
            ["go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}", "./cmd/njulogin"],
            cwd=ROOT, env=env, check=True, text=True, stdout=subprocess.PIPE,
        ).stdout
        for line in output.splitlines():
            if line.strip():
                path, version = line.split()
                modules[path] = version
    return modules


def module_license(path: str, version: str) -> Path:
    info = json.loads(subprocess.run(
        ["go", "mod", "download", "-json", f"{path}@{version}"],
        cwd=ROOT, check=True, text=True, stdout=subprocess.PIPE,
    ).stdout)
    directory = Path(info["Dir"])
    for name in LICENSE_NAMES:
        if (directory / name).is_file():
            return directory / name
    raise SystemExit(f"{path}@{version} ships no license file")


def retained_name(path: str) -> str:
    return path.replace("/", "_") + "_LICENSE"


def main() -> None:
    write = sys.argv[1:] == ["--write"]
    modules = linked_modules()
    notices = (ROOT / "THIRD_PARTY_NOTICES.md").read_text(encoding="utf-8")
    expected = set()
    problems = []
    for path, version in sorted(modules.items()):
        name = retained_name(path)
        expected.add(name)
        source = module_license(path, version).read_bytes()
        retained = LICENSE_DIR / name
        if not retained.is_file() or retained.read_bytes() != source:
            if write:
                retained.write_bytes(source)
            else:
                problems.append(f"{retained.relative_to(ROOT)} is missing or differs from {path}@{version}")
        if f"third_party_licenses/{name}" not in notices:
            problems.append(f"THIRD_PARTY_NOTICES.md does not reference third_party_licenses/{name}")
    for stale in sorted(p.name for p in LICENSE_DIR.iterdir() if p.name not in expected):
        problems.append(f"third_party_licenses/{stale} belongs to no linked module")
    if problems:
        raise SystemExit("\n".join(problems))
    print(f"third_party_licenses=pass files={len(expected)}")


if __name__ == "__main__":
    main()
