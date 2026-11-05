---
last_updated: 2026-11-05
id: pre-commit-hooks
title: "Pre-commit Hooks"
sidebar_label: Pre-commit Hooks
description: Install and understand the local checks that protect secrets, keep public behavior documented, and gate Go formatting and linting.
doc_type: how-to
audience: "contributors"
tags: [contributing, hooks, documentation]
---
# Pre-commit Hooks

**Purpose:** For contributors, shows how to install and understand the local checks that protect secrets and keep public behavior documented.

Install the repository's tracked hook configuration with:

```bash
mise install
mise run install-hooks
```

`install-hooks` installs the tracked pre-commit configuration; it does not
merely check whether an old `.git/hooks/pre-commit` file exists. The
configuration keeps the gitleaks hook, runs the staged documentation policy, and
gates Go source with `gofmt` and `golangci-lint`.

## Documentation policy

A staged create, modify, or rename of a public CLI/runtime implementation file
must be accompanied by a staged create, modify, or rename of a maintained
reader-facing Markdown page. The implementation boundary includes:

* `main.go`;
* non-test Go files under `cmd/`, except `cmd/docs/`;
* non-test Go files under `internal/config/`, `internal/cloud/`,
  `internal/cluster/`, `internal/plugins/`, and `internal/operations/`; and
* JSON schema files under `schema/`.

Tests, tooling, and CI files do not trigger the policy. Maintained
documentation is root `README.md` or `SECURITY.md`, or Markdown under `docs/`
except `docs/CODEMAPS/`, `docs/superpowers/`, reports, plans, work artifacts,
and generated CLI pages under `docs/reference/opencenter/`.

The check consumes the filenames supplied by pre-commit and is also safe to run
directly:

```bash
python3 hack/scripts/check_staged_docs.py cmd/cluster_status.go README.md
```

Missing paths are ignored so staged deletions do not count as implementation or
documentation additions. Add the documentation path to the same staged change
when the check reports a missing maintained page.

## Go formatting and linting

Two hooks gate Go source so formatting and lint violations are caught at commit
time rather than at pull-request time. Both fire only when a `.go` file is
staged and resolve their tools through the mise shim on `PATH` (the same
convention as the Vale hook), so they run the exact versions pinned in
`.mise.toml`.

* **`gofmt`** runs `hack/scripts/check_gofmt.sh` over the staged files. Stock
  `gofmt -l` lists unformatted files but always exits `0`, and the pinned Go
  toolchain has no `--fail` flag, so the wrapper prints the offending paths and
  exits `1` to block the commit. Fix with `gofmt -w <file>`.
* **`golangci-lint`** runs `golangci-lint run ./...` against `.golangci.yml`,
  mirroring the `lint` job in `.github/workflows/test.yml`. Because
  golangci-lint operates on packages rather than individual files, the hook
  lints the whole module serially (`pass_filenames: false`) instead of the
  staged subset, so a violation introduced anywhere fails the commit.

Run either hook directly against the current tree:

```bash
pre-commit run gofmt --all-files
pre-commit run golangci-lint --all-files
```
