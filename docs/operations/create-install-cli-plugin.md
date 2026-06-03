---
id: create-install-cli-plugin
title: "Create and Install a CLI Plugin"
sidebar_label: Create and Install a CLI Plugin
description: Build an external CLI plugin, install it into the openCenter plugins directory, and register its checksum.
doc_type: how-to
audience: "developers, platform engineers"
tags: [plugins, cli, extensions, checksum, installation]
---
# Create and Install a CLI Plugin

**Purpose:** For developers and platform engineers, shows how to create an external CLI plugin, install it into the openCenter plugins directory, and register its SHA256 checksum for verified execution.

## Task Summary

External CLI plugins extend `opencenter` with new top-level subcommands. A plugin is any executable named `opencenter-<name>` placed in a discovery location. This guide covers building, installing, and checksum-registering a plugin so it runs without warnings and is protected against tampering.

For background on how the plugin mechanism works, see [Plugin External CLI](../concepts/plugin-external-cli.md).

## Prerequisites

* openCenter CLI installed and on `PATH`
* Go toolchain (or your language of choice) for building the plugin binary
* `shasum` available (ships with macOS; `sha256sum` on Linux)

## Steps

### 1. Create the Plugin Executable

Name the binary `opencenter-<name>`. The prefix is required for discovery.

```bash
# Example: a Go-based plugin
mkdir opencenter-myplugin && cd opencenter-myplugin
go mod init opencenter-myplugin
```

Create `main.go`:

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    fmt.Println("myplugin running with args:", os.Args[1:])
}
```

Build it:

```bash
go build -o opencenter-myplugin .
```

The binary must be executable. `go build` handles this, but if you copy from another source:

```bash
chmod +x opencenter-myplugin
```

### 2. Resolve the Plugins Directory

openCenter discovers plugins from three locations, checked in order:

1. `OPENCENTER_PLUGINS_DIR` environment variable
2. `~/.config/opencenter/plugins/` (default config-based path)
3. Any directory on `PATH`

The recommended location is the config-based plugins directory. Query it:

```bash
PLUGIN_DIR=$(opencenter settings get paths.pluginsDir)
echo "${PLUGIN_DIR}"
# typically: ~/.config/opencenter/plugins/
```

Create the directory if it does not exist:

```bash
mkdir -p "${PLUGIN_DIR}"
```

### 3. Install the Binary

Copy the plugin into the plugins directory:

```bash
cp opencenter-myplugin "${PLUGIN_DIR}/opencenter-myplugin"
chmod +x "${PLUGIN_DIR}/opencenter-myplugin"
```

### 4. Register the Checksum

openCenter verifies plugins against `checksums.txt` in the plugins directory. Without an entry, the plugin runs but emits a warning. With a mismatched entry, execution is blocked.

Compute the SHA256 hash and append it:

```bash
CHECKSUM_FILE="${PLUGIN_DIR}/checksums.txt"
PLUGIN_NAME="opencenter-myplugin"
SHA256=$(shasum -a 256 "${PLUGIN_DIR}/${PLUGIN_NAME}" | awk '{print $1}')

# Remove any stale entry for this plugin
if [ -f "${CHECKSUM_FILE}" ]; then
  grep -v "  ${PLUGIN_NAME}\$" "${CHECKSUM_FILE}" > "${CHECKSUM_FILE}.tmp" || true
  mv "${CHECKSUM_FILE}.tmp" "${CHECKSUM_FILE}"
fi

# Append the new entry
echo "${SHA256}  ${PLUGIN_NAME}" >> "${CHECKSUM_FILE}"
```

The file uses standard `sha256sum` format: `<hex-digest>  <filename>` (two spaces between digest and name).

### 5. Verify the Plugin

Confirm openCenter discovers and verifies the plugin:

```bash
opencenter plugin list
```

Expected output includes:

```
opencenter-myplugin    verified    checksum verified
```

Run the plugin through the CLI:

```bash
opencenter myplugin --help
```

All arguments after the subcommand name are forwarded directly to the plugin executable.

## Verification States

| State | Meaning | Behavior |
| --- | --- | --- |
| `verified` | Checksum matches `checksums.txt` entry | Runs normally |
| `unverified` | No entry in `checksums.txt` | Runs with a warning on stderr |
| `checksum-mismatch` | Entry exists but hash differs | Execution blocked |
| `verification-error` | `checksums.txt` could not be read | Execution blocked |

## Automating Installation with Mise

For plugins built with Mise, add a `local-install` task that handles build, copy, and checksum registration in one step. Example from `opencenter-rmpk`:

```toml
[tasks.local-install]
description = "Build, install, and register checksum for the plugin"
depends = ["build"]
run = '''
PLUGIN_DIR=$(opencenter settings get paths.pluginsDir)
if [ -z "${PLUGIN_DIR}" ]; then
  echo "ERROR: could not resolve plugins directory" >&2
  exit 1
fi
mkdir -p "${PLUGIN_DIR}"
cp release/opencenter-myplugin "${PLUGIN_DIR}/opencenter-myplugin"
chmod +x "${PLUGIN_DIR}/opencenter-myplugin"

CHECKSUM_FILE="${PLUGIN_DIR}/checksums.txt"
PLUGIN_NAME="opencenter-myplugin"
SHA256=$(shasum -a 256 "${PLUGIN_DIR}/${PLUGIN_NAME}" | awk '{print $1}')

if [ -f "${CHECKSUM_FILE}" ]; then
  grep -v "  ${PLUGIN_NAME}\$" "${CHECKSUM_FILE}" > "${CHECKSUM_FILE}.tmp" || true
  mv "${CHECKSUM_FILE}.tmp" "${CHECKSUM_FILE}"
fi

echo "${SHA256}  ${PLUGIN_NAME}" >> "${CHECKSUM_FILE}"
echo "Installed and registered ${PLUGIN_NAME}"
'''
```

Run with:

```bash
mise run local-install
```

## Troubleshooting

### Plugin not discovered

* Confirm the binary name starts with `opencenter-`.
* Confirm the execute bit is set: `ls -l "${PLUGIN_DIR}/opencenter-myplugin"`.
* Confirm the plugins directory matches what the CLI expects: `opencenter settings get paths.pluginsDir`.

### "Warning: plugin myplugin is unverified"

The plugin has no entry in `checksums.txt`. Register the checksum as shown in Step 4.

### "refusing to run plugin: checksum mismatch"

The binary changed since the checksum was recorded. Rebuild, reinstall, and re-register the checksum. This is expected after every rebuild.

### Plugin shadows a built-in command

Built-in commands always take precedence. Rename the plugin to avoid the collision.

## Related Reading

* [Plugin External CLI](../concepts/plugin-external-cli.md) -- how the discovery and execution model works
* [Plugin Internal Services](../concepts/plugin-internal-services.md) -- for adding platform services (cert-manager, Loki, etc.)
* [File Locations](../reference/file-locations.md) -- plugins directory and checksums.txt location
* [CLI Commands](../reference/cli-commands.md) -- `opencenter plugin list` reference

## Evidence

* Plugin discovery and checksum verification: `internal/plugins/loader.go`
* Plugins directory resolution: `internal/config/cli_config_helpers.go:GetPluginsDir()`
* Checksum format: standard `sha256sum` output, parsed in `loadPluginChecksums()`