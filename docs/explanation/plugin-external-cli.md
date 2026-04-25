---
id: plugin-external-cli
title: "Plugin External CLI"
sidebar_label: Plugin External CLI
description: How openCenter CLI discovers and runs external command plugins named opencenter-<name>.
doc_type: explanation
audience: "developers, platform engineers"
tags: [plugins, cli, extensions, cobra]
---

# Plugin External CLI

**Purpose:** For developers and platform engineers, explains the external CLI plugin mechanism in openCenter CLI, how plugins are discovered and executed, and when to use this path instead of the internal service plugin system.

## What This Plugin Mechanism Is

The external CLI plugin mechanism extends the `opencenter` command with new top-level subcommands.

It is file-based, not service-based:

- you create an executable named `opencenter-<name>`
- openCenter discovers it at startup
- the executable becomes available as `opencenter <name>`

This mechanism is for adding commands, not for adding platform services such as cert-manager, Loki, Harbor, or Keycloak.

If you want to add a cluster-managed service, use the internal service plugin path described in [Plugin Internal Services](plugin-internal-services.md).

## Discovery Model

At startup, the root command calls the external plugin loader. The loader scans for executables whose names start with `opencenter-`.

Discovery order:

1. `OPENCENTER_PLUGINS_DIR`
2. `<config-dir>/plugins`
3. `PATH`

The root command also pre-parses `--config-dir` before Cobra initialization so plugin discovery can honor a custom config directory early enough.

Evidence:
- `cmd/root.go`
- `internal/plugins/loader.go`

## How a Plugin Becomes a Command

The loader performs a few straightforward steps:

1. Build a set of built-in command names from the existing Cobra tree.
2. Discover executables matching the `opencenter-` prefix.
3. Strip the prefix from the filename.
4. Create a Cobra command with that stripped name.
5. Forward all arguments directly to the external executable.

For example:

- executable: `opencenter-rmpk`
- exposed command: `opencenter rmpk`

Built-in commands always win. If an external plugin would collide with an existing command name, it is skipped rather than overriding the built-in command.

## Execution Behavior

External plugin commands are created with `DisableFlagParsing: true`.

That means:

- Cobra does not parse the plugin's flags locally
- all arguments after the subcommand name are passed straight through
- the plugin executable is responsible for its own argument parsing and help output

The command is executed as a child process with:

- stdout connected to the current terminal
- stderr connected to the current terminal
- stdin connected to the current terminal

If the child exits non-zero, openCenter preserves that failure as an error.

Evidence:
- `internal/plugins/loader.go`

## How To Add an External CLI Plugin

### Step 1: Build an Executable

Create an executable named:

```text
opencenter-my-command
```

### Step 2: Make It Executable

The file must have an execute bit set. Non-executable files are ignored by the loader.

### Step 3: Put It in a Discovery Location

Choose one of:

- `OPENCENTER_PLUGINS_DIR`
- `<config-dir>/plugins`
- any directory on `PATH`

### Step 4: Run It Through the Main CLI

If the executable is named `opencenter-my-command`, run:

```bash
opencenter my-command
```

All remaining arguments are passed to the executable:

```bash
opencenter my-command --flag value arg1 arg2
```

## When To Use This Mechanism

Use the external CLI plugin path when you want to:

- add a new top-level operational command
- integrate an auxiliary tool into the `opencenter` command surface
- ship an extension independently from the main repository
- prototype command workflows without modifying core service logic

Do not use it when you want to:

- add a new platform service rendered into GitOps
- add service-specific config types
- add service validation, dependencies, or service status handling
- generate Flux, Kustomize, or Kubernetes manifests for a cluster service

Those are internal service plugin concerns.

## Operational Constraints

This mechanism is intentionally narrow:

- discovery is filename-based
- there is no plugin manifest
- there is no API handshake
- there is no typed contract beyond "be an executable command"
- built-in commands cannot be replaced

That simplicity is the main design tradeoff: easy to extend, but limited to command delegation.

## Checksum Verification

External plugins can be verified with a checksum allowlist:

- store checksums in `<config-dir>/plugins/checksums.txt`
- use standard `sha256sum` formatting: `<sha256>  <filename>`
- entries are matched by plugin basename
- unverified plugins emit a warning when executed
- checksum mismatches block execution

## Related Reading

- [Create and Install a CLI Plugin](../how-to/create-install-cli-plugin.md)
- [Plugin Internal Services](plugin-internal-services.md)
- [Code Structure](../dev/code-structure.md)
- [CLI Commands](../reference/cli-commands.md)

## Evidence

Primary code paths referenced in this explanation:

- `cmd/root.go`
- `internal/plugins/loader.go`
