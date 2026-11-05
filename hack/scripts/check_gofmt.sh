#!/usr/bin/env bash
# gofmt gate for pre-commit.
#
# Stock `gofmt -l` lists files whose formatting differs but always exits 0, and
# the mise-pinned Go toolchain (see .mise.toml) has no `--fail` flag. This
# wrapper fails the commit when any staged Go file is unformatted: it prints the
# offending paths and exits 1 so the pre-commit hook blocks the commit.
#
# Usage: hack/scripts/check_gofmt.sh [file.go ...]
# With no arguments it checks nothing (pre-commit always passes staged files).
set -euo pipefail

if [ "$#" -eq 0 ]; then
  exit 0
fi

unformatted="$(gofmt -l "$@")"

if [ -n "${unformatted}" ]; then
  echo "The following Go files are not gofmt-formatted:" >&2
  printf '%s\n' "${unformatted}" >&2
  echo "Run 'gofmt -w <file>' (or 'mise run lint' tooling) to fix them." >&2
  exit 1
fi

exit 0
