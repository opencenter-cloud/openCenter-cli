---
id: normalize-legacy-renderer-metadata
title: Normalize a legacy configuration
sidebar_label: Normalize a legacy configuration
description: Normalize an existing openCenter configuration and review its generated defaults.
doc_type: how-to
audience: openCenter operators
tags: [migration, configuration]
last_updated: 2026-09-25
---
# Normalize a legacy configuration

**Purpose:** For openCenter operators, shows how to normalize an existing openCenter configuration and review its generated defaults.

Use `cluster normalize` to load an existing configuration with defaults and
write missing fields back to the file. Existing values are preserved.

```bash
opencenter cluster normalize ORG/CLUSTER
```

The command creates a timestamped `<config-file>.backup.<timestamp>` before
writing unless `--no-backup` is supplied. Preview it without writing:

```bash
opencenter --dry-run cluster normalize ORG/CLUSTER
```

After reviewing the result, validate and regenerate:

```bash
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
```

Use `cluster set` for intentional individual edits; normalize is for missing
default fields, not for selecting arbitrary values.

## Evidence

- Command behavior: `cmd/cluster_normalize.go`
- Normalization tests: `cmd/cluster_normalize_test.go`
- v2 serialization: `internal/config/v2/public_serialization.go`
