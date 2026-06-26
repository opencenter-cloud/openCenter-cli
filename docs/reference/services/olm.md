---
id: service-olm
title: "Operator Lifecycle Manager"
sidebar_label: OLM
description: Operator Lifecycle Manager for installing and managing Kubernetes operators.
doc_type: reference
audience: "platform engineers, operators"
tags: [operators, lifecycle, olm]
---

> **Purpose:** For platform engineers, documents the Operator Lifecycle Manager for automated operator installation, upgrades, and dependency resolution.

## Overview

The Operator Lifecycle Manager (OLM) provides a declarative way to install, manage, and upgrade Kubernetes operators and their dependencies. It handles operator discovery from catalog sources, resolves inter-operator dependencies, and manages operator upgrades through subscription channels. OLM is a prerequisite for services that deploy operators from OperatorHub catalogs (e.g., Strimzi for Kafka).

## Configuration

```yaml
opencenter:
  services:
    olm:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Operator Lifecycle Manager |

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable olm
opencenter cluster service disable olm
opencenter cluster service status olm
opencenter cluster service options olm
```
