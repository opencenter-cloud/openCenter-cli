# BDD Test Suite (Godog)

This suite validates openCenter’s behavior end‑to‑end using Gherkin feature files and Godog step definitions.

## Running the suite

- Skip @wip scenarios (default): `mise run godog`
- Only @wip scenarios: `mise run godog-wip`

## Tag failing scenarios as @wip

Run `mise run tag-wip-failures` to execute the suite in cucumber (JSON) format, detect failing scenarios, and tag them with `@wip` in their `.feature` files. Subsequent runs of `mise run godog` will skip those scenarios by default.

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
