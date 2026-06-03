#!/usr/bin/env python3
"""Repair the well-defined staleness patterns left over from the
AsciiDoc-to-Markdown migration.

The previous migration pipeline (``downdoc`` + ``convert_adoc_to_md``)
produced syntactically valid Markdown but did not normalise three
conversion artefacts:

1.  **Pages-rooted links.**  Antora ``xref:`` targets are resolved
    relative to ``docs/modules/ROOT/pages``.  ``downdoc`` translates
    them as Markdown links with the same target string, so a sibling
    reference inside ``docs/contributing/contributing.md`` becomes
    ``[Setup](contributing/development-setup.md)``.  The Markdown
    resolver expands that to ``docs/contributing/contributing/development-setup.md``,
    which does not exist.

2.  **Old Diátaxis-typed lifecycle directories.**  Pre-migration docs
    used ``tutorials/``, ``how-to/``, and ``explanation/``.  The
    repository now uses ``getting-started/``, ``operations/``, and
    ``concepts/`` (Documentation steering rule §3).  Some bodies still
    point at the old names.

3.  **Residual references to the Antora tree.**  A few sentences and
    list items still mention ``docs/modules/ROOT/pages``, ``.adoc``
    extensions, or the Antora build flow.

This script fixes all three categories deterministically.  It also
emits a report of any link that still does not resolve after the
rewrites — those need a human's eyes.

The script is idempotent: running it twice on a clean tree is a no-op.

Usage::

    python3 hack/scripts/refresh_docs.py [--repo-root .]
                                         [--dry-run]
                                         [--verbose]

Exits 0 if every fix could be applied and every link resolves; exits
non-zero otherwise.
"""

from __future__ import annotations

import argparse
import re
import sys
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

# Mapping from the obsolete Diátaxis-type directory names to the
# lifecycle-category directory names defined in the Documentation
# steering rule.  Apply to both link targets and prose mentions.
LEGACY_DIR_RENAMES = {
    "tutorials": "getting-started",
    "how-to": "operations",
    "explanation": "concepts",
}

# Filenames or path fragments that come from the old layout but still
# refer to a real page in the new layout.  These are full-path renames
# rather than directory renames.
LEGACY_FILE_RENAMES = {
    # Old monolithic tutorial → new entry-point page.
    "docs/tutorials/getting-started.md": "docs/getting-started/getting-started.md",
}

# Paths excluded from rewriting.  These directories hold internal
# planning specs and architectural maps that are not reader-facing
# Diátaxis pages and may legitimately still mention the old layout.
DEFAULT_IGNORES = (
    "docs/CODEMAPS",
    "docs/superpowers",
)

# External link prefixes that we leave alone.
_EXTERNAL_PREFIXES = ("http://", "https://", "mailto:", "ssh://", "git@")


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------

@dataclass
class FileEdit:
    """One rewrite applied to one file by the refresh pass."""

    path: Path
    rule: str
    old: str
    new: str


@dataclass
class BrokenLink:
    """A Markdown link that still does not resolve after the refresh."""

    source: Path
    target: str


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_MD_LINK_RE = re.compile(r"\]\(([^)#\s]+?)(#[^)]*)?\)")


def is_external(target: str) -> bool:
    """True if a link target points outside the repository."""
    return target.startswith(_EXTERNAL_PREFIXES) or target.startswith("/")


def strip_prefix(path: str, prefix: str) -> str:
    """Drop a leading ``prefix`` from ``path`` if present."""
    return path[len(prefix):] if path.startswith(prefix) else path


def collect_markdown(repo_root: Path) -> list[Path]:
    """Return every Markdown file under ``docs/`` outside the ignore set."""
    docs = repo_root / "docs"
    if not docs.is_dir():
        return []
    paths: list[Path] = []
    for md in sorted(docs.rglob("*.md")):
        rel = md.relative_to(repo_root).as_posix()
        if any(rel.startswith(ignore + "/") for ignore in DEFAULT_IGNORES):
            continue
        paths.append(md)
    return paths


# ---------------------------------------------------------------------------
# Rewrite rules
# ---------------------------------------------------------------------------

