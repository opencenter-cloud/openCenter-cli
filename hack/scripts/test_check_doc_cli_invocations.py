"""Focused tests for check_doc_cli_invocations.py."""

from __future__ import annotations

from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))

from check_doc_cli_invocations import (
    command_paths,
    fenced_lines,
    is_shell_fence,
    strip_shell_comment,
)


class FenceSelectionTests(unittest.TestCase):
    def test_shell_languages_are_scanned(self) -> None:
        for info in ("bash", "sh", "shell", "zsh", "console", ""):
            self.assertTrue(is_shell_fence(info), info)

    def test_non_shell_languages_are_skipped(self) -> None:
        for info in ("yaml", "go", "json", "text", "diff", "toml"):
            self.assertFalse(is_shell_fence(info), info)

    def test_yaml_fence_contents_are_ignored(self) -> None:
        text = "\n".join(("```yaml", "opencenter cluster zzz", "```"))
        self.assertEqual(fenced_lines(text), [])

    def test_tilde_marker_does_not_close_a_backtick_fence(self) -> None:
        # `~~~` inside an open ``` block is content, not a delimiter; the block
        # stays open until its own marker appears.
        text = "\n".join(
            (
                "```bash",
                "opencenter cluster validate",
                "~~~",
                "opencenter cluster generate",
                "```",
            )
        )
        self.assertEqual(
            fenced_lines(text),
            ["opencenter cluster validate", "~~~", "opencenter cluster generate"],
        )

    def test_lines_outside_any_fence_are_ignored(self) -> None:
        text = "before\nopencenter cluster zzz\n```bash\nopencenter cluster validate\n```\nafter\n"
        self.assertEqual(fenced_lines(text), ["opencenter cluster validate"])


class ShellCommentTests(unittest.TestCase):
    def test_trailing_comment_is_removed(self) -> None:
        self.assertEqual(
            strip_shell_comment("mise run build  # bin/opencenter only"),
            "mise run build  ",
        )

    def test_quoted_hash_is_kept(self) -> None:
        line = 'opencenter cluster set x --value "a#b"'
        self.assertEqual(strip_shell_comment(line), line)

    def test_hash_in_word_position_is_kept(self) -> None:
        line = "opencenter cluster set key#1"
        self.assertEqual(strip_shell_comment(line), line)


class CommandPathExtractionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.docs = self.root / "docs"
        self.docs.mkdir()
        self.page = self.docs / "page.md"

    def tearDown(self) -> None:
        self.directory.cleanup()

    def keys(self, body: str) -> set[tuple[str, ...]]:
        self.page.write_text(body, encoding="utf-8")
        return set(command_paths([self.page], self.root))

    def test_command_and_arguments_are_captured(self) -> None:
        self.assertEqual(
            self.keys("```bash\nopencenter cluster validate my-cluster\n```\n"),
            {("opencenter", "cluster", "validate", "my-cluster")},
        )

    def test_flags_terminate_the_command_path(self) -> None:
        self.assertEqual(
            self.keys("```bash\nopencenter cluster destroy test --force\n```\n"),
            {("opencenter", "cluster", "destroy", "test")},
        )

    def test_plugin_binary_is_attributed_separately(self) -> None:
        self.assertEqual(
            self.keys(
                "```bash\nopencenter-local gitea up\nopencenter plugins list\n```\n"
            ),
            {
                ("opencenter-local", "gitea", "up"),
                ("opencenter", "plugins", "list"),
            },
        )

    def test_comment_text_is_not_read_as_a_command(self) -> None:
        self.assertEqual(
            self.keys(
                "```bash\n"
                "mise run build  # -> ./bin/opencenter and ./bin/opencenter-local\n"
                "```\n"
            ),
            set(),
        )

    def test_prose_outside_fences_is_ignored(self) -> None:
        self.assertEqual(self.keys("Run opencenter cluster zzz to do it.\n"), set())

    def test_placeholder_names_are_skipped(self) -> None:
        self.assertEqual(
            self.keys("```bash\nopencenter my-command --flag value\n```\n"), set()
        )

    def test_relative_binary_path_is_recognized(self) -> None:
        self.assertEqual(
            self.keys("```bash\n./bin/opencenter cluster generate\n```\n"),
            {("opencenter", "cluster", "generate")},
        )


if __name__ == "__main__":
    unittest.main()
