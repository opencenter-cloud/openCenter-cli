"""Focused tests for audit_doc_frontmatter.py."""

from __future__ import annotations

import contextlib
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))

from audit_doc_frontmatter import audit_file, body_last_changed_date, main


class FrontmatterYamlSafetyTests(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.path = Path(self.directory.name) / "page.md"

    def tearDown(self) -> None:
        self.directory.cleanup()

    def audit(self, sidebar_label: str, tags: str = "[services, templates]") -> list[str]:
        self.path.write_text(
            "\n".join(
                (
                    "---",
                    "last_updated: 2026-09-24",
                    "id: services-templates",
                    'title: "Service Templates: How They Work"',
                    f"sidebar_label: {sidebar_label}",
                    "description: Explains how service templates work.",
                    "doc_type: explanation",
                    'audience: "platform engineers, operators"',
                    f"tags: {tags}",
                    "---",
                    "# Service Templates",
                    "",
                )
            ),
            encoding="utf-8",
        )
        return [issue.message for issue in audit_file(self.path)]

    def test_unquoted_mapping_separator_is_rejected(self) -> None:
        issues = self.audit("Service Templates: How They Work")
        self.assertTrue(any("contains ': '" in issue for issue in issues))

    def test_quoted_mapping_separator_passes(self) -> None:
        self.assertEqual(self.audit('"Service Templates: How They Work"'), [])

    def test_unquoted_comment_marker_is_rejected(self) -> None:
        issues = self.audit("Service Templates # overview")
        self.assertTrue(any("YAML treats as a comment" in issue for issue in issues))

    def test_inline_tags_sequence_passes(self) -> None:
        self.assertEqual(self.audit("Service Templates", "[services, templates]"), [])

    def test_unterminated_quote_is_rejected(self) -> None:
        issues = self.audit('"Service Templates')
        self.assertTrue(any("unterminated quoted string" in issue for issue in issues))


class GitRepo:
    """Throwaway repository with a deterministic identity and commit date."""

    def __init__(self, root: Path) -> None:
        self.root = root

    def git(self, *args: str, date: str = "2026-01-05T12:00:00+00:00") -> str:
        return subprocess.run(
            ("git", *args),
            cwd=self.root,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            env={
                **os.environ,
                "GIT_AUTHOR_NAME": "Test",
                "GIT_AUTHOR_EMAIL": "t@example.com",
                "GIT_COMMITTER_NAME": "Test",
                "GIT_COMMITTER_EMAIL": "t@example.com",
                "GIT_AUTHOR_DATE": date,
                "GIT_COMMITTER_DATE": date,
            },
        ).stdout.decode()


def page(body: str, stamp: str = "2026-01-01") -> str:
    return "\n".join(
        (
            "---",
            f"last_updated: {stamp}",
            "id: page",
            'title: "Page"',
            "sidebar_label: Page",
            "description: A page.",
            "doc_type: how-to",
            "audience: users",
            "tags: [test]",
            "---",
            body,
        )
    )


class BodyChangeDateTests(unittest.TestCase):
    """`last_updated` must track body edits and ignore date-only commits."""

    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.repo = GitRepo(self.root)
        self.repo.git("init", "-q", "--initial-branch=main")
        self.docs = self.root / "docs"
        self.docs.mkdir()
        self.path = self.docs / "page.md"

    def tearDown(self) -> None:
        self.directory.cleanup()

    def commit(self, body: str, *, date: str = "2026-01-05T12:00:00+00:00") -> None:
        self.path.write_text(page(body), encoding="utf-8")
        self.repo.git("add", "docs/page.md")
        self.repo.git("commit", "-q", "-m", "change", date=date)

    def stale(self) -> list[str]:
        return [issue.message for issue in audit_file(self.path, repo_root=self.root)]

    def test_untracked_page_is_not_judged(self) -> None:
        self.path.write_text(page("# Page"), encoding="utf-8")
        self.assertIsNone(body_last_changed_date(self.path, self.root))
        self.assertEqual(self.stale(), [])

    def test_body_edit_makes_recorded_date_stale(self) -> None:
        self.commit("# Page")
        self.assertEqual(body_last_changed_date(self.path, self.root), "2026-01-05")
        self.assertTrue(
            any("last_updated is stale" in message for message in self.stale())
        )

    def test_matching_date_passes(self) -> None:
        self.commit("# Page")
        self.path.write_text(page("# Page", stamp="2026-01-05"), encoding="utf-8")
        self.repo.git("add", "docs/page.md")
        self.repo.git("commit", "-q", "-m", "bump")
        self.assertEqual(self.stale(), [])

    def test_later_frontmatter_only_commit_is_not_a_body_change(self) -> None:
        self.commit("# Page")
        # A commit dated after the recorded stamp that touches only the
        # frontmatter must not move the goalposts, otherwise bumping the date
        # would manufacture freshness.
        self.path.write_text(page("# Page", stamp="2026-01-05"), encoding="utf-8")
        self.repo.git("add", "docs/page.md")
        self.repo.git(
            "commit", "-q", "-m", "bump", date="2026-03-09T12:00:00+00:00"
        )
        self.assertEqual(body_last_changed_date(self.path, self.root), "2026-01-05")
        self.assertEqual(self.stale(), [])

    def test_staleness_is_opt_in(self) -> None:
        self.commit("# Page")
        self.assertEqual([issue.message for issue in audit_file(self.path)], [])


class RefreshStagedTests(unittest.TestCase):
    """The pre-commit refresh mode rewrites docs and ignores everything else."""

    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.docs = self.root / "docs"
        self.docs.mkdir()
        self.path = self.docs / "page.md"
        self.path.write_text(page("# Page"), encoding="utf-8")

    def tearDown(self) -> None:
        self.directory.cleanup()

    def refresh(self, *paths: str) -> int:
        return main(
            [
                "--repo-root",
                str(self.root),
                "--refresh-staged",
                "--date",
                "2026-02-02",
                *paths,
            ]
        )

    def test_refreshes_documentation_and_ignores_code(self) -> None:
        (self.root / "main.go").write_text("package main\n", encoding="utf-8")
        self.assertEqual(self.refresh("docs/page.md", "main.go"), 0)
        self.assertIn("last_updated: 2026-02-02", self.path.read_text())

    def test_no_documentation_targets_is_not_a_failure(self) -> None:
        self.assertEqual(self.refresh("README.md"), 0)

    def test_update_and_refresh_are_mutually_exclusive(self) -> None:
        # argparse writes its error to the real stderr, which the unittest
        # runner surfaces as spurious output; capture it and assert on the
        # message rather than only on the exit.
        with self.assertRaises(SystemExit) as caught, contextlib.redirect_stderr(
            io.StringIO()
        ) as err:
            main(
                [
                    "--repo-root",
                    str(self.root),
                    "--refresh-staged",
                    "--update-last-updated",
                    "docs/page.md",
                ]
            )
        self.assertEqual(caught.exception.code, 2)
        self.assertIn("mutually exclusive", err.getvalue())


if __name__ == "__main__":
    unittest.main()