def rewrite_legacy_dirs(text: str) -> tuple[str, list[tuple[str, str, str]]]:
    """Rewrite obsolete Diátaxis-type dir names everywhere in the file.

    We rewrite both inside Markdown link targets and in inline code
    spans / prose, so backticked references like ``docs/tutorials/`` are
    fixed too.  Returns the new text and the list of (rule, old, new)
    triples for reporting.
    """
    edits: list[tuple[str, str, str]] = []
    new_text = text
    for old_dir, new_dir in LEGACY_DIR_RENAMES.items():
        # Rewrite ``docs/<old>/`` and ``<old>/`` segments after a
        # path separator or word boundary so we do not match things
        # like ``how-to-deploy``.  We keep it conservative by anchoring
        # on either start-of-line, ``/``, ``.``, or whitespace.
        pattern = re.compile(
            r"(?P<lead>(?:^|[\s/(`'\".])docs/)" + re.escape(old_dir) + r"(?=/)"
        )
        new_text, n1 = pattern.subn(rf"\g<lead>{new_dir}", new_text)
        if n1:
            edits.append(("legacy-dir-rename", f"docs/{old_dir}/", f"docs/{new_dir}/"))

        # Also rewrite bare relative paths (``../<old>/`` and ``./<old>/``).
        pattern_rel = re.compile(
            r"(?P<lead>(?:\.\./|\./))" + re.escape(old_dir) + r"(?=/)"
        )
        new_text, n2 = pattern_rel.subn(rf"\g<lead>{new_dir}", new_text)
        if n2:
            edits.append(
                (
                    "legacy-dir-rename-relative",
                    f"./{old_dir}/ or ../{old_dir}/",
                    f"./{new_dir}/ or ../{new_dir}/",
                )
            )
    return new_text, edits


def rewrite_legacy_files(text: str) -> tuple[str, list[tuple[str, str, str]]]:
    """Rewrite full-path renames listed in ``LEGACY_FILE_RENAMES``."""
    edits: list[tuple[str, str, str]] = []
    new_text = text
    for old_path, new_path in LEGACY_FILE_RENAMES.items():
        if old_path in new_text:
            new_text = new_text.replace(old_path, new_path)
            edits.append(("legacy-file-rename", old_path, new_path))
    return new_text, edits


def rewrite_pages_rooted_links(text: str, source: Path, repo_root: Path) -> tuple[str, list[tuple[str, str, str]]]:
    """Convert pages-rooted link targets into source-relative targets.

    For each Markdown link target that is *not* external and does not
    resolve as-is, try treating it as a path rooted at ``docs/``.  If
    that resolves to an existing file, rewrite the link to the
    correct relative path.

    This is the single mechanical transformation that fixes the bulk
    of the broken-link reports from the AsciiDoc conversion.
    """
    docs = repo_root / "docs"
    edits: list[tuple[str, str, str]] = []

    def repl(match: re.Match[str]) -> str:
        target = match.group(1)
        anchor = match.group(2) or ""
        if is_external(target):
            return match.group(0)
        candidate = (source.parent / target).resolve()
        if candidate.exists():
            return match.group(0)
        rooted = (docs / target).resolve()
        if not rooted.exists():
            return match.group(0)
        try:
            rel = rooted.relative_to(source.parent.resolve())
            new_target = rel.as_posix()
        except ValueError:
            new_target = "../" + str(rooted.relative_to(docs).as_posix())
            # Walk up the directory tree to compose a valid relative path.
            depth = len(source.parent.resolve().relative_to(docs).parts)
            if depth:
                new_target = "../" * depth + rooted.relative_to(docs).as_posix()
            else:
                new_target = rooted.relative_to(docs).as_posix()
        edits.append(("pages-rooted-link", target, new_target))
        return f"]({new_target}{anchor})"

    new_text = _MD_LINK_RE.sub(repl, text)
    return new_text, edits


def rewrite_antora_residue(text: str) -> tuple[str, list[tuple[str, str, str]]]:
    """Drop residual references to the Antora tree from the prose.

    The migration commit removed ``docs/modules/ROOT`` and the
    Antora playbooks, but a handful of how-to instructions still
    mention them.  We rewrite the most common shapes to point at the
    Markdown layout instead.
    """
    edits: list[tuple[str, str, str]] = []
    new_text = text

    replacements = [
        # Cobra docs target moved to lifecycle layout.
        (
            r"docs/modules/ROOT/pages/reference/opencenter/",
            "docs/reference/opencenter/",
            "antora-path-cobra",
        ),
        # Cobra docs target written as a parent-path reference.
        (
            r"modules/ROOT/pages/reference/opencenter/",
            "reference/opencenter/",
            "antora-path-cobra-rel",
        ),
        # Generic Antora pages prefix (anything else).
        (
            r"docs/modules/ROOT/pages/",
            "docs/",
            "antora-path-pages-prefix",
        ),
    ]
    for pattern, repl, rule in replacements:
        compiled = re.compile(pattern)
        new_text, n = compiled.subn(repl, new_text)
        if n:
            edits.append((rule, pattern, repl))
    return new_text, edits


