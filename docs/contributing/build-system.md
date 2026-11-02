---
last_updated: 2026-11-02
id: build-system
title: "Build System (Mise)"
sidebar_label: Build System (Mise)
description: How the openCenter-cli build pipeline is wired through mise tasks, and the full inventory of tasks defined in .mise.toml.
doc_type: explanation
audience: "developers"
tags: [contributing, build, mise]
---
# Build System (Mise)

**Purpose:** For developers, explains the mise-based build system, its tool pins, and every task defined in `.mise.toml`.

## Why mise

openCenter-cli uses [mise](https://mise.jdx.dev/) rather than Make: it pins tool versions (Go, kubectl, kind, helm, sops, and Go/aqua-installed CLI tools) in one file, works identically on macOS/Linux/WSL2, and replaces ad hoc shell scripts with named, discoverable tasks.

## Tool pins (`[tools]`)

| Tool | Version | Install method |
| --- | --- | --- |
| `go` | `1.27.1` | mise-native |
| `golangci-lint` | `2.14.0` | mise-native |
| `vale` | `3.23.0` | mise-native |
| `kubectl` | `latest` | mise-native |
| `kind` | `latest` | mise-native |
| `helm` | `latest` | mise-native |
| `sops` | `3.13.3` | mise-native |
| `golang.org/x/vuln/cmd/govulncheck` | `latest` | `go:` backend |
| `gitleaks/gitleaks` | `latest` | `aqua:` backend |

Go, golangci-lint, Vale, and SOPS are hard-pinned; `kubectl`, `kind`, `helm`, `pre-commit`, `govulncheck`, and `gitleaks` float on `latest` locally. The mise key for the Go toolchain is `go`, not `golang`. `go.mod` declares `go 1.26.9` with `toolchain go1.27.3`: the language floor the module requires, and the toolchain the Go command selects when it runs outside mise. CI workflows pin some deployment tools independently for reproducibility -- notably `deploy-kind.yml` pins `KIND_VERSION=v0.29.0`, `KUBECTL_VERSION=v1.35.4`, `HELM_VERSION=v3.19.0`, `FLUX_VERSION=v2.6.4`, and `SOPS_VERSION=3.13.3`. See [GitHub Actions Workflows](../reference/github-actions-workflows.md).

## Environment defaults (`[env]`)

```toml
KIND_EXPERIMENTAL_PROVIDER = "podman"
CONTAINER_RUNTIME = "podman"
```

`.mise.toml` also documents (commented out, opt-in) `OPENCENTER_CONFIG_DIR`, `OPENCENTER_DEBUG`, and `OPENCENTER_PLUGINS_DIR` overrides for reproducible local testing.

## Full task inventory

### Build

| Task | What it does |
| --- | --- |
| `build-cli` | `go build` with `-ldflags` injecting `main.version`, `main.gitCommit`, `main.gitBranch`, `main.gitTag`, `main.buildDate` -> `bin/opencenter`. `VERSION` is the current exact git tag if one exists, else `0.0.1`. |
| `build-info` | Prints `go version` and `go env GOOS GOARCH CGO_ENABLED GOROOT GOENV GOFLAGS` -- the toolchain inputs that affect reproducible builds/race tests. |
| `build-local-plugin` | `go build -o bin/opencenter-local ./cmd/opencenter-local`. |
| `build` | Runs `build-cli` then `build-local-plugin`. |
| `build-linux` | Same as `build-cli` but cross-compiled `GOOS=linux GOARCH=amd64` -> `bin/opencenter-linux`. |
| `build-all` | Cross-compiles all four release platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) into `bin/opencenter-<os>-<arch>`. |
| `local-install` | Builds both binaries, installs `opencenter` to `~/.local/bin`, installs `opencenter-local` into the plugins dir, and registers its checksum in `checksums.txt` so plugin discovery treats it as verified. |

### Release

| Task | What it does |
| --- | --- |
| `release <version> [publish-target]` | Cross-compiles four release binaries into `bin/release/`, then calls `publish` to generate release notes, then prints the manual `git tag` / `gh release create` next steps. `publish-target` defaults to `opencenter-cloud/opencenter-cli`. This is a **local preflight helper**, not the publishing mechanism -- see [Release Process](release-process.md). |
| `publish <version> [publish-target]` | Generates `bin/release/RELEASE_NOTES_<version>.md` from `git log <last-tag>..HEAD --oneline --no-merges`, grouping commits whose subject starts with `feat`/`fix`/`docs`, plus a boilerplate installation section. |

### Code quality

