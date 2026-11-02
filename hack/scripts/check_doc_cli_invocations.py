#!/usr/bin/env python3
"""Verify every documented CLI invocation resolves to a real command.

Documentation drifts from the command tree in one specific, high-cost way: a
page keeps an example for a verb that was renamed or removed, and the reader
gets ``Error: unknown command``. The existing drift test catches a fixed list of
banned strings; this check needs no list. It asks the binary itself whether each
documented command path resolves, so a future rename is caught by the page that
still shows the old name.

Invocations are read only from fenced code blocks, never from prose: prose
sentences such as "opencenter and its plugins" would otherwise parse as command
paths. ``--help`` is appended to each path, which cobra answers with exit status
0 for a real command and non-zero for an unknown one, without executing it.
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

DOC_GLOBS = ("docs/**/*.md",)
ROOT_DOCS = ("README.md", "llms.txt")

# Pages that document plugin *authoring* show illustrative plugin command names
# (`opencenter my-command`) rather than shipped commands, so they are excluded.
EXCLUDED_PATH_PARTS = ("reference/opencenter", "/superpowers/")

# Placeholder names used in generic examples, plus the documented plugin binary.
PLACEHOLDER_WORDS = {
    "my-command",
    "my-flag",
    "some-command",
    "the-command",
    "your-command",
}

# A fence marker is a run of three or more backticks or tildes. A fence closes
# only on its own marker *character* (compared below, not by string equality),
# so `~~~` inside a ``` block is content rather than a delimiter.
FENCE_RE = re.compile(r"^\s*(`{3,}|~{3,})")
# Fence info strings whose contents are shell sessions. Other fences reuse the
# literal word `opencenter` for non-commands, so they are not scanned.
SHELL_FENCE_LANGUAGES = {"", "bash", "sh", "shell", "zsh", "console", "shell-session"}
# `opencenter` / `./bin/opencenter` / `bin/opencenter-local` at the start of a
# command, followed by up to four bare words. Anchoring to the command position
# (line start, or after a shell prefix) plus stripping comments below keeps
# prose inside a comment out: `mise run build  # bin/opencenter only` would
# otherwise read as the command `opencenter only`.
SHELL_PREFIX = r"(?:sudo[ \t]+|env[ \t]+|[A-Z_][A-Z0-9_]*=\S*[ \t]+)*"
INVOCATION_RE = re.compile(
    r"(?<![\w./-])"
    + SHELL_PREFIX
    + r"(?:\./)?(?:bin/)?opencenter(-local)?((?:[ \t]+[a-z][a-z0-9-]*){1,4})"
)

def fenced_lines(text: str) -> list[str]:
    """Return the shell lines inside fenced blocks, comments removed.

    Only fences whose info string names a shell are scanned. A `text` or `yaml`
    fence can contain the literal word `opencenter` for something that is not a
    runnable command: `CODEMAPS/runtime-extensions-and-local-development.md`
    shows the Go symbol path `opencenter-local newRootCmd` in a `text` diagram.

    A trailing ``# ...`` is a shell comment, so it is dropped rather than
    scanned; a ``#`` inside a quoted argument is left alone.
    """
    lines: list[str] = []
    fence: str | None = None
    scan = False
    for raw in text.splitlines():
        match = FENCE_RE.match(raw)
        if match:
            marker = match.group(1)
            if fence is None:
                fence = marker[0]
                scan = is_shell_fence(raw[match.end(1) :])
                continue
            if marker[0] == fence:
                fence = None
                scan = False
                continue
            if scan:
                # A different marker character is content of the open fence.
                lines.append(raw)
            continue
        if fence is not None and scan:
            lines.append(strip_shell_comment(raw))
    return lines


def is_shell_fence(info: str) -> bool:
    """Return whether a fence info string marks a shell session."""
    language = info.strip().split(",")[0].strip().lower()
    return language in SHELL_FENCE_LANGUAGES


def strip_shell_comment(line: str) -> str:
    """Remove an unquoted trailing ``#`` comment from a shell line."""
    quote: str | None = None
    escaped = False
    for index, character in enumerate(line):
        if escaped:
            escaped = False
            continue
        if character == "\\":
            escaped = True
            continue
        if quote:
            if character == quote:
                quote = None
            continue
        if character in ("\"", "'"):
            quote = character
            continue
        if character == "#" and (index == 0 or line[index - 1] in " \t"):
            return line[:index]
    return line


