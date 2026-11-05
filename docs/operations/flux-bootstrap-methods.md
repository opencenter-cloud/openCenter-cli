---
id: flux-bootstrap-methods
title: Flux and GitOps configuration
sidebar_label: Flux and GitOps configuration
description: Configure and generate GitOps assets with the openCenter command path.
doc_type: how-to
audience: openCenter operators
tags: [flux, gitops]
last_updated: 2026-09-25
---
# Flux and GitOps configuration

**Purpose:** For openCenter operators, shows how to configure and generate GitOps assets with the openCenter command path.

The CLI generates GitOps assets from the cluster file. The supported
CLI path is:

```bash
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
opencenter secrets sync ORG/CLUSTER
opencenter cluster validate ORG/CLUSTER --manifests
```

Generation accepts `--gitops-auth ssh|token`; when omitted, the command uses
the configured cluster default and otherwise its built-in default. The selected
method controls base repository source rendering.

The actual deploy behavior is provider-specific. Kind has a local bootstrap
implementation; the OpenStack lifecycle has its own provider steps. Do not
copy a Kind Gitea URL, certificate, or Flux command into an OpenStack
configuration.

The checkout does not expose a single built-in `flux bootstrap` command for all
providers. Inspect generated manifests and use `cluster deploy` for the
provider lifecycle rather than assuming a universal bootstrap sequence.

## Evidence

- GitOps flags and auth resolution: `cmd/cluster_generate.go`
- Secret synchronization: `cmd/secrets_sync.go`
- Provider lifecycle split: `internal/cluster/kind_bootstrap_provider.go`,
  `internal/cluster/bootstrap_provider_infra.go`
- Generated tree fixture: `tests/features/workflow.feature`
