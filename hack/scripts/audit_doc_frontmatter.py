#!/usr/bin/env python3
"""Audit Markdown docs for Diátaxis frontmatter compliance.

Walks ``docs/**/*.md`` and verifies every page satisfies the project's
Documentation steering rule:

    1. Frontmatter is the very first block in the file.
    2. The required keys are present:
        ``id``, ``title``, ``sidebar_label``, ``description``,
        ``doc_type``, ``audience``, ``tags``, ``last_updated``.
    3. ``doc_type`` is one of ``tutorial``, ``how-to``, ``reference``,
       or ``explanation``.
    4. ``id`` is a URL-safe slug (lowercase, digits, hyphens only).
    5. ``tags`` has at least one entry.
    6. ``last_updated`` is an ISO-8601 calendar date (``YYYY-MM-DD``).

Audit mode does not modify files.  Update mode modifies only the explicitly
supplied paths and exits non-zero on invalid or missing targets.  Audit mode
exits non-zero if any page fails, making it suitable for pre-merge checks.

Usage::

    python3 hack/scripts/audit_doc_frontmatter.py [--repo-root .]
                                                  [--strict]
                                                  [--ignore PATTERN]...
                                                  [--files PATH ...]

    # Update only explicitly supplied maintained pages.  Outside-docs and
    # default-excluded paths are rejected.
    python3 hack/scripts/audit_doc_frontmatter.py \
        --update-last-updated docs/page.md docs/other-page.md
"""

from __future__ import annotations

import argparse
import datetime as _datetime
import re
import subprocess
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
    "last_updated",
)

ALLOWED_DOC_TYPES = {"tutorial", "how-to", "reference", "explanation"}

# Default-ignored paths.  These are component-level configuration files
# that do not represent reader-facing pages and so are exempt from the
# steering rule.
DEFAULT_IGNORES = (
    "docs/CODEMAPS/**",           # Internal architectural maps.
    "docs/superpowers/**",        # Internal automation specs.
    "docs/README.md",             # Repo-navigation README, not a doc page.
)

# Slug must match the steering rule: lowercase + digits + hyphens.
_SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
_DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_YAML_INDICATOR_START = frozenset(",[]{}#&*!|>%@`")


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


def plain_scalar_problem(value: str) -> str | None:
    """Return why a raw value is unsafe in the supported YAML subset."""
    scalar = value.strip()
    if not scalar:
        return None

    if scalar[0] in ("\"", "'"):
        if len(scalar) < 2 or scalar[-1] != scalar[0]:
            return "has an unterminated quoted string"
        return None

    if scalar[0] in "[{":
        expected_end = "]" if scalar[0] == "[" else "}"
        if not scalar.endswith(expected_end):
            return "has an unterminated flow collection"
        return None

    if scalar[0] in _YAML_INDICATOR_START or (
        scalar[0] in "-?:" and (len(scalar) == 1 or scalar[1].isspace())
    ):
        return f"starts with YAML indicator '{scalar[0]}'; quote the value"
    if ": " in scalar or scalar.endswith(":"):
        return "contains ': ', which is invalid in an unquoted YAML value; quote it"
    if " #" in scalar:
        return "contains ' #', which YAML treats as a comment; quote it"
    return None


# ---------------------------------------------------------------------------
# Audit rules
# ---------------------------------------------------------------------------

def split_frontmatter(text: str) -> tuple[str, str] | None:
    """Return ``(frontmatter_block, body)``, or ``None`` without valid block."""
    match = _FRONTMATTER_RE.match(text)
    if not match:
        return None
    # group(1) is the block content; the closing delimiter follows it.
    return match.group(1), text[match.end(1) + 3 :]