| Task | What it does |
| --- | --- |
| `fmt` | `gofmt -w .` |
| `tidy` | `go mod tidy` |
| `upgrade-deps` | `go get -u ./...` then `go mod tidy` |
| `vet` | `go vet ./...` |
| `lint` | `golangci-lint run ./...` |

### Test

| Task | What it does |
| --- | --- |
| `test` | `go test ./internal/config/... ./cmd/... -count=1` then `go test ./internal/cloud/... -count=1`. |
| `test-race` | `go test ./internal/... ./cmd/... -count=1 -race`. |
| `test-build` | `go build ./...` (compile-only, no test execution). |
| `test-remediation` | `go test ./internal/sops ./internal/secrets ./internal/secretartifacts ./internal/gitops ./cmd -count=1`. |
| `test-remediation-race` | Runs two specific ownership/rollback tests three times under `-race`. |
| `test-docs` | `test-docs-scripts`, the generator tests, `test-docs-idempotency`, the removed-command and YAML-snippet Go tests, `test-docs-frontmatter`, `test-docs-links`, `test-docs-mermaid`. It does not publish or render documentation. |
| `test-go-all` | Local `go test ./... -count=1 -timeout 15m` sweep; on failure, a transient failure log is written under `.tmp/` for diagnosis, not kept as a repository artifact. |
| `test-kustomize` | `go test ./internal/gitops -run '^TestGeneratedDefaultOverlayKustomizeFailureMatrix$'`. |
| `test-diff` | `git diff --check` (rejects whitespace errors in the working tree). |
| `godog` | Non-`@wip` BDD scenarios. |
| `godog-wip` | Only `@wip` BDD scenarios. |
| `godog-tag <tag>` | BDD scenarios filtered to `@<tag>`. |
| `property` (alias `test-properties`) | `-run "TestProperty"` across `./internal/... ./cmd/...`. |
| `govulncheck` | `govulncheck ./...`. |
| `gitleaks` | `gitleaks detect --source . --redact --no-banner` with the long configuration-file flag pointing at `.gitleaks.toml` (full-history scan; not run in any CI workflow -- local/manual only). |
| `integration` | Runs three named suites in sequence: cluster-setup, resilience, operations. |
| `perf` | `go test -tags perf ./internal/config -run "TestMemoryUsageRegression"`. |
| `test:all` | `test`, `test-race`, `vet`, `godog`, `property`, `govulncheck`, in order. This aggregate is separate from the local full-package `test-go-all` sweep and the repository-only `test-docs` checks. |
| `verify` | `test`, `test-race`, `test-properties`, `govulncheck` -- the local pre-push subset. |
| `test-remediation-all` | The full remediation-branch validation matrix (build, vet, test, remediation tests + race, godog, docs tests, kustomize test, docs-idempotency, docs-frontmatter-remediation, diff check). |

### Schema and validation

| Task | What it does |
| --- | --- |
| `schema` | Legacy schema task definition; the current built-in command tree does not provide its old CLI entry point. Use `schema-v2` for the authoritative v2 schema. |
| `schema-gen` | **Unavailable:** legacy task definition invoking `cmd/schema-gen/main.go`; that entrypoint is not present in the current tree. The checked-in v2 schema task is `schema-v2`. |
| `schema-v2` | Regenerates `schema/opencenter-v2.schema.json` by writing and running a throwaway Go test against `internal/config/v2schema`, then deleting the test file. |
| `validate` | `./bin/opencenter cluster validate`. |
| `schema-verify` | **Unavailable as a supported workflow:** legacy end-to-end helper with an obsolete update step. Use `schema-v2` and targeted config tests instead. |

### Documentation

| Task | What it does |
| --- | --- |
| `docs-gen` | `go run cmd/docs/generate.go` -- regenerates the auto-generated Cobra reference pages under `docs/reference/opencenter/`. |
| `docs-update-timestamps -- <page.md> ...` | `python3 hack/scripts/audit_doc_frontmatter.py --update-last-updated` on the pages you name; exits 2 with usage text when you name none. |
| `test-docs-idempotency` | Runs `docs-gen` twice and diffs the two results; fails if generation is not byte-for-byte stable. Chained by `test-docs`. |
| `test-docs-scripts` | `python3 -m unittest discover -s hack/scripts -p 'test_*.py'` -- unit tests for every documentation policy/helper script. First step chained by `test-docs`. |
| `test-docs-links` | `python3 hack/scripts/check_doc_links.py` -- checks local Markdown links without network access. Chained by `test-docs`. |
| `test-docs-mermaid` | `python3 hack/scripts/check_mermaid_fences.py` -- checks Mermaid fence structure without a renderer. Chained by `test-docs`. |
| `test-docs-frontmatter` | `python3 hack/scripts/audit_doc_frontmatter.py --strict` across every Markdown page. Chained by `test-docs`. |
| `test-docs-timestamps` | `python3 hack/scripts/audit_doc_frontmatter.py --strict --check-stale-timestamps` -- fails a page whose `last_updated` predates the newest commit that changed its body. Needs full Git history; on a shallow clone it reports nothing. Chained by `test-docs`. |
| `test-docs-task-coverage` | `python3 hack/scripts/check_mise_task_docs.py` -- fails when `.mise.toml` defines a task that `docs/reference/mise-tasks.md` omits, or that page documents a task mise does not define. Chained by `test-docs`. |
| `test-docs-cli-invocations` | `python3 hack/scripts/check_doc_cli_invocations.py` -- asks the built binary, through cobra completion, whether every command shown in a shell code block is registered. Skips cleanly when `bin/opencenter` is absent, so run `mise run build` first. Chained by `test-docs`. |
| `test-docs-frontmatter-remediation` | Same audit, with an explicit `--ignore` list for a documented set of legacy pages not yet remediated. |
| `tag-wip-failures` | `python3 hack/tag_wip_failures.py` -- runs the Godog suite as Cucumber JSON and tags currently-failing scenarios `@wip`. |

### Local development environment

| Task | What it does |
| --- | --- |
| `gitea-up` | `go run ./cmd/opencenter-local gitea up` -- starts and provisions a local Gitea instance. |
| `gitea-cleanup` | `go run ./cmd/opencenter-local gitea destroy`. |
| `active` | `./bin/opencenter cluster status`. |
| `terraform-generate <cluster> [output-dir]` | **Unavailable:** legacy task definition invoking `cluster terraform-generate`; that command is not registered in the current CLI tree. Generate infrastructure through `cluster generate`. |
| `preflight` | **Unavailable:** legacy task definition invoking a removed `cluster`-`preflight` verb that is not registered in the current CLI tree. Audit local prerequisites with `cluster doctor`. |
| `install-shell-integration` | `./hack/install-shell-integration.sh`. |
| `install-hooks` | Fails unless `pre-commit` is on `PATH` (`mise install` first), then runs `pre-commit install --hook-type pre-commit` pointing it at the tracked `.pre-commit-config.yaml`. The installed hook runs gitleaks, the docs frontmatter audit, Vale on staged Markdown, and the maintained-docs requirement for public behavior changes. |

### Cleanup

| Task | What it does |
| --- | --- |
| `clean` | Removes `bin/`, `testdata/`, `new-schema.json`, `terraform-output/`. |
| `kind-cleanup [cluster]` | Destroys the named cluster (default `my-cluster`) and the local Gitea instance, verifies no stray Kind cluster remains, and removes a stale lock file. Its two existence probes invoke commands that are not registered in the current CLI tree -- a removed `cluster`-`info` verb and the separate plugin binary's command group -- so both probes fail and each of those steps is skipped. |
| `demo-cleanup` | `kind-cleanup` then `clean`. |

### OpenStack utilities

| Task | What it does |
| --- | --- |
| `openstack-reset` | Runs `hack/scripts/openstack-reset.sh` -- resets an OpenStack project to a clean state (keeps the default security group and `PUBLICNET`). Accepts `--os-cloud`, `--force`, `--dry-run` after `--`. |

## Task anatomy

Tasks are either a single command string, an ordered array of `mise run` calls, or a bash heredoc script:

```toml
[tasks]
fmt = "gofmt -w ."

verify = [
  "mise run test",
  "mise run test-race",
  "mise run test-properties",
  "mise run govulncheck"
]

build-cli = '''
#!/usr/bin/env bash
set -e
...
'''
```

Tasks that accept positional arguments (`release`, `publish`, the legacy
`terraform-generate` helper, `godog-tag`, and `kind-cleanup`) read `$1`, `$2`, ...
from arguments passed after the task name: `mise run release v0.0.1-rc3`.

## Discovering and debugging tasks

```bash
mise tasks                 # list every task
mise task show <name>       # show a task's definition
mise run -v <name>           # verbose
mise run --dry-run <name>     # show what would run without running it
```

## Common workflows

```bash
# Everyday development loop
mise install && mise run build && mise run test && mise run fmt

# Before opening a PR
mise run verify && mise run godog

# Full local Go package sweep (separate from test:all)
mise run test-go-all

# Repository-only documentation checks (does not publish or render docs)
mise run test-docs

# Schema change
mise run schema-v2

# Cutting a release build locally (does not publish)
mise run release v1.2.0
```

See [Release Process](release-process.md) for what actually publishes a release (a pushed `v*` tag drives `.github/workflows/release.yml`; the `release`/`publish` mise tasks are local-only helpers).
