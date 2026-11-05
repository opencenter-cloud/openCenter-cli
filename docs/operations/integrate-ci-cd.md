---
id: integrate-ci-cd
title: Use openCenter in CI
sidebar_label: Use openCenter in CI
description: Use openCenter validation and generation commands in a CI job.
doc_type: how-to
audience: platform engineers
tags: [ci-cd, automation]
last_updated: 2026-09-25
---
# Use openCenter in CI

**Purpose:** For platform engineers, shows how to use openCenter validation and generation commands in a CI job.

The repository exposes validation and generation commands suitable for a CI
job. Keep provider-mutating commands out of a job unless the job intentionally
has those credentials and targets.

## Validate a file

```bash
set -euo pipefail
opencenter cluster validate --config-file ./cluster.yaml --output json
```

Use `--validation online` only when the job is intended to contact provider and
Git remotes. `--generate-debug-config --output-dir artifacts` can save a debug
configuration, but treat generated files as sensitive.

## Render without applying

```bash
opencenter --dry-run cluster generate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER --render-only
```

Review the generated Git diff with the CI system's normal tooling. The CLI does
not establish a particular Git hosting service, token format, branch policy, or
workflow file.

## Optional deployment gate

```bash
opencenter --dry-run cluster deploy ORG/CLUSTER
```

An actual deploy uses provider credentials, local tools, and the configured
GitOps working tree. The command supports `--yes` globally and reports its own
logs and resume state.

## Evidence

- Validation options: `cmd/cluster_validate.go`
- Generation options: `cmd/cluster_generate.go`
- Global dry run/output flags: `cmd/root.go`
- CI-like validation fixture: `tests/features/validation.feature`
