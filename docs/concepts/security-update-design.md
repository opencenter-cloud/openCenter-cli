---
id: security-update-design
title: "Secret and Config Separation Design"
sidebar_label: Secret and Config
description: Design for splitting cluster config, secrets, and the GitOps working tree so secrets never enter git-tracked directories.
doc_type: explanation
audience: "platform engineers, CLI maintainers"
tags: [security, secrets, gitops, paths, design]
---
# Secret and Config Separation Design

**Purpose:** For CLI maintainers, explains a breaking design change that separates the cluster config YAML, secrets, and the GitOps working tree so that `cluster init` cannot place sensitive files inside the git-tracked directory.

## Concept summary

Today `cluster init` creates everything under a single organization directory, then runs `git init` on that same directory. The cluster config YAML, the Age private key, the SSH private key, and the GitOps manifests end up siblings, which means any `git add .` in that tree commits secrets.

The design splits cluster storage into three zones with different lifecycles and trust levels:

* **GitOps zone** -- meant to be committed and pushed. Rendered manifests plus repository hygiene files.
* **State zone** -- machine-local runtime data. The cluster config YAML lives here, plus kubeconfig, inventory, and binaries. The config is the local authoritative input; the other files are runtime state derived from it.
* **Secrets zone** -- local, restricted permissions. Age keys and SSH keys.

`git init` runs against the GitOps zone only. The state and secrets zones are outside the repo tree, so they cannot be staged by accident. The old layout is not supported as a compatibility mode; commands fail when they detect an org-root Git repository that also contains state or secrets.

## How it works

### Layout (step A)

All three zones live under `~/.config/opencenter/` (or the user’s `OPENCENTER_CONFIG_DIR`) by default. The zones are roots, not per-cluster override targets. The resolver always appends organization and cluster scope where needed so two organizations with the same cluster name cannot share secrets or state by accident.

```text
~/.config/opencenter/                    ← OPENCENTER_CONFIG_DIR
├── config.yaml                          ← CLI config (existing)
└── clusters/                            ← OPENCENTER_CLUSTERS_DIR (existing env var)
    ├── gitops/                          ← OPENCENTER_GITOPS_DIR root (new)
    │   └── <org>/                       ← effective GitOps repo
    │       ├── .git/                    ← git init runs here, only here
    │       ├── .gitignore
    │       ├── .opencenter/hooks/
    │       ├── applications/overlays/<cluster>/
    │       └── infrastructure/clusters/<cluster>/
    │
    ├── state/                           ← OPENCENTER_CLUSTER_STATE_DIR root (new)
    │   └── <org>/<cluster>/
    │       ├── <cluster>-config.yaml    ← cluster config input
    │       ├── kubeconfig.yaml
    │       ├── inventory/
    │       ├── venv/
    │       └── .bin/
    │
    └── secrets/                         ← OPENCENTER_SECRETS_DIR root (new), 0700
        └── <org>/<cluster>/
            ├── age/keys/<cluster>-key.txt
            └── ssh/<cluster>-<env>-<region>
```

The cluster config YAML moves out of `infrastructure/clusters/<cluster>/` (which is now part of the GitOps tree) and into `<cluster-state-dir>/<org>/<cluster>/`. The leading dot is dropped from the filename; it served no purpose once the file is outside a shared tree.

### Path resolution and environment variables (step B)

Path resolution uses a strict precedence order per zone. The order matches the existing `OPENCENTER_CLUSTERS_DIR` / `OPENCENTER_STATE_DIR` pattern in `internal/config/cli_config_helpers.go`, so users do not learn a new model.

For each zone root the resolver walks the list and returns the first non-empty value:

1. Zone-specific environment variable (see table below).
2. Corresponding field in `CLIConfig.Paths` (loaded from `~/.config/opencenter/config.yaml`).
3. Default derived from the config root.

