---
last_updated: 2026-11-05
id: reconcile-gitops-changes
title: "Reconcile GitOps Changes"
sidebar_label: Reconcile GitOps Changes
description: Push existing cluster manifest changes to the GitOps remote and drive Flux reconciliation with opencenter cluster reconcile, including dry-run planning and push-only or flux-only modes.
doc_type: how-to
audience: "operators, platform engineers"
tags: [gitops, flux, reconcile, day-2, deployment]
---
# Reconcile GitOps Changes

**Purpose:** For operators, explains how to converge a cluster on manifest
changes that already exist in the local GitOps working tree, using
`opencenter cluster reconcile` and `opencenter cluster service reconcile`.

This is a Day-2 workflow. It does not create a cluster and does not run the
initial deploy. Use it after something has changed the rendered manifests and you
need the remote repository and the running cluster to agree again.

## When to use this

| Situation | Command |
| --- | --- |
| You edited the cluster configuration and want the change live | `opencenter cluster generate` then `opencenter cluster reconcile` |
| Manifests are already correct on disk; only the push and Flux sync are missing | `opencenter cluster reconcile` |
| One service is out of sync | `opencenter cluster service reconcile <service>` |
| Infrastructure drift between deployed and configured state | [`cluster drift reconcile`](../reference/opencenter/opencenter_cluster_drift_reconcile.md) - a different command |

`cluster reconcile` commits, pushes, and triggers Flux. It does not detect or
repair infrastructure drift; that is `cluster drift reconcile`.

## Prerequisites

- A GitOps repository already initialised and pushed at least once. Reconcile
  needs a branch with an upstream to rebase against.
- A configured `git_dir` for the cluster.
- The `flux` CLI on `PATH` and a cluster-owned kubeconfig, unless you pass
  `--push-only`.
- A clean-enough working tree that the changes you want are visible to
  `git status`.

## Steps

### 1. Preview the plan

`--dry-run` prints the exact commands without touching Git or the cluster:

```bash
opencenter cluster reconcile myorg/my-cluster --dry-run
```

The plan lists four groups of steps: stage and commit, pull-rebase and push,
reconcile the `flux-system` GitRepository source, then reconcile each target
service's Kustomizations and HelmRelease. Read it before running for real,
especially the target service list.

Note that in `--dry-run` the command does **not** inspect `git status` to
auto-detect services, so the plan may show an empty target list. Pass
`--service` explicitly when you need an accurate preview of what would be
reconciled.

### 2. Reconcile

```bash
opencenter cluster reconcile myorg/my-cluster
```

With no `--service` flag the command infers the affected services from
`git status`, looking under `applications/overlays/<cluster>/services/`. A changed
file under `services/<name>/` selects `<name>`; a changed
`services/fluxcd/<name>.yaml` also selects `<name>`. Changes under `fluxcd/` and
`sources/` alone are not treated as a service.

To skip detection, name the services:

```bash
opencenter cluster reconcile myorg/my-cluster --service openstack-ccm --service headlamp
opencenter cluster reconcile myorg/my-cluster --all
```

`--all` targets every enabled service in the configuration, whether or not it
changed.

### 3. Re-render first when the configuration changed

`--generate` re-renders and re-encrypts the target service overlay before
committing, so you do not have to run `cluster generate` separately:

```bash
opencenter cluster reconcile myorg/my-cluster --service headlamp --generate
```

When services were auto-detected, the list is re-derived from `git status` after
generation, so newly generated files are picked up.

## Scoping and safety behaviour

- **Scoped staging.** Only paths inside the target cluster's overlay, its
  `infrastructure/clusters/<cluster>` directory, the cluster ownership file, and
  (when no services are targeted) `.gitignore` and `.sops.yaml` are staged. Other
  work in a shared repository is left alone.
- **Unencrypted secret guard.** Before committing, the staged set is scanned for
  plaintext secrets. Any finding aborts the run with
  `refusing to commit: unencrypted secrets detected` and nothing is pushed.
  Encrypt the file, then retry.
- **Remote resolution.** `--remote` wins. Otherwise the branch's configured
  remote is used, then `upstream`, then `origin`. If none exists the command fails
  rather than pushing nowhere.
- **Branch resolution.** The configured Git branch wins, then the current branch,
  then `main`.
- **Rebase before push.** The command runs `git pull --rebase` before pushing, so
  a rejected non-fast-forward push usually means a genuine conflict to resolve.

## Partial runs

| Flag | Effect |
| --- | --- |
| `--push-only` (or `--skip-flux`) | Commit and push; do not touch the cluster |
| `--flux-only` | Trigger Flux reconciliation; do not commit or push |
| `--wait=false` | Return without waiting for Flux resources to report Ready |
| `--timeout 10m` | Raise the per-resource Flux wait (default `5m`) |
| `--kubeconfig <path>` | Override the cluster-owned kubeconfig |
| `-m, --message` | Custom commit message |

`--push-only` and `--flux-only` are opposites; passing both skips everything and
is not a useful run.

Use `--push-only` when the cluster is not reachable or you want CI to pick the
change up instead. Use `--flux-only` after a manual `git push`, or when the source
is lagging and you only need to force a sync.

## Service-scoped form

`cluster service reconcile` is the same workflow with the service as the required
argument, which removes the auto-detection step:

```bash
opencenter cluster service reconcile openstack-ccm
opencenter cluster service reconcile headlamp --generate
opencenter cluster service reconcile openstack-ccm --cluster myorg/my-cluster
```

Prefer it when you know exactly which service changed: the staging scope and the
Flux targets are both derived from that one name, so there is no chance of
sweeping up an unrelated edit.

## Verify

```bash
flux get kustomizations -n flux-system
flux get helmreleases -A
git -C <git_dir> log --oneline -1
git -C <git_dir> status --short
```

A successful run reports the commit SHA. `git status --short` should be empty for
the paths the command staged; anything left there was outside the scope it
computed.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `refusing to commit: unencrypted secrets detected` | A staged manifest holds plaintext secret material. Encrypt it with SOPS, or fix `.sops.yaml` matching, then retry. |
| `cluster name is required for reconcile` | No cluster argument and no active cluster. Pass `myorg/my-cluster` or select one. |
| `git_dir is not configured for cluster ...` | The configuration has no GitOps working directory; re-run `cluster init` or set the Git directory. |
| `no git remote found for repository in ...` | The working tree has no remote. Add `upstream` or `origin`, or pass `--remote`. |
| `kubeconfig is required for Flux reconciliation` | No cluster-owned kubeconfig and no `--kubeconfig`. Finish the deploy first, or use `--push-only`. |
| Nothing committed, no error | The scoped paths had no staged changes. Confirm your edit is under `applications/overlays/<cluster>/`. |
| A service was silently skipped | Its Flux target could not be resolved from the configuration, so reconcile moves on. Check the service name against [Platform services](../reference/platform-services.md). |

## Related

- [GitOps Workflow](../concepts/gitops-workflow.md) - why the repository is the source of truth.
- [Detect and Reconcile Drift](../operations/troubleshoot-deployment.md) - diagnosing a cluster that will not converge.
- [`cluster reconcile` reference](../reference/opencenter/opencenter_cluster_reconcile.md) - every flag.
- [`cluster service reconcile` reference](../reference/opencenter/opencenter_cluster_service_reconcile.md) - every flag.
