---
last_updated: 2026-09-26
id: configuration-precedence
title: "Configuration Precedence"
sidebar_label: Configuration Precedence
description: The two distinct precedence systems in openCenter -- generic configuration-file merging, and CLI-tool path resolution.
doc_type: reference
audience: "operators, developers"
tags: [configuration, precedence, cli, reference]
---
# Configuration Precedence

**Purpose:** For operators and developers, explains how openCenter resolves a value when it could come from more than one place. There are two independent precedence systems -- do not conflate them.

## 1. Generic configuration-file merging (`internal/config/flags/configuration_merger.go`)

`DefaultConfigurationMerger` is the generic merger used by `CLIIntegration.applyConfigFileFlags` when CLI flag handling loads and combines configuration files. It is **not** the general cluster-init layering pipeline. `InitService` instead loads an explicit `--config-file` or creates a v2 default, then applies initialization options in `loadOrCreateConfig` and `applyOverrides`; that path does not call `DefaultConfigurationMerger`.

The merger's default strategy orders source types as follows, lowest to highest precedence:

```
SourceDefault  (lowest)
SourceFile
SourceTemplate
SourceCLI      (highest -- CLI flags always win)
```

The source types are generic merger metadata. A particular caller may provide only a subset of them; their presence should not be inferred from this strategy.

`InitService.applyOverrides` (see [Cluster Init Details](../contributing/cluster-init-details.md)) also tracks which values were set explicitly (a map of "was this key touched by the user") specifically so that later path-resolution and Git-auth-default logic never silently overwrites a user-supplied value.

## 2. CLI-tool path precedence (per-directory, `internal/config/cli_settings_helpers.go`)

This is a *completely separate* precedence system that resolves *where on disk* the CLI reads/writes things -- it has nothing to do with cluster config field values. For every directory role (clusters, GitOps, blueprints, cluster state, secrets, plugins, general state), the resolution order is the same three steps:

```
1. The role's OPENCENTER_<X>_DIR environment variable, if set
2. The matching `paths.<x>Dir` value in `<config-dir>/settings.yaml` (the CLI settings file)
3. A computed default (usually <clustersDir>/<role>, or a platform-specific base for clustersDir/configDir/stateDir themselves)
```

See [File Locations](file-locations.md) for the exact default path and environment variable for every role, and [Environment Variables](environment-variables.md) for the full list of recognized variables.

No environment variable overrides a cluster-config *field* value directly -- environment variables in this system only ever change *where files live*, never what a loaded cluster config's fields contain. The one partial exception is `OPENCENTER_DEBUG`, which changes CLI *behavior* (verbose logging, and triggers `cluster validate`'s debug-config export) rather than a config field.

## Practical implications

* Changing `OPENCENTER_CONFIG_DIR` mid-project does not change any value inside an already-loaded cluster config file; it changes which `settings.yaml`/`clusters/` tree the CLI looks at next. Cluster-init defaults have a separate compatibility read of `<config-dir>/config.yaml`; see [Default Values](default-values.md).
* A dotted CLI override on `cluster init`/`cluster set` is applied by the command's override path. If a value looks wrong after one of these commands, check the exact flags passed before suspecting a stale file.
* `cluster configure --guided` reuses `InitService`'s `createDefaultConfig`, `applyOverrides`, and `updateConfigPaths` when no config exists yet. It therefore follows that initialization path, not the generic merger's source list.
