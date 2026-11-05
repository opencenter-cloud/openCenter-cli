---
last_updated: 2026-11-05
id: mise-tasks
title: "Mise Tasks Reference"
sidebar_label: Mise Tasks
description: Flat reference of every task and tool pin defined in .mise.toml.
doc_type: reference
audience: "developers, contributors"
tags: [mise, build, tasks, reference]
---
# Mise Tasks Reference

**Purpose:** For developers, provides a complete, flat reference of every tool pin and task defined in `.mise.toml`. For the *why* behind this build system, see [Build System (Mise)](../contributing/build-system.md).

## Tool pins

| Tool | Version |
| --- | --- |
| `go` | `1.27.3` |
| `golangci-lint` | `2.14.0` |
| `vale` | `3.23.0` |
| `kubectl` | `latest` |
| `kind` | `latest` |
| `helm` | `latest` |
| `sops` | `3.13.3` |
| `pre-commit` | `latest` |
| `go:golang.org/x/vuln/cmd/govulncheck` | `latest` |
| `aqua:gitleaks/gitleaks` | `latest` |

## Environment defaults

| Variable | Value | Purpose |
| --- | --- | --- |
| `KIND_EXPERIMENTAL_PROVIDER` | `podman` | Tells `kind` to use Podman instead of Docker. |
| `CONTAINER_RUNTIME` | `podman` | Container runtime used by local Kind/Gitea workflows. |

`.mise.toml` also documents (commented out) opt-in overrides for `OPENCENTER_CONFIG_DIR`, `OPENCENTER_DEBUG`, and `OPENCENTER_PLUGINS_DIR` for reproducible local test runs.

## Tasks

