---
id: kind-local-development
title: Local development with Kind
sidebar_label: Local development with Kind
description: Configure, render, and deploy a local Kind cluster reconciled by Flux from a local Gitea.
doc_type: tutorial
audience: openCenter developers
tags: [kind, local-development]
last_updated: 2026-10-13
---
# Local development with Kind

This tutorial brings up a local Kind cluster end-to-end: init the config, point
GitOps at a local Gitea, validate, generate, deploy, and confirm the cluster is
reconciled by Flux from that Gitea. The CLI handles the awkward parts — TLS
cert reissue on attach, GitRepository source re-homing, and source-secret CA
refresh — so the happy path is about six commands.

## Prerequisites

- A container runtime (`docker` or `podman`) with a running daemon; on macOS
  with Podman, `podman machine start` first (see
  [Troubleshooting](#troubleshooting)).
- On Apple Silicon and other ARM hosts, Rosetta emulation for amd64
  containers. The `goharbor/*` images are published for amd64 only, and
  without Rosetta the Harbor Go services (core, exporter, jobservice,
  registry and Trivy) crash on start with `fatal error: lfstack.push`.
  In Docker Desktop, enable **Use Rosetta for x86_64/amd64 emulation on Apple
  Silicon**. With Colima, start the VM with `--vm-type vz --vz-rosetta`.
- `kind`, `kubectl`, `helm`, and `flux` on your PATH (mise installs the first
  three; install flux once: `brew install fluxcd age`).
- From the repository root, build the CLI:

  ```bash
  mise run build-cli        # -> ./bin/opencenter
  ```

  Use `./bin/opencenter` in the commands below (or add `bin/` to your PATH).

## 1. Start the local services

```bash
opencenter local gitea up      # Gitea + tokens + test repo
opencenter local gitea status  # expect "Running: true"
```

This creates the Gitea container, the `admin`/user tokens under
`~/.config/opencenter/local/tokens/`, and the `test-repo` that the cluster's
GitOps `origin` points at. If Loki/Tempo S3 storage is enabled, also start the
local object store:

```bash
opencenter local rustfs up     # S3 API on :9000, credentials.json written
cat ~/.config/opencenter/local/rustfs/credentials.json
```

## 2. Initialize the cluster configuration

```bash
opencenter cluster init dev --org local --type kind
opencenter cluster describe local/dev   # inspect generated paths and values
```

To let openCenter manage the CNI instead of Kind's default (kindnet), add
`--kind-disable-default-cni` to the init command. Kind init defaults the GitOps
provider to **gitea**, disables OpenTofu, and clears OpenStack auth.

## 3. Point GitOps at the local Gitea

Pick the git URL by where **you** will be running deploy from:

| You run deploy from | Use this URL |
|---|---|
| This host (typical) | `https://<host-ip>:3001/newuser/test-repo.git` |
| A remote workstation | `https://gitea.oc-baremetal:3001/newuser/test-repo.git` (add the IP + name to `/etc/hosts`) |

Avoid `localhost` as the URL host: it works for host-side tools but not from
inside the cluster, and kind-network IPs work only inside the VM. The host-routable
IP works on both sides, and the deploy steps fix up the in-cluster URLs
themselves (see
[How the Git URL is handled](#how-the-git-url-is-handled)).

Set the URL and token file, and align the local checkout's `origin` (the deploy
pre-check rejects a mismatch):

```bash
git remote get-url origin    # if no repo exists yet, skip
opencenter cluster set local/dev \
  "opencenter.gitops.repository.url=https://<host-ip>:3001/newuser/test-repo.git" \
  "opencenter.gitops.auth.token.token_file=$HOME/.config/opencenter/local/tokens/gitea-user.token"
```

If you change the URL later, re-point `origin` again:

```bash
git -C <gitops-local-dir> remote set-url origin https://<host-ip>:3001/newuser/test-repo.git
```

## 4. Make the configuration valid

A fresh Kind init commonly fails validation on placeholder secrets and missing
S3 endpoints. Set them (dot notation via `cluster set`):

```bash
RUSTFS_CREDS=~/.config/opencenter/local/rustfs/credentials.json
ACCESS_KEY=$(python3 -c "import json;print(json.load(open('$RUSTFS_CREDS'))['access_key'])")
SECRET_KEY=$(python3 -c "import json;print(json.load(open('$RUSTFS_CREDS'))['secret_key'])")

opencenter cluster set local/dev \
  "opencenter.services.loki.s3_endpoint=http://<host-ip>:9000" \
  "opencenter.services.tempo.s3_endpoint=http://<host-ip>:9000" \
  "secrets.loki.s3_access_key_id=$ACCESS_KEY" \
  "secrets.loki.s3_secret_access_key=$SECRET_KEY" \
  "secrets.tempo.access_key=$ACCESS_KEY" \
  "secrets.tempo.secret_key=$SECRET_KEY" \
  "secrets.keycloak.admin_password=<strong-password>" \
  "opencenter.services.keycloak.instances=2"

opencenter cluster validate local/dev     # expect: passed, 0 failed
```

Other common failures:

- **Placeholder secrets** — anything still `CHANGEME`
  (`secrets.*`, `opencenter.gitops.auth.token.token`).
- **Schedulability** — `opencenter.services.keycloak.instances` must not exceed
  the schedulable Linux workers
  (`opencenter.infrastructure.kind.worker_count`, default 2); either lower
  instances or raise worker count.

## 5. Generate manifests, sync secrets, commit

```bash
opencenter cluster generate local/dev     # render GitOps manifests
opencenter secrets sync local/dev         # encrypt + wire secret manifests

cd <gitops-local-dir>                     # shown by cluster describe
git add -A && git commit -m "add dev cluster gitops manifests"
```

The tree must be clean **and committed before deploy**: `gitea up` seeds the
remote test-repo from this checkout, and the deploy's rebase step pushes local
commits on top of whatever Flux already wrote. Preview generation with the
global `--dry-run`, or limit to template rendering with `--render-only`.

If a previous cluster left the tree dirty, restore tracked files
(`git checkout -- .gitignore .sops.yaml`), then commit or remove the rest.

## 6. Deploy

```bash
opencenter cluster deploy local/dev --container-runtime podman   # or docker
```

Preview without side effects:

```bash
opencenter --dry-run cluster deploy local/dev --container-runtime podman
```

Ordered steps (as reported by the plan):

1. `kind-create`
2. `kind-export-kubeconfig`
3. `kind-label-workers`
4. `kind-install-cni` (managed-CNI mode only)
5. `gitea-attach-kind` — attaches Gitea to the Kind network and **reissues its TLS cert + CA**
6. `flux-bootstrap` — bootstraps Flux from the Gitea URL, then refreshes the in-cluster source secret's CA to match
7. `reconcile-sops-age-secret`
8. `reconcile-grafana-admin-secret`
9. `gitea-source-rehome` — rewrites generated GitRepository sources to the in-cluster `gitea:3001` alias
10. `gitea-rebase`
11. `gitops-push`
12. `flux-verify`

Resume control: rerunning deploy resumes from saved state
(`~/.local/state/opencenter/bootstrap/<org>/<name>/state.json`); `--restart`
ignores saved state, `--from-step <id>` starts at a named step, `--break-lock`
clears a stale operation lock.

## 7. Confirm the cluster

```bash
opencenter cluster status local/dev --refresh
export KUBECONFIG=~/.config/opencenter/clusters/state/local/dev/kubeconfig.yaml
kubectl get nodes                          # all Ready
flux get sources git -n flux-system        # flux-system True
kubectl get kustomizations -n flux-system  # flux-system True
```

A successful deploy reports `Cluster ready at https://127.0.0.1:6443`.

## Teardown

```bash
opencenter cluster destroy local/dev --force          # removes the Kind cluster
opencenter local gitea destroy                        # optional
opencenter local rustfs destroy                       # optional
```

`destroy` preserves local configuration and GitOps files; add
`--remove-files` to delete those too.

## How the Git URL is handled

No single Gitea URL is reachable from **both** the host and inside the cluster:

| URL | Host (macOS) | In-cluster pods |
|---|---|---|
| `localhost:3001` | ✓ | ✗ |
| Host-routable IP `:3001` | ✓ | ✓ (port-forwarded into the bridge) |
| Kind-network IP `10.89.0.x:3001` | ✗ | ✓ while Gitea keeps that IP |
| Podman DNS alias `gitea:3001` | ✗ | ✓ stable across restarts |

The CLI therefore uses two views of the same repository:

1. **Host side** — `flux bootstrap` clones with your configured `git_url`
   (host IP works best). During `gitea-attach-kind`, that host is added to the
   reissued certificate as a SAN (alongside `localhost`, `gitea`, the
   Kind-network IP, and the host's routable IP), so a friendly `/etc/hosts`
   name works with TLS. Verify anytime:

   ```bash
   curl --cacert ~/.config/opencenter/local/gitea/gitea/certs/ca.pem \
     https://<git-host>:3001/ -o /dev/null -w '%{http_code}\n'   # expect 200
   ```

2. **In-cluster side** — after bootstrap, `gitea-source-rehome` rewrites the
   generated GitRepository sources to `gitea:3001` (resolvable from pods via
   Podman's built-in DNS), and the bootstrap step refreshes the
   `flux-system` source secret's `ca.crt` because `gitea-attach-kind` rotated
   the CA right before the secret was created. Without both, the in-cluster
   source fails with `x509: certificate signed by unknown authority` or
   `no route to host`.

Changing the URL after generating? Re-run `cluster generate` (it rewrites the
`opencenter-keycloak-config` / `opencenter-olm-config` sources and leaves the
deploy-managed `gotk-*` files alone), re-point `origin`, commit, and push with
`opencenter cluster deploy local/dev --step gitops-push`.

## Troubleshooting

### `VM does not exist` / cannot connect to Podman socket

The machine record can survive but fail to boot. Rebuild it:

```bash
podman machine rm podman-machine-default
podman machine init --cpus 4 --memory 8192 --disk-size 50 && podman machine start
```

### RustFS refuses to start: `image drifted`

The plugin compares image refs against the pinned immutable reference and
validates the running container against it (digest-tolerant, since podman
normalizes `repo:tag@digest` to `repo@digest` on container create). A leftover
container from an interrupted run can trip this check; remove the container
and its state dir, then retry:

```bash
podman rm -f rustfs
rm -rf ~/.config/opencenter/local/rustfs
opencenter local rustfs up
```

### `flux bootstrap` times out cloning the repository

Usually means the git URL isn't reachable from the host — switch to the
host-routable IP (Section 3) and redeploy. If the *in-cluster* side shows
`x509: certificate signed by unknown authority`, the stored source-secret CA is
stale; rerun deploy (the CA refresh runs with `flux-bootstrap`), or check the
fingerprints:

```bash
diff <(openssl x509 -in ~/.config/opencenter/local/gitea/gitea/certs/ca.pem -noout -fingerprint) \
     <(KUBECONFIG=$KUBECONFIG kubectl -n flux-system get secret flux-system \
        -o jsonpath='{.data.ca\.crt}' | base64 -d | openssl x509 -noout -fingerprint)
```

### `git pull --rebase` conflicts during `gitea-rebase`

An interrupted rebase in the GitOps checkout poisons later resumes. Abort and
reset to what the remote actually has, then regenerate:

```bash
git -C <gitops-local-dir> rebase --abort
git -C <gitops-local-dir> reset --hard origin/main
opencenter cluster generate local/dev && opencenter secrets sync local/dev
git -C <gitops-local-dir> add -A && git commit -m "regen"
opencenter cluster deploy local/dev --container-runtime podman
```

### Deploy skips cluster creation after a destroy

Saved step state marks `kind-create` etc. as complete. Either rerun deploy with
`--restart`, or delete those entries from
`~/.local/state/opencenter/bootstrap/local/dev/state.json`.

### Probing the cluster hits PodSecurity denials

`kubectl run ... -it` violates the restricted PSA profile. Use a manifest pod
with an explicit command instead:

```yaml
apiVersion: v1
kind: Pod
metadata: {name: nettest, namespace: flux-system}
spec:
  restartPolicy: Never
  containers:
  - name: c
    image: alpine:3.20
    command: ["/bin/sh","-c","nslookup gitea; wget -q --no-check-certificate -T5 https://gitea:3001/api/v1/version && echo OK"]
```

## Evidence

- Kind flags: `cmd/cluster_init.go`, `cmd/cluster_deploy.go`
- Provider validation: `internal/config/v2/readiness.go`
- Kind bootstrap implementation: `internal/cluster/kind_bootstrap_provider.go`
- Local Gitea attach + cert reissue: `internal/localdev/gitea/service.go`
- Flux bootstrap + source-secret CA refresh: `internal/localdev/flux/service.go`
- GitOps origin pre-check: `cmd/cluster_deploy.go` (`verifyOriginMatchesGitURL`)
- Local Gitea lifecycle: `cmd/opencenter-local` (`gitea up|status|attach-kind|destroy`)
- Workflow limitation: `tests/features/workflow.feature:1-8`
- Deep dive on failure modes: `docs/notes/kind-local-dev-lessons.md`
