---
id: openstack-first-cluster
title: Configure an OpenStack cluster
sidebar_label: Configure an OpenStack cluster
description: Configure, validate, and deploy an OpenStack cluster with openCenter.
doc_type: tutorial
audience: openCenter users
tags: [openstack, configuration]
last_updated: 2026-10-15
---
# Configure an OpenStack cluster

This tutorial walks through configuring an OpenStack cluster file, reviewing
provider selections from a live cloud, validating the result, and deploying
the cluster. Every command below is part of the `opencenter` CLI surface; no
external tools are required until deployment itself.

## 1. Initialize and inspect

```bash
opencenter cluster init demo --org my-org --type openstack
opencenter cluster describe my-org/demo
```

The generated v2 file lives at:

```text
~/.config/opencenter/clusters/blueprints/<org>/<name>/<name>-config.yaml
```

Use `opencenter cluster set` to update individual fields without opening the
file. The dot-path for OpenStack provider values is under
`opencenter.infrastructure.cloud.openstack`:

```bash
opencenter cluster set my-org/demo \
  opencenter.infrastructure.cloud.openstack.auth_url=https://keystone.api.<region>.example.com/v3/ \
  opencenter.infrastructure.cloud.openstack.region=<REGION>
```

The readiness rules require non-placeholder values for `auth_url`, `region`,
`project_id`, `image_id`, and `availability_zone`. Application credential ID
and secret are validated as a pair — either both present or both empty.

## 2. Plan provider selections

Plan performs read-only discovery against the target cloud and prints a typed
change set without writing the cluster file or mutating any OpenStack resource:

```bash
opencenter cluster provider openstack plan my-org/demo \
  --os-cloud my-profile
```

Selectors include `--image-name`, `--windows-image-name`, `--network-id`,
`--external-network-id`, `--subnet-id`, and `--availability-zone`. Each accepts a
resource **name or ID** — the name printed by the `Selection required:` list is
what you pass, e.g. `--image-name "Ubuntu 24.04"`. Matching is case-insensitive on
the exact name; a name matching several discovered resources is rejected with the
candidate list so nothing is chosen silently. The value written to the config is
always the resolved ID, which is what OpenTofu consumes. If the
generated infrastructure should own the internal network and subnet, use:

```bash
opencenter cluster provider openstack plan my-org/demo \
  --os-cloud my-profile --create-internal-network
```

In this mode OpenTofu creates a dedicated tenant network, subnet, router,
and VRRP port for the cluster. Leave `network_id` and `subnet_id` empty. This
is the recommended mode when multiple clusters share the same project, because
it avoids two clusters' routers competing on a shared subnet.

That mode cannot be combined with `--network-id` or `--subnet-id`. If those
fields are already populated, add `--replace` to clear them.

### Choosing the right mode

| Mode | When to use |
|------|-------------|
| existing `--network-id` / `--subnet-id` | The tenant has a dedicated, single-cluster network managed outside OpenTofu |
| `--create-internal-network` | Multiple clusters in the same project, or you want full OpenTofu ownership of networking |

Using an existing shared subnet without `--create-internal-network` will cause
OpenTofu to create a second router interface on that subnet, which fails with
`IpAddressAlreadyAllocated` if another router is already attached.

## 3. Apply the reviewed selections

Apply writes the planned selections to the cluster configuration file:

```bash
# Using an existing network
opencenter cluster provider openstack apply my-org/demo \
  --os-cloud my-profile --image-name "Ubuntu 24.04" --network-id cluster-net \
  --external-network-id PUBLICNET --subnet-id cluster-subnet

# Letting OpenTofu manage the internal network
opencenter cluster provider openstack apply my-org/demo \
  --os-cloud my-profile --create-internal-network --replace \
  --image-name "Ubuntu 24.04" --windows-image-name "Windows Server 2022"
```

`--import-auth` copies the application credential ID and secret from the
clouds.yaml profile into the cluster config so they never appear in plain
text on the command line. Always prefer it over pasting credential values
manually:

```bash
opencenter cluster provider openstack apply my-org/demo \
  --os-cloud my-profile --import-auth --yes
```

