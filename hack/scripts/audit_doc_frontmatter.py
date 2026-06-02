#!/usr/bin/env python3
"""Audit Markdown docs for Diátaxis frontmatter compliance.

Walks ``docs/**/*.md`` and verifies every page satisfies the project's
Documentation steering rule:

    1. Frontmatter is the very first block in the file.
    2. The required keys are present:
       ``id``, ``title``, ``sidebar_label``, ``description``,
       ``doc_type``, ``audience``, ``tags``.
    3. ``doc_type`` is one of ``tutorial``, ``how-to``, ``reference``,
       or ``explanation``.
    4. ``id`` is a URL-safe slug (lowercase, digits, hyphens only).
    5. ``tags`` has at least one entry.

The script does not modify files.  It exits non-zero if any page fails,
making it suitable for use as a pre-merge check.

Usage::

    python3 hack/scripts/audit_doc_frontmatter.py [--repo-root .]
                                                  [--strict]
                                                  [--ignore PATTERN]...
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

REQUIRED_KEYS = (
    "id",
    "title",
    "sidebar_label",
    "description",
    "doc_type",
    "audience",
    "tags",
)

ALLOWED_DOC_TYPES = {"tutorial", "how-to", "reference", "explanation"}

# Default-ignored paths.  These are component-level configuration files
# that do not represent reader-facing pages and so are exempt from the
# steering rule.
DEFAULT_IGNORES = (
    "docs/modules/**",            # Antora tree (slated for removal).
    "docs/CODEMAPS/**",           # Internal architectural maps.
    "docs/superpowers/**",        # Internal automation specs.
    "docs/antora.yml",
    "docs/local-playbook.yml",
    "docs/README.md",             # Repo-navigation README, not a doc page.
)

# Slug must match the steering rule: lowercase + digits + hyphens.
_SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")


# ---------------------------------------------------------------------------
# Audit data structures
# ---------------------------------------------------------------------------

@dataclass
class Issue:
    """Single audit failure for a single file."""

    path: Path
    message: str

    def render(self, repo_root: Path) -> str:
        try:
            rel = self.path.relative_to(repo_root)
        except ValueError:
            rel = self.path
        return f"{rel}: {self.message}"


# ---------------------------------------------------------------------------
# Frontmatter parsing
# ---------------------------------------------------------------------------

# Match the leading frontmatter block.  Tolerates a leading byte-order
# mark and any leading newlines.
_FRONTMATTER_RE = re.compile(
    r"^\ufeff?\s*---\s*\n(.*?)\n---\s*\n", re.DOTALL
)

# Permissive scalar matcher.  We do not need a full YAML parser; the
# steering rule prescribes a flat block of scalar keys plus a ``tags``
# inline sequence.
_KEY_VALUE_RE = re.compile(r"^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*?)\s*$")


def parse_frontmatter(text: str) -> dict[str, str] | None:
    """Return a dict of frontmatter keys, or ``None`` if missing.

    Values are returned as raw strings (including any quoting and
    bracketed list markers); validation is the caller's job so the
    audit produces precise messages.
    """
    match = _FRONTMATTER_RE.match(text)
    if not match:
        return None
    block = match.group(1)
    fm: dict[str, str] = {}
    for line in block.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        kv = _KEY_VALUE_RE.match(line)
        if kv:
            fm[kv.group(1)] = kv.group(2)
    return fm


def unquote(value: str) -> str:
    """Strip matching surrounding quotes from a raw frontmatter value."""
    v = value.strip()
    if len(v) >= 2 and v[0] == v[-1] and v[0] in ("\"", "'"):
        return v[1:-1]
    return v


def parse_tags(raw: str) -> list[str]:
    """Parse the ``tags`` value into a list of stripped strings."""
    raw = raw.strip()
    if raw.startswith("[") and raw.endswith("]"):
        inner = raw[1:-1].strip()
        if not inner:
            return []
        return [unquote(p.strip()) for p in inner.split(",")]
    # Block-list style:
    #   tags:
    #     - foo
    # is not handled here; we only see flat lines via parse_frontmatter,
    # which means a block list collapses to "" — the audit will flag it.
    return [unquote(raw)] if raw else []


# ---------------------------------------------------------------------------
# Audit rules
# ---------------------------------------------------------------------------

def audit_file(path: Path) -> list[Issue]:
    """Apply every audit rule to a single Markdown file."""
    issues: list[Issue] = []
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        return [Issue(path, f"could not read file: {exc}")]

    fm = parse_frontmatter(text)
    if fm is None:
        return [Issue(path, "missing or invalid YAML frontmatter")]

    for key in REQUIRED_KEYS:
        if key not in fm:
            issues.append(Issue(path, f"missing required key: {key}"))

    doc_type = unquote(fm.get("doc_type", ""))
    if doc_type and doc_type not in ALLOWED_DOC_TYPES:
        issues.append(
            Issue(
                path,
                f"doc_type '{doc_type}' is not one of "
                f"{sorted(ALLOWED_DOC_TYPES)}",
            )
        )

    doc_id = unquote(fm.get("id", ""))
    if doc_id and not _SLUG_RE.match(doc_id):
        issues.append(
            Issue(path, f"id '{doc_id}' is not a URL-safe slug")
        )

    if "tags" in fm:
        tag_list = parse_tags(fm["tags"])
        if not tag_list or tag_list == [""]:
            issues.append(Issue(path, "tags is empty"))

    if "title" in fm and not unquote(fm["title"]):
        issues.append(Issue(path, "title is empty"))

    if "description" in fm and not unquote(fm["description"]):
        issues.append(Issue(path, "description is empty"))

    return issues


# ---------------------------------------------------------------------------
# File discovery
# ---------------------------------------------------------------------------

def collect_markdown(repo_root: Path, ignores: list[str]) -> list[Path]:
    """Find every ``docs/**/*.md`` page that is not in an ignore pattern."""
    docs_root = repo_root / "docs"
    if not docs_root.exists():
        return []

    matches = sorted(p for p in docs_root.rglob("*.md"))
    return [p for p in matches if not _is_ignored(p, repo_root, ignores)]


def _is_ignored(path: Path, repo_root: Path, ignores: list[str]) -> bool:
    """Return True if ``path`` matches any of the configured ignores."""
    rel = path.relative_to(repo_root)
    rel_str = rel.as_posix()
    for pattern in ignores:
        if rel.match(pattern) or _glob_match(rel_str, pattern):
            return True
    return False


def _glob_match(rel_str: str, pattern: str) -> bool:
    """Path-style glob match supporting ``**`` segments.

    ``Path.match`` does not support ``**``, so we approximate by
    converting the pattern to a regex.  Good enough for the small set
    of curated ignore patterns we use.
    """
    regex = re.escape(pattern)
    regex = regex.replace(r"\*\*", ".*").replace(r"\*", "[^/]*")
    return re.fullmatch(regex, rel_str) is not None


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Audit Markdown docs for Diátaxis frontmatter compliance."
    )
    parser.add_argument(
        "--repo-root",
        default=".",
        type=Path,
        help="Repository root (default: current directory)",
    )
    parser.add_argument(
        "--ignore",
        action="append",
        default=[],
        help="Add a path glob to the ignore list (can repeat)",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Treat warnings as errors (no warnings exist today)",
    )
    args = parser.parse_args(argv)

    repo_root = args.repo_root.resolve()
    ignores = list(DEFAULT_IGNORES) + args.ignore
    files = collect_markdown(repo_root, ignores)

    if not files:
        print(f"No Markdown files found under {repo_root}/docs")
        return 0

    all_issues: list[Issue] = []
    for path in files:
        all_issues.extend(audit_file(path))

    if not all_issues:
        print(f"OK: {len(files)} files passed.")
        return 0

    for issue in all_issues:
        print(issue.render(repo_root))
    print(
        f"\nFAILED: {len(all_issues)} issues across "
        f"{len({i.path for i in all_issues})} files "
        f"(checked {len(files)} files)"
    )
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
