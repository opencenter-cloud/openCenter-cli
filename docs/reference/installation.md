---
id: installation
title: "Install the CLI"
sidebar_label: "Install the CLI"
description: "Install the openCenter CLI and the opencenter-local plugin with scripts/install.sh: options, environment variables, checksum provenance, and uninstall."
doc_type: how-to
audience: "all users"
tags: [installation, cli, plugins, release]
last_updated: 2026-11-02
---
# Install the CLI

`scripts/install.sh` installs the `opencenter` CLI binary and the
`opencenter-local` plugin from a GitHub release of
`opencenter-cloud/openCenter-cli`.

```bash
curl -fsSL https://raw.githubusercontent.com/opencenter-cloud/openCenter-cli/main/scripts/install.sh | bash
```

To build from source instead, see [Development Setup](../contributing/development-setup.md).

## What it installs

| Artifact | Installed to | Default location |
| --- | --- | --- |
| `opencenter` | `$BIN_DIR/opencenter` | `/usr/local/bin` when running as root, else `~/.local/bin` |
| `opencenter-local` | `$PLUGINS_DIR/opencenter-local` | `~/.config/opencenter/plugins` |
| checksum registration | `$PLUGINS_DIR/checksums.txt` | `~/.config/opencenter/plugins` |

The plugin goes to the plugins directory **only**. `opencenter plugins list`
discovers it through `OPENCENTER_PLUGINS_DIR`, `<config-dir>/plugins`, and
`PATH`; a second copy in `BIN_DIR` would be discovered through `PATH` without a
recorded checksum. See
[Discover CLI plugins](../operations/create-install-cli-plugin.md).

## Options

| Flag | Environment variable | Default | Effect |
| --- | --- | --- | --- |
| `-v`, `--version <ver>` | `OPENCENTER_VERSION` | latest release | Version to install. Accepts `v1.0.0-rc07` or `1.0.0-rc07`. |
| `-b`, `--bin-dir <dir>` | `BIN_DIR` (or `INSTALL_DIR`) | `/usr/local/bin` as root, else `~/.local/bin` | Directory for the CLI binary. |
| `-p`, `--plugins-dir <dir>` | `OPENCENTER_PLUGINS_DIR` | `<config-dir>/plugins` | Directory for the plugin and its checksum registration. |
| `--no-checksum` | `VERIFY_CHECKSUM=false` | verification on | Skip SHA-256 verification. |
| `--dry-run` | `DRY_RUN=true` | off | Print the planned actions; download and install nothing. |
| `-h`, `--help` | — | — | Show usage. |

Flags take precedence over environment variables.

## Version resolution

With no version specified the installer queries the GitHub releases API for the
latest release, falling back to the full releases list when every release is a
prerelease, and finally to the default baked into the script if the API is
unreachable or rate-limited. The resolved value is echoed as
`==> Found release <tag>`; `--dry-run` prints `Target Release: <tag> (<version>)`.

Release tags in this repository have not been tagged consistently — some carry
a `v` prefix, some do not — so the installer normalizes any requested version to
`v<version>` and also tries the unprefixed tag. `--version 1.0.0-rc06` resolves
to the tag `1.0.0-rc06`; `--version v1.0.0-rc07` resolves to `v1.0.0-rc07`.

## Asset naming

For the resolved version, platform, and architecture the installer downloads:

```text
opencenter-<version>-<os>-<arch>          # CLI
opencenter-local-<version>-<os>-<arch>    # plugin
```

`<os>` and `<arch>` come from `uname` and are normalized to the release naming
(`Darwin` → `darwin`, `x86_64` → `amd64`, `aarch64` → `arm64`). A missing
expected asset is a warning, not a failure, so a release that ships only the
CLI still installs.

## Checksum provenance

Downloaded binaries are verified against **`release-checksums.txt` from the same
GitHub release** — the manifest is fetched before the binaries. The installer
never verifies a download against a hash computed from that same download, which
would certify a tampered artifact.

The plugin's manifest hash is registered in `$PLUGINS_DIR/checksums.txt` so that
`internal/plugins` treats the plugin as verified. `opencenter plugins list`
reports `verified checksum`; if the plugin is later modified the command reports
`checksum mismatch` and the plugin refuses to run.

If the release carries no `release-checksums.txt`, the installer reports that
verification is unavailable and exits rather than installing unverified
binaries. `--no-checksum` skips verification explicitly.

## Uninstall

The installer writes only under `$BIN_DIR` and `$PLUGINS_DIR`. Removing the
installed files is sufficient:

```bash
rm -f "$BIN_DIR/opencenter" "$PLUGINS_DIR/opencenter-local" "$PLUGINS_DIR/checksums.txt"
```

Cluster configuration, state, and GitOps workspaces live elsewhere; see
[File Locations](file-locations.md).

## Evidence

- Installer: `scripts/install.sh`
- Release asset naming: `hack/scripts/release.sh`
- Plugin discovery and checksum verification: `internal/plugins/`
- Plugin command: `cmd/plugins.go`
