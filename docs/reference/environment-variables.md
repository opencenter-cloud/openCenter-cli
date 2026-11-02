---
last_updated: 2026-11-02
id: environment-variables
title: "Environment Variables"
sidebar_label: Environment Variables
description: Every environment variable openCenter CLI reads, verified by source grep, with what it controls and its default.
doc_type: reference
audience: "all users"
tags: [environment, variables, configuration, cli]
---
# Environment Variables

**Purpose:** For all users, documents every environment variable openCenter CLI
recognizes. Every variable below was confirmed against a reader in the
repository -- a Go `os.Getenv`/`os.LookupEnv` call, a shell export, or, for the
installer variables, a `scripts/install.sh` default -- and none are invented.
`hack/shell-integration/shell-integration.sh` also defines
`OPENCENTER_CACHE_FILE` and exports `OPENCENTER_ACTIVE_CLUSTER`, and the BDD
harness sets `OPENCENTER_TEST_TMP` (`tests/features/steps/helpers.go:258`). No
reader in the repository consumes any of the three, so they are omitted from the
tables below.

`OPENCENTER_CONFIG_DIR` selects the directory, not a file name. The CLI settings
manager uses `<config-dir>/settings.yaml`; cluster-init's compatibility loader
also reads `<config-dir>/config.yaml` for `cluster_defaults` (see [Default
Values](default-values.md)).

## Path-resolution variables

These only change *where files live* -- see [Configuration Precedence](configuration-precedence.md) and [File Locations](file-locations.md) for the full resolution order (env var, then CLI settings file, then computed default).

| Variable | Controls | Default if unset |
| --- | --- | --- |
| `OPENCENTER_CONFIG_DIR` | CLI settings/config directory | `~/.config/opencenter` (or platform equivalent) |
| `OPENCENTER_CLUSTERS_DIR` | Cluster storage root | `<config-dir>/clusters` |
| `OPENCENTER_GITOPS_DIR` | GitOps repository root | `<clusters-dir>/gitops` |
| `OPENCENTER_BLUEPRINTS_DIR` | Cluster blueprints root | `<clusters-dir>/blueprints` |
| `OPENCENTER_CLUSTER_STATE_DIR` | Per-cluster state root | `<clusters-dir>/state` |
| `OPENCENTER_SECRETS_DIR` | Per-cluster secrets root | `<clusters-dir>/secrets` |
| `OPENCENTER_PLUGINS_DIR` | External plugin discovery/install directory | `<config-dir>/plugins` |
| `OPENCENTER_STATE_DIR` | General runtime state (locks, bootstrap state, logs) | `~/.local/state/opencenter` (or `$XDG_STATE_HOME`, or platform equivalent) |

## Behavior variables

| Variable | Controls | Notes |
| --- | --- | --- |
| `OPENCENTER_CLUSTER` | Active-cluster override | Read by `internal/config/v2/manager.go` and `cmd/cluster_active.go` as one of the sources for "which cluster am I operating on" resolution (alongside the persistent active-cluster marker and `cluster use` session state). |
| `OPENCENTER_SESSION_FILE` | Active-cluster session file path | Written by `cluster use` (`cmd/cluster_use.go:548`) and read by `cluster active` (`cmd/cluster_active.go:59`) and `internal/config/v2/manager.go:742` to resolve the shell-session-scoped active cluster instead of the global marker. `opencenter shell-init` exports it at `~/.config/openCenter/.session-$OPENCENTER_SESSION_ID` (`cmd/shell-integration/integration.bash:7`); with the variable unset, `cluster use` warns that shell integration is not detected and falls back to the persistent marker. |
| `OPENCENTER_DEBUG` | Enables debug behavior | When set (to any non-empty value), `cluster validate` exports the effective resolved config to `.opencenter-v2.yaml` (same as passing `--generate-debug-config`); `secrets keys` commands print extra environment diagnostics. |
| `OPENCENTER_LOG_LEVEL` | Default log level | One of `debug`, `info`, `warn`, `error`. Only takes effect if `--log-level` was not explicitly passed on the command line. Default (when neither the flag nor this variable is set) is `warn`. |
| `OPENCENTER_TEST_MODE` | Skips real side effects in specific code paths | Confirmed at least in the Kind bootstrap provider's `gitea-attach-kind` step, where it short-circuits the real Gitea-service call entirely. Intended for tests, not documented as a general user-facing switch -- do not set it in normal use. |
| `OPENCENTER_ALLOW_INSECURE_FILE_MODES` | Relaxes a file-permission check | Set to exactly `1` to bypass a strict file-mode check in `internal/cluster/init_service.go`. Security-relevant -- only use for environments (e.g. certain CI filesystems) where the normal permission model genuinely cannot apply. |
| `OPENCENTER_GUIDED_ANSWERS` | Pre-scripted answers for guided flows | Inline JSON object of prompt-ID to answer, read by `internal/ui/guided_prompter.go:61` -- not a file path. `cluster configure --guided` consults it only when `OPENCENTER_TEST_MODE` is set (`cmd/cluster_configure.go:36`); unset or malformed JSON falls back to interactive prompting. |
| `OPENCENTER_SESSION_ID` | Shell session identifier | Exported by `opencenter shell-init` as `$$-$(date +%s)` (`cmd/shell-integration/integration.bash:6`, `.zsh:6`, `.fish:7`) and used only to name the session file above; the compiled CLI never reads it. |
| `OPENCENTER_SKIP_HOOKS` | Bypasses the generated pre-commit checks | Set to exactly `1`. The `pre-commit` script written into `.opencenter/hooks` checks the variable first and exits `0` before running `cluster validate-manifests --staged --security-only`, printing a warning to stderr (`internal/cluster/init_service.go:1260`, `internal/secrets/hooks.go:186`). |
| `OPENCENTER_SECURE_*` | Secure template variable values | Any variable under this prefix is loaded into the secure template variable map with the prefix stripped, so `OPENCENTER_SECURE_DB_PASSWORD` supplies `DB_PASSWORD` (`internal/config/flags/secure_template_processor.go:187-215`). Keeps secret values out of the argument list and shell history. `LoadSecureVariablesFromEnv` has no non-test caller yet, so no shipped command reads the prefix today. |
| `OPENCENTER_RUN_KIND_SMOKE` | Opt-in for the real Kind lifecycle smoke test | Set to `1` to run the smoke test at `cmd/cluster_deploy_integration_test.go:404`; when unset, that test skips. Test-only. |

