"""Focused tests for check_mise_task_docs.py."""

from __future__ import annotations

from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))

from check_mise_task_docs import documented_task_names, mise_task_names


PAGE = """---
id: mise-tasks
---
# Mise Tasks

## Tool pins

| Tool | Version |
| --- | --- |
| `go` | `1.27.1` |
| `aqua:gitleaks/gitleaks` | `latest` |

## Environment defaults

| Variable | Value |
| --- | --- |
| `CONTAINER_RUNTIME` | `podman` |

## Tasks

| Task | Description |
| --- | --- |
| `build` | builds |
| `godog-tag <tag>` | runs godog with a tag |
| `property` (alias `test-properties`) | property suite |

## Discovering tasks interactively

| `not-a-task` | ignored |
"""


class DocumentedTaskNamesTests(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.page = self.root / "mise-tasks.md"
        self.page.write_text(PAGE, encoding="utf-8")

    def tearDown(self) -> None:
        self.directory.cleanup()

    def test_only_the_tasks_section_is_read(self) -> None:
        self.assertEqual(
            documented_task_names(self.page),
            {"build", "godog-tag", "property"},
        )

    def test_argument_placeholder_is_stripped(self) -> None:
        self.assertIn("godog-tag", documented_task_names(self.page))

    def test_missing_page_yields_nothing(self) -> None:
        self.assertEqual(
            documented_task_names(self.root / "absent.md"), set()
        )


class MiseTaskNamesTests(unittest.TestCase):
    def test_absent_mise_is_reported_as_unavailable(self) -> None:
        # An empty directory has no mise config; the helper must not raise, and
        # must return either a set or None so the caller can skip cleanly.
        with tempfile.TemporaryDirectory() as directory:
            names = mise_task_names(Path(directory))
        self.assertTrue(names is None or isinstance(names, set))


if __name__ == "__main__":
    unittest.main()
