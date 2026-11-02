---
last_updated: 2026-11-02
id: security-update-design
title: "Secret and Config Separation"
sidebar_label: Secret and Config
description: How openCenter separates the GitOps working tree from cluster state and secrets so secrets never enter git-tracked directories.
doc_type: explanation
audience: "platform engineers, CLI maintainers"
tags: [security, secrets, gitops, paths, design]
---
# Secret and Config Separation

**Purpose:** For CLI maintainers and platform engineers, explains why cluster configuration, secrets, and the GitOps working tree live in separate filesystem zones, and how that separation is enforced.

## Concept Summary

`opencenter cluster init` never places the cluster config YAML, an Age private key, or an SSH private key inside the Git-tracked directory. If it did, a routine `git add .` in that tree would commit secrets. Instead, cluster storage is split into four zones with different lifecycles and trust levels, each resolved by `internal/core/paths`:

* **Blueprints zone** — the cluster config YAML. Local, authoritative input; not committed by openCenter itself.
* **GitOps zone** — meant to be committed and pushed. Rendered manifests, repository hygiene files, and self-contained runtime build artifacts (`inventory/`, `venv/`, `kubespray/`, `.bin/`, `kubespray-outputs.json`) ignored by `.gitignore`. `git init` runs against this zone only.
* **Secrets zone** — restricted-permission (mode 0700/0600) Age private keys, SSH keys, and `kubernetes/kubeconfig.yaml`.

`internal/core/paths.ClusterPaths.Validate()` rejects any resolved blueprints, secrets, or kubeconfig path that is equal to or nested inside the GitOps zone, including through symlinked parents. This runs on every `PathResolver.Resolve` call, not only at `init` time, and it runs unconditionally — there is no compatibility mode that permits unencrypted secrets inside the Git repository. Commands that encounter a legacy org-root Git repository containing co-located secrets return a `*paths.LegacyLayoutError` and direct the operator to `opencenter cluster migrate-layout`.

**Evidence:** `internal/core/paths/types.go`, `internal/core/paths/secure.go`, `internal/core/paths/resolver.go`, `internal/core/paths/strategies.go`, `internal/config/v2/manager.go`

## Layout

Blueprints and secrets live outside the GitOps repository under `~/.config/opencenter/clusters/` (or `$OPENCENTER_CLUSTERS_DIR`) by default. Each zone root can be overridden independently:

```text
~/.config/opencenter/                       ← OPENCENTER_CONFIG_DIR
├── settings.yaml                           ← CLI settings file
└── clusters/                               ← OPENCENTER_CLUSTERS_DIR
    ├── blueprints/                         ← OPENCENTER_BLUEPRINTS_DIR
    │   └── <org>/<cluster>/
    │       └── <cluster>-config.yaml       ← cluster config input, mode 0600
    │
    ├── gitops/                             ← OPENCENTER_GITOPS_DIR
    │   └── <org>/                          ← effective GitOps repo, git init runs here
    │       ├── .git/
    │       ├── .gitignore                  ← ignores inventory, venv, kubespray, .bin, kubespray-outputs.json
    │       ├── .opencenter/hooks/pre-commit
    │       ├── .opencenter/scripts/scan-secrets
    │       ├── .github/workflows/opencenter-secret-scan.yml
    │       ├── applications/overlays/<cluster>/
    │       └── infrastructure/clusters/<cluster>/
    │           ├── inventory/              ← Kubespray inventory (gitignored)
    │           ├── venv/                   ← Python virtualenv (gitignored)
    │           ├── kubespray/              ← Kubespray repo (gitignored)
    │           ├── .bin/                   ← Local binaries (gitignored)
    │           └── kubespray-outputs.json  ← Tofu outputs (gitignored)
    │
    └── secrets/                            ← OPENCENTER_SECRETS_DIR, 0700
        └── <org>/<cluster>/
            ├── age/keys/<cluster>-key.txt   ← mode 0600
            ├── ssh/<cluster>                ← mode 0600 (.pub: 0644)
            └── kubernetes/
                └── kubeconfig.yaml          ← admin kubeconfig, mode 0600
```

**Evidence:** `internal/core/paths/secure.go` (`DefaultPathRoots`), `internal/cluster/init_service.go`

## Path Resolution and Environment Variables

Each zone root resolves through the same precedence order, matching the existing `OPENCENTER_CLUSTERS_DIR`/`OPENCENTER_STATE_DIR` pattern:

