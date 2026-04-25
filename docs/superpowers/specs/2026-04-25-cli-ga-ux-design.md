# CLI GA UX Design

Date: 2026-04-25

## Summary

This design reshapes the openCenter CLI command and flag surface for a cleaner GA user experience. It allows breaking command and flag renames in favor of a workflow-oriented model with fewer overlapping commands, consistent global flags, predictable output formats, and more useful human output.

The command tree keeps `cluster`, `config`, `secrets`, and `plugins` as the main domains, but renames and consolidates cluster lifecycle commands around the words users naturally use while operating clusters:

```text
init -> configure -> validate -> doctor -> generate -> deploy -> operate
```

The design also defines which flags are truly global, which flags must be scoped to specific commands, and how commands should report plans, changes, skipped work, paths, and next steps.

## Goals

- Produce a GA command surface that is easier to learn and remember.
- Rename commands when the current name exposes implementation detail or overlaps another command.
- Consolidate duplicated functionality such as `setup` versus `render`, `cluster update` versus `cluster config update`, and cluster-level key commands versus `secrets keys`.
- Make global flags actually global only when their behavior is meaningful across the command tree.
- Standardize output flags across commands.
- Improve default human output for mutating commands, validation commands, and error messages.
- Keep the design grounded in the existing Go/Cobra implementation and generated docs.
- Add tests that prevent command docs and help output from drifting again.

## Non-Goals

- Preserve backwards-compatible command aliases as part of the GA surface.
- Redesign the underlying cluster configuration schema.
- Replace the Cobra command framework.
- Redesign external plugin binaries such as `opencenter-rmpk`.
- Implement the changes in this design document.

## Current Problems

The current command tree has grown organically and now exposes several internal distinctions to users.

### Overlapping Lifecycle Commands

`cluster setup`, `cluster render`, and `cluster bootstrap` describe implementation phases rather than user goals. `setup` generates repository structure, `render` renders templates into that repository, and `bootstrap` provisions or deploys the cluster. Users see a lifecycle, but the command names make them learn internal phase names.

The README and workflow docs still reference `cluster setup --render`, while the live `cluster setup` command does not expose a `--render` flag. This is a concrete example of command semantics drifting across docs, tests, and implementation.

### Ambiguous Configuration Commands

There are three different configuration surfaces:

```text
opencenter config ...
opencenter cluster update ...
opencenter cluster config update ...
```

They do different things:

- `config` manages CLI defaults.
- `cluster update` mutates specific cluster fields through dotted flags.
- `cluster config update` adds missing default fields to a cluster config.

The names do not make those boundaries clear.

### Inconsistent Output Flags

Commands currently use a mix of:

```text
--json
--format table|json|yaml
--output text|json|yaml
--output <file>
--out <file>
--output-file <file>
```

This creates avoidable cognitive load and makes scripting inconsistent.

### Global Flags Are Over-Advertised

The root help advertises these global flags:

```text
--config
--config-dir
--dry-run
--log-level
--set
--show-active
--break-lock
```

Only some behave as true global flags:

- `--log-level` is parsed and applied globally.
- `--config-dir` is pre-parsed before plugin discovery and affects config path resolution.
- `--dry-run` is inherited by all commands, but only commands that explicitly read it implement preview behavior.
- `--config` collides with command-local meanings such as `cluster init --config` and `cluster validate --config`.
- `--set` is parsed globally but does not act as a reliable global override contract across all relevant commands.
- `--show-active` is parsed but does not provide a clear command-wide behavior.
- `--break-lock` only makes sense for mutating cluster operations that acquire operation locks.

### Docs and Live Help Drift

The generated reference, README, and tutorial docs contain stale command examples. Examples include `cluster setup --render`, `cluster config get/set`, `opencenter sops ...`, and `cluster select --activate`. The live command tree also contains commands such as `cluster import` and hidden `cluster credentials` that are not represented consistently in the hand-authored command reference.

## Command Model

The GA command surface should be organized by user intent, not implementation phase.

### Root Domains