# ---------------------------------------------------------------------------
# Pass driver
# ---------------------------------------------------------------------------

def refresh_file(path: Path, repo_root: Path) -> tuple[bool, list[FileEdit]]:
    """Apply every refresh rule to one file.

    Returns (changed, edits).  ``edits`` is a list of every individual
    rewrite that fired, useful for the audit report.
    """
    text = path.read_text(encoding="utf-8")
    original = text
    all_edits: list[FileEdit] = []

    for rule_fn in (
        rewrite_legacy_files,
        rewrite_legacy_dirs,
        rewrite_antora_residue,
    ):
        text, triples = rule_fn(text)
        for rule, old, new in triples:
            all_edits.append(FileEdit(path, rule, old, new))

    text, triples = rewrite_pages_rooted_links(text, path, repo_root)
    for rule, old, new in triples:
        all_edits.append(FileEdit(path, rule, old, new))

    return text != original, [
        FileEdit(path, e.rule, e.old, e.new) for e in all_edits
    ] if text != original else []


def find_broken_links(repo_root: Path) -> list[BrokenLink]:
    """Re-scan the tree for links that still do not resolve.

    Reports each broken target so a human can decide whether to fix
    the doc, create the missing target, or remove the reference.
    """
    docs = repo_root / "docs"
    if not docs.is_dir():
        return []
    broken: list[BrokenLink] = []
    for md in sorted(docs.rglob("*.md")):
        rel = md.relative_to(repo_root).as_posix()
        if any(rel.startswith(ignore + "/") for ignore in DEFAULT_IGNORES):
            continue
        text = md.read_text(encoding="utf-8")
        for match in _MD_LINK_RE.finditer(text):
            target = match.group(1).strip()
            if is_external(target):
                continue
            candidate = (md.parent / target).resolve()
            if candidate.exists():
                continue
            broken.append(BrokenLink(md, target))
    return broken


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Refresh Markdown documentation after the AsciiDoc migration."
    )
    parser.add_argument(
        "--repo-root",
        default=".",
        type=Path,
        help="Repository root (default: current directory)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would change without writing files",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Print every individual rewrite",
    )
    args = parser.parse_args(argv)

    repo_root = args.repo_root.resolve()
    files = collect_markdown(repo_root)
    if not files:
        print(f"No Markdown files under {repo_root}/docs")
        return 0

    changed_files: list[Path] = []
    rule_counts: dict[str, int] = defaultdict(int)
    edits_by_file: dict[Path, list[FileEdit]] = defaultdict(list)

    for path in files:
        changed, edits = refresh_file(path, repo_root)
        if not changed:
            continue
        changed_files.append(path)
        for edit in edits:
            rule_counts[edit.rule] += 1
            edits_by_file[path].append(edit)
        if not args.dry_run:
            new_text = path.read_text(encoding="utf-8")  # re-read for write
            new_text, _ = rewrite_legacy_files(new_text)
            new_text, _ = rewrite_legacy_dirs(new_text)
            new_text, _ = rewrite_antora_residue(new_text)
            new_text, _ = rewrite_pages_rooted_links(new_text, path, repo_root)
            path.write_text(new_text, encoding="utf-8")

    if args.verbose:
        for path, edits in edits_by_file.items():
            print(f"\n{path.relative_to(repo_root)}")
            for edit in edits:
                print(f"  [{edit.rule}] {edit.old} -> {edit.new}")

    print(f"\nFiles changed: {len(changed_files)}")
    for rule, count in sorted(rule_counts.items()):
        print(f"  {rule}: {count}")

    if args.dry_run:
        print("\n(dry-run, no files written)")
        return 0

    broken = find_broken_links(repo_root)
    if broken:
        print(f"\nResidual broken links ({len(broken)}):")
        for issue in broken[:50]:
            print(f"  {issue.source.relative_to(repo_root)} -> {issue.target}")
        if len(broken) > 50:
            print(f"  ... and {len(broken) - 50} more")
        return 1

    print("\nAll intra-doc links resolve.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
