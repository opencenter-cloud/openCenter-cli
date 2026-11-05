---
id: "opencenter-cluster-service-enable"
title: "Opencenter_Cluster_Service_Enable"
sidebar_label: "Opencenter_Cluster_Service_Enable"
description: "Enable a service in the cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster service enable

**Purpose:** For operators, developers, documents `opencenter cluster service enable`: what it does, its flags, and its examples.

Enable a service in the cluster configuration

### Synopsis

This command enables a service in the cluster configuration.
If the service requires additional parameters or secrets, they must be provided
as flags. If they are missing, the command will return an error with an example.

Service configuration is decoupled from manifest rendering. After enabling a service,
the next step is to execute 'opencenter cluster generate' (or 'opencenter cluster generate --render-only')
to render all changes across the cluster manifests.

Examples:
  # Enable the 'cert-manager' service with Cloudflare DNS (the default provider) and pin
  # the token's credential as the cluster default issuer; --default=false clears that pin
  opencenter cluster service enable cert-manager --param="email=$CLOUDFLARE_DNS_EMAIL" --param="dns_provider=cloudflare" --secret="cloudflare_api_token=$CLOUDFLARE_API_TOKEN" --default --force

  # Next step: render all cluster manifests
  opencenter cluster generate

  # Enable a managed service with a secret
  opencenter cluster service enable my-managed-service --managed --secret="api_key=some_secret_value"

  # Force re-enable an already enabled service
  opencenter cluster service enable prometheus --force

  # Enable a service and scoped-render only its own templates
  opencenter cluster service enable loki --render

```
opencenter cluster service enable <service-name> [flags]
```

### Options

```
      --adopt-generated     Claim differing planned files after creating backups during --render
      --cluster string      Specify the cluster name
      --default             cert-manager: pin the DNS credential these secrets populate as the cluster default issuer (cert-manager only; --default=false clears the pin)
      --force               Force re-enable an already enabled service to re-render configuration
  -h, --help                help for enable
      --managed             Enable the service as a managed service
      --param stringArray   Set a service parameter (e.g., --param key=value)
      --prune               Remove stale generated files during --render (default true)
      --render              Render the service templates immediately after enabling (scoped render; run 'opencenter cluster generate' to update dependent resources)
      --secret strings      Set a service secret (e.g., --secret key=value)
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

* [opencenter cluster service](opencenter_cluster_service.md)	 - Manage cluster services