```text
opencenter cluster     cluster lifecycle and operations
opencenter config      local CLI defaults and paths
opencenter secrets     secret values, encryption, keys, and hooks
opencenter plugins     external command plugin discovery and verification
opencenter completion  shell completion
opencenter version     version and build metadata
opencenter shell-init  shell integration for active cluster sessions
```

External plugin commands remain top-level delegated commands discovered from `opencenter-<name>` executables. Built-in commands continue to take precedence over plugins.

### Cluster Commands

```text
opencenter cluster init [name]
opencenter cluster configure [cluster]
opencenter cluster edit [cluster]
opencenter cluster set [cluster] <path=value>...
opencenter cluster normalize [cluster]
opencenter cluster export [cluster]
opencenter cluster validate [cluster]
opencenter cluster doctor [cluster]
opencenter cluster generate [cluster]
opencenter cluster deploy [cluster]
opencenter cluster status [cluster]
opencenter cluster describe [cluster]
opencenter cluster list
opencenter cluster use [cluster]
opencenter cluster active
opencenter cluster env [cluster]
opencenter cluster destroy [cluster]
opencenter cluster service ...
opencenter cluster drift ...
opencenter cluster backup ...
opencenter cluster import ...
```

#### Responsibilities

- `cluster init` creates a new non-interactive cluster config skeleton.
- `cluster configure` runs the guided provider-aware configuration workflow.
- `cluster edit` opens an existing cluster config in the user's editor.
- `cluster set` changes one or more explicit config fields.
- `cluster normalize` adds missing default fields while preserving existing values.
- `cluster export` writes or prints the fully resolved effective cluster config.
- `cluster validate` validates cluster configuration and, with flags, generated manifests.
- `cluster doctor` checks local tools, credentials, provider readiness, and environment prerequisites.
- `cluster generate` creates or updates the GitOps repository and rendered manifests.
- `cluster deploy` provisions and bootstraps the cluster.
- `cluster status` shows lifecycle progress and concise runtime state.
- `cluster describe` shows detailed config, paths, locks, active state, and diagnostics.
- `cluster use` sets the active cluster.
- `cluster active` prints the active cluster and selection source.
- `cluster env` prints shell exports for the selected or specified cluster.

## Command Renames and Consolidation

Breaking renames are allowed for GA. The old names should be removed from normal help, docs, and the public command tree. No long-lived compatibility aliases or hidden old-name shims are part of this design.

| Current command | GA command | Rationale |
|---|---|---|
| `cluster setup` | `cluster generate` | Names the user goal: generate GitOps assets. |
| `cluster render` | `cluster generate --render-only` | Rendering remains available as an explicit generation mode without a separate public command. |
| `cluster bootstrap` | `cluster deploy` | Users deploy clusters; bootstrap is internal phase language. |
| `cluster preflight` | `cluster doctor` | Matches diagnostic intent and broader checks. |
| `cluster info` | `cluster describe` | Existing command already provides detailed description. |
| `cluster select` | `cluster use` | Better shell workflow phrasing: use this cluster. |
| `cluster current` | `cluster active` | Names the active-cluster concept directly. |
| `cluster update` | `cluster set` | Makes field mutation explicit. |
| `cluster config update` | `cluster normalize` | Describes adding missing default fields. |
| `cluster config export-effective` | `cluster export` | Shorter and clearer. |
| `cluster validate-manifests` | `cluster validate --manifests` | Manifest validation is a validation mode. |
| `cluster sync-status` | `cluster status --sync` | Keeps status ownership together. |
| `cluster check-keys` | `secrets keys check` | Key health belongs with secrets. |
| `cluster rotate-keys` | `secrets keys rotate --cluster <cluster>` | Key rotation belongs with secrets, scoped by cluster when needed. |
| `cluster revoke-key` | `secrets keys revoke --cluster <cluster>` | Key revocation belongs with secrets. |
| `cluster install-hooks` | `secrets hooks install` | Hooks protect secrets workflow. |

`cluster config` should be removed as a public subgroup. Its useful functionality should move to `cluster set`, `cluster normalize`, and `cluster export`.

`cluster import scan/report/apply` should remain. It maps cleanly to the existing artifact workflow and should be promoted in docs as the import/adoption flow.

Hidden `cluster credentials` should be removed. Its environment export behavior should be folded into `cluster env`. The GA public path for environment activation should be:

```bash
eval "$(opencenter cluster env prod)"
```

or:

```bash
opencenter cluster use prod
eval "$(opencenter cluster env)"
```

## Global Flag Contract

Global flags should be limited to behavior that can be applied consistently by root-level command wiring or shared command helpers.

### GA Global Flags

```text
--config-dir <dir>          root openCenter config directory
--log-level <level>         debug|info|warn|error
--output <format>           text|json|yaml where structured output is supported
--quiet                     suppress nonessential human output
--yes                       answer yes to confirmation prompts
--dry-run                   preview mutating operations without writing or acting
```

### Flags Removed or Scoped

| Current flag | GA treatment | Rationale |
|---|---|---|
| `--config` | Remove as global; use `--cluster-config` or `--file` on specific commands | Avoids collision with CLI config and command-local file input. |
| `--set` | Remove as global; use `cluster set` or command-specific `--override path=value` | Global override semantics are too broad and currently inconsistent. |
| `--show-active` | Remove | Use `cluster active`; include active context in normal human output when useful. |
| `--break-lock` | Scope to mutating cluster commands | It only applies where operation locks are acquired. |

### Cluster Targeting

Cluster commands should support a consistent target model:

```text
<cluster>             preferred human form where natural
--cluster <org/name>  accepted where a positional cluster would be awkward
--org <name>          creation and filtering only
--all                 all clusters, only where meaningful
```

Commands that act on an existing cluster should resolve target in this order:

1. Explicit positional cluster.
2. Explicit `--cluster`.
3. Active cluster.
4. Clear error with fix commands.

### Output Flags

Output destination and output format must not share the same flag name.

```text
--output text|json|yaml      structured output format
-o, --output-file <path>     destination file
--file <path>                input file where appropriate
--cluster-config <path>      explicit cluster config input where appropriate
```

This replaces command-specific variants such as `--json`, `--format`, `--out`, and file-oriented `--output`.

### Dry-Run Semantics

`--dry-run` is meaningful only for mutating commands. A mutating command with `--dry-run` must not:

- write files
- change the active cluster
- acquire or break persistent locks except in a pure preview path
- run provider provisioning
- run destructive actions
- push to remotes
- create commits

Read-only commands should not advertise `--dry-run` in command-specific help. If inherited global help still shows it, read-only commands should reject it with a clear message when it is explicitly set:

```text
Error: --dry-run has no effect for read-only command "cluster list"
```

## Command Output Design

Human output should be concise by default and more informative where a command mutates state or performs a multi-step workflow.

### Mutating Commands

Mutating commands should report:

- target cluster or organization
- config file path
- GitOps path or affected directory
- files created, changed, skipped, or backed up
- external commands that were run or would run
- status changes
- next recommended command

Example:

```text
Plan: generate GitOps assets for acme/prod
Config: ~/.config/opencenter/clusters/acme/.prod-config.yaml
GitOps path: ~/platform/prod

Would create:
  applications/overlays/prod/kustomization.yaml
  infrastructure/clusters/prod/main.tf

Would skip:
  git commit: dry-run
  file writes: dry-run

Next: opencenter cluster generate prod
```

### Read-Only Commands

Read-only commands should default to compact human output and support structured output where data exists.

Examples:

```bash
opencenter cluster list --output text
opencenter cluster list --output json
opencenter secrets list --output yaml
opencenter cluster describe prod --output json
```

`--quiet` should produce script-friendly output for commands with a single primary value, such as `cluster active`.

### Parent Command Help

Parent commands should show workflows, not just command lists.

For `opencenter cluster`, help should include a short journey:

```text
Common workflow:
  opencenter cluster init prod --org acme
  opencenter cluster configure acme/prod
  opencenter cluster validate acme/prod
  opencenter cluster doctor acme/prod
  opencenter cluster generate acme/prod
  opencenter cluster deploy acme/prod
```

## Error Message Design

Errors should include the problem, likely cause when known, and concrete fix commands.

Example:

```text
Error: no active cluster is set

Fix:
  opencenter cluster list
  opencenter cluster use <org/name>

Or pass a cluster explicitly:
  opencenter cluster validate <org/name>
```