| Task | Command(s) |
| --- | --- |
| `build-cli` | `go build -ldflags "-X main.version=... -X main.gitCommit=... -X main.gitBranch=... -X main.gitTag=... -X main.buildDate=..." -o bin/opencenter` |
| `build-info` | `go version`; `go env GOOS GOARCH CGO_ENABLED GOROOT GOENV GOFLAGS` |
| `build-local-plugin` | `go build -o bin/opencenter-local ./cmd/opencenter-local` |
| `build` | `mise run build-cli` then `mise run build-local-plugin` |
| `build-linux` | Same as `build-cli`, cross-compiled `GOOS=linux GOARCH=amd64` -> `bin/opencenter-linux` |
| `build-all` | Cross-compiles `bin/opencenter-{linux,darwin}-{amd64,arm64}` |
| `local-install` | Builds both binaries; installs to `~/.local/bin/opencenter` and `<plugins-dir>/opencenter-local`; updates `checksums.txt` |
| `release <version> [target]` | Cross-compiles 4 release binaries into `bin/release/`, then runs `publish`, then prints manual tag/push/`gh release create` steps |
| `publish <version> [target]` | Generates `bin/release/RELEASE_NOTES_<version>.md` from `git log <last-tag>..HEAD` grouped by `feat`/`fix`/`docs` |
| `fmt` | `gofmt -w .` |
| `tidy` | `go mod tidy` |
| `upgrade-deps` | `go get -u ./...` then `go mod tidy` |
| `vet` | `go vet ./...` |
| `lint` | `golangci-lint run ./...` |
| `test` | `go test ./internal/config/... ./cmd/... -count=1`; `go test ./internal/cloud/... -count=1` |
| `test-race` | `go test ./internal/... ./cmd/... -count=1 -race` |
| `test-build` | `go build ./...` |
| `test-remediation` | `go test ./internal/sops ./internal/secrets ./internal/secretartifacts ./internal/gitops ./cmd -count=1` |
| `test-remediation-race` | `go test -race ./internal/secrets -run 'TestSyncSecretsSerializesConcurrentTransactions\|TestReconcileStateWriteFailureRollsBackAllMutationsAndRetry' -count=3` |
| `test-docs` | `test-docs-scripts`, `go test -tags tools ./cmd/docs -count=1`, `test-docs-idempotency`, `go test ./cmd -run '^TestDocsDoNotUseRemovedGACommands$' -count=1`, `go test ./internal/config/v2schema -run '^TestDocumentationYAMLSnippetsMatchSchema$' -count=1`, `test-docs-frontmatter`, `test-docs-timestamps`, `test-docs-links`, `test-docs-portability`, `test-docs-mermaid`; it does not publish or render documentation |
| `test-go-all` | Local `go test ./... -count=1 -timeout 15m` sweep; on failure, a transient failure log is written under `.tmp/` for diagnosis, not kept as a repository artifact. |
| `test-kustomize` | `go test ./internal/gitops -run '^TestGeneratedDefaultOverlayKustomizeFailureMatrix$' -count=1` |
| `test-diff` | `git diff --check` |
| `godog` | `go test ./tests/features/steps/... -v -- --godog.tags=~@wip --godog.paths=tests/features` |
| `godog-wip` | Same, with `--godog.tags=@wip` |
| `godog-tag <tag>` | Same, with `--godog.tags=@<tag>` |
| `property` | alias of `test-properties`; `go test ./internal/... ./cmd/... -v -run "TestProperty" -count=1` |
| `test-properties` | the same property-based suite under its canonical name |
| `govulncheck` | `govulncheck ./...` |
| `gitleaks` | `gitleaks detect --source . --redact --no-banner` with the long configuration-file flag pointing at `.gitleaks.toml` |
| `integration` | Runs `TestClusterSetup` (`./cmd`), then `TestRetry\|TestCircuitBreaker\|TestLockManager\|TestProperty` (`./internal/resilience/...`), then `TestDriftDetector\|TestBackupManager\|TestProperty` (`./internal/operations/...`) |
| `perf` | `go test -tags perf ./internal/config -run "TestMemoryUsageRegression" -count=1` |
| `test:all` | `test`, `test-race`, `vet`, `godog`, `property`, `govulncheck`, `test-docs`; separate from the local full-package `test-go-all` sweep |
| `verify` | `test`, `test-race`, `test-properties`, `govulncheck` |
| `test-remediation-all` | `test-build`, `vet`, `test`, `test-remediation`, `test-race`, `test-remediation-race`, `godog`, `test-docs`, `test-kustomize`, `test-docs-idempotency`, `test-docs-frontmatter-remediation`, `test-diff` |
| `schema` | **Unavailable:** the subcommand its body invokes is not registered, so `--pretty` falls through to `cluster` and fails with `unknown flag: --pretty`. Its output target `schema/cluster.schema.json` is not in the tree either. Use `schema-v2`. |
| `schema-gen` | **Unavailable:** legacy command flow invokes `./cmd/schema-gen/main.go`, which is not present in the current tree. |
| `schema-v2` | Writes and runs a throwaway `TestRegenSchema` against `internal/config/v2schema`, writes `schema/opencenter-v2.schema.json`, deletes the test file; this is the task for the checked-in v2 schema |
| `validate` | `./bin/opencenter cluster validate` |
| `schema-verify` | **Unavailable as a supported workflow:** legacy end-to-end helper with an obsolete update step; do not use it. |
| `docs-gen` | `go run cmd/docs/generate.go` |
| `docs-add-purpose` | `python3 hack/scripts/add_purpose_line.py` -- inserts the mandated `**Purpose:**` line on pages missing one, derived from each page's own frontmatter; skips `docs/CODEMAPS/**` and the generated command pages, which `docs-gen` stamps itself |
| `docs-add-purpose-dry-run` | `python3 hack/scripts/add_purpose_line.py --dry-run` -- previews the above without writing |
| `docs-update-timestamps -- <page.md> ...` | `python3 hack/scripts/audit_doc_frontmatter.py --update-last-updated` on the explicitly supplied pages; exits with status 2 and prints usage when no page is given |
| `test-docs-idempotency` | Runs `docs-gen` twice, diffs the two results, fails if unstable; chained by `test-docs` |
| `test-docs-scripts` | `python3 -m unittest discover -s hack/scripts -p 'test_*.py'` -- unit tests for every documentation policy/helper script; first step chained by `test-docs` |
| `test-docs-links` | `python3 hack/scripts/check_doc_links.py` -- checks local Markdown links without network access; chained by `test-docs` |
| `test-docs-portability` | `python3 hack/scripts/check_doc_links.py --root docs --exclude 'CODEMAPS/**' --check-reachability` -- fails when a page links outside `docs/` (which breaks once the tree is copied into a site generator) or cannot be reached from `docs/index.md`; `docs/CODEMAPS/**` is excluded because those maps link Go source paths on purpose; chained by `test-docs` |
| `test-docs-mermaid` | `python3 hack/scripts/check_mermaid_fences.py` -- checks Mermaid fence structure without a renderer; chained by `test-docs` |
| `test-docs-frontmatter` | `python3 hack/scripts/audit_doc_frontmatter.py --strict` across maintained Markdown pages; chained by `test-docs` |
| `test-docs-frontmatter-remediation` | Same audit, with an `--ignore` list of documented legacy pages |
| `test-docs-timestamps` | `python3 hack/scripts/audit_doc_frontmatter.py --strict --check-stale-timestamps` -- fails when a page's `last_updated` predates the newest commit that changed its body; needs full Git history, and is chained by `test-docs` |
| `test-docs-task-coverage` | `python3 hack/scripts/check_mise_task_docs.py` -- fails when `.mise.toml` defines a task that `docs/reference/mise-tasks.md` does not document, or the page documents a task that no longer exists; chained by `test-docs` |
| `test-docs-cli-invocations` | `python3 hack/scripts/check_doc_cli_invocations.py` -- asks the built binary, through cobra completion, whether every command shown in a shell code block is registered; skips when `bin/opencenter` is absent, so run `mise run build` first; chained by `test-docs` |
| `tag-wip-failures` | `python3 hack/tag_wip_failures.py` |
| `gitea-up` | `go run ./cmd/opencenter-local gitea up` |
| `gitea-cleanup` | `go run ./cmd/opencenter-local gitea destroy` |
| `active` | `./bin/opencenter cluster status` |
| `terraform-generate <cluster> [dir]` | **Unavailable:** legacy command flow invokes `./bin/opencenter cluster terraform-generate`, which is not registered in the current command tree. |
| `preflight` | **Unavailable:** legacy command flow invokes a removed `cluster`-`preflight` verb that is not registered in the current command tree. |
| `install-shell-integration` | `./hack/install-shell-integration.sh` |
| `install-hooks` | Errors out unless `pre-commit` is on `PATH` (run `mise install` first), then runs `pre-commit install --hook-type pre-commit` pointing it at the tracked `.pre-commit-config.yaml`; that hook runs gitleaks, the docs frontmatter audit, Vale on staged Markdown, and the maintained-docs requirement for public behavior changes |
| `clean` | Removes `bin/`, `testdata/`, `new-schema.json`, `terraform-output/` |
| `kind-cleanup [cluster]` | Destroys the named Kind cluster (default `my-cluster`) and local Gitea; removes a stale lock file. Its two existence probes invoke commands that are not registered in the current command tree -- a removed `cluster`-`info` verb and the separate plugin binary's command group -- so both probes fail and each step is skipped. |
| `demo-cleanup` | `kind-cleanup` then `clean` |
| `openstack-reset` | `hack/scripts/openstack-reset.sh` (accepts `--os-cloud`, `--force`, `--dry-run` after `--`) |

## Discovering tasks interactively

```bash
mise tasks                # list all tasks
mise task show <name>      # show a task's full definition
mise run -v <name>          # verbose run
mise run --dry-run <name>    # show what would run
```

See [Testing Guide](../contributing/testing-guide.md) for which of these tasks CI actually runs, and [GitHub Actions Workflows](github-actions-workflows.md) for the workflows themselves.

## Legacy command references in task definitions

The following task bodies are still present in `.mise.toml`, but their command
names are not in the current built-in CLI command tree. Treat them as task
maintenance items rather than supported CLI examples:

* `schema` is a legacy task definition; use `schema-v2` for
  `schema/opencenter-v2.schema.json`.
* `schema-gen` is unavailable because its `cmd/schema-gen/main.go` entrypoint
  is not present in the current tree.
* `schema-verify` is unavailable as a supported workflow because its command
  flow contains an obsolete update step.
* `terraform-generate` is unavailable because it invokes a `cluster` subcommand that is
  not registered in the current tree; generate infrastructure with `cluster generate`.
* `preflight` is unavailable because it invokes a removed `cluster`-`preflight` verb;
  audit local prerequisites with `cluster doctor` and validate a configuration with
  `cluster validate`.
* `validate` invokes `cluster validate`, which is registered, but passes no cluster
  name, so it only resolves the active cluster.
* `kind-cleanup` probes for the named cluster and the local Gitea instance through
  commands that are not registered in the current tree, so both cleanup steps are
  skipped and only the stale lock file is removed.

For current command spelling and flags, use the generated [CLI command
reference](cli-commands.md).
