#!/usr/bin/env python3
"""Verify that ``docs/reference/mise-tasks.md`` covers every mise task.

The reference page is the only place a contributor learns which ``mise run``
names exist. Tasks added to ``.mise.toml`` without a row there are invisible,
and rows left behind by a removed task send readers to a command that fails.
Both directions are checked, so the page cannot drift in either direction.

Task names come from ``mise tasks ls --json`` rather than from parsing
``.mise.toml``, so tasks contributed by a ``.mise/tasks/`` directory or an
installed plugin are covered too.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

DOC_PAGE = "docs/reference/mise-tasks.md"

# A Markdown table cell that is exactly one backtick-quoted identifier, with an
# optional argument placeholder after it: ``| `build` |`` or
# ``| `godog-tag <tag>` |``. Argument text and alias parentheticals are dropped.
TASK_CELL_RE = re.compile(r"^\|\s*`([a-z0-9][a-z0-9._:-]*)\b[^|]*`")


def mise_task_names(repo_root: Path) -> set[str] | None:
    """Return the task names mise reports, or ``None`` when mise is absent."""
    try:
        result = subprocess.run(
            ("mise", "tasks", "ls", "--json"),
            cwd=repo_root,
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )
    except OSError:
        return None
    if result.returncode != 0:
        return None
    try:
        payload = json.loads(result.stdout.decode("utf-8", "replace") or "null")
    except json.JSONDecodeError:
        return None
    if isinstance(payload, dict):
        return {str(name) for name in payload}
    if isinstance(payload, list):
        return {
            str(item["name"])
            for item in payload
            if isinstance(item, dict) and "name" in item
        }
    return None


def documented_task_names(page: Path) -> set[str]:
    """Return task names named in the first cell of a ``## Tasks`` row.

    Only that section is parsed: the tool-pin table uses the same
    backtick-in-first-cell shape for plugin names such as
    ``aqua:gitleaks/gitleaks``, and the environment table does likewise for
    variable names, none of which are tasks.
    """
    if not page.is_file():
        return set()
    names: set[str] = set()
    in_tasks = False
    for line in page.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped.startswith("#"):
            in_tasks = stripped.lstrip("#").strip() == "Tasks"
            continue
        if not in_tasks:
            continue
        match = TASK_CELL_RE.match(stripped)
        if match:
            names.add(match.group(1))
    return names


def check(repo_root: Path) -> int:
    """Compare mise's task list with the reference page."""
    page = repo_root / DOC_PAGE
    if not page.is_file():
        print(f"{DOC_PAGE}: reference page is missing.")
        return 1

    tasks = mise_task_names(repo_root)
    if tasks is None:
        print(f"Skipped: `mise tasks ls --json` unavailable; cannot enumerate tasks.")
        return 0

    documented = documented_task_names(page)
    undocumented = sorted(tasks - documented)
    phantom = sorted(documented - tasks)

    for name in undocumented:
        print(f"{DOC_PAGE}: `.mise.toml` defines `{name}` but the page has no row for it.")
    for name in phantom:
        print(f"{DOC_PAGE}: row names `{name}`, which mise does not define.")

    if undocumented or phantom:
        print(
            f"\nFAILED: {len(undocumented)} undocumented task(s), "
            f"{len(phantom)} phantom row(s) "
            f"({len(tasks)} tasks defined, {len(documented)} documented)."
        )
        return 1

    print(f"OK: all {len(tasks)} mise tasks are documented in {DOC_PAGE}.")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path.cwd())
    args = parser.parse_args(argv)
    return check(args.repo_root)


if __name__ == "__main__":
    sys.exit(main())