| Zone root | Env variable | CLI config field | Root default | Effective scoped path |
| --- | --- | --- | --- | --- |
| Config | `OPENCENTER_CONFIG_DIR` | `paths.configDir` | `~/.config/opencenter/` | `<config-dir>` |
| Clusters | `OPENCENTER_CLUSTERS_DIR` | `paths.clustersDir` | `<config-dir>/clusters/` | `<clusters-dir>` |
| GitOps | `OPENCENTER_GITOPS_DIR` | `paths.gitopsDir` | `<clusters-dir>/gitops/` | `<gitops-dir>/<org>/` |
| Cluster state | `OPENCENTER_CLUSTER_STATE_DIR` | `paths.clusterStateDir` | `<clusters-dir>/state/` | `<cluster-state-dir>/<org>/<cluster>/` |
| CLI runtime | `OPENCENTER_STATE_DIR` | `paths.stateDir` | platform default | `<state-dir>` |
| Secrets | `OPENCENTER_SECRETS_DIR` | `paths.secretsDir` | `<clusters-dir>/secrets/` | `<secrets-dir>/<org>/<cluster>/` |
| Plugins | `OPENCENTER_PLUGINS_DIR` | `paths.pluginsDir` | `<config-dir>/plugins/` | `<plugins-dir>` |

`OPENCENTER_CONFIG_DIR`, `OPENCENTER_CLUSTERS_DIR`, `OPENCENTER_STATE_DIR`, and `OPENCENTER_PLUGINS_DIR` already exist and keep their meanings. `OPENCENTER_STATE_DIR` remains CLI runtime state such as session files and caches. Per-cluster state (config YAML, kubeconfig, inventory, venv, `.bin/`) moves under the new `OPENCENTER_CLUSTER_STATE_DIR` root so it is clearly separated from CLI-wide state.

The new variables are:

* `OPENCENTER_GITOPS_DIR` -- points at the root that contains GitOps repositories.
* `OPENCENTER_CLUSTER_STATE_DIR` -- points at the root that contains per-cluster runtime state.
* `OPENCENTER_SECRETS_DIR` -- points at the root that contains per-cluster secrets.

Overrides are per-zone roots, not final per-cluster paths. A user who points `OPENCENTER_SECRETS_DIR` at an encrypted disk gets that disk for every org and every cluster, but the CLI still writes to `<secrets-dir>/<org>/<cluster>/...`. `OPENCENTER_GITOPS_DIR` is also a root; the effective repository is `<gitops-dir>/<org>/`, which prevents multiple organizations from sharing one repository unless they intentionally choose the same organization name.

### CLI config integration

`PathsConfig` in `internal/config/cli_config.go` gains three fields:

```go
type PathsConfig struct {
    ConfigDir       string `yaml:"configDir"`
    ClustersDir     string `yaml:"clustersDir"`
    PluginsDir      string `yaml:"pluginsDir"`
    StateDir        string `yaml:"stateDir"`
    // New fields:
    GitOpsDir       string `yaml:"gitopsDir"`
    ClusterStateDir string `yaml:"clusterStateDir"`
    SecretsDir      string `yaml:"secretsDir"`
}
```

All three new fields are optional zone roots. Defaults resolve relative to `ClustersDir` so a user who overrides only `ClustersDir` still gets a consistent layout.

New helpers `GetGitOpsDir()`, `GetClusterStateDir()`, and `GetSecretsDir()` ship next to the existing `GetClustersDir()` / `GetStateDir()` helpers, with the same precedence logic.

Updating the CLI config happens through the existing `opencenter settings set` path:

```bash
opencenter settings set paths.gitopsDir ~/work/opencenter-gitops
opencenter settings set paths.clusterStateDir ~/.local/state/opencenter/clusters
opencenter settings set paths.secretsDir /Volumes/encrypted/opencenter-secrets
```

Validation rejects values where secrets or cluster-state roots are equal to, or descendants of, the gitops root after path normalization and symlink resolution (the same invariant enforced in step G).

### Git scope and hygiene (step C)

