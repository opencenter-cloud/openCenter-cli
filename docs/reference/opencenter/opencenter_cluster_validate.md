---
id: "opencenter-cluster-validate"
title: "Opencenter_Cluster_Validate"
sidebar_label: "Opencenter_Cluster_Validate"
description: "Validate cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster validate

**Purpose:** For operators, developers, documents `opencenter cluster validate`: what it does, its flags, and its examples.

Validate cluster configuration

### Synopsis

Validate cluster configuration against schema and business rules.

This command performs comprehensive validation including:
  • Schema validation against JSON schema
  • Required field validation
  • Cross-field dependency validation
  • GitOps configuration and local repository validation
  • Network configuration validation
  • SOPS key validation
  • Stub secret detection (CHANGEME/PLACEHOLDER values in GitOps manifests)
  • Secret encryption verification (files missing SOPS encryption)

Validation mode is selected from global CLI config behavior.validation
(default: offline) and can be overridden for one run with --validation.
Offline mode does not contact providers, Git remotes, Kubernetes APIs, or
external services. Online mode adds provider discovery/connectivity and Git
remote checks.

Only v2 configurations (schema_version: "2.0") are supported.
Configurations with any other schema version are invalid.

If no cluster name is provided, validates the currently active cluster.

```
opencenter cluster validate [name] [flags]
```

### Examples

```
  # Validate active cluster
  opencenter cluster validate

  # Validate specific cluster
  opencenter cluster validate my-cluster

  # Validate with organization/cluster-name format
  opencenter cluster validate my-org/my-cluster

  # Validate with online provider and Git remote checks
  opencenter cluster validate my-cluster --validation online

  # Validate generated GitOps manifests
  opencenter cluster validate my-cluster --manifests

  # Output as JSON (for CI/CD pipelines)
  opencenter cluster validate my-cluster --output json

  # Validate and generate debug config
  opencenter cluster validate my-cluster --generate-debug-config
```

### Options

```
      --config-file string      path to configuration file to validate
      --generate-debug-config   generate complete config for debugging
  -h, --help                    help for validate
      --manifests               validate generated GitOps manifests
      --output-dir string       directory to save debug config (defaults to current directory)
      --validation string       validation mode for this run: offline or online
  -v, --verbose                 verbose output
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
