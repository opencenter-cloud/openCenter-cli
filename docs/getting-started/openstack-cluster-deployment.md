---
id: openstack-cluster-deployment
title: Deploy an OpenStack cluster
sidebar_label: Deploy an OpenStack cluster
description: Repeatable SJC3 workflow to deploy a Kubernetes cluster on OpenStack with Kubespray, Flux, and Calico.
doc_type: tutorial
audience: openCenter users
tags: [openstack, deployment, kubespray]
last_updated: 2026-10-24
---
# Deploying a Kubernetes Cluster on OpenStack with openCenter

## A repeatable SJC3 workflow with Kubespray, Flux, Calico, and OpenStack cloud integration

Deploying Kubernetes on OpenStack is more than provisioning a few virtual machines. A reliable deployment must align the OpenStack inventory, infrastructure-as-code, Kubespray, the cloud controller, storage, networking, and GitOps reconciliation.

This article describes the repeatable OpenStack deployment path used with openCenter in the SJC3 region. It focuses on the OpenStack/Kubespray provider.

## The target architecture

The deployment is composed of several layers:

1. openCenter renders the cluster configuration and infrastructure manifests.
2. OpenTofu provisions the tenant network, compute instances, security groups, volumes, and related infrastructure.
3. Kubespray bootstraps Kubernetes on the provisioned instances.
4. Flux reconciles the cluster configuration and application services from Git.
5. The OpenStack cloud controller manager assigns provider IDs and manages cloud-facing Kubernetes resources.
6. Calico supplies the cluster networking layer, while OpenStack CSI provides block storage.

## Before you begin

You need:

- An openCenter CLI built from the intended branch.
- An approved OpenStack cloud profile for the target region.
- `yq`, OpenTofu or Terraform, Helm, kubectl, SOPS, and Flux installed at the versions required by the repository.
- Access to create a GitOps repository and credentials that the cluster can use.
- An SSH keypair registered in OpenStack.
- A fresh cluster name and a unique GitOps path.

Keep OpenStack credentials, application credentials, age keys, SSH private keys, and tokens in the local secret or cloud configuration. They must never be placed in the blueprint, this article, or the Git repository.

## Validate the regional inventory first

Do not assume that an image, flavor, volume type, or network from another OpenStack region is available in SJC3. Confirm the inventory before creating a blueprint.

The SJC3 profile used by the current deployment includes:

| Resource | Value |
| --- | --- |
| Region | `SJC3` |
| Image | Ubuntu 24.04, selected from the current SJC3 inventory |
| Master flavor | `gp.0.4.8` |
| Worker flavor | `gp.0.8.32` |
| Bastion flavor | `gp.0.2.4` |
| Masters / workers | `3 / 5` |
| Volume type | `local` |
| External network | `PUBLICNET` |
| Availability zone | `az1` |
| Kubernetes pod CIDR | `10.42.0.0/16` |
| Kubernetes service CIDR | `10.43.0.0/16` |
| Server group policy | Soft anti-affinity |

The external network is used for routing and floating IP connectivity. It is not the tenant network on which the Kubernetes instances should be booted. Leave tenant `network_id` and `subnet_id` empty when the infrastructure module is configured to create and manage those resources.

Example checks:

```sh
export OS_CLOUD=<approved-sjc3-cloud>

openstack image list
openstack flavor show gp.0.4.8
openstack flavor show gp.0.8.32
openstack network show PUBLICNET
openstack keypair list
```

## Create the cluster configuration

Set the organization and a new, unique cluster name:

```sh
export ORG=<organization>
export CLUSTER=<fresh-cluster-name>
export GH_USER=<github-user-or-organization>
```

For a first deployment, initialize a complete configuration from the schema:

```sh
opencenter cluster init "$CLUSTER" \
  --org "$ORG" \
  --type openstack \
  --region SJC3 \
  --full-schema

CONFIG="$HOME/.config/opencenter/clusters/blueprints/$ORG/$CLUSTER/$CLUSTER-config.yaml"
${EDITOR:-vi} "$CONFIG"
```

Fill in the validated SJC3 values, OpenStack application credentials, enabled services, Git authentication, and the GitOps repository URL. The repository URL should be:

```text
https://github.com/<github-user-or-organization>/<cluster-name>-gitops.git
```

If a complete, secret-free OpenStack profile already exists, it can be used instead:

