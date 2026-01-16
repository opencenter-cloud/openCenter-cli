# BDD Test Suite (Godog)

This suite validates openCenter’s behavior end‑to‑end using Gherkin feature files and Godog step definitions.

## Running the suite

- Skip @wip scenarios (default): `mise run godog`
- Only @wip scenarios: `mise run godog-wip`

## Tag failing scenarios as @wip

Run `mise run tag-wip-failures` to execute the suite in cucumber (JSON) format, detect failing scenarios, and tag them with `@wip` in their `.feature` files. Subsequent runs of `mise run godog` will skip those scenarios by default.

## Priority Tags

Test scenarios are tagged with priority levels (`@priority1` through `@priority8`) to organize the testing-tasks-fix implementation work:

- **@priority1**: Config Loader for Organization Structure (18 failing tests)
  - Core configuration loading and organization-based path resolution
  
- **@priority2**: GitOps Idempotency and Validation (7 failing tests)
  - GitOps setup idempotency checks and validation
  
- **@priority3**: Cluster List and Select Commands (4 failing tests)
  - List and select commands with organization structure support
  
- **@priority4**: Validation Logic (6 failing tests)
  - VRRP validation, service secrets validation, format validation
  
- **@priority5**: Bootstrap Cleanup (1 failing test)
  - Resilient cleanup logic in bootstrap command
  
- **@priority6**: Info Command Output (2 failing tests)
  - Info command output format and help text generation
  
- **@priority7**: Cluster Destroy Command (2 failing tests)
  - Destroy command with organization structure support
  
- **@priority8**: Init Validation (2 failing tests)
  - Minimal validation during init command

Run priority-specific tests using:
- `mise run test-priority1` through `mise run test-priority8`
- `mise run godog -- --godog.tags=@priority1` (alternative syntax)

Conventions
- Organization by feature area and goal:
  - `config_*.feature`: configuration flows (init, update, select/list/info)
  - `gitops_*.feature`: GitOps lifecycle (setup, render, bootstrap)
  
  - `schema.feature`: JSON schema generation
  - `secrets_sops.feature`: SOPS/age helpers and auto‑keygen
  - `preflight.feature`: provider preflight checks (OpenStack, etc.)
  - `destroy.feature`: cluster teardown and safety checks
  - `idempotency_errors.feature`: idempotency and error reporting

- Tags are used for selective runs: `@config`, `@gitops`, `@schema`, `@secrets`, `@preflight`, `@destroy`, `@idempotent`, `@errors`.

- Background blocks set up isolated config directories and temp repos.

- Prefer dotted flags for updates and init overrides, e.g.: `--iac.counts.master=3`.

How to run
- Entire suite (via mise): `mise run godog`
- Only configuration flows: `mise run godog -- --godog.tags=@config`
- Only GitOps flows: `mise run godog -- --godog.tags=@gitops`

Adding new scenarios
- Place under the appropriate `*.feature` file based on behavior.
- Use clear, task‑oriented scenario names and keep steps minimal and reusable.
- If a new step is truly needed, add it to `tests/features/steps/helpers.go` with care for reuse.
