---
id: create-kind-cluster
title: "Create a Local Kind Cluster"
sidebar_label: Create Kind Cluster
description: Create a local openCenter Kubernetes cluster using Kind with Gitea and Flux bootstrap.
doc_type: how-to
audience: "platform engineers, developers"
tags: [kind, cluster, local, gitea, bootstrap, gitops, flux]
---

# Create a Local Kind Cluster

**Purpose:** For platform engineers and developers, shows how to stand up a local openCenter cluster on Kind with a disposable Gitea instance and FluxCD.

## Prerequisites

- Podman or Docker running
- `kind`, `flux` installed
- 8 GB RAM minimum
- This repository checked out locally

Install the CLI and the `opencenter-local` plugin:

```bash
mise run local-install
```

Verify:

```bash
opencenter version
opencenter local --help
kind --version
flux --version
```

## Bootstrap Flow

The sequence diagram below shows the internal calls made by `opencenter cluster bootstrap` for a Kind cluster. Podman binds the Gitea container port on `0.0.0.0`, so the host's routable IP (e.g. `172.16.0.146:3001`) is reachable from both the macOS host and from inside the Kind cluster. During `gitea-attach-kind`, the TLS certificate is regenerated with the host IP as a SAN, and all subsequent operations use this single URL.

```mermaid
sequenceDiagram
    autonumber
    participant User as User (host)
    participant CLI as opencenter CLI
    participant BS as BootstrapService
    participant KBP as KindBootstrapProvider
    participant Kind as kind CLI
    participant Gitea as Gitea Container
    participant GitOps as GitOps Service
    participant Flux as flux CLI
    participant K8s as Kind Cluster (K8s API)

    User->>CLI: opencenter cluster bootstrap my-cluster --container-runtime podman
    CLI->>BS: Bootstrap(ctx, opts)
    BS->>BS: loadConfig, resolvePaths, acquireLock
    BS->>KBP: BuildSteps(cfg, clusterPaths, opts)
    KBP-->>BS: []bootstrapStep (6 steps)

    Note over BS: Step execution loop (resumable via state file)

    rect rgb(230, 245, 255)
        Note over BS,Kind: Step 1: kind-create
        BS->>Kind: kind create cluster --config kind-config.yaml
        Kind->>Kind: Pull kindest/node image, create containers
        Kind-->>BS: cluster ready (control-plane + workers)
    end

    rect rgb(230, 245, 255)
        Note over BS,Kind: Step 2: kind-export-kubeconfig
        BS->>Kind: kind export kubeconfig --name my-cluster
        Kind-->>BS: kubeconfig written to infrastructure/clusters/my-cluster/kubeconfig.yaml
    end

    rect rgb(255, 243, 224)
        Note over BS,Gitea: Step 3: gitea-attach-kind
        BS->>Gitea: gitea.NewService → Status(ctx)
        Gitea-->>BS: Running: true
        BS->>Gitea: AttachKind(ctx)
        Gitea->>Gitea: podman network connect kind gitea
        Gitea->>Gitea: kindIP(ctx) → e.g. 10.89.0.38
        Gitea->>Gitea: hostRoutableIP() → e.g. 172.16.0.146
        Gitea->>Gitea: writeCertificates([10.89.0.38, 172.16.0.146])<br/>SANs: localhost, gitea, 127.0.0.1, ::1, 10.89.0.38, 172.16.0.146
        Gitea->>Gitea: podman restart gitea
        Gitea->>Gitea: waitForAPI(ctx)
        Note over Gitea: Verify IP stable after restart<br/>(re-cert + restart if IP changed)
        Gitea-->>BS: AttachResult{HostIP: 172.16.0.146, HostRepoURL: https://172.16.0.146:3001/...}
    end

    rect rgb(232, 245, 233)
        Note over BS,K8s: Step 4: flux-bootstrap
        BS->>Flux: flux.NewService → Bootstrap(ctx, "local/my-cluster")
        Flux->>Gitea: Status(ctx)
        Gitea-->>Flux: HostRepoURL: https://172.16.0.146:3001/..., UserToken: present

        Note over Flux: Single URL for both host and cluster:<br/>repoURL = https://172.16.0.146:3001/...

        Flux->>Flux: flux bootstrap git --url=repoURL --branch=main --path=applications/overlays/my-cluster --token-auth --ca-file=ca.pem
        Note over Flux,K8s: flux bootstrap git internally:
        Flux->>Gitea: 1. Clone repo via 172.16.0.146:3001 (host → Gitea ✅)
        Flux->>Flux: 2. Generate gotk-components.yaml, gotk-sync.yaml
        Flux->>Gitea: 3. Commit & push manifests via 172.16.0.146:3001 ✅
        Flux->>K8s: 4. kubectl apply gotk-components (installs Flux controllers)
        Flux->>K8s: 5. kubectl apply gotk-sync (creates GitRepository CR with url=172.16.0.146:3001)
        K8s->>Gitea: 6. source-controller clones https://172.16.0.146:3001/... ✅
        Note over K8s,Gitea: ✅ Works: Podman binds on 0.0.0.0<br/>so the host IP is reachable from<br/>inside the Kind cluster via the<br/>Podman VM bridge.
        Flux-->>Flux: ✅ health check passes (GitRepository ready)
        Flux-->>BS: BootstrapResult{RepoURL: https://172.16.0.146:3001/...}
    end

    rect rgb(245, 235, 255)
        Note over BS,Gitea: Step 5: gitea-rebase
        BS->>GitOps: PullRebase(ctx, gitDir)
        GitOps->>Gitea: Status(ctx)
        Gitea-->>GitOps: CAPath, UserToken
        GitOps->>GitOps: git pull --rebase origin main (via localhost:3001, token auth, CA cert)
        Gitea-->>GitOps: Flux bootstrap commits rebased into local checkout
        GitOps-->>BS: branch: main
    end

    rect rgb(232, 245, 233)
        Note over BS,Gitea: Step 6: gitops-push
        BS->>GitOps: Push(ctx, "local/my-cluster")
        GitOps->>GitOps: resolve cluster → git_dir
        GitOps->>Gitea: Status(ctx)
        Gitea-->>GitOps: LocalRepoURL: https://localhost:3001/newuser/test-repo.git
        GitOps->>GitOps: git remote set-url origin https://localhost:3001/...
        GitOps->>Gitea: git push -u origin main (via localhost:3001, token auth, CA cert)
        Gitea-->>GitOps: push accepted
        GitOps-->>BS: PushResult{RemoteURL: localhost:3001, Branch: main}
    end

    BS-->>CLI: ✅ bootstrap complete
    CLI-->>User: Bootstrap successful
```