## Non-`OPENCENTER_*` variables the CLI reads

| Variable | Used for |
| --- | --- |
| `SOPS_AGE_KEY_FILE` | Standard SOPS/Age variable -- path to an Age private key file, consulted by SOPS operations outside the per-cluster `secrets/age/` layout. |
| `KUBECONFIG` | Standard kubeconfig path; set explicitly by `cluster deploy`/bootstrap steps rather than relying on ambient state. |
| `EDITOR`, `VISUAL` | Used by `cluster edit`/`config edit`/`settings edit` to launch an interactive editor. |
| `HOME`, `SHELL`, `PATH` | Standard shell environment, used for home-directory resolution and shell-integration setup (`cmd/shell_init.go`, `hack/install-shell-integration.sh`). |
| `XDG_STATE_HOME` | XDG state-directory base on Linux/macOS, consulted by `DefaultStateDir()`. |
| `USER`, `USERNAME` | Used where a default "created by" / audit identity is needed. |
| `APPDATA`, `LOCALAPPDATA`, `USERPROFILE` | Windows path resolution fallbacks for the config/state directory defaults. |
| `CONTAINER_RUNTIME` | Selects `docker` or `podman` for local Kind/Gitea workflows (set via `.mise.toml`'s `[env]` block, or override on the command line). |
| `KIND_EXPERIMENTAL_PROVIDER` | Tells the `kind` binary to use Podman instead of Docker (set via `.mise.toml`). |
| `OS_CLIENT_CONFIG_FILE`, `OS_USERNAME`, `OS_PASSWORD`, and the rest of the standard `OS_*` OpenStack client variables | Read when extracting OpenStack credentials for deploy and readiness checks -- see [`cluster deploy` -- OpenStack Provider](../contributing/cluster-deploy-openstack.md) for the full `OS_*` variable table injected into bootstrap steps. |
| `PSModulePath` | Windows PowerShell detection: a non-empty value makes `cluster env`/`cluster use` emit PowerShell syntax (`cmd/cluster_use.go:267`). |

## Installer variables (`scripts/install.sh`)

Read by the installer script, not by the compiled CLI. See
[Install the CLI](installation.md).

| Variable | Equivalent flag | Default if unset |
| --- | --- | --- |
| `OPENCENTER_VERSION` | `--version` | latest release |
| `BIN_DIR` | `--bin-dir` | `/usr/local/bin` when running as root, else `~/.local/bin` |
| `INSTALL_DIR` | `--bin-dir` | fallback for `BIN_DIR` when that is unset |
| `OPENCENTER_PLUGINS_DIR` | `--plugins-dir` | `<config-dir>/plugins` |
| `VERIFY_CHECKSUM` | `--no-checksum` | `true`; set to `false` to skip SHA-256 verification |
| `DRY_RUN` | `--dry-run` | `false`; set to `true` to print actions without installing |

`OPENCENTER_PLUGINS_DIR` and `OPENCENTER_CONFIG_DIR` are shared with the CLI: the
installer defaults the plugins directory to `<config-dir>/plugins`, so a
configured `OPENCENTER_CONFIG_DIR` places the plugin where the CLI will find it.

## Cross-references

* [Configuration Precedence](configuration-precedence.md) -- how these interact with the CLI settings file and computed defaults.
* [File Locations](file-locations.md) -- the exact directory each path variable controls.
* [Exit Codes](exit-codes.md) -- process exit-code contract (not environment-variable controlled).
