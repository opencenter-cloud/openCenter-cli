#!/usr/bin/env python3
"""Convert an Antora AsciiDoc tree into a Diátaxis-shaped Markdown tree.

Reads ``docs/modules/ROOT/pages/**/*.adoc`` and writes
``docs/<category>/<basename>.md`` (or ``docs/<basename>.md`` for the
``index`` and ``glossary`` pages).  Each output file is given a
Docusaurus-compatible YAML frontmatter block built from the AsciiDoc
page attributes (``:description:``, ``:page-type:``, ``:category:``,
``:audience:``, ``:tags:``, ``:id:``).

Why this tool exists:
    The five core openCenter repositories are migrating from
    Antora/AsciiDoc back to Markdown so the docs render natively in
    Docusaurus and so contributors only need to learn one markup.
    ``downdoc`` produces clean Markdown body text but does not emit the
    Diátaxis frontmatter required by the project's Documentation
    steering rule, nor does it rewrite intra-doc ``.adoc`` link
    extensions to ``.md``.  This script wraps ``downdoc`` and fills in
    those gaps idempotently.

Behaviour:
    * Default: skip any output path that already has a hand-curated
      Markdown file.  Pass ``--overwrite`` to replace them.
    * ``nav.adoc`` is skipped — it is an Antora-only construct.
    * The ``:page-type:`` attribute drives the Diátaxis ``doc_type``
      field.  Missing values are inferred from the category directory.
    * Intra-doc links rewrite ``something.adoc`` and ``something#frag``
      to ``something.md`` / ``something.md#frag``.
    * The script never deletes the AsciiDoc tree.  Removal is a separate
      decision for the operator after they confirm the conversion.

Usage::

    python3 hack/scripts/convert_adoc_to_md.py [--repo-root .]
                                               [--dry-run]
                                               [--overwrite]
                                               [--verbose]

Exits non-zero if ``downdoc`` is missing or any file fails to convert.
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

# Directory names recognised as Diátaxis lifecycle categories.  The
# converter uses these to (a) decide where to write the output and (b)
# infer ``doc_type`` when the AsciiDoc page does not specify one.
KNOWN_CATEGORIES = {
    "getting-started",
    "operations",
    "reference",
    "concepts",
    "contributing",
    "providers",
    "release",
    "releases",
}

# Default Diátaxis ``doc_type`` per lifecycle directory.  Pages that
# declare ``:page-type:`` win; this is only the fallback.
CATEGORY_TO_DOC_TYPE = {
    "getting-started": "tutorial",
    "operations": "how-to",
    "reference": "reference",
    "concepts": "explanation",
    "contributing": "explanation",
    "providers": "reference",
    "release": "reference",
    "releases": "reference",
}

# Map AsciiDoc ``:page-type:`` values to Diátaxis ``doc_type`` values.
# The AsciiDoc tree uses ``task`` for both tutorials and how-tos; the
# distinction is made by the lifecycle directory at conversion time.
PAGE_TYPE_TO_DOC_TYPE = {
    "task": "how-to",          # may be re-mapped to ``tutorial`` for getting-started
    "tutorial": "tutorial",
    "concept": "explanation",
    "explanation": "explanation",
    "reference": "reference",
    "how-to": "how-to",
}


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------

@dataclass
class PageMetadata:
    """Parsed AsciiDoc page-level attributes used to build frontmatter.

    All fields default to empty so a page that omits attributes still
    produces valid frontmatter via the inference rules below.
    """

    id: str = ""
    title: str = ""
    description: str = ""
    page_type: str = ""
    category: str = ""
    audience: str = ""
    tags: list[str] = field(default_factory=list)


# ---------------------------------------------------------------------------
# AsciiDoc parsing
# ---------------------------------------------------------------------------

# Match ``:attr-name: value`` lines that appear in the page header.  We
# do not parse the body — the body is delegated to ``downdoc``.
_ATTR_RE = re.compile(r"^:([A-Za-z0-9_-]+):\s*(.*)$")
_TITLE_RE = re.compile(r"^=\s+(.+?)\s*$")


def parse_asciidoc_header(text: str) -> PageMetadata:
    """Extract title and ``:attr:`` values from the AsciiDoc header.

    The header is the run of lines from the document title (``= …``)
    until the first blank line that precedes prose.  Anything after
    that blank line is considered body and ignored.
    """
    meta = PageMetadata()
    in_header = False
    saw_blank_after_title = False

    for raw in text.splitlines():
        line = raw.rstrip()

        if not in_header:
            m = _TITLE_RE.match(line)
            if m:
                meta.title = m.group(1)
                in_header = True
            continue

        # Blank line *immediately after* the attribute block ends the
        # header.  The title-only single blank line is allowed.
        if line == "":
            if saw_blank_after_title:
                break
            saw_blank_after_title = True
            continue

        # Once we've seen prose content, stop scanning attributes.
        m = _ATTR_RE.match(line)
        if not m:
            # Allow purpose line and keep scanning a couple more
            # attribute lines if they appear right after.  Be strict:
            # if the line is not an attribute we are done.
            break

        key, val = m.group(1), m.group(2).strip()
        if key == "id":
            meta.id = val
        elif key == "description":
            meta.description = val
        elif key == "page-type":
            meta.page_type = val
        elif key == "category":
            meta.category = val
        elif key == "audience":
            meta.audience = val
        elif key == "tags":
            meta.tags = _parse_tag_list(val)

    return meta


def _parse_tag_list(raw: str) -> list[str]:
    """Parse ``:tags:`` value (JSON array or comma-separated) into a list.

    AsciiDoc allows ``:tags: ["a", "b"]`` or ``:tags: a, b``.  We accept
    both shapes and strip surrounding whitespace and quoting.
    """
    raw = raw.strip()
    if not raw:
        return []
    if raw.startswith("[") and raw.endswith("]"):
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            parsed = [p.strip().strip("'\"") for p in raw[1:-1].split(",")]
        return [str(p).strip() for p in parsed if str(p).strip()]
    return [p.strip().strip("'\"") for p in raw.split(",") if p.strip()]


# ---------------------------------------------------------------------------
# Frontmatter assembly
# ---------------------------------------------------------------------------

def slugify(value: str) -> str:
    """Lowercase + hyphen-only slug for fallback frontmatter ``id`` values.

    The Documentation steering rule requires URL-safe slugs; this is
    only used when the AsciiDoc page does not declare ``:id:``.
    """
    s = re.sub(r"[^A-Za-z0-9]+", "-", value).strip("-")
    return s.lower() or "doc"


def _yaml_quote(value: str) -> str:
    """Quote a string for YAML frontmatter, escaping embedded quotes."""
    escaped = value.replace("\\", "\\\\").replace("\"", "\\\"")
    return f"\"{escaped}\""


def _yaml_list(values: list[str]) -> str:
    """Render a list of tags as an inline YAML flow sequence."""
    if not values:
        return "[]"
    return "[" + ", ".join(values) + "]"


def derive_doc_type(meta: PageMetadata, category: str) -> str:
    """Pick the Diátaxis ``doc_type`` for a page.

    Priority:
      1. Explicit ``:page-type:`` (with the special case that ``task``
         in a ``getting-started`` directory becomes ``tutorial``).
      2. Default for the category directory.
      3. Final fallback: ``reference`` — safest since it makes no claim
         about narrative style.
    """
    pt = meta.page_type.strip().lower()
    if pt:
        mapped = PAGE_TYPE_TO_DOC_TYPE.get(pt, pt)
        if mapped == "how-to" and category == "getting-started":
            return "tutorial"
        return mapped
    return CATEGORY_TO_DOC_TYPE.get(category, "reference")


def build_frontmatter(meta: PageMetadata, category: str, basename: str) -> str:
    """Build a Docusaurus YAML frontmatter block for an output page.

    Fields populated, in order: ``id``, ``title``, ``sidebar_label``,
    ``description``, ``doc_type``, ``audience``, ``tags``.
    """
    doc_id = meta.id or slugify(basename)
    title = meta.title or basename.replace("-", " ").title()
    sidebar = title if len(title) <= 32 else _short_label(title)
    description = meta.description or f"Documentation for {title}."
    doc_type = derive_doc_type(meta, category)
    audience = meta.audience or "platform engineers"
    # Ensure tags always has at least one entry per the steering rule.
    # Empty category (root-level pages like index/glossary) falls back
    # to a generic "documentation" tag so the audit passes.
    tags = meta.tags or [category or "documentation"]

    lines = [
        "---",
        f"id: {doc_id}",
        f"title: {_yaml_quote(title)}",
        f"sidebar_label: {sidebar}",
        f"description: {description}",
        f"doc_type: {doc_type}",
        f"audience: {_yaml_quote(audience)}",
        f"tags: {_yaml_list(tags)}",
        "---",
        "",
    ]
    return "\n".join(lines)


def _short_label(title: str) -> str:
    """Trim a long title down to ~3 words for the sidebar label."""
    words = title.split()
    return " ".join(words[:3]) if words else title[:24]


# ---------------------------------------------------------------------------
# Body conversion
# ---------------------------------------------------------------------------

# Match Markdown link extensions that came from AsciiDoc xrefs after
# downdoc converts them.  We match the closing of the URL portion so
# anchor fragments survive the rewrite.
_ADOC_LINK_RE = re.compile(r"(\]\([^\)\s]+?)\.adoc(?=[\)\#])")


def rewrite_links(markdown: str) -> str:
    """Rewrite ``foo.adoc`` link targets to ``foo.md``.

    ``downdoc`` leaves the ``.adoc`` extension on Markdown links.  This
    pass restores them to ``.md`` so the converted tree stays
    self-consistent.
    """
    return _ADOC_LINK_RE.sub(r"\1.md", markdown)


def strip_purpose_duplication(body: str) -> str:
    """Drop the duplicate H1 + Purpose block that downdoc emits.

    AsciiDoc pages start with ``= Title`` and a ``*Purpose:* …`` line.
    Once we add a Markdown frontmatter block plus our own H1, downdoc's
    rendering of those header lines becomes redundant.  We keep the
    Purpose paragraph but drop the duplicate H1.
    """
    lines = body.splitlines()
    out: list[str] = []
    dropped_h1 = False
    for line in lines:
        if not dropped_h1 and line.startswith("# "):
            dropped_h1 = True
            continue
        out.append(line)
    return "\n".join(out).lstrip("\n")


# ---------------------------------------------------------------------------
# File-system planning
# ---------------------------------------------------------------------------

def categorise(rel_path: Path) -> tuple[str, str]:
    """Return (category, basename) for a page under ``modules/ROOT/pages``.

    ``index`` and ``glossary`` at the root of ``pages/`` are special:
    they go to ``docs/index.md`` and ``docs/glossary.md`` respectively.
    Other pages must live inside one of ``KNOWN_CATEGORIES``; nested
    sub-directories (``reference/services/foo``) are preserved.
    """
    parts = rel_path.with_suffix("").parts
    if len(parts) == 1:
        return "", parts[0]
    head = parts[0]
    if head not in KNOWN_CATEGORIES:
        # Unknown top-level directory: treat the head as the category
        # name verbatim so we do not silently lose pages.
        return head, "/".join(parts[1:])
    return head, "/".join(parts[1:])


def output_path(repo_root: Path, category: str, basename: str) -> Path:
    """Resolve where the converted Markdown should be written."""
    if category == "":
        return repo_root / "docs" / f"{basename}.md"
    return repo_root / "docs" / category / f"{basename}.md"


# ---------------------------------------------------------------------------
# Conversion driver
# ---------------------------------------------------------------------------

def run_downdoc(adoc_path: Path) -> str:
    """Invoke ``downdoc`` to render the AsciiDoc body to Markdown.

    Output goes to stdout so we can post-process it without touching
    disk.  Errors from downdoc are surfaced as ``RuntimeError``.
    """
    result = subprocess.run(
        ["downdoc", "-o", "-", str(adoc_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"downdoc failed for {adoc_path}: {result.stderr.strip()}"
        )
    return result.stdout


def convert_one(adoc_path: Path, repo_root: Path, *, overwrite: bool, verbose: bool, dry_run: bool) -> str:
    """Convert a single AsciiDoc page.  Returns a one-line status."""
    rel = adoc_path.relative_to(repo_root / "docs" / "modules" / "ROOT" / "pages")
    category, basename = categorise(rel)
    dest = output_path(repo_root, category, basename)

    if basename == "nav":
        return f"skip nav: {rel}"

    if dest.exists() and not overwrite:
        if verbose:
            return f"skip existing: {dest.relative_to(repo_root)}"
        return ""

    raw = adoc_path.read_text(encoding="utf-8")
    meta = parse_asciidoc_header(raw)
    body = run_downdoc(adoc_path)
    body = strip_purpose_duplication(body)
    body = rewrite_links(body)

    frontmatter = build_frontmatter(meta, category, Path(basename).name)
    title = meta.title or Path(basename).name.replace("-", " ").title()
    document = f"{frontmatter}# {title}\n\n{body.lstrip()}"

    if dry_run:
        return f"plan: {dest.relative_to(repo_root)}"

    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(document, encoding="utf-8")
    return f"wrote: {dest.relative_to(repo_root)}"


def collect_pages(repo_root: Path) -> list[Path]:
    """Find every ``.adoc`` file under the Antora pages tree."""
    pages_root = repo_root / "docs" / "modules" / "ROOT" / "pages"
    if not pages_root.exists():
        return []
    return sorted(p for p in pages_root.rglob("*.adoc"))


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Convert an Antora AsciiDoc tree to Diátaxis Markdown."
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
        help="Print the conversion plan without writing files",
    )
    parser.add_argument(
        "--overwrite",
        action="store_true",
        help="Overwrite existing Markdown destinations",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Log every action (including skips)",
    )
    args = parser.parse_args(argv)

    if shutil.which("downdoc") is None:
        print(
            "ERROR: downdoc is not on PATH.  Install with `npm i -g downdoc`.",
            file=sys.stderr,
        )
        return 2

    repo_root = args.repo_root.resolve()
    pages = collect_pages(repo_root)
    if not pages:
        print(f"No AsciiDoc pages under {repo_root}/docs/modules/ROOT/pages")
        return 0

    converted = skipped = errors = 0
    for adoc_path in pages:
        try:
            status = convert_one(
                adoc_path,
                repo_root,
                overwrite=args.overwrite,
                verbose=args.verbose,
                dry_run=args.dry_run,
            )
        except Exception as exc:  # noqa: BLE001 — we want to keep going.
            errors += 1
            print(f"ERROR {adoc_path}: {exc}", file=sys.stderr)
            continue
        if not status:
            skipped += 1
            continue
        if status.startswith("skip"):
            skipped += 1
        else:
            converted += 1
        if args.verbose or not status.startswith("skip"):
            print(status)

    print(
        f"Summary: {converted} converted, {skipped} skipped, {errors} errors"
    )
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