`--replace` permits replacing already-populated selections. `--yes` is
required for non-interactive confirmation; global `--dry-run` stops before
the local write.

### Flavor and volume-type availability

Region flavor names differ between Rackspace regions. Confirm the exact names
before deploying:

```bash
export OS_CLOUD=my-profile
openstack flavor list | grep "^|.*gp\."
openstack volume type list
```

If the config inherited flavors from a different region's defaults, update
them via `cluster set`:

```bash
opencenter cluster set my-org/demo \
  opencenter.infrastructure.compute.flavor_bastion=gp.0.2.4 \
  opencenter.infrastructure.compute.flavor_master=gp.0.4.8 \
  opencenter.infrastructure.compute.flavor_worker=gp.0.4.16
```

A mismatch produces `Error: Unable to find flavor with name <flavor>` during
`tofu apply`.

### Capacity planning

Check Nova quota before sizing the cluster. A typical dev cluster
(1 bastion + 1 master + 2 workers) needs approximately 14 vCPU / 28 GB RAM:

```bash
openstack quota show
openstack server group list   # groups left behind by destroyed clusters consume quota
```

If the project quota cannot accommodate the configured node counts, reduce
`master_count` and/or `worker_count` rather than expecting the scheduler to
fit the instances:

```bash
opencenter cluster set my-org/demo \
  opencenter.infrastructure.compute.master_count=1 \
  opencenter.infrastructure.compute.worker_count=2
```

Exceeding quota causes instances to enter `ERROR` state with fault code 413
(`Quota exceeded for cores, ram`). Destroy the failed instances and re-deploy
after freeing capacity.

## 4. Resolve validation errors

Run offline validation after completing the configuration:

```bash
opencenter cluster validate my-org/demo
```

Common validation errors and how to resolve them using only the CLI:

**GitOps repository URL missing.** Create an empty private GitHub repo first,
then point the cluster at it:

```bash
gh repo create <user-or-org>/my-cluster-gitops --private --disable-wiki

opencenter cluster set my-org/demo \
  opencenter.gitops.repository.url=https://github.com/<user-or-org>/my-cluster-gitops.git \
  opencenter.gitops.repository.path=clusters/my-cluster \
  opencenter.gitops.repository.local_dir=$HOME/.config/opencenter/clusters/gitops/my-cluster-gitops
```

**GitOps auth token missing.** HTTPS GitOps requires a token value or
`token_file`. Supply the token directly:

```bash
GH_TOKEN=$(gh auth token)
opencenter cluster set my-org/demo \
  opencenter.gitops.auth.token.provider=github \
  opencenter.gitops.auth.token.token="$GH_TOKEN"
```

**Keycloak admin password placeholder.** Generate and set a real value:

```bash
opencenter cluster set my-org/demo \
  "secrets.keycloak.admin_password=$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"
```

**Loki / Tempo S3 endpoint missing.** Provision object storage for each
enabled S3-backed service via the CLI. `--rotate-credentials` creates new
Keystone EC2 credentials for the service container:

```bash
S3_ENDPOINT="https://swift.api.<region>.rackspacecloud.com"

for service in loki tempo; do
  opencenter cluster service storage apply "$service" \
    --cluster my-org/demo \
    --backend s3 \
    --os-cloud my-profile \
    --s3-endpoint "$S3_ENDPOINT" \
    --rotate-credentials \
    --yes
done
```

Run this for every S3-backed service enabled in `opencenter.services` where
`storage_type` is not yet set to `s3`. Services such as `mimir`, `velero`,
and `etcd-backup` use the same command if they are enabled.

After resolving all errors, re-run validation:

```bash
opencenter cluster validate my-org/demo
```

Validation must pass with zero errors before generating or deploying.

## 5. Generate, sync secrets, and publish

Generate the manifests, then synchronize encrypted secrets into the GitOps
tree. Secret synchronization must come after the final generation because
generation can recreate service overlays:

```bash
opencenter cluster generate my-org/demo --force
opencenter secrets sync my-org/demo
```

Confirm that `secret.yaml` files contain SOPS-encrypted `ENC[...]` values
rather than plaintext before publishing:

