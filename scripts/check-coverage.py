#!/usr/bin/env python3

import re
import subprocess
import sys


CRITICAL_FUNCTIONS = {
    "setupCommand",
    "statusCommand",
    "infoCommand",
    "loginCommand",
    "resolveLoginCredential",
    "qrCommand",
    "qrCommandWithClient",
    "finishQRLogin",
    "logoutCommand",
    "kickCommand",
    "quickCommand",
    "remoteCommand",
    "remoteStatusCommand",
    "remoteLoginCommand",
    "remoteLogoutCommand",
    "remoteInfoCommand",
    "remoteQuickCommand",
    "remoteKickCommand",
    "remoteQRCommand",
    "newSSHHTTPClient",
    "dialSSHTunnel",
    "run",
}


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: check-coverage.py PROFILE MIN_PERCENT", file=sys.stderr)
        return 2

    profile = sys.argv[1]
    minimum = float(sys.argv[2])
    result = subprocess.run(
        ["go", "tool", "cover", f"-func={profile}"],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )

    functions: dict[str, float] = {}
    total = None
    function_pattern = re.compile(r"^.+?:\d+:\s+(\S+)\s+([0-9.]+)%$")
    total_pattern = re.compile(r"^total:.*\s([0-9.]+)%$")
    for line in result.stdout.splitlines():
        match = function_pattern.match(line)
        if match:
            functions[match.group(1)] = float(match.group(2))
            continue
        match = total_pattern.match(line)
        if match:
            total = float(match.group(1))

    if total is None:
        print("coverage total was not found", file=sys.stderr)
        return 1
    if total < minimum:
        print(f"coverage {total:.1f}% is below required {minimum:.1f}%", file=sys.stderr)
        return 1

    missing = sorted(name for name in CRITICAL_FUNCTIONS if name not in functions)
    uncovered = sorted(name for name in CRITICAL_FUNCTIONS if functions.get(name, 0.0) <= 0.0)
    if missing:
        print("critical functions missing from coverage report: " + ", ".join(missing), file=sys.stderr)
        return 1
    if uncovered:
        print("release-critical functions have zero coverage: " + ", ".join(uncovered), file=sys.stderr)
        return 1

    print(f"coverage={total:.1f}% minimum={minimum:.1f}% critical_functions=covered")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