def doc_files(repo_root: Path) -> list[Path]:
    """Return documentation files whose examples must resolve."""
    files: list[Path] = []
    for pattern in DOC_GLOBS:
        files.extend(repo_root.glob(pattern))
    for name in ROOT_DOCS:
        path = repo_root / name
        if path.is_file():
            files.append(path)
    selected: list[Path] = []
    for path in sorted(set(files)):
        rendered = path.as_posix()
        if any(part in rendered for part in EXCLUDED_PATH_PARTS):
            continue
        if path.is_file():
            selected.append(path)
    return selected


def command_paths(files: list[Path], repo_root: Path) -> dict[tuple[str, ...], set[str]]:
    """Map each documented command path to the files that show it.

    Keys are ``(binary, *command-words)`` so `opencenter-local gitea up` is
    probed against the plugin binary rather than the CLI, which has no `gitea`
    group of its own.
    """
    paths: dict[tuple[str, ...], set[str]] = {}
    for path in files:
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        relative = path.relative_to(repo_root).as_posix()
        for line in fenced_lines(text):
            for match in INVOCATION_RE.finditer(line):
                binary_name = (
                    "opencenter-local" if match.group(1) else "opencenter"
                )
                words = match.group(2).split()
                if any(word in PLACEHOLDER_WORDS for word in words):
                    continue
                key = (binary_name, *words)
                paths.setdefault(key, set()).add(relative)
    return paths


