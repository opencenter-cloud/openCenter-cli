---
id: customize-services
title: Customize services
sidebar_label: Customize services
description: Inspect and change service enablement and values in an openCenter cluster file.
doc_type: how-to
audience: openCenter operators
tags: [services, configuration]
last_updated: 2026-11-01
---
# Customize services

Service enablement and typed service values live in the cluster file. Use the
service command to inspect and change that file.

```bash
opencenter cluster service status --cluster ORG/CLUSTER
opencenter cluster service options SERVICE --cluster ORG/CLUSTER
opencenter cluster service enable SERVICE --cluster ORG/CLUSTER
opencenter cluster service disable SERVICE --cluster ORG/CLUSTER
```

`enable` accepts repeatable `--param key=value` and `--secret key=value`
options. Use `--default` with `cert-manager` to pin the DNS credential as the
cluster default issuer. Service enablement is decoupled from rendering: after
enabling or modifying services, the next step is to execute `opencenter cluster generate`
(or `opencenter cluster generate --render-only`) to render all changes across
the cluster manifests. The optional `--render` flag performs a scoped render of
that single service only. Both render paths accept `--prune` and `--adopt-generated`.

For direct field edits, use native dot notation:

```bash
opencenter cluster set ORG/CLUSTER opencenter.services.SERVICE.enabled=true
```

Use `cluster edit` when a structured YAML edit is clearer. Keep snippets to
fields shown by `service options` or accepted by the v2 schema; renderer
metadata is not a user configuration contract.

## Validate and generate

```bash
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
```

Generation owns its generated files. Put unsupported hand-authored manifests in
the generated service's user-owned custom area only when that service's output
provides one; do not edit generated files in place.

## Evidence

- Service command and flags: `cmd/cluster_service.go`
- Dot-notation setter: `cmd/cluster_set.go`
- Typed service configuration: `internal/config/v2/services.go`
- Validation: `internal/config/v2/readiness.go`
