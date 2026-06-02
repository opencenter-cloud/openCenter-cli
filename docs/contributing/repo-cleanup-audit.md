---
id: repo-cleanup-audit
title: "Repo Cleanup Audit"
sidebar_label: Repo Cleanup Audit
description: Snapshot of the first dead-code and duplicate-code cleanup pass for openCenter-cli, recorded so future passes can extend the same scope.
doc_type: reference
audience: "developers"
tags: [contributing]
---
# Repo Cleanup Audit

**Purpose:** For developers, records the first conservative cleanup pass for dead code and duplicate code in openCenter-cli.

Date: 2026-05-11

This audit records the first conservative cleanup pass for dead code and duplicate code. The goal is to remove only code that is demonstrably unused and internal, while keeping migration paths, generated/reference docs, CLI compatibility shims, and renderer cutover rollback code intact.

## Baseline

Commands run from the cleanup worktree with the Mise Go binary:

```bash
/Users/victor.palma/.local/share/mise/installs/go/1.26.3/bin/go vet ./...
/Users/victor.palma/.local/share/mise/installs/go/1.26.3/bin/go test ./internal/di ./tests/features/steps -count=1
/Users/victor.palma/.local/share/mise/installs/go/1.26.3/bin/go run golang.org/x/tools/cmd/deadcode@latest -test -json ./... | jq '[.[].Funcs | length] | add'
/Users/victor.palma/.local/share/mise/installs/go/1.26.3/bin/go run github.com/mibk/dupl@latest -threshold 160 -plumbing cmd internal tests | wc -l
```

Results before edits:

* `go vet ./...`: passed.
* `go test ./internal/di ./tests/features/steps -count=1`: failed with the pre-existing `internal/di` config-path failures and BDD failures around missing `config` command / cluster path expectations.
* `deadcode -test`: 505 unreachable functions.
* `dupl -threshold 160`: 55 duplicate-fragment lines.

## Remove Now

* Removed the unreferenced `internal/util/template` package. No Go package imported it; active template rendering lives under `internal/gitops` and `internal/template`.
* Trimmed `internal/util/files` to the atomic write helper still used by `internal/sops` and `internal/util/crypto`.
* Updated `docs/explanation/architecture.md` so the template-engine evidence no longer points at the removed package.

## Refactor Duplicate

* Collapsed duplicated `ConfigurationManager.Load` and `LoadWithoutValidation` logic into one private `loadFromCacheOrDisk` helper.
* Replaced the repeated Bash/Fish/PowerShell OpenStack environment rendering branches with one ordered environment list and shell-specific formatting.

## Defer With Reason

* `cmd` dead-code findings are deferred because command registration affects public CLI behavior, hidden compatibility commands, and generated reference docs.
* `internal/gitops` findings such as descriptor-renderer and deprecated renderer helpers are deferred because `docs/dev/services-rendering-parity-plan.md` still gates legacy renderer cleanup on cutover owner approval.
* `internal/config/flags`, `internal/util/security`, and `internal/template` findings remain for later package-level review. Several are exported internal helper APIs with tests or feature-specific call paths, so they should be removed only after package owners confirm they are not planned compatibility seams.
* Remaining duplicate findings in config validation, storage prompts, provider validation, and property tests are intentionally left for smaller behavior-preserving refactors with focused tests.

## Current Signal

After this pass:

* `deadcode -test`: 401 unreachable functions.
* `dupl -threshold 160`: 49 duplicate-fragment lines.
* Focused tests passed for `./internal/config/...`, `./internal/util/...`, `./internal/sops`, and `./internal/credentials`.