---
id: configuration-precedence
title: "Configuration Precedence"
sidebar_label: Config Precedence
description: How flags, environment variables, config files, and built-in defaults interact.
doc_type: reference
audience: "all users"
tags: [configuration, precedence, flags, environment, defaults]
---

# Configuration Precedence

**Purpose:** For all users, documents the exact order in which the CLI resolves configuration values from flags, environment variables, files, and built-in defaults.

## General Rule

When the same setting is specified in multiple places, the CLI uses the value from the highest-precedence source:

```
1. Command-line flags        (highest)
2. Environment variables
3. Cluster config file       (.<cluster>-config.yaml)
4. CLI config file           (~/.config/opencenter/config.yaml)
5. Built-in defaults         (lowest)
```

A higher-numbered source is used only when no higher source provides a value.

## Detailed Breakdown

### 1. Command-Line Flags

Flags always win. Two categories:

**Persistent flags** (available on every command):

| Flag | Type | Description |
|------|------|-------------|
| `--log-level` | string | Log level: `debug`, `info`, `warn`, `error` |
| `--dry-run` | bool | Preview without executing |
| `--yes` | bool | Auto-confirm destructive operations |
| `--break-lock` | bool | Force removal of existing lock |

**the set override mechanism** overrides individual fields using dot-path notation:

```bash
opencenter cluster set my-cluster opencenter.meta.env=staging
```

Evidence: `cmd/root.go` — `addGlobalFlags()`, `parseGlobalFlags()`

### 2. Environment Variables

Environment variables override config file values but lose to explicit flags.

**Log level example** — the code checks whether `--log-level` is still at its default (`warn`). If so, it reads `OPENCENTER_LOG_LEVEL`. If the flag was explicitly set to any value, the env var is ignored:

```go
// Precedence: flag > env var > default ("warn")
if globalFlags.LogLevel == "warn" {
    if envLevel := os.Getenv("OPENCENTER_LOG_LEVEL"); envLevel != "" {
        globalFlags.LogLevel = envLevel
    }
}
```

Evidence: `cmd/root.go` lines 165–171

### 3. Cluster Configuration File

The per-cluster config at `<clustersDir>/<organization>/.<cluster>-config.yaml`. This is the primary source for cluster-specific settings (provider, networking, services, secrets, gitops).

Loaded by `ConfigurationManager.Load()` which resolves the path via the `PathResolver`.

Evidence: `internal/config/manager.go` — `Load()`

### 4. CLI Configuration File

The user-level CLI config at `~/.config/opencenter/config.yaml`. Provides defaults that apply across all clusters:

```yaml
logging:
    level: warn
paths:
    configDir: ~/.config/opencenter
    clustersDir: ~/.config/opencenter/clusters
    pluginsDir: ~/.config/opencenter/plugins
defaults:
    provider: openstack
    region: dfw3
    environment: dev
```

Evidence: `internal/config/cli_config.go` — `DefaultCLIConfig()`, `NewConfigManager()`

### 5. Built-In Defaults

Hard-coded in the Go source. Applied when no other source provides a value:

| Setting | Default | Source |
|---------|---------|--------|
| `logging.level` | `warn` | `DefaultCLIConfig()` |
| `logging.format` | `text` | `DefaultCLIConfig()` |
| `defaults.provider` | `openstack` | `DefaultCLIConfig()` |
| `defaults.region` | `dfw3` | `DefaultCLIConfig()` |
| `defaults.environment` | `dev` | `DefaultCLIConfig()` |
| `gitops.repository.branch` | `main` | `normalize()` in loader |
| `gitops.flux.interval` | `5m` | `normalize()` in loader |

Evidence: `internal/config/cli_config.go` — `DefaultCLIConfig()`, `internal/config/v2/loader.go` — `normalize()`

## Directory Resolution

Directories follow their own precedence chains. Each directory type is resolved independently.

### Config Directory

Determines where cluster configs and secrets are stored.

```
1. OPENCENTER_CONFIG_DIR env var
2. Platform default:
   - macOS/Linux: ~/.config/opencenter
   - Windows: %APPDATA%\opencenter
```

Evidence: `internal/config/persistence/paths.go` — `DefaultConfigDir()`

### Clusters Directory

Determines the base path for all organization and cluster directories.

