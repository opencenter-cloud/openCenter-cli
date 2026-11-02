---
id: create-install-cli-plugin
title: Discover CLI plugins
sidebar_label: Discover CLI plugins
description: Discover and inspect external CLI plugins available to openCenter.
doc_type: how-to
audience: openCenter operators
tags: [plugins, cli]
last_updated: 2026-09-25
---
# Discover CLI plugins

The built-in CLI can list external plugins discovered on the host:

```bash
opencenter plugins list
opencenter plugins list --output json
opencenter plugins list --output yaml
```

Discovery order is `OPENCENTER_PLUGINS_DIR`, `<config-dir>/plugins`, then
`PATH`. The list is read-only and reports each discovered plugin's name, path,
status, and message.

The repository ships `scripts/install.sh`, which downloads the `opencenter` CLI
and the `opencenter-local` plugin from a GitHub release and registers the
plugin's checksum. See [Install the CLI](../reference/installation.md) for its
options and checksum behavior. The built-in CLI itself has no command that
creates, installs, or publishes a plugin; authoring a new plugin is outside this
page's evidence boundary.

## Evidence

- Plugin command: `cmd/plugins.go`
- Discovery implementation: `internal/plugins/`
- Installer: `scripts/install.sh`