1. The zone-specific environment variable.
2. The corresponding `paths.*Dir` field in the CLI settings file (`~/.config/opencenter/settings.yaml`).
3. A default derived from the clusters root.

| Zone root | Env variable | CLI settings field | Default |
| --- | --- | --- | --- |
| Config | `OPENCENTER_CONFIG_DIR` | — | `~/.config/opencenter/` |
| Clusters | `OPENCENTER_CLUSTERS_DIR` | `paths.clustersDir` | `<config-dir>/clusters/` |
| Blueprints | `OPENCENTER_BLUEPRINTS_DIR` | `paths.blueprintsDir` | `<clusters-dir>/blueprints/` |
| GitOps | `OPENCENTER_GITOPS_DIR` | `paths.gitopsDir` | `<clusters-dir>/gitops/` |
| Cluster state | `OPENCENTER_CLUSTER_STATE_DIR` | `paths.clusterStateDir` | `<clusters-dir>/state/` |
| Secrets | `OPENCENTER_SECRETS_DIR` | `paths.secretsDir` | `<clusters-dir>/secrets/` |
| CLI runtime state | `OPENCENTER_STATE_DIR` | `paths.stateDir` | platform default |
| Plugins | `OPENCENTER_PLUGINS_DIR` | `paths.pluginsDir` | `<config-dir>/plugins/` |

`OPENCENTER_STATE_DIR` is CLI-wide runtime state (session files, caches) and is distinct from `OPENCENTER_CLUSTER_STATE_DIR`, which is per-cluster state (kubeconfig, inventory, venv, `.bin/`).

Update these through the existing settings path, for example:

```bash
opencenter settings set paths.gitopsDir ~/work/opencenter-gitops
opencenter settings set paths.clusterStateDir ~/.local/state/opencenter/clusters
opencenter settings set paths.secretsDir /Volumes/encrypted/opencenter-secrets
```

**Evidence:** `internal/config/cli_settings.go` (`PathsConfig`), `internal/config/cli_settings_helpers.go`, `internal/core/paths/secure.go`

## Git Scope and Hygiene

`initGitRepo` (`internal/cluster/init_service.go`) targets `ClusterPaths.GitOpsDir` only — never the blueprints, state, or secrets zones — and:

1. Runs `git init` in the GitOps directory if `.git` does not already exist.
2. Configures `git config core.hooksPath .opencenter/hooks` so the tracked hook directory is used instead of the untracked, unshared `.git/hooks/`.
3. Writes a defense-in-depth `.gitignore` that rejects private-key shapes and the cluster config filename even if one were copied in by mistake (`*.key`, `*-key.txt`, `id_rsa*`, `id_ed25519*`, `*.pem`, `*.age`, `/*-config.yaml`, `/.*-config.yaml`).
4. Writes a tracked `.opencenter/hooks/pre-commit` hook (mode `0755`) that runs `opencenter cluster validate-manifests --repo-path <repo> --staged --security-only`, skippable only via `OPENCENTER_SKIP_HOOKS=1` (with a printed warning).
5. Writes `.opencenter/scripts/scan-secrets` (mode `0755`) and a GitHub Actions workflow, `.github/workflows/opencenter-secret-scan.yml`, that runs the same scanner in CI so protection does not depend on the local hook being installed.

**Evidence:** `internal/cluster/init_service.go` (`initGitRepo`, `writeGitOpsHygiene`)

## Config as Input, Not Artifact

The `<cluster>-config.yaml` file is declarative input, not a rendered output:

* It lives in the blueprints zone at `<blueprints-dir>/<org>/<cluster>/<cluster>-config.yaml`.
* `cluster generate` reads it and writes manifests into the GitOps zone; the config file itself is never copied into the GitOps tree.
* `cluster init --config-file <path>` accepts any path and, after loading and validating it, writes the canonical copy into the blueprints zone.

**Evidence:** `internal/cluster/init_service.go`, `internal/core/paths/strategies.go`

## Filesystem Permissions

`internal/cluster.InitService` sets permissions explicitly rather than relying on the caller's umask, and re-verifies the mode immediately after each write:

| Path | Mode |
| --- | --- |
| `<secrets-dir>/<org>/<cluster>/`, `.../age/`, `.../ssh/` | `0700` |
| `<secrets-dir>/.../age/keys/<cluster>-key.txt` | `0600` |
| `<secrets-dir>/.../ssh/<cluster>` | `0600` |
| `<gitops-dir>/<org>/infrastructure/clusters/<cluster>/`, `inventory/`, `venv/`, `.bin/` | `0755` |
| `<blueprints-dir>/.../<cluster>-config.yaml` | `0600` |
| `.gitignore`, secret-scan CI workflow | `0644` |
| `.opencenter/hooks/pre-commit`, `.opencenter/scripts/scan-secrets` | `0755` |

