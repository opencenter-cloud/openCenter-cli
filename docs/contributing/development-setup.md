---
last_updated: 2026-11-02
id: development-setup
title: "Development Environment Setup"
sidebar_label: Development Environment Setup
description: Set up a complete development environment for openCenter-cli, including toolchain, editor, and a local Kind cluster.
doc_type: how-to
audience: "developers"
tags: [contributing, setup]
---
# Development Environment Setup

**Purpose:** For developers, shows how to set up a complete development environment for openCenter-cli from scratch.

## What you'll build

* Working `opencenter` and `opencenter-local` binaries built from source, with version metadata baked into the CLI.
* A pinned toolchain managed by [mise](https://mise.jdx.dev/).
* Passing unit and BDD test runs.
* Optionally, a disposable local Kind cluster to exercise the full `init` -> `validate` -> `generate` -> `deploy` flow.

## Prerequisites

* macOS, Linux, or WSL2.
* Git.
* Docker or Podman (only needed if you will run a local Kind cluster).
* A text editor or IDE with Go support.

## Step 1: Install mise

`.mise.toml` pins every tool this project needs. Install mise itself first:

```bash
# macOS
brew install mise

# Linux
curl https://mise.run | sh

mise --version
```

## Step 2: Clone the repository

```bash
git clone git@github.com:opencenter-cloud/openCenter-cli.git
cd openCenter-cli
```

If you are contributing from a fork, clone your fork and add the upstream remote:

```bash
git clone git@github.com:YOUR-USERNAME/openCenter-cli.git
cd openCenter-cli
git remote add upstream git@github.com:opencenter-cloud/openCenter-cli.git
```

## Step 3: Install pinned tools

```bash
mise install
```

This installs, per `.mise.toml`:

| Tool | Pinned version |
| --- | --- |
| `go` | 1.27.1 |
| `golangci-lint` | 2.14.0 |
| `vale` | 3.23.0 |
| `kubectl` | latest |
| `kind` | latest |
| `helm` | latest |
| `sops` | 3.13.3 |
| `golang.org/x/vuln/cmd/govulncheck` (via `go:`) | latest |
| `gitleaks/gitleaks` (via `aqua:`) | latest |

Verify with `mise list`. The repository also declares two mise-scoped environment defaults relevant to local Kind work: `KIND_EXPERIMENTAL_PROVIDER=podman` and `CONTAINER_RUNTIME=podman`. Override these (e.g. `CONTAINER_RUNTIME=docker`) if you use Docker instead of Podman.

## Step 4: Fetch Go module dependencies

```bash
go mod download
go mod verify
```

## Step 5: Build the CLI

```bash
mise run build          # builds bin/opencenter and bin/opencenter-local
# or individually:
mise run build-cli       # bin/opencenter only
mise run build-local-plugin  # bin/opencenter-local only
```

`build-cli` embeds version metadata via `-ldflags`: `main.version` (from the nearest exact git tag, else `0.0.1`), `main.gitCommit`, `main.gitBranch`, `main.gitTag`, `main.buildDate`.

```bash
./bin/opencenter version
```

## Step 6: Run tests

```bash
mise run test    # internal/config/..., cmd/..., internal/cloud/...
mise run godog   # BDD scenarios, excluding @wip
```

See [Testing Guide](testing-guide.md) for the full test matrix (`test-race`, `property`, `integration`, `govulncheck`, `gitleaks`, `test:all`, `verify`).

If tests fail, check that:

* `go version` reports the pinned `1.27.1` (`mise which go` shows the mise-managed binary). `go.mod` declares `go 1.26.9` with `toolchain go1.27.3`, so outside mise the Go command selects `go1.27.3` on its own.
* Dependencies are downloaded (`go mod download`).
* No stray local config from a previous run is interfering (`rm -rf testdata/config` is safe -- it is a generated test artifact, not checked-in fixture data).

## Step 7: Editor configuration

Any editor with Go tooling support works. Configure it to run `gofmt` on save and `golangci-lint run ./...` (the exact command `mise run lint` runs) as the linter. There is no repository-specific editor config beyond this -- `opencenter settings ide` generates JSON-Schema-aware editor configuration for **cluster config files** (not for editing the Go source itself); see [`opencenter settings ide`](../reference/opencenter/opencenter_settings_ide.md).

## Step 8: Shell integration (optional)

```bash
mise run install-shell-integration
```

This runs `hack/install-shell-integration.sh`, which installs `shell-integration.sh` / `shell-integration.fish` / `starship-opencenter.toml` into `${OPENCENTER_CONFIG_DIR:-$HOME/.config/opencenter}/shell/`, prints (and optionally appends, with confirmation) a `source` line for bash/zsh into your shell rc file, and for fish copies the integration file directly into `~/.config/fish/conf.d/opencenter.fish`. It adds the `opencenter_active` / `opencenter_prompt` / `opencenter_active_short` shell functions and the `oc-active`, `oc-status`, `oc-select`, `oc-list` aliases, and prints Starship setup instructions if `starship` is on your `PATH`.

## Step 9: Create a disposable Kind cluster (optional)

```bash
./bin/opencenter cluster init test-dev --org my-org --type kind --kind-disable-default-cni
./bin/opencenter cluster validate test-dev
./bin/opencenter cluster generate test-dev
./bin/opencenter cluster deploy test-dev
```

Clean up:

```bash
./bin/opencenter cluster destroy test-dev --force
```

For the fully automated version of this flow (including a local Gitea instance for GitOps push), see `.github/workflows/deploy-kind.yml`, which is also runnable as a reference for the exact command sequence, or use `mise run gitea-up` / `mise run gitea-cleanup` directly. See [Kind Cluster Verification](kind-cluster-verification.md).

## Check your work

```bash
mise list                 # tools installed
mise run build && ./bin/opencenter version
mise run test
mise run godog
mise run fmt
mise run tidy
```

## Troubleshooting

**`mise: command not found`** -- add mise's install location to `PATH` (typically `$HOME/.local/bin`) and reload your shell.

**Go version mismatch** -- run `mise install go` and `mise which go` to confirm the mise-managed `1.27.1` toolchain is what `go` resolves to in your shell.

**Tests fail with a config-directory error** -- remove any stale local test config (`rm -rf testdata/config`) and re-run.

**Build fails with a missing package** -- `go mod download && go mod verify`, then rebuild.

**Kind cluster creation fails** -- confirm your container runtime is running (`docker ps` or `podman ps`), and that `CONTAINER_RUNTIME` / `KIND_EXPERIMENTAL_PROVIDER` match the runtime you actually have installed.

## Next steps

1. [Codebase Organization](code-structure.md)
2. [Build System (Mise)](build-system.md)
3. [Testing Guide](testing-guide.md)
4. [Contributing to openCenter-cli](contributing.md)