```
1. CLI config paths.clustersDir   (from ~/.config/opencenter/config.yaml)
2. OPENCENTER_CONFIG_DIR + /clusters
3. <DefaultConfigDir>/clusters
```

The CLI config value takes priority even when `OPENCENTER_CONFIG_DIR` is set. This allows pointing the config dir at one location while storing clusters elsewhere.

Evidence: `internal/config/cli_config_helpers.go` — `ResolveClustersDir()`

### State Directory

Stores runtime artifacts: audit logs, bootstrap state, file locks.

```
1. OPENCENTER_STATE_DIR env var
2. CLI config paths.stateDir
3. XDG_STATE_HOME + /opencenter
4. Platform default:
   - macOS/Linux: ~/.local/state/opencenter
   - Windows: %LOCALAPPDATA%\opencenter\state
```

Evidence: `internal/config/persistence/paths.go` — `DefaultStateDir()`

### Plugins Directory

```
1. CLI config paths.pluginsDir
2. <DefaultConfigDir>/plugins
```

Evidence: `internal/config/cli_config_helpers.go` — `GetPluginsDir()`

## Cluster Path Resolution

When a command receives a cluster identifier, the resolver uses two strategies depending on format:

**With organization** (`opencenter-cloud/gizmo`):

```
<clustersDir>/<organization>/infrastructure/clusters/<cluster>/
```

Uses `PathResolver.Resolve()` — scoped to the specified organization.

**Without organization** (`gizmo`):

Scans all organization directories under `clustersDir` looking for a matching `infrastructure/clusters/<cluster>/` directory. Uses `PathResolver.ResolveWithFallback()`.

Evidence: `internal/core/paths/resolver.go` — `Resolve()`, `ResolveWithFallback()`

## Provider Credential Resolution

Cloud provider credentials follow provider-specific precedence. The CLI does not invent its own credential chain — it defers to the provider's standard mechanism.

### OpenStack

```
1. flags (e.g., secrets.global.openstack_username=...)
2. Cluster config secrets.global.openstack_* fields
3. OS_* environment variables (OS_CLOUD, OS_AUTH_URL, OS_USERNAME, etc.)
4. clouds.yaml (~/.config/openstack/clouds.yaml)
```

### AWS (for service integrations)

```
1. flags
2. Cluster config secrets.global.aws_* fields
3. AWS_* environment variables
4. AWS credentials file (~/.aws/credentials)
5. IAM instance role (EC2/ECS)
```

### VMware

```
1. flags
2. Cluster config cloud.vmware.* fields
3. VSPHERE_* environment variables
```

## Debugging Precedence

Run any command with `--log-level debug` to see which sources the CLI is reading:

```bash
opencenter cluster use opencenter-cloud/gizmo --log-level debug
```

The debug output includes:

```
=== OpenCenter CLI Debug Information ===
Command: opencenter cluster use
Environment Variables:
  OPENCENTER_CONFIG_DIR: /custom/path  (or "not set")
  OPENCENTER_LOG_LEVEL: debug          (or "not set")
Configuration Paths:
  Clusters Directory: /Users/you/.config/opencenter/clusters
Global Flags:
  --log-level: debug========================================
```

Evidence: `cmd/root.go` — `PersistentPreRunE`

## Common Pitfalls

**CLI config `clustersDir` vs `OPENCENTER_CONFIG_DIR`:** Setting `OPENCENTER_CONFIG_DIR` does not automatically change `clustersDir`. If your CLI config has an explicit `paths.clustersDir`, that value takes priority. To override both, either update the CLI config or set `clustersDir` to empty so the env var fallback kicks in.

**`--log-level` default masking:** The env var `OPENCENTER_LOG_LEVEL` is only read when the flag is at its default value (`warn`). If you pass `--log-level warn` explicitly, the CLI treats it as "flag was set" and the env var is still read (because the value matches the default). This is a Cobra limitation — the CLI checks the value, not whether the flag was explicitly passed.

**the config file flag scope:** The the config file flag provides an alternative cluster configuration file path. It does not change the CLI config (`~/.config/opencenter/config.yaml`) or the clusters directory.

## Related Topics

- [Environment Variables](environment-variables.md) — Full env var reference
- [File Locations](file-locations.md) — All file paths
- [Configuration Schema](configuration-schema.md) — Cluster config structure
- [CLI Commands](cli-commands.md) — Flag reference per command