The host's routable IP (`172.16.0.146`) works from both contexts because Podman binds the Gitea container port on `0.0.0.0`. The TLS certificate includes this IP as a SAN (added during `gitea-attach-kind`), so HTTPS verification succeeds everywhere. No post-bootstrap patching is needed.

## Steps

### 1. Start Local Gitea

```bash
opencenter local gitea up
```

Starts a disposable Gitea container (`docker.gitea.com/gitea:1.24.5`) and provisions a test user, API tokens, and a `test-repo` repository. State is written to `$OPENCENTER_CONFIG_DIR/local/` (typically `~/.config/opencenter/local/`).

Verify:

```bash
opencenter local gitea status
```

You should see `Running: true` and a repository URL at `https://localhost:3001/newuser/test-repo.git`.

### 2. Initialize the Cluster Configuration

```bash
opencenter cluster init my-cluster --org local --type kind
```

Creates a Kind cluster config under the `local` organization directory. Generates SOPS Age keys and an SSH key pair automatically.

Confirm the resolved paths:

```bash
opencenter cluster info my-cluster
```

### 3. Validate the Configuration

```bash
opencenter cluster validate my-cluster
```

### 4. Generate the GitOps Tree

```bash
opencenter cluster setup my-cluster --force
```

Produces the overlay tree under the cluster's `git_dir`: `applications/overlays/my-cluster/`, `infrastructure/clusters/my-cluster/`, and `secrets/`. The `flux-system/` directory does not exist yet; it is created later during bootstrap by `flux bootstrap git`.

### 5. Set the Git Remote and Token Provider

```bash
GITEA_REPO_URL=$(opencenter local gitea status 2>/dev/null | grep "Bootstrap repo URL:" | awk '{print $NF}')
GITEA_TOKEN_PATH=$(opencenter local gitea status 2>/dev/null | grep "User token present:" | sed 's/.*(\(.*\))/\1/')
opencenter cluster update my-cluster \
  --opencenter.gitops.git_url="$GITEA_REPO_URL" \
  --opencenter.gitops.git_token="$GITEA_TOKEN_PATH" \
  --opencenter.gitops.git_token_provider=gitea
```

Points the cluster configuration at the local Gitea repository using the host-routable IP (e.g. `https://172.16.0.146:3001/...`). This IP is reachable from both the macOS host and from inside the Kind cluster because Podman binds on `0.0.0.0`. The `git_token` field is set to the path of the Gitea user token file, and `git_token_provider` tells bootstrap how to read it.

Verify:

```bash
opencenter cluster info my-cluster | grep git_url
```

### 6. Bootstrap the Cluster

```bash
opencenter cluster bootstrap my-cluster --container-runtime podman
```

Substitute `docker` if that is your runtime. This command runs the full Kind bootstrap sequence:

Before starting, the command checks whether the GitOps directory has uncommitted changes. If it does, you are prompted to commit them. The `gitea-rebase` step runs `git pull --rebase`, which requires a clean working tree. If you decline, bootstrap aborts with a message explaining what to do.