```sh
opencenter cluster init "$CLUSTER" \
  --org "$ORG" \
  --type openstack \
  --region SJC3 \
  --config-file /path/to/sjc3-openstack-profile.yaml \
  --force
```

When reusing a profile, replace every old cluster name and regional value. In particular, verify the cluster FQDNs,
OpenStack and object-storage endpoints, bucket names, GitOps repository URL, repository path, and SSH key path. Give
each cluster its own clean checkout rather than reusing an organization-wide working tree:

```yaml
opencenter:
  gitops:
    repository:
      url: https://github.com/<github-user-or-organization>/<cluster-name>-gitops.git
      path: clusters/<cluster-name>
      local_dir: /absolute/path/to/gitops/<cluster-name>-gitops
```

Set `opencenter.gitops.base_repo.release` to the intended release, for example `2026.03-rc05`. Any explicit
OpenTofu module `source` override must reference a tag, commit, or branch that exists in the remote repository;
a local-only branch causes `tofu init` to fail before infrastructure is applied. Check custom refs with
`git ls-remote` before deployment.

Create the empty remote GitOps repository before generating manifests:

```sh
gh repo create "$GH_USER/$CLUSTER-gitops" --private --disable-wiki
```

Do not initialize the remote repository with a README, `.gitignore`, or license. Confirm that the repository URL in `$CONFIG` matches this new repository before continuing.

Keep credentials in the local configuration or referenced secret files. The generated repository must contain only encrypted Kubernetes secrets.

## Provision storage for enabled services

Before validation and generation, provision object storage for each enabled S3-backed service. For the standard SJC3 service set:

```sh
export S3_ENDPOINT=https://swift.api.sjc3.rackspacecloud.com

for service in harbor loki mimir tempo velero etcd-backup; do
  opencenter cluster service storage apply "$service" \
    --cluster "$ORG/$CLUSTER" \
    --backend s3 \
    --os-cloud "$OS_CLOUD" \
    --s3-endpoint "$S3_ENDPOINT" \
    --rotate-credentials \
    --yes
done
```

Run this only for services enabled in the cluster configuration. `--rotate-credentials` creates or replaces service credentials; omit it when a complete existing credential pair should be reused.

## Validate the configuration

Run validation after completing the configuration:

```sh
opencenter cluster validate "$ORG/$CLUSTER"
```

Resolve every validation error before generating or deploying the cluster.

The current command performs local validation. It does not prove that a custom remote module ref exists, so perform
the `git ls-remote` check above whenever the profile overrides an OpenTofu module source.

## Generate, synchronize, and publish GitOps

Generate the manifests, then synchronize encrypted secrets. Secret synchronization must happen after the final generation because generation can recreate service overlays.

```sh
opencenter cluster generate "$ORG/$CLUSTER" --force
opencenter secrets sync "$ORG/$CLUSTER"
```

Before committing, confirm that required `secret.yaml` files exist and contain encrypted `ENC[...]` values rather than plaintext credentials.

The deployment command does not commit or push the generated GitOps repository. Publish it before starting deployment:

```sh
GITOPS_DIR="$HOME/.config/opencenter/clusters/gitops/$CLUSTER-gitops"
cd "$GITOPS_DIR"

git init
git branch -M main
git remote add origin "https://github.com/$GH_USER/$CLUSTER-gitops.git"
git add -A
git commit -m "Deploy $ORG/$CLUSTER"
git push -u origin main
```

If `origin` already exists, verify it with `git remote -v` instead of adding it again. Whenever generation or secret synchronization changes the GitOps tree, commit and push those changes before deploying.

## Deploy the infrastructure and cluster

On macOS, use a writable temporary Helm cache. This avoids failures caused by an unwritable default cache directory.

```sh
cache_dir=$(mktemp -d /private/tmp/opencenter-helm-cache.XXXXXX)
HELM_CACHE_HOME="$cache_dir" opencenter cluster deploy "$ORG/$CLUSTER" \
  --break-lock \
  --yes \
  --log-level info
```

The deployment is resumable. If a step fails, correct the source configuration or infrastructure cause and resume from the failed step rather than manually patching live workloads.

The bootstrap log is written beneath:

```text
~/.local/state/opencenter/logs/bootstrap/<org>/<cluster>/
```

Save the log path as part of the release evidence.

The generated kubeconfig is written to:

```sh
export KUBECONFIG="$HOME/.config/opencenter/clusters/secrets/$ORG/$CLUSTER/kubernetes/kubeconfig.yaml"
```