```bash
grep -rl "ENC\[" ~/.config/opencenter/clusters/gitops/my-cluster-gitops/applications/overlays/my-cluster/
```

The deploy command does not commit or push the GitOps repository. Publish it
before starting deployment:

```bash
GITOPS_DIR="$HOME/.config/opencenter/clusters/gitops/my-cluster-gitops"
cd "$GITOPS_DIR"

git init
git branch -M main
git remote add origin "https://github.com/<user-or-org>/my-cluster-gitops.git"
git add -A
git commit -m "Deploy my-org/my-cluster"
git push -u origin main
```

Whenever you change the cluster configuration and re-generate, commit and push
the updated tree before running `deploy` again.

## 6. Deploy

On macOS, use a writable temporary Helm cache to avoid permission failures in
the default cache directory:

```bash
cache_dir=$(mktemp -d /private/tmp/opencenter-helm-cache.XXXXXX)
HELM_CACHE_HOME="$cache_dir" opencenter cluster deploy my-org/demo \
  --break-lock \
  --yes \
  --log-level info
```

The deployment runs 13 steps: preflight, OpenTofu init, OpenTofu apply
(networking, security groups, instances), Kubespray bootstrap, kubeconfig
normalization, CNI install, and Flux-driven service reconciliation.

Bootstrap logs are written under:

```text
~/.local/state/opencenter/logs/bootstrap/<org>/<cluster>/
```

Tail the log while the cluster comes up:

```bash
tail -f ~/.local/state/opencenter/logs/bootstrap/<org>/my-cluster/*.log
```

### Resumable deploys

If a step fails, correct the underlying cause (quota, flavor, network) and
re-run the same command — the deploy resumes from its saved state. Use
`--from-step <step-id>` to skip completed steps explicitly, or `--restart`
to force a clean rerun from step one.

Before retrying after a failed `tofu apply`, destroy the partial resources so
the next apply starts from a clean slate:

```bash
cd ~/.config/opencenter/clusters/gitops/my-cluster-gitops/infrastructure/clusters/my-cluster
export OS_CLOUD=my-profile
tofu destroy -auto-approve
rm -rf .terraform .opentofu-local-*
```

Then re-generate, re-sync, commit, push, and re-deploy.

### SSH keys in the GitOps tree

OpenTofu writes `id_rsa` / `id_rsa.pub` into the infrastructure directory
during the keypair step. These files block subsequent `cluster generate`
calls because the generator refuses to overwrite user-authored content.
Remove them before regenerating:

```bash
rm -f ~/.config/opencenter/clusters/gitops/my-cluster-gitops/infrastructure/clusters/my-cluster/id_rsa*
```

## 7. Verify the deployed cluster

Once the API is reachable, export the generated kubeconfig and confirm the
cluster is healthy:

```bash
export KUBECONFIG="$HOME/.config/opencenter/clusters/state/<org>/my-cluster/kubeconfig.yaml"

kubectl get nodes \
  -o custom-columns=NAME:.metadata.name,PROVIDER_ID:.spec.providerID,TAINTS:.spec.taints

kubectl get pods -A
kubectl get gitrepositories,kustomizations,helmreleases -A
```

An OpenTofu apply completing successfully is only the infrastructure milestone.
The cluster is ready when:

- All expected OpenStack instances are active.
- Kubernetes nodes are Ready.
- Every node has an OpenStack provider ID.
- The external CCM has removed the `node.cloudprovider.kubernetes.io/uninitialized` taint.
- Calico is rolled out and healthy.
- Flux Kustomizations reconcile successfully.
- Selected HelmReleases are Ready.
- OpenStack CSI is functioning for enabled persistent volumes.

## Evidence

- OpenStack command surface: `cmd/cluster_provider_openstack.go`
- Service storage provisioning: `cmd/cluster_service_storage.go`
- Initialization and configuration: `cmd/cluster_init.go`, `cmd/cluster_set.go`
- Readiness rules: `internal/config/v2/readiness.go`
- OpenStack lifecycle: `internal/cluster/bootstrap_provider_infra.go`
- Provider workflow examples: `tests/features/workflow.feature`