The `initGitRepo` step changes in four ways:

1. Target `ClusterPaths.GitOpsDir` (now `<gitops-dir>/<org>/`), not the org root.
2. Write a `.gitignore` at the root of that working tree that rejects known secret shapes even if someone copies a file in by mistake:

   ```gitignore
   # Private keys and secrets (defense in depth; these paths should not exist in the tree)
   *.key
   *-key.txt
   id_rsa*
   id_ed25519*
   *.pem
   *.age
   # Cluster config input must not be committed
   /*-config.yaml
   /.*-config.yaml
   ```
3. Install tracked hooks at `.opencenter/hooks/pre-commit` and configure `git config core.hooksPath .opencenter/hooks`. The hook scans staged blobs for Age private keys, OpenSSH private keys, PEM private keys, inline Git tokens, and unencrypted Kubernetes Secrets, then aborts the commit.
4. Add a CI secret-scanning workflow to the generated GitOps repository so protection does not depend on local hooks. The CI check must fail when it finds private key material, credential-looking tokens, or Kubernetes `Secret` manifests without SOPS metadata.

A follow-up can replace the hook with [gitleaks](https://github.com/gitleaks/gitleaks) once the baseline is stable.

### Config as input, not artifact (step E)

The `<cluster>-config.yaml` file is declarative input, similar to a `Brewfile` or `package.json`. It is not a rendered output. The design treats it accordingly:

* It lives in the cluster-state zone at `<cluster-state-dir>/<org>/<cluster>/<cluster>-config.yaml`.
* `cluster generate` reads it and writes manifests into the GitOps zone. The config itself is never copied into the GitOps zone.
* `cluster init` with `--config-file` accepts any path; after load it writes the canonical copy into the state zone.

This preserves the "single source of truth" property without forcing the source of truth into git. Teams that want config in git can commit an encrypted copy produced by `sops` as a separate, opt-in step, but that flow is outside the default path.

### Filesystem permissions (step F)

`createDirectories` and `generateKeys` set permissions explicitly rather than relying on the caller’s umask:

| Path | Mode |
| --- | --- |
| `<secrets-dir>/<org>/<cluster>/` | `0700` |
| `<secrets-dir>/<org>/<cluster>/age/keys/<cluster>-key.txt` | `0600` |
| `<secrets-dir>/<org>/<cluster>/ssh/<cluster>-<env>-<region>` | `0600` |
| `<secrets-dir>/<org>/<cluster>/ssh/<cluster>-<env>-<region>.pub` | `0644` |
| `<cluster-state-dir>/<org>/<cluster>/` | `0700` |
| `<cluster-state-dir>/<org>/<cluster>/<cluster>-config.yaml` | `0600` |
| `<gitops-dir>/<org>/` and subdirectories | `0755` |

After each write, the init code re-stats the file and fails if the mode does not match. This catches filesystems that silently ignore mode bits (for example some network mounts) before keys are left on disk with broad permissions. A documented `OPENCENTER_ALLOW_INSECURE_FILE_MODES=1` escape hatch may downgrade this to a warning for test environments and constrained filesystems only; normal `cluster init` must fail closed.

### Resolver invariants (step G)

`paths.ClusterPaths` gains a `Validate()` method called by `PathResolver` on every `Resolve` call:

```go
// Validate enforces that secret and config paths never live inside the git-tracked tree.
// This turns layout regressions into unit-test failures rather than leaked keys.
func (p *ClusterPaths) Validate() error {
  gitopsDir, err := secureAbs(p.GitOpsDir)
  if err != nil {
    return fmt.Errorf("resolving gitops dir: %w", err)
  }

  checks := map[string]string{
    "cluster state dir": p.ClusterStateDir,
    "secrets dir":       p.SecretsDir,
    "config path":       p.ConfigPath,
    "SOPS key path":     p.SOPSKeyPath,
    "SSH key path":      p.SSHKeyPath,
  }
  for label, candidate := range checks {
    resolved, err := secureAbs(candidate)
    if err != nil {
      return fmt.Errorf("resolving %s %q: %w", label, candidate, err)
    }
    if sameOrSubpath(gitopsDir, resolved) {
      return fmt.Errorf("%s %q must not be equal to or inside gitops dir %q", label, candidate, p.GitOpsDir)
    }
  }
  return nil
}
```

`sameOrSubpath` uses `filepath.Rel` after `filepath.Abs`, `filepath.Clean`, and symlink-aware normalization. Equality is always rejected. For paths that do not exist yet, `secureAbs` resolves the nearest existing parent with `filepath.EvalSymlinks` and appends the remaining clean path components. On case-insensitive platforms, comparison must normalize the path casing consistently before the `Rel` check. Tests cover direct descendants, equality, symlinked parents, missing leaf paths, sibling names with common prefixes, and case-insensitive collisions where the platform supports them.

## Trade-offs and alternatives

* **Three zones vs. single tree with `.gitignore`.** A single tree with a well-maintained ignore file is simpler but brittle: one `git add -f` or one misnamed file leaks a key. The three-zone layout makes the leak physically impossible, at the cost of walking separate trees for humans.
* **Keeping `~/.config/opencenter/` vs. full XDG split.** XDG would spread data across `~/.local/share`, `~/.local/state`, and `~/.config`. The design keeps everything under `~/.config/opencenter/` because that is where users already look, and the env variable pattern (`OPENCENTER_*_DIR`) is already established. Users who want XDG-style separation set the individual env vars.
* **New `OPENCENTER_GITOPS_DIR` and `OPENCENTER_SECRETS_DIR` vs. reusing existing variables.** Reusing `OPENCENTER_CLUSTERS_DIR` for everything keeps the count low but collapses the zones back into one. Distinct variables are the point of the exercise.
* **Tracked hook + CI scanner vs. local-only pre-commit hook.** A local `.git/hooks/pre-commit` hook is not cloned and can be skipped. The generated repository should track its hook through `core.hooksPath` and include CI secret scanning so the protection follows the repository.
* **Encrypted config in git.** Not adopted by default because it adds a decrypt step to every read, complicates schema validation, and couples the config lifecycle to the key lifecycle. Users who want it can run `sops` manually on a copy.
* **OS keystore for the Age key.** Deferred. Worth doing, but orthogonal to the layout split. Once zones are separate, a keystore backend can replace the file in `<secrets-dir>/<org>/<cluster>/age/keys/` without changing any other code.

## Common misconceptions

* **"The organization directory is the GitOps repo."** It was, in the original design. Under this proposal the GitOps repo is the scoped path `<gitops-dir>/<org>/`; state and secrets live under different zone roots.
* **"``.gitignore`` is enough."** Only if every future contributor writes perfect globs and nobody runs `git add -f`. The zone split removes the human from the loop.
* **"Moving the config breaks backward compatibility."** Yes. This is an intentional security break. The CLI rejects the old mixed org-root repo layout instead of silently supporting it.

## Migration plan

The work lands in small PRs. The secure layout is unconditional from the first path-model PR; there is no feature flag or runtime fallback to the unsafe layout.

1. **Path model.** Add `GitOpsDir`, `ClusterStateDir`, and cluster-scoped `SecretsDir` fields to `ClusterPaths` and the `Validate()` method. Teach `PathResolver` to build only the new layout. Remove fallback discovery of `.<cluster>-config.yaml` in the org root and `infrastructure/clusters/<cluster>` as a valid state location. Add unit tests for the invariants.
2. **CLI config + env vars.** Extend `PathsConfig` in `internal/config/cli_config.go` with `gitopsDir`, `clusterStateDir`, and `secretsDir` as zone roots. Add `GetGitOpsDir()`, `GetClusterStateDir()`, and `GetSecretsDir()` helpers in `cli_config_helpers.go` mirroring `GetClustersDir()`. Wire `OPENCENTER_GITOPS_DIR`, `OPENCENTER_CLUSTER_STATE_DIR`, and `OPENCENTER_SECRETS_DIR` with the same precedence model as the existing variables. Leave `OPENCENTER_STATE_DIR` pointed at CLI runtime state.
3. **Init wiring.** Update `InitService.Initialize` to write config into `<cluster-state-dir>/<org>/<cluster>/` and secrets into `<secrets-dir>/<org>/<cluster>/`. Scope `initGitRepo` to `GitOpsDir`. Write the `.gitignore`, tracked hook directory, and CI scanner configuration inside the GitOps tree.
4. **Permissions.** Update `createDirectories` and `generateKeys` to set modes explicitly and fail after post-write verification when private files or directories are broader than expected.
5. **One-shot migration command.** Add `opencenter cluster migrate-layout` as an explicit upgrade command, not a compatibility layer. It moves an existing org directory into the new zones, updates `opencenter.gitops.repository.local_dir` in each cluster config, rewrites SSH / SOPS key paths, removes secrets from the old tree, and prints a diff of what moved. Include a `--dry-run` flag. Normal commands must continue to reject the old layout before and after this command exists.
6. **Docs.** Update `docs/dev/cluster-init-details.md`, `docs/reference/` path references, and the getting-started guide to reflect the new layout and env variables.

Each PR includes unit tests for the behavior it changes. The layout invariants (step G) land in PR 1 so later PRs cannot regress the separation.

## Verification

Before calling this complete, the following must hold:

* `opencenter cluster init <name>` produces no files under `GitOpsDir` other than manifests, `.gitignore`, tracked `.opencenter/hooks/`, and CI scanner configuration.
* `git status` in `GitOpsDir` right after init shows either a clean tree (bootstrap committed) or only tracked files -- never an Age key, SSH key, or cluster config YAML.
* `grep -R 'AGE-SECRET-KEY-\|BEGIN OPENSSH PRIVATE KEY\|BEGIN .*PRIVATE KEY\|ghp_\|glpat-' <GitOpsDir>` returns nothing.
* A YAML-aware scanner walks `GitOpsDir` and fails on any Kubernetes `Secret` with plaintext `data` or `stringData`, or any secret-like manifest that lacks SOPS metadata.
* Every generated secret manifest in `GitOpsDir` is encrypted before staging, contains `sops:` metadata, and decrypts successfully with the cluster Age key.
* `stat` on every secret file reports mode `0600` (or `0700` for directories), and init fails when the filesystem reports broader permissions unless `OPENCENTER_ALLOW_INSECURE_FILE_MODES=1` is set.
* Each new env variable (`OPENCENTER_GITOPS_DIR`, `OPENCENTER_CLUSTER_STATE_DIR`, `OPENCENTER_SECRETS_DIR`) has a unit test that sets it, runs `cluster init`, and asserts the zone resolves to the override path.
* `opencenter settings get paths.gitopsDir`, `paths.clusterStateDir`, and `paths.secretsDir` return the expected values after `opencenter settings set`.
* The resolver’s `Validate()` unit tests fail when any invariant is violated, including equality and symlink-based containment.
* A property-based test that generates random valid layouts confirms `Validate()` accepts them and rejects layouts where zones overlap, share a root incorrectly, or differ only by case on case-insensitive filesystems.
* Normal commands reject a legacy org-root Git repository containing `secrets/`, `.<cluster>-config.yaml`, or cluster state under `infrastructure/clusters/<cluster>/`.

## Further reading

* Current init flow: `docs/dev/cluster-init-details.md`
* Path resolver code: `internal/core/paths/resolver.go`, `internal/core/paths/types.go`
* Init service: `internal/cluster/init_service.go`
* Existing path helpers and env variables: `internal/config/cli_config_helpers.go`
* CLI config schema: `internal/config/cli_config.go` (`PathsConfig`)
* SOPS key management: `internal/sops/`