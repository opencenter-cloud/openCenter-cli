#!/usr/bin/env python3
"""Focused tests for check_doc_links.py."""

from __future__ import annotations

from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))

from check_doc_links import collect_markdown, reachability, validate_file


class DocumentationLinkTests(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        (self.root / "docs").mkdir()
        (self.root / "docs" / "guide.md").write_text("# Guide\n", encoding="utf-8")

    def tearDown(self) -> None:
        self.directory.cleanup()

    def write(self, name: str, text: str) -> Path:
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def test_existing_local_target_passes_and_external_and_anchor_are_skipped(self) -> None:
        page = self.write(
            "docs/page.md",
            "[guide](guide.md#intro) [anchor](#intro) [web](https://example.test/nope)\n",
        )
        self.assertEqual(validate_file(page, self.root), [])

    def test_missing_target_has_source_line_number(self) -> None:
        page = self.write("docs/page.md", "Intro\n\n[missing](missing.md)\n")
        issues = validate_file(page, self.root)
        self.assertEqual(len(issues), 1)
        self.assertEqual(issues[0].line, 3)
        self.assertIn("missing.md", issues[0].message)

    def test_reference_definitions_and_code_fences_are_handled(self) -> None:
        page = self.write(
            "docs/page.md",
            "[guide][g]\n\n[g]: guide.md\n\n```text\n[not-a-link](missing.md)\n```\n",
        )
        self.assertEqual(validate_file(page, self.root), [])

    def test_target_cannot_escape_repository(self) -> None:
        page = self.write("docs/page.md", "[outside](../../outside.md)\n")
        issues = validate_file(page, self.root)
        self.assertEqual(len(issues), 1)
        self.assertIn("outside repository", issues[0].message)


class PortabilityTests(unittest.TestCase):
    """The published-tree checks: --root scoping, exclusion, reachability."""

    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)

    def tearDown(self) -> None:
        self.directory.cleanup()

    def write(self, name: str, text: str) -> Path:
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def docs(self) -> Path:
        """The documentation root, created on first use."""
        root = self.root / "docs"
        root.mkdir(parents=True, exist_ok=True)
        return root

    def write_docs(self, name: str, text: str) -> Path:
        """Write a page under the documentation root, creating parent dirs."""
        path = self.docs() / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def test_root_mode_rejects_links_escaping_the_documentation_root(self) -> None:
        """The core portability rule: a link above docs/ breaks once copied out.

        This is the defect the old checker could not see -- resolving against the
        repository root made ../README.md look valid while the tree was in fact
        unshippable. The target must exist at repo root so the failure is about
        scope, not about a missing file.
        """
        self.write("README.md", "# Readme\n")
        self.write_docs("guide.md", "# Guide\n")
        page = self.write_docs("page.md", "[guide](guide.md)\n[readme](../README.md)\n")

        # Against the repository root the upward link is fine.
        self.assertEqual(validate_file(page, self.root), [])
        # Against docs/ as the root it is not.
        issues = validate_file(page, self.docs())
        self.assertEqual(len(issues), 1)
        self.assertIn("outside repository", issues[0].message)

    def test_exclude_drops_matching_pages_from_collection(self) -> None:
        docs = self.docs()
        self.write_docs("page.md", "# Page\n")
        self.write_docs("CODEMAPS/map.md", "# Map\n")

        collected = {p.relative_to(docs).as_posix() for p in collect_markdown(docs)}
        self.assertEqual(collected, {"page.md", "CODEMAPS/map.md"})

        excluded = {
            p.relative_to(docs).as_posix()
            for p in collect_markdown(docs, ("CODEMAPS/**",))
        }
        self.assertEqual(excluded, {"page.md"})

    def test_reachability_reports_pages_with_no_inbound_path(self) -> None:
        docs = self.docs()
        self.write_docs("index.md", "[a](a.md)\n")
        self.write_docs("a.md", "[b](b.md)\n")
        self.write_docs("b.md", "# B\n")
        self.write_docs("orphan.md", "# Orphan\n")

        issues = reachability(docs, "index.md", ())
        self.assertEqual(len(issues), 1)
        self.assertIn("orphan.md", str(issues[0].path))
        self.assertIn("not reachable", issues[0].message)

    def test_reachability_follows_transitive_links(self) -> None:
        """A page reachable only through another page is not an orphan."""
        docs = self.docs()
        self.write_docs("index.md", "[a](a.md)\n")
        self.write_docs("a.md", "[b](sub/b.md)\n")
        self.write_docs("sub/b.md", "# B\n")

        self.assertEqual(reachability(docs, "index.md", ()), [])

    def test_reachability_ignores_excluded_pages(self) -> None:
        docs = self.docs()
        self.write_docs("index.md", "# Index\n")
        self.write_docs("CODEMAPS/map.md", "# Map\n")

        self.assertEqual(len(reachability(docs, "index.md", ())), 1)
        self.assertEqual(reachability(docs, "index.md", ("CODEMAPS/**",)), [])

    def test_reachability_fails_loudly_on_a_missing_entrypoint(self) -> None:
        """A wrong --entrypoint must not silently pass every page as reachable."""
        docs = self.docs()
        self.write_docs("page.md", "# Page\n")

        issues = reachability(docs, "index.md", ())
        self.assertEqual(len(issues), 1)
        self.assertIn("entrypoint", issues[0].message)


if __name__ == "__main__":
    unittest.main()