def resolves(binary: Path, words: tuple[str, ...], plugins_dir: Path | None) -> bool:
    """Return whether the binary recognizes *words* as a command path.

    ``--help`` cannot answer this: for an unknown subcommand cobra prints the
    *nearest existing parent's* help and exits 0, so `opencenter cluster
    promote --help` looks exactly like a working command even though `promote`
    was removed. Bare invocation is no better: a leaf command with no arguments
    supplied exits 0 or 1 for its own reasons, unrelated to registration.

    Cobra's completion endpoint answers exactly. `__complete <path>` treats the
    last word as a prefix to complete, and its output separates three cases:

    * registered path whose last word is a command name -> the resolved command
      is echoed as a `<name><TAB><usage>` candidate, e.g. `__complete cluster
      validate` prints `validate<TAB>Validate ...`;
    * registered path whose last word is an argument rather than a command name
      -> no command name matches, and completion hands the token to that
      command's own argument completion: ``ShellCompDirectiveDefault``;
    * unregistered word -> resolution falls back to the root, which offers only
      command names and so reports ``ShellCompDirectiveNoFileComp`` with no
      candidates at all.

    A path therefore resolves when it yields a candidate line *or* the Default
    directive, and fails only when it yields neither. Nothing is executed.
    Candidates go to stdout and the directive line to stderr; both are read,
    since a completion error surfaces on stderr.
    """
    env = dict(os.environ)
    if plugins_dir is not None:
        env["OPENCENTER_PLUGINS_DIR"] = str(plugins_dir)
    result = subprocess.run(
        (str(binary), "__complete", *words),
        cwd=None,
        env=env,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    stdout = result.stdout.decode("utf-8", "replace")
    stderr = result.stderr.decode("utf-8", "replace")
    if "ShellCompDirectiveError" in stderr:
        return False
    if any(_CANDIDATE_RE.match(line) for line in stdout.splitlines()):
        return True
    return "ShellCompDirectiveDefault" in stderr


def resolves_path(binary: Path, words: tuple[str, ...], plugins_dir: Path | None) -> bool:
    """Return whether *words* is a registered command path.

    Completion can only test a prefix, because cobra stops resolving the command
    tree at the first word that is not a registered child and hands the rest to
    that command's argument completion. `cluster promote my-cluster` therefore
    looks identical to `cluster validate my-cluster`: in both cases cobra
    resolves `cluster` and treats the remaining words as arguments.

    So each word is resolved in turn, and resolution stops at the first word
    that is not a registered child. That word is either an argument of the
    command resolved so far — legitimate — or a phantom verb. The two are
    separated by whether the command accepts positional arguments, which its own
    help line states as `Usage: opencenter <path> [flags]` (no positional
    argument) versus a usage line naming one.
    """
    for index in range(len(words)):
        if not resolves(binary, words[: index + 1], plugins_dir):
            # `words[:index]` resolved (or index is 0, checked by the first
            # iteration) and `words[index]` did not, so cobra would treat
            # `words[index]` as an argument of `words[:index]`.
            if index == 0:
                return False
            return command_takes_positional_argument(binary, words[:index], plugins_dir)
    return True


def command_takes_positional_argument(
    binary: Path, words: tuple[str, ...], plugins_dir: Path | None
) -> bool:
    """Return whether the command named by *words* accepts a positional argument.

    Cobra renders `Usage:` as `Use [flags]` when the command takes no positional
    argument, and `Use <arg> [flags]` / `Use [arg] [flags]` when it does.
    """
    env = dict(os.environ)
    if plugins_dir is not None:
        env["OPENCENTER_PLUGINS_DIR"] = str(plugins_dir)
    result = subprocess.run(
        (str(binary), *words, "--help"),
        cwd=None,
        env=env,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    text = result.stdout.decode("utf-8", "replace")
    # The `Usage:` label sits on its own line; the usage line itself is indented
    # below it. `[flags]` and any bracketed argument are dropped, and what is
    # left over is the positional-argument specification, if any.
    usage = re.search(r"^Usage:\n[ \t]+(\S.*)$", text, re.MULTILINE)
    if not usage:
        # No usage line: assume permissive rather than fail documentation.
        return True
    # A parent command lists two usage lines: `<path> [flags]` and
    # `<path> [command]`. The `[command]` marker means children are reached by
    # name, so a bare word after it is never an argument -- `cluster zzz` is a
    # phantom verb, not `cluster` taking an argument named `zzz`.
    block = re.search(r"^Usage:\n((?:[ \t]+\S.*\n?)+)", text, re.MULTILINE)
    if not block:
        # No usage line: assume permissive rather than fail documentation.
        return True
    if "[command]" in block.group(1):
        return False
    line = re.sub(r"\s*\[[^\]]*\]", " ", block.group(1).splitlines()[0])
    return bool(line.split())


_CANDIDATE_RE = re.compile(r"^[^\t\n]+\t")


def check(repo_root: Path, binary: Path) -> int:
    """Check every documented invocation against the built binaries."""
    if not binary.is_file():
        print(f"Skipped: {binary} not found (run `mise run build` first).")
        return 0

    binaries = {"opencenter": binary, "opencenter-local": binary.parent / "opencenter-local"}
    plugins_dir = Path.home() / ".config" / "opencenter" / "plugins"
    files = doc_files(repo_root)
    paths = command_paths(files, repo_root)
    if not paths:
        print(f"Skipped: no `opencenter` invocations found in {len(files)} file(s).")
        return 0

    failures: list[tuple[tuple[str, ...], set[str]]] = []
    # Longest paths first: a failing `cluster drift detect` is reported once
    # rather than also as its `cluster drift` and `cluster` prefixes.
    for key in sorted(paths, key=lambda w: (-len(w), w)):
        target = binaries.get(key[0])
        if target is None or not target.is_file():
            # The plugin binary is optional here: a checkout that built only
            # the CLI must not fail on plugin examples.
            continue
        probe_dir = plugins_dir if plugins_dir.is_dir() else None
        if not resolves_path(target, key[1:], probe_dir):
            failures.append((key, paths[key]))

    for key, sources in failures:
        print(f"unresolved: {' '.join(key)}")
        for source in sorted(sources):
            print(f"  in {source}")

    if failures:
        print(
            f"\nFAILED: {len(failures)} documented command path(s) "
            f"do not resolve ({len(paths)} checked)."
        )
        return 1

    print(
        f"OK: all {len(paths)} documented command path(s) resolve "
        f"via {binary.name} and plugins."
    )
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path.cwd())
    parser.add_argument(
        "--binary",
        type=Path,
        default=None,
        help="CLI binary to probe (default: <repo-root>/bin/opencenter)",
    )
    args = parser.parse_args(argv)
    repo_root = args.repo_root.resolve()
    binary = args.binary or repo_root / "bin" / "opencenter"
    return check(repo_root, binary.resolve())


if __name__ == "__main__":
    sys.exit(main())
