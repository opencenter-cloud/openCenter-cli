---
last_updated: 2026-11-02
id: testing-guide
title: "Testing Guide"
sidebar_label: Testing Guide
description: Write and run unit, BDD, and property-based tests for openCenter-cli.
doc_type: how-to
audience: "developers"
tags: [contributing, testing]
---
# Testing Guide

**Purpose:** For developers, shows how to write and run tests for openCenter-cli.

## Test types

openCenter-cli uses four kinds of Go tests, all driven through `mise` tasks defined in `.mise.toml`:

1. **Unit tests** -- `*_test.go` files next to the source they test.
2. **Property-based tests** -- `*_property_test.go` files using [`gopter`](https://github.com/leanovate/gopter) (`go.mod`: `github.com/leanovate/gopter v0.2.11`).
3. **BDD tests** -- Gherkin scenarios in `tests/features/*.feature`, run through [Godog](https://github.com/cucumber/godog) (`go.mod`: `github.com/cucumber/godog v0.16.0`).
4. **Integration tests** -- `*_integration_test.go` files that exercise multi-package flows (cluster provisioning, resilience, operations).

## Running tests

```bash
# Unit tests: internal/config/..., cmd/..., internal/cloud/...
mise run test

# Full package suite under the race detector
mise run test-race

# Compile every package without producing binaries
mise run test-build

# BDD scenarios, excluding @wip
mise run godog

# Only @wip scenarios
mise run godog-wip

# BDD scenarios filtered by a single tag
mise run godog-tag <tag>          # e.g. mise run godog-tag keycloak

# All property-based tests (TestProperty*) across internal/... and cmd/...
mise run property
# alias used in this doc set and elsewhere:
mise run test-properties

# Vulnerability scan (govulncheck ./...)
mise run govulncheck

# Secret scan across full git history
mise run gitleaks

# Integration tests: cluster provisioning, resilience, operations
mise run integration

# Documentation generator test (requires the `tools` build tag)
mise run test-docs

# Full local Go package sweep, with a 15-minute Go test timeout
# On failure, a transient failure log is written under .tmp/ for diagnosis
mise run test-go-all

# Doc generation idempotency check
mise run test-docs-idempotency

# Kustomize build check for every generated default overlay
mise run test-kustomize

# Whitespace-error check on the working tree diff
mise run test-diff

# Aggregate checks: unit + race + vet + BDD + property + vulncheck
mise run test:all

# The subset developers should run before pushing
mise run verify
```

`mise run test` only covers `internal/config/...`, `cmd/...`, and `internal/cloud/...` -- it is not the full suite. `mise run test-go-all` is the local full-repository Go sweep; it applies Go's 15-minute test timeout and, on failure, writes a transient failure log under `.tmp/` for diagnosis rather than keeping a repository artifact. It is separate from `mise run test:all`, whose aggregate is the unit + race + vet + BDD + property + vulnerability-check sequence and does not replace either repository documentation checks or the full-package sweep. Use `go test ./<package>` directly to scope to one package during development.

### Documentation checks and publication boundary

`mise run test-docs` is intentionally repository-only. In addition to the
documentation-generator tests, it checks maintained Markdown structure,
frontmatter, local links, and Mermaid fence structure. A passing Mermaid check
is evidence only that the fences meet the repository's structural rules; it
does **not** render diagrams or verify their visual output. The repository does
not configure a documentation publication or deployment platform, so this task
does not publish a site.

### Running one package or one test

```bash
go test -v ./internal/config
go test -v -cover ./internal/config
go test -v ./internal/config -run TestValidateClusterName
```

## Writing unit tests

Create test files alongside the source file:

```
internal/config/v2/
├── config.go
├── config_test.go            # unit tests
└── config_property_test.go   # property tests
```

Use table-driven tests with `testify` (`github.com/stretchr/testify`, already a direct dependency):

```go
package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateClusterName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "my-cluster", wantErr: false},
		{name: "invalid characters", input: "my_cluster!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClusterName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

Store fixtures under a package-local `testdata/` directory, or under the repository-root `testdata/` (used for multi-package fixtures such as `testdata/example-inc/` and `testdata/config/`).

## Writing BDD tests

Feature files live in `tests/features/*.feature` (there are eight: `cli_config.feature`, `cluster_generate_deploy.feature`, `cluster_init.feature`, `cluster_selection.feature`, `config_template_rendering.feature`, `secrets.feature`, `validation.feature`, `workflow.feature`). Step definitions live in `tests/features/steps/` (`helpers.go`, `steps_test.go`).

```gherkin
Feature: Cluster Initialization
  Scenario: Initialize cluster with defaults
    When I run "opencenter cluster init demo --org my-org"
    Then the command should succeed
    And a configuration file should exist at "blueprints/my-org/demo/demo-config.yaml"

  @wip
  Scenario: Initialize cluster with invalid name
    When I run "opencenter cluster init invalid_name --org my-org"
    Then the command should fail
```

Tag scenarios you are actively working on with `@wip` -- `mise run godog` excludes them by default (`--godog.tags=~@wip`), and `mise run godog-wip` runs only them. The repository also carries many feature-specific tags (`@init`, `@deploy`, `@keycloak`, `@cert-manager`, `@backup`, `@drift`, and so on); run a single tag with `mise run godog-tag <tag>`.

`hack/tag_wip_failures.py` (invoked via `mise run tag-wip-failures`) automatically appends `@wip` to scenarios that are currently failing, so a red BDD run can be triaged without blocking unrelated work -- do not leave scenarios tagged this way permanently; remove the tag once the scenario is fixed.

## Writing property-based tests

Property tests assert an invariant holds for many generated inputs, using `gopter`:

```go
package v2

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestPropertyClusterNameValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("names with invalid characters always fail", prop.ForAll(
		func(name string) bool {
			return ValidateClusterName(name+"!") != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}
```

Name the test function so it matches `TestProperty*` -- `mise run property` filters on `-run "TestProperty"`.

## Test best practices

**Do:** test behavior rather than implementation; use table-driven tests; cover edge cases (empty strings, nil, boundary values) and error paths; give tests descriptive names (`TestValidateClusterName_WithInvalidCharacters`); keep unit tests fast (milliseconds); store fixtures in `testdata/`; clean up temporary files/directories in `defer` or `t.Cleanup`.

**Don't:** call real external services (mock cloud providers/APIs); re-test third-party libraries; make tests depend on execution order; use real credentials in fixtures (`internal/security/credential_masker.go` and `internal/util/security/credential_masker.go` exist specifically so secrets never need to appear in test fixtures or logs); skip cleanup.

## Debugging tests

```bash
go test -v ./internal/config -run TestValidateClusterName

# Verbose debug logging (see mise.toml env section)
OPENCENTER_DEBUG=true go test -v ./internal/config

# Delve
go install github.com/go-delve/delve/cmd/dlv@latest
dlv test ./internal/config -- -test.run TestValidateClusterName
```

## What CI actually runs

CI coverage is defined entirely by `.github/workflows/*.yml`; see [GitHub Actions Workflows](../reference/github-actions-workflows.md) for the complete, verified breakdown of triggers, jobs, and steps. In short:

* `test.yml` runs on pull requests and pushes to `main`: a `go-test` job that directly runs `go test ./internal/... ./cmd/... -count=1 -race` and `go vet ./...`, plus an independent `property-tests` job using the same package scope and `-run 'TestProperty'`.
* `pre-commit.yaml` runs the pre-commit hook set for changed files on every pull request.
* `vulncheck.yml` runs `govulncheck ./...` on pull requests, on a weekly schedule, and on manual dispatch.
* `docs-p0.yml` runs for pull requests that touch Markdown files; it audits the changed files with `audit_doc_frontmatter.py --strict`, checks every page's `last_updated` against its last body change, runs Vale, and runs `mise run test-docs`.
* `deploy-kind.yml` is a manually dispatched, disposable Kind + Gitea end-to-end workflow -- it is not a per-commit gate.

The general test workflow does **not** run the BDD suite, the `integration`
task, the documentation-generator tests, the Kustomize check,
`mise run test-go-all`, or `mise run test:all`; the docs workflow runs
`mise run test-docs` for documentation pull requests. Run the relevant checks
locally before opening a PR:

```bash
mise run godog
mise run integration
mise run test-docs
mise run test-docs-idempotency
mise run test-go-all
mise run test:all
```

## Coverage

```bash
go test -v -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

There is no enforced coverage gate in CI; treat validation, security, and secrets-handling code as needing the most thorough coverage since regressions there are the highest-impact.
