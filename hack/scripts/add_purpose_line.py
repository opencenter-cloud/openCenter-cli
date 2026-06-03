#!/usr/bin/env python3
"""Insert a ``**Purpose:**`` line on Markdown pages that are missing one.

The Documentation steering rule (§2) requires every doc page to start
with::

    **Purpose:** For <audience>, explains/shows <what>, covering <scope>.

Most curated pages already have one.  This script fills in the gap on
pages where the line was lost during the AsciiDoc-to-Markdown migration
or where a contributor added a new page without it.

Behaviour:
    1. Skip ignored trees (CODEMAPS, superpowers, docs/README.md).
    2. Skip auto-generated Cobra reference pages: any file under
       ``docs/reference/opencenter/`` whose title starts with
       ``Opencenter_`` is regenerated from the binary, so we leave it
       alone — the better fix is to update the doc generator.
    3. For everything else, derive the Purpose line from frontmatter
       (``audience``, ``description``) and insert it after the H1.

The derived sentence intentionally stays close to the existing
``description`` field so the page does not gain new claims it cannot
back up with evidence (steering rule §5).

Usage::

    python3 hack/scripts/add_purpose_line.py [--repo-root .]
                                              [--dry-run]
                                              [--verbose]
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

DEFAULT_IGNORES = (
    "docs/CODEMAPS",
    "docs/superpowers",
)

# Pages whose H1 starts with one of these prefixes are treated as
# auto-generated Cobra command references and skipped — the source of
# truth is the Cobra command metadata, not the Markdown.
AUTOGEN_TITLE_PREFIXES = (
    "Opencenter_",
    "opencenter ",  # rendered by older generator versions
)

_FRONTMATTER_RE = re.compile(r"^---\n(.*?)\n---\n", re.DOTALL)
_KV_RE = re.compile(r"^([A-Za-z_][\w-]*):\s*(.*?)\s*$")
_PURPOSE_RE = re.compile(
    r"\*\*Purpose:?\*\*|^>\s*\*\*Purpose", re.IGNORECASE | re.MULTILINE
)


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------

@dataclass
class PageInsert:
    """A planned insertion of a Purpose line into a single page."""

    path: Path
    purpose_line: str


# ---------------------------------------------------------------------------
# Frontmatter parsing
# ---------------------------------------------------------------------------

def parse_frontmatter(text: str) -> tuple[dict[str, str], int] | None:
    """Return (mapping, end-of-frontmatter-index) or ``None``."""
    match = _FRONTMATTER_RE.match(text)
    if not match:
        return None
    mapping: dict[str, str] = {}
    for line in match.group(1).splitlines():
        kv = _KV_RE.match(line)
        if kv:
            mapping[kv.group(1)] = kv.group(2)
    return mapping, match.end()


def unquote(value: str) -> str:
    """Strip matching surrounding quotes from a frontmatter value."""
    v = value.strip()
    if len(v) >= 2 and v[0] == v[-1] and v[0] in ("\"", "'"):
        return v[1:-1]
    return v


# ---------------------------------------------------------------------------
# Heuristic verb selection
# ---------------------------------------------------------------------------

# Purpose-line opening verb depends on the Diátaxis ``doc_type``.  This
# matches the "explains/shows" guidance in the steering rule.
_VERB_BY_TYPE = {
    "tutorial": "shows how to",
    "how-to": "shows how to",
    "reference": "documents",
    "explanation": "explains",
}


def derive_purpose(meta: dict[str, str]) -> str:
    """Build a ``**Purpose:**`` line from the page frontmatter.

    The shape is::

        **Purpose:** For <audience>, <verb> <description>.

    We deliberately base the second clause on the ``description`` field
    so we do not invent any new claims about the page's content.
    """
    audience = unquote(meta.get("audience", "")) or "all readers"
    description = unquote(meta.get("description", "")).rstrip(".")
    doc_type = unquote(meta.get("doc_type", "reference")).lower()
    verb = _VERB_BY_TYPE.get(doc_type, "documents")

    if not description:
        return f"**Purpose:** For {audience}, {verb} the topic of this page."

    # Lower-case the leading word of the description if it is a noun
    # phrase ("How to …", "The …") so the sentence flows.  Keep the
    # original capitalisation for explicit proper nouns.
    first, _, rest = description.partition(" ")
    if first.istitle() and not first.isupper():
        first = first[0].lower() + first[1:]
    body = first + (" " + rest if rest else "")

    # Avoid stutter where the verb redundantly restates the
    # description's leading noun phrase.
    body = _strip_stutter(verb, body)

    return f"**Purpose:** For {audience}, {verb} {body}."


# Stutter-removal helpers.  Each tuple is (verb-suffix, prefix to strip).
# When the body starts with the prefix, we drop it because the verb
# already covers the same intent.
_STUTTER_RULES = (
    ("how to", "how to "),
    ("how to", "complete guide for "),
    ("how to", "complete guide to "),
    ("how to", "guide for "),
    ("how to", "guide to "),
    ("how to", "overview of "),
    ("how to", "quick reference for "),
    ("explains", "explains "),
    ("explains", "explanation of "),
    ("explains", "developer notes for "),
    ("explains", "notes for "),
    ("documents", "documentation for "),
    ("documents", "reference for "),
)


def _strip_stutter(verb: str, body: str) -> str:
    """Drop a leading noun phrase from ``body`` that duplicates ``verb``."""
    body_lower = body.lower()
    for verb_suffix, prefix in _STUTTER_RULES:
        if verb.endswith(verb_suffix) and body_lower.startswith(prefix):
            return body[len(prefix):]
    return body


# ---------------------------------------------------------------------------
# Body inspection helpers
# ---------------------------------------------------------------------------

def title_after_frontmatter(body: str) -> str:
    """Return the page H1 stripped of its leading ``# ``."""
    for line in body.splitlines():
        s = line.strip()
        if not s:
            continue
        if s.startswith("# "):
            return s[2:].strip()
        return ""
    return ""


def is_autogenerated(body: str) -> bool:
    """True if the H1 looks like an auto-generated Cobra reference."""
    title = title_after_frontmatter(body)
    return any(title.startswith(prefix) for prefix in AUTOGEN_TITLE_PREFIXES)


def has_purpose_line(body: str) -> bool:
    """True if the page already has a Purpose paragraph near the top."""
    snippet = "\n".join(body.splitlines()[:20])
    return bool(_PURPOSE_RE.search(snippet))


# ---------------------------------------------------------------------------
# Insert logic
# ---------------------------------------------------------------------------

def insert_purpose(text: str, purpose_line: str, title: str = "") -> str:
    """Insert the Purpose line on the first blank line after the H1.

    If the page does not have an H1 (some legacy how-to pages omit it),
    we synthesise one from ``title`` so the inserted Purpose paragraph
    has a heading to attach to.  If ``title`` is empty, the Purpose
    paragraph is simply prepended after the frontmatter.
    """
    fm = parse_frontmatter(text)
    if fm is None:
        return text
    _meta, fm_end = fm

    head = text[:fm_end]
    body = text[fm_end:]

    lines = body.splitlines(keepends=True)

    # If no H1 in the first 25 lines, prepend one (or the Purpose
    # alone) at the top of the body.
    h1_index = -1
    for i, line in enumerate(lines[:25]):
        if line.lstrip().startswith("# "):
            h1_index = i
            break

    if h1_index == -1:
        prefix_lines: list[str] = []
        # Strip leading blank lines we are about to displace.
        while lines and lines[0].strip() == "":
            lines.pop(0)
        if title:
            prefix_lines.append(f"# {title}\n\n")
        prefix_lines.append(f"{purpose_line}\n\n")
        return head + "".join(prefix_lines) + "".join(lines)

    # H1 exists.  Walk past it and the following blank line, then
    # insert the Purpose paragraph.
    out: list[str] = []
    inserted = False
    for i, line in enumerate(lines):
        out.append(line)
        if not inserted and i == h1_index:
            # Look ahead for the following blank line.
            if i + 1 < len(lines) and lines[i + 1].strip() == "":
                out.append(lines[i + 1])
                # Skip the blank we just appended on the next iteration.
                lines[i + 1] = ""  # ignore on next round
            out.append(f"{purpose_line}\n\n")
            inserted = True

    return head + "".join(out)


# ---------------------------------------------------------------------------
# File-system traversal
# ---------------------------------------------------------------------------

def collect_targets(repo_root: Path) -> list[Path]:
    """Return curated Markdown pages that may need a Purpose line."""
    docs = repo_root / "docs"
    if not docs.is_dir():
        return []
    targets: list[Path] = []
    for md in sorted(docs.rglob("*.md")):
        rel = md.relative_to(repo_root).as_posix()
        if any(rel.startswith(prefix + "/") for prefix in DEFAULT_IGNORES):
            continue
        if rel == "docs/README.md":
            continue
        targets.append(md)
    return targets


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Insert **Purpose:** lines on pages that are missing one."
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
        help="Print the plan without writing files",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Print each generated Purpose line",
    )
    args = parser.parse_args(argv)

    repo_root = args.repo_root.resolve()
    targets = collect_targets(repo_root)
    if not targets:
        print(f"No Markdown files under {repo_root}/docs")
        return 0

    inserts: list[PageInsert] = []
    skipped_autogen = 0
    skipped_already = 0

    for path in targets:
        text = path.read_text(encoding="utf-8")
        fm = parse_frontmatter(text)
        if fm is None:
            continue
        meta, fm_end = fm
        body = text[fm_end:]

        if is_autogenerated(body):
            skipped_autogen += 1
            continue
        if has_purpose_line(body):
            skipped_already += 1
            continue

        purpose_line = derive_purpose(meta)
        inserts.append(PageInsert(path, purpose_line))

    if args.verbose or args.dry_run:
        for insert in inserts:
            rel = insert.path.relative_to(repo_root)
            print(f"{rel}\n  {insert.purpose_line}")

    if not args.dry_run:
        for insert in inserts:
            text = insert.path.read_text(encoding="utf-8")
            fm = parse_frontmatter(text)
            title = ""
            if fm is not None:
                title = unquote(fm[0].get("title", ""))
            new_text = insert_purpose(text, insert.purpose_line, title=title)
            insert.path.write_text(new_text, encoding="utf-8")

    print(
        f"\nSummary: {len(inserts)} pages updated, "
        f"{skipped_already} already had a Purpose line, "
        f"{skipped_autogen} auto-generated pages skipped."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