Monitor bootstrap and, once the API is reachable, Flux reconciliation:

```sh
tail -f "$HOME/.local/state/opencenter/logs/bootstrap/$ORG/$CLUSTER/"*.log

kubectl get nodes
kubectl get kustomizations -A
kubectl get helmreleases -A
```

## DNS and portal access in SJC3

The current SJC3 OpenStack catalog does not expose a public Designate DNS endpoint, and the rc2 service catalog does
not deploy ExternalDNS. Keep both OpenStack `use_designate` settings `false`. After the Gateway obtains its external
IP, create records in the authoritative external DNS service for the generated portal hostnames.

For browser-only validation before authoritative DNS is available, add the Gateway IP and generated portal names to
the client machine's `/etc/hosts`. This affects only that machine; it does not provide DNS to pods. Services such as
Headlamp that contact the OIDC issuer from inside the cluster also require the issuer hostname to resolve through DNS
visible to CoreDNS. Do not enable Designate merely to bypass this requirement when the regional endpoint is absent.

## What success looks like

An OpenTofu apply completing successfully is only the infrastructure milestone. The cluster is ready when the following layers are healthy:

- All expected OpenStack instances are active.
- Kubernetes nodes are Ready.
- Every node has an OpenStack provider ID.
- The external CCM has removed the cloud-provider initialization taint.
- Calico is rolled out and its status is healthy.
- Flux GitRepositories and Kustomizations reconcile successfully.
- Selected HelmReleases are Ready.
- OpenStack CSI is functioning for enabled persistent volumes.

Useful checks include:

```sh
kubectl get nodes \
  -o custom-columns=NAME:.metadata.name,PROVIDER_ID:.spec.providerID,TAINTS:.spec.taints

kubectl get pods -A
kubectl get gitrepositories,kustomizations,helmreleases -A
kubectl get tigerastatus -A
```

## Capacity planning matters

OpenStack block storage attachment limits can become the hidden constraint. A worker with many Cinder-backed services may reach its attachment ceiling before CPU or memory is exhausted. Storage-heavy services such as Kafka and Loki can consume several volumes each.

Before enabling the full service matrix, calculate the expected volume count per worker. If the count is too high, add workers, move suitable services to filesystem or Longhorn storage, or reduce the initial service footprint.

Nova server-group quota is a separate prerequisite. This topology needs a master group, a worker group, and may create
a group for each configured additional worker pool even when its count is zero. Check capacity before deployment:

```sh
openstack --os-cloud "$OS_CLOUD" server group list
openstack --os-cloud "$OS_CLOUD" quota show
```

If quota is exhausted, inspect group membership and remove only empty groups belonging to clusters that have already
been destroyed. Never delete a group that contains instances or belongs to another active cluster. After freeing
quota, rerun the same `cluster deploy` command; the deployment resumes from its saved state.

## Release evidence

For each deployment, record:

- Cluster and organization name.
- CLI commit and GitOps base release.
- OpenStack region, image, flavors, counts, and volume type.
- Cluster validation output.
- Deployment log path.
- OpenTofu and Kubespray completion status.
- Node provider IDs and Kubernetes readiness.
- Flux, Calico, CSI, and HelmRelease status.
- Any approved deviations from the standard profile.

Do not record passwords, tokens, application-credential secrets, age private keys, or SSH private keys.

## Final checklist

```text
[ ] Regional image, flavors, network, keypair, and volume type verified
[ ] Nova server-group quota can accommodate all configured pools
[ ] Fresh cluster name and unique GitOps path selected
[ ] GitOps release and all custom module refs exist remotely
[ ] Flux, Weave GitOps, Calico, and CSI configured intentionally
[ ] Empty GitOps repository created and its URL added to the cluster configuration
[ ] Object storage provisioned for every enabled S3-backed service
[ ] Cluster validation passed
[ ] Cluster generated and secrets synchronized after final generation
[ ] Application overlays committed and pushed
[ ] Infrastructure and Kubernetes bootstrap completed
[ ] Generated kubeconfig exported
[ ] Nodes have provider IDs and are Ready
[ ] Flux, Calico, CSI, and HelmReleases are healthy
[ ] Portal hostnames resolve both from the test client and, where required, from cluster pods
[ ] Deployment evidence captured without secrets
```

With these checks in place, OpenStack deployment becomes a repeatable release process rather than a sequence of region-specific recovery steps.