If a write does not verify at the expected mode (`verifyMode` in `internal/cluster/init_service.go`), `init` fails. Setting `OPENCENTER_ALLOW_INSECURE_FILE_MODES=1` downgrades that failure to a warning — intended only for constrained filesystems and test environments, not normal use.

**Evidence:** `internal/cluster/init_service.go` (`ensureMode`, `verifyMode`, `createDirectories`, `generateKeys`)

## Resolver Invariant

`ClusterPaths.Validate()` is the single enforcement point for the "secrets/state never inside GitOps" rule:

```go
// Validate enforces that local state and secrets cannot resolve into the
// GitOps worktree, including through pre-existing symlinked parents.
func (p *ClusterPaths) Validate() error {
    gitopsDir, err := secureAbs(p.GitOpsDir)
    ...
    for label, candidate := range map[string]string{
        "cluster state dir": p.ClusterStateDir,
        "secrets dir":       p.SecretsDir,
        "config path":       p.ConfigPath,
        "SOPS key path":     p.SOPSKeyPath,
        "SSH key path":      p.SSHKeyPath,
    } {
        resolved, err := secureAbs(candidate)
        ...
        if sameOrSubpath(gitopsDir, resolved) {
            return fmt.Errorf("%s %q must not be equal to or inside gitops dir %q", ...)
        }
    }
    return nil
}
```

`secureAbs` normalizes with `filepath.Abs`/`Clean`, resolves symlinks on the nearest existing parent for paths that don't exist yet, and normalizes case where the platform is case-insensitive, before `sameOrSubpath` does the containment check.

**Evidence:** `internal/core/paths/secure.go`

## Migration From the Legacy Layout

`opencenter cluster migrate-layout` is the explicit, one-shot upgrade path — not a compatibility layer commands fall back to automatically:

```bash
opencenter cluster migrate-layout --org <organization> --dry-run
opencenter cluster migrate-layout --org <organization>
opencenter cluster migrate-layout --org <organization> --force
opencenter cluster migrate-layout --custom --org <organization> --cluster <cluster>
opencenter cluster migrate-layout --custom --org <organization> --cluster <cluster> --apply
```

Without `--custom`, it moves an org directory from the legacy single-tree layout into the four current zones. With `--custom`, it separately identifies unrecognized (hand-authored) files inside a cluster's generator-owned overlay paths and offers to move them into that service's `custom/` directory; this is a dry run by default and only moves files with `--apply`. `--force` cannot be combined with `--custom` — destination collisions in a custom migration are always refused rather than overwritten.

**Evidence:** `cmd/cluster_migrate_layout.go`

## Trade-offs

* **Four zones vs. a single tree with `.gitignore`.** A single tree with a well-maintained ignore file is simpler but brittle — one `git add -f` or one misnamed file leaks a key. Separate zones make the leak physically impossible (a secret is never inside the directory `git init` ran in), at the cost of separate directories for humans to navigate.
* **`~/.config/opencenter/` vs. full XDG split.** Everything stays under one config root because that is where users already look and the `OPENCENTER_*_DIR` pattern was already established; users who want XDG-style separation can still set the individual environment variables to point anywhere.
* **Tracked hook + CI scanner vs. local-only pre-commit hook.** An untracked `.git/hooks/pre-commit` is not cloned and can be silently skipped by a new clone. `core.hooksPath` plus a CI workflow means the check travels with the repository.

## Common Misconceptions

* **"The organization directory is the GitOps repo."** Not anymore — the GitOps repo is the scoped path `<gitops-dir>/<org>/`; blueprints, state, and secrets live under separate zone roots.
* **"`.gitignore` is enough."** Only if every future contributor writes perfect globs and nobody runs `git add -f`. `ClusterPaths.Validate()` removes the human from that loop by making the unsafe layout unresolvable, not merely discouraged.

## Further Reading

* [Security Model](security-model.md) — CLI, secrets, and platform security layers
* Path resolver code: `internal/core/paths/resolver.go`, `internal/core/paths/secure.go`, `internal/core/paths/types.go`
* Init service: `internal/cluster/init_service.go`
* CLI settings and environment variables: `internal/config/cli_settings.go`, `internal/config/cli_settings_helpers.go`
* Migration command: `cmd/cluster_migrate_layout.go`
