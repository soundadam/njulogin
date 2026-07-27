#!/usr/bin/env python3

import re
import sys
from pathlib import Path


LINK_PATTERN = re.compile(r"\[[^\]]*\]\(([^)]+)\)")
REQUIRED_DOCUMENTS = {
    Path("SPEC.md"),
    Path("README.md"),
    Path("SECURITY.md"),
    Path("docs/architecture.md"),
    Path("docs/cli-contract.md"),
    Path("docs/development-contract.md"),
    Path("docs/development-topology.md"),
    Path("docs/release-readiness.md"),
    Path("docs/review-handoff.md"),
    Path("docs/security.md"),
}


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    missing_documents = sorted(path for path in REQUIRED_DOCUMENTS if not (root / path).is_file())
    if missing_documents:
        for path in missing_documents:
            print(f"missing required document: {path}", file=sys.stderr)
        return 1

    missing_links: list[tuple[Path, str]] = []
    for path in sorted(root.rglob("*.md")):
        if ".git" in path.parts or any(part.startswith(".") and part not in {".github"} for part in path.relative_to(root).parts):
            continue
        text = path.read_text(encoding="utf-8")
        for target in LINK_PATTERN.findall(text):
            if target.startswith(("http://", "https://", "mailto:", "#")):
                continue
            clean = target.split("#", 1)[0]
            if clean and not (path.parent / clean).exists():
                missing_links.append((path.relative_to(root), target))

    if missing_links:
        for path, target in missing_links:
            print(f"broken local link: {path}: {target}", file=sys.stderr)
        return 1

    print(f"documentation=pass markdown_files={sum(1 for _ in root.rglob('*.md'))}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