1. `kind-create` — Creates the Kind cluster using the generated `kind-config.yaml`
2. `kind-export-kubeconfig` — Exports the kubeconfig to the cluster's infrastructure directory
3. `gitea-attach-kind` — Connects the local Gitea container to the Kind network and reissues the TLS certificate with the in-cluster IP
4. `flux-bootstrap` — Runs `flux bootstrap git` against the in-cluster Gitea URL
5. `gitea-rebase` — Rebases the local checkout to include the Flux bootstrap commits from Gitea
6. `gitops-push` — Pushes the generated GitOps repository to the local Gitea instance

The command is resumable. If a step fails, fix the issue and re-run. Use `--restart` to re-run all steps from scratch, or `--from-step <id>` to resume from a specific step (e.g., `--from-step gitea-attach-kind`).

## Verification

```bash
GITOPS_DIR=$(opencenter cluster info my-cluster 2>/dev/null | grep "git_dir:" | awk '{print $2}')
export KUBECONFIG="$GITOPS_DIR/infrastructure/clusters/my-cluster/kubeconfig.yaml"

kubectl get nodes
kubectl get pods -n flux-system
flux get sources git -n flux-system
flux get kustomizations -n flux-system
```

All Kind nodes should be `Ready`, Flux pods running in `flux-system`, and the `flux-system` Git source `READY=True`.

## Cleanup

### Destroy the cluster

```bash
opencenter cluster destroy my-cluster --force
```

This runs `kind delete cluster`, removes the GitOps directory, the cluster configuration file, and the infrastructure and applications directories. It also clears the active cluster marker if `my-cluster` was selected.

The command prompts for confirmation unless `--force` is passed.

### Destroy the local Gitea instance

```bash
opencenter local gitea destroy
```

Stops the Gitea container and removes the local state directory (`$OPENCENTER_CONFIG_DIR/local/` — metadata, tokens, certificates, mounted data). This is separate from `cluster destroy` because a single Gitea instance can serve multiple local clusters.

### Verify nothing is left

```bash
kind get clusters                # should not list my-cluster
opencenter cluster list          # should not list my-cluster
```

## Troubleshooting

### `opencenter: unknown command "local"`

The plugin is not installed. Run `mise run local-install`.

### `kindest/node` Image Not Found

The selected Kubernetes version does not have a published `kindest/node` image. Pick a valid tag, update the config, then rerun:

```bash
opencenter cluster setup my-cluster --force
opencenter cluster bootstrap my-cluster --container-runtime podman --restart
```

### Bootstrap Fails at `gitea-attach-kind`

Gitea must be running before bootstrap. Verify with `opencenter local gitea status`. If it is not running, start it with `opencenter local gitea up` and re-run bootstrap.

### Bootstrap Fails at `gitops-push` With "rejected (fetch first)"

The remote Gitea repository already contains content from a previous run. Either destroy and recreate Gitea (`opencenter local gitea destroy && opencenter local gitea up`) or restart bootstrap from scratch:

```bash
opencenter cluster bootstrap my-cluster --container-runtime podman --restart
```

### Bootstrap Fails at `flux-bootstrap` With Connection Timeout

The Flux source-controller inside the cluster cannot reach Gitea at the host IP. Verify Gitea is attached to the Kind network and the host IP is set:

```bash
opencenter local gitea status
```

Check that `Kind Attached: true`, `Host IP` is set, and `Bootstrap repo URL` shows the host IP (not `localhost`). If not, re-run bootstrap from the attach step:

```bash
opencenter cluster bootstrap my-cluster --container-runtime podman --from-step gitea-attach-kind
```

If the host IP changed (e.g. after switching networks), destroy and recreate Gitea to regenerate the TLS certificate:

```bash
opencenter local gitea destroy
opencenter local gitea up
opencenter cluster bootstrap my-cluster --container-runtime podman --from-step gitea-attach-kind
```

### Stale Bootstrap Lock

If bootstrap was interrupted:

```bash
rm -f ~/.local/state/opencenter/locks/my-cluster.lock
opencenter cluster bootstrap my-cluster --container-runtime podman --restart
```

## Evidence

- Kind bootstrap provider (step definitions): `internal/cluster/kind_bootstrap_provider.go`
- Bootstrap service (step execution engine): `internal/cluster/bootstrap_service.go`
- Flux bootstrap logic: `internal/localdev/flux/service.go`
- GitOps push logic: `internal/localdev/gitops/service.go`
- Gitea service: `internal/localdev/gitea/service.go`
- Kind provider (create/delete/kubeconfig): `internal/cloud/kind/provider.go`
- Kind defaults: `internal/config/defaults/kind.yaml`
- Renderer vs bootstrap ownership: `docs/dev/rendering-contract.md`