Removed command names should not remain registered as hidden commands. Replacement guidance belongs in the migration notes, docs, and tests that prevent stale examples from returning.

## Secrets Command Ownership

Secrets should own encryption, secret values, SOPS operations, key lifecycle, and hooks.

```text
opencenter secrets list|get|set|delete|describe
opencenter secrets sync|validate
opencenter secrets encrypt|decrypt|status
opencenter secrets keys generate|rotate|backup|validate|check|revoke
opencenter secrets hooks install
opencenter secrets login
```

Cluster-specific key behavior should be modeled through explicit cluster scoping:

```bash
opencenter secrets keys rotate --cluster acme/prod --type age
opencenter secrets keys check --cluster acme/prod
opencenter secrets keys revoke --cluster acme/prod --user user@example.com
```

## Plugin Behavior

The core CLI cannot enforce flags for external plugins because plugin commands use `DisableFlagParsing` and forward arguments directly to the plugin executable. The GA design should keep that simple plugin contract.

The built-in `plugins` command should become more useful:

```text
opencenter plugins list --output text|json|yaml
opencenter plugins verify
opencenter plugins path
```

`plugins list` should show plugin command name, executable path, verification status, and why a plugin is unverified or skipped.

## Documentation Requirements

The README, tutorials, how-to guides, generated command reference, and command examples must be updated together.

Required documentation updates:

- Replace `cluster setup --render` with `cluster generate`.
- Replace `cluster bootstrap` with `cluster deploy`.
- Replace `cluster select/current` with `cluster use/active`.
- Replace `cluster info` with `cluster describe`.
- Replace `cluster update` and `cluster config update` with `cluster set` and `cluster normalize`.
- Replace `--json`, `--format`, and file-oriented `--output` examples with the GA flag model.
- Remove `opencenter sops ...` examples and use `opencenter secrets ...`.
- Document `cluster import scan/report/apply` in the main CLI reference.
- Document the global flag contract and command-specific exceptions.

The generated docs should be treated as the source of command inventory truth, while hand-authored docs should link to that reference and use tested examples.

## Testing Strategy

The implementation plan should add tests before or alongside command changes.

### Command Inventory Tests

Add a golden test for the public command tree:

- command path
- aliases, if any
- hidden status
- flags
- inherited flags
- short description

This catches accidental public command drift.

### Global Flag Matrix

Test global flags across representative commands:

- root help
- `cluster list`
- `cluster generate`
- `cluster deploy`
- `cluster destroy`
- `config get`
- `secrets list`
- `plugins list`

The matrix should assert which global flags are accepted, rejected, or meaningful.

### Dry-Run Tests

For each mutating workflow, test that `--dry-run` changes output but does not change state:

- no files written
- no active cluster changed
- no lock broken
- no commits created
- no remotes pushed
- no provider action executed

### Output Format Tests

For commands that support structured data, test:

- `--output text`
- `--output json`
- `--output yaml`
- invalid output values
- `--quiet` when applicable

### Removed Command Tests

For renamed commands, test that old public commands are absent from help and unavailable in the command tree.

### Documentation Drift Tests

Add a docs check that scans README and docs examples for removed command names and removed flags. This should fail CI when stale examples reappear.

## Implementation Boundaries

This design should be implemented in phases:

1. Shared flag and output helpers.
2. Command rename/consolidation with tests.
3. Output and error-message improvements.
4. Docs regeneration and hand-authored docs cleanup.
5. Docs regeneration and command-reference verification.

The implementation should avoid unrelated refactors in cluster configuration internals, provider logic, template rendering internals, or secret backend behavior except where needed to support the renamed commands.

## Open Decisions Resolved

- Breaking command and flag renames are allowed for the GA surface.
- The recommended approach is workflow-oriented consolidation, not a minimal cleanup and not a full `kubectl`-style resource model.
- `cluster` remains the primary domain for lifecycle operations.
- `secrets` owns secret values, encryption, key lifecycle, and hooks.
- `--output` means output format; file destinations use `--output-file` or command-specific file flags.
- `--config` is not a global GA flag.
- `--set` is not a global GA flag.