def body_last_changed_date(path: Path, repo_root: Path) -> str | None:
    """Return the author date of the newest commit that changed the body.

    "Body" is everything after the closing frontmatter delimiter, so a commit
    that only bumps ``last_updated`` does not move the goalposts and a page
    cannot be made to look fresh by editing the date alone.

    ``git log -p --format=commit:%H %as -- <path>`` is parsed for hunk headers:
    a hunk whose old-line range starts below the frontmatter block touched the
    body, one starting inside the block did not. Returns ``None`` when Git
    cannot answer (no repository, untracked file) or when no commit touched the
    body, so the caller skips the check instead of guessing.
    """
    try:
        relative = path.resolve().relative_to(repo_root.resolve())
    except ValueError:
        return None
    try:
        current = split_frontmatter(path.read_text(encoding="utf-8"))
    except OSError:
        return None
    if current is None:
        return None
    body_start = current[0].count("\n") + 3

    result = subprocess.run(
        (
            "git",
            "log",
            "-p",
            "--no-color",
            "--format=commit:%H %as",
            "--",
            relative.as_posix(),
        ),
        cwd=repo_root,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    if result.returncode != 0:
        return None

    stale_date: str | None = None
    commit_date: str | None = None
    for line in result.stdout.decode("utf-8", "replace").splitlines():
        if line.startswith("commit:"):
            _, _, commit_date = line.partition(" ")
            continue
        if stale_date is not None or not commit_date:
            continue
        header = re.match(r"^@@ -(\d+)(?:,\d+)? \+", line)
        if not header:
            continue
        old_start = int(header.group(1))
        # `-0,0` means an added file: every line, body included, is new.
        if old_start == 0 or old_start > body_start:
            stale_date = commit_date
    return stale_date


def audit_file(path: Path, *, repo_root: Path | None = None) -> list[Issue]:
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

    for key, raw_value in fm.items():
        problem = plain_scalar_problem(raw_value)
        if problem:
            issues.append(Issue(path, f"{key} {problem}"))

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

    last_updated = unquote(fm.get("last_updated", ""))
    if "last_updated" in fm and not is_iso_date(last_updated):
        issues.append(
            Issue(
                path,
                "last_updated must be an ISO-8601 date in YYYY-MM-DD format",
            )
        )

    # A content edit that leaves `last_updated` untouched is invisible in the
    # diff, so the format-only rules above never notice a stale date. When a
    # repository root is supplied, the recorded date must not predate the last
    # commit that changed the page body.
    if repo_root is not None and is_iso_date(last_updated):
        body_date = body_last_changed_date(path, repo_root)
        if body_date is not None and last_updated < body_date:
            issues.append(
                Issue(
                    path,
                    "last_updated is stale: the page body changed on "
                    f"{body_date} but last_updated is {last_updated} "
                    "(run: mise run docs-update-timestamps -- <page>)",
                )
            )

    return issues


def duplicate_sidebar_labels(files: list[Path], repo_root: Path) -> list[Issue]:
    """Report every page whose ``sidebar_label`` is shared by another page.

    The comparison ignores case and surrounding whitespace: a site generator
    renders the label as given, and readers do not treat "Architecture" and
    "architecture" as two navigation entries.
    """
    by_label: dict[str, list[Path]] = {}
    for path in files:
        try:
            frontmatter = parse_frontmatter(path.read_text(encoding="utf-8"))
        except OSError:
            continue
        if not frontmatter:
            continue
        label = frontmatter.get("sidebar_label", "").strip()
        if not label:
            continue
        by_label.setdefault(label.lower(), []).append(path)

    issues: list[Issue] = []
    for label, paths in sorted(by_label.items()):
        if len(paths) < 2:
            continue
        ordered = sorted(paths)
        for path in ordered:
            others = ", ".join(
                str(other.relative_to(repo_root)) for other in ordered if other != path
            )
            issues.append(
                Issue(
                    path,
                    f'sidebar_label "{label}" duplicates another page ({others}); '
                    "navigation cannot distinguish them",
                )
            )
    return issues


def is_iso_date(value: str) -> bool:
    """Return whether *value* is a real ISO-8601 calendar date."""
    if not _DATE_RE.fullmatch(value):
        return False
    try:
        _datetime.date.fromisoformat(value)
    except ValueError:
        return False
    return True


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


def supplied_markdown(
    repo_root: Path,
    paths: list[str],
    ignores: list[str],
    *,
    reject_excluded: bool = False,
) -> tuple[list[Path], list[Issue]]:
    """Resolve explicitly supplied maintained Markdown paths.

    Audit targets outside ``docs/`` or in an ignored path are skipped because
    they are not maintained documentation pages.  Update targets pass
    ``reject_excluded=True`` and receive an error instead; this prevents an
    explicit update from silently succeeding without changing any file.
    """
    repo_root = repo_root.resolve()
    files: list[Path] = []
    issues: list[Issue] = []
    docs_root = (repo_root / "docs").resolve()
    for raw_path in paths:
        path = Path(raw_path)
        if not path.is_absolute():
            path = repo_root / path
        path = path.resolve()
        try:
            path.relative_to(docs_root)
        except ValueError:
            if reject_excluded:
                issues.append(Issue(path, "path is outside the maintained docs directory"))
            continue
        if path.suffix.lower() != ".md":
            issues.append(Issue(path, "path is not a Markdown file"))
            continue
        if _is_ignored(path, repo_root, ignores):
            if reject_excluded:
                issues.append(Issue(path, "path is excluded from maintained docs"))
            continue
        if not path.is_file():
            issues.append(Issue(path, "file does not exist"))
            continue
        files.append(path)
    return sorted(set(files)), issues


def update_last_updated(path: Path, value: str) -> None:
    """Set ``last_updated`` in one explicitly selected Markdown page."""
    if not is_iso_date(value):
        raise ValueError("last_updated must be an ISO-8601 date in YYYY-MM-DD format")
    text = path.read_text(encoding="utf-8")
    match = _FRONTMATTER_RE.match(text)
    if not match:
        raise ValueError("missing or invalid YAML frontmatter")

    block = match.group(1)
    lines = block.splitlines(keepends=True)
    replaced = False
    updated_lines: list[str] = []
    for line in lines:
        line_without_ending = line.rstrip("\r\n")
        kv = _KEY_VALUE_RE.match(line_without_ending)
        if kv and kv.group(1) == "last_updated":
            prefix = line_without_ending[: line_without_ending.find(":") + 1]
            raw_value = kv.group(2).strip()
            spacing_match = re.match(r"^([^:]*:)(\s*)", line_without_ending)
            spacing = spacing_match.group(2) if spacing_match else " "
            # Preserve the page's existing style: an unquoted date stays
            # unquoted. Forcing quotes would churn every page the refresh hook
            # touches and diverge from the tree's dominant bare-date form.
            quote = (
                raw_value[0]
                if len(raw_value) >= 2
                and raw_value[0] == raw_value[-1]
                and raw_value[0] in ("\"", "'")
                else ""
            )
            ending = line[len(line_without_ending) :]
            updated_lines.append(f"{prefix}{spacing}{quote}{value}{quote}{ending}")
            replaced = True
        else:
            updated_lines.append(line)

    if replaced:
        new_block = "".join(updated_lines)
    else:
        new_block = block
        if new_block and not new_block.endswith(("\n", "\r")):
            new_block += "\n"
        new_block += f"last_updated: {value}"

    start, end = match.span(1)
    updated = text[:start] + new_block + text[end:]
    path.write_text(updated, encoding="utf-8")


def update_supplied_paths(
    repo_root: Path, paths: list[str], ignores: list[str], value: str
) -> int:
    """Update only supplied paths, after validating the complete target set."""
    files, issues = supplied_markdown(
        repo_root, paths, ignores, reject_excluded=True
    )
    for issue in issues:
        print(issue.render(repo_root))
    if issues:
        return 1
    if not files:
        print("No valid Markdown update targets supplied.")
        return 1

    pending: list[tuple[Path, str]] = []
    for path in files:
        try:
            text = path.read_text(encoding="utf-8")
            match = _FRONTMATTER_RE.match(text)
            if not match:
                raise ValueError("missing or invalid YAML frontmatter")
            pending.append((path, text))
        except (OSError, ValueError) as exc:
            print(Issue(path, str(exc)).render(repo_root))
    if len(pending) != len(files):
        return 1

    for path, _ in pending:
        try:
            update_last_updated(path, value)
        except OSError as exc:
            print(Issue(path, f"could not update file: {exc}").render(repo_root))
            return 1
    print(f"Updated last_updated in {len(files)} files to {value}.")
    return 0

def refresh_staged_paths(
    repo_root: Path, paths: list[str], ignores: list[str], value: str
) -> int:
    """Refresh timestamps for staged paths, skipping non-documentation inputs.

    Pre-commit supplies every staged file, so paths outside maintained
    documentation are skipped here rather than rejected the way an explicit
    ``--update-last-updated`` target would be.
    """
    files, _ = supplied_markdown(repo_root, paths, ignores)
    if not files:
        print("No staged maintained documentation to refresh.")
        return 0

    pending: list[tuple[Path, str]] = []
    for path in files:
        try:
            text = path.read_text(encoding="utf-8")
            if not _FRONTMATTER_RE.match(text):
                raise ValueError("missing or invalid YAML frontmatter")
            pending.append((path, text))
        except (OSError, ValueError) as exc:
            print(Issue(path, str(exc)).render(repo_root))
    if len(pending) != len(files):
        return 1

    for path, _ in pending:
        try:
            update_last_updated(path, value)
        except OSError as exc:
            print(Issue(path, f"could not update file: {exc}").render(repo_root))
            return 1
    print(f"Refreshed last_updated in {len(files)} staged file(s) to {value}.")
    return 0



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
    parser.add_argument(
        "--files",
        nargs="+",
        action="append",
        metavar="PATH",
        help="Audit only these maintained Markdown paths (may be repeated)",
    )
    parser.add_argument(
        "--update-last-updated",
        action="store_true",
        help="Update timestamps in the explicitly supplied trailing Markdown paths",
    )
    parser.add_argument(
        "--refresh-staged",
        action="store_true",
        help=(
            "Refresh last_updated in the supplied paths that are maintained "
            "documentation, ignoring every other path (pre-commit hook mode)"
        ),
    )
    parser.add_argument(
        "--check-stale-timestamps",
        action="store_true",
        help=(
            "Fail when last_updated predates the newest commit that changed a "
            "page body (needs Git history; CI guard for the pre-commit refresh hook)"
        ),
    )
    parser.add_argument(
        "--date",
        help="Date for --update-last-updated / --refresh-staged (default: current UTC date)",
    )
    parser.add_argument("paths", nargs="*", metavar="PATH")
    args = parser.parse_args(argv)

    repo_root = args.repo_root.resolve()
    ignores = list(DEFAULT_IGNORES) + args.ignore
    timestamp_value = args.date or _datetime.datetime.now(
        _datetime.timezone.utc
    ).date().isoformat()
    if args.update_last_updated and args.refresh_staged:
        parser.error("--update-last-updated and --refresh-staged are mutually exclusive")
    if args.update_last_updated:
        if not args.paths:
            parser.error("--update-last-updated requires at least one Markdown path")
        if not is_iso_date(timestamp_value):
            parser.error("--date must be an ISO-8601 date in YYYY-MM-DD format")
        return update_supplied_paths(repo_root, args.paths, ignores, timestamp_value)
    if args.refresh_staged:
        if not is_iso_date(timestamp_value):
            parser.error("--date must be an ISO-8601 date in YYYY-MM-DD format")
        return refresh_staged_paths(repo_root, args.paths, ignores, timestamp_value)

    if args.paths:
        parser.error("paths require --update-last-updated")

    explicit_issues: list[Issue] = []
    if args.files is not None:
        selected_paths = [path for group in args.files for path in group]
        files, explicit_issues = supplied_markdown(
            repo_root, selected_paths, ignores
        )
    else:
        files = collect_markdown(repo_root, ignores)

    if not files and not explicit_issues:
        print(f"No Markdown files found under {repo_root}/docs")
        return 0

    all_issues: list[Issue] = list(explicit_issues)
    for path in files:
        all_issues.extend(
            audit_file(
                path,
                repo_root=repo_root if args.check_stale_timestamps else None,
            )
        )

    # sidebar_label is what a site generator prints in its navigation. Two pages
    # sharing one label -- as docs/architecture.md and docs/concepts/architecture.md
    # both did, labelled "Architecture" -- are indistinguishable to a reader
    # choosing between them, and per-file checks cannot see the collision.
    all_issues.extend(duplicate_sidebar_labels(files, repo_root))

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
