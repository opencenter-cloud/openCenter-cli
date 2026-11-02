---
last_updated: 2026-09-26
id: pre-commit-hooks
title: "Pre-commit Hooks"
sidebar_label: Pre-commit Hooks
description: Install and understand the local checks that protect secrets and keep public behavior documented.
doc_type: how-to
audience: "contributors"
tags: [contributing, hooks, documentation]
---
# Pre-commit Hooks

Install the repository's tracked hook configuration with:

```bash
mise install
mise run install-hooks
```

`install-hooks` installs the tracked pre-commit configuration; it does not
merely check whether an old `.git/hooks/pre-commit` file exists. The
configuration keeps the gitleaks hook and also runs the staged documentation
policy.

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
