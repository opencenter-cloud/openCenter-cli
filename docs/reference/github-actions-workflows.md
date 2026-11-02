---
last_updated: 2026-10-02
id: github-actions-workflows
title: "GitHub Actions Workflows"
sidebar_label: GitHub Actions Workflows
description: Repository workflow summary with links to the canonical YAML definitions under .github/workflows.
doc_type: reference
audience: "developers, maintainers, devops engineers"
tags: [ci, github-actions, workflows, reference]
---
# GitHub Actions Workflows

**Purpose:** For developers and maintainers, summarizes the workflows currently
tracked in `.github/workflows/`. The YAML files linked below are canonical;
consult them for exact triggers, inputs, permissions, actions, commands, and
runner settings. This page intentionally avoids duplicating line-sensitive
workflow behavior.

All current workflows set `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`. The
repository's workflows use self-hosted runners; `deploy-kind.yml` uses the
`self-hosted-kvm` label. Go workflows use the Go version from `go.mod`.

## Workflow summary

| Workflow | Repository role | Trigger summary | Canonical definition |
| --- | --- | --- | --- |
| `build-binaries.yml` | Builds Linux amd64 and arm64 CLI artifacts for manual publication. | `workflow_dispatch` | [`build-binaries.yml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/build-binaries.yml) |
| `deploy-kind.yml` | Exercises a disposable Kind, Gitea, and FluxCD deployment using the CLI and local plugin. | `workflow_dispatch` with deployment inputs | [`deploy-kind.yml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/deploy-kind.yml) |
| `docs-p0.yml` | Checks changed Markdown files with the strict frontmatter audit and Vale. | Pull requests that change Markdown | [`docs-p0.yml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/docs-p0.yml) |
| `pre-commit.yaml` | Runs the configured pre-commit hooks on pull-request changes. | Pull requests | [`pre-commit.yaml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/pre-commit.yaml) |
| `release.yml` | Builds multi-platform CLI and local-plugin artifacts, signs release outputs, generates an SBOM, and publishes a GitHub release. | Version-tag pushes and manual dispatch | [`release.yml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/release.yml) |
| `test.yml` | Runs the repository Go tests, race-detector suite, property tests, and `go vet`. | Pull requests and pushes to `main` | [`test.yml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/test.yml) |
| `vulncheck.yml` | Runs Go dependency vulnerability analysis. | Pull requests, weekly schedule, and manual dispatch | [`vulncheck.yml`](https://github.com/opencenter-cloud/openCenter-cli/blob/main/.github/workflows/vulncheck.yml) |

## CI boundaries

The workflows above are the repository evidence for what runs in GitHub
Actions. The local `mise` tasks are not automatically equivalent to CI tasks;
for example, the BDD suite, integration task, documentation-generator tests,
Kustomize check, and `mise run test:all` are not invoked by a workflow in this
repository. See [Mise Tasks Reference](mise-tasks.md) and the [Testing
Guide](../contributing/testing-guide.md) for local task definitions and
coverage guidance.

## Related references

* [Mise Tasks Reference](mise-tasks.md) — local build and test task definitions.
* [Build System (Mise)](../contributing/build-system.md) — contributor tooling.
* [Release Process](../contributing/release-process.md) — release guidance.
