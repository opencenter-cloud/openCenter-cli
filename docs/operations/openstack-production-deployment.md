---
id: openstack-production-deployment
title: "Production OpenStack deployment runbook"
sidebar_label: "Production OpenStack deployment"
description: "Repeatable production workflow, capacity guidelines, volume attachment constraints, and release checklist for OpenStack Kubernetes clusters."
doc_type: how-to
audience: "openCenter operators, platform engineers"
tags: [openstack, deployment, operations, production, kubespray]
last_updated: 2026-11-02
---

# Production OpenStack Deployment Runbook

**Purpose:** For platform engineers and operators, provides the production deployment runbook, capacity guidelines, volume attachment constraints, and formal release checklist for OpenStack Kubernetes clusters with Kubespray, Flux, and Calico.

---

## Architectural Layers

Deploying Kubernetes in production on OpenStack involves aligning the cloud inventory, infrastructure-as-code, Kubespray bootstrap, cloud controller integration, storage, networking, and GitOps reconciliation:

1. **Manifest Rendering**: openCenter renders the cluster configuration and infrastructure manifests.
2. **Infrastructure Provisioning**: OpenTofu provisions the tenant network, compute instances, security groups, Cinder volumes, and related resources.
3. **Kubernetes Provisioning**: Kubespray installs Kubernetes and container runtimes across the provisioned instances.
4. **GitOps Engine**: Flux reconciles cluster configurations, controllers, and platform services from Git.
5. **Cloud Integration**: The OpenStack Cloud Controller Manager (CCM) assigns provider IDs and manages cloud-facing Kubernetes resources.
6. **Networking & Storage**: Calico supplies the network policy layer, while OpenStack CSI provides block storage.

---

## Prerequisites

Before beginning a production deployment:

- An openCenter CLI built from the target release branch.
- An approved OpenStack cloud profile for the target region in `~/.config/openstack/clouds.yaml`.
- Tooling installed: `yq`, OpenTofu or Terraform, Helm, `kubectl`, SOPS, `age`, and Flux CLI.
- Access to create a private GitOps repository and token credentials.
- An SSH keypair registered in OpenStack Nova.
- A fresh cluster name and unique GitOps repository path.

> [!CAUTION]
> Keep OpenStack credentials, application credentials, Age private keys, SSH private keys, and tokens in local secret stores or cloud configurations. They must never be placed in blueprints or unencrypted in the Git repository.

---

## Regional Inventory Validation

Do not assume that an image, flavor, volume type, or network from one OpenStack region is available in another. Always confirm the inventory before creating a blueprint.

### Sample Regional Baseline (e.g. SJC3)

| Resource | Baseline Value | Description |
|---|---|---|
| **Region** | `SJC3` | Target OpenStack region |
| **Image** | Ubuntu 24.04 | Resolved from live regional Glance inventory |
| **Master Flavor** | `gp.0.4.8` | 4 vCPU / 8 GB RAM |
| **Worker Flavor** | `gp.0.8.32` | 8 vCPU / 32 GB RAM |
| **Bastion Flavor** | `gp.0.2.4` | 2 vCPU / 4 GB RAM |
| **Node Topology** | 3 masters / 5 workers | High-availability control plane and scaled worker pool |
| **Volume Type** | `local` | Regional Cinder block storage type |
| **External Network** | `PUBLICNET` | Routing and floating IP connectivity |
| **Availability Zone** | `az1` | Compute placement zone |
| **Pod CIDR** | `10.42.0.0/16` | Kubernetes pod network |
| **Service CIDR** | `10.43.0.0/16` | Kubernetes service network |
| **Server Group Policy** | Soft anti-affinity | Spread instances across physical hypervisors |

Verify inventory prior to initialization:

```bash
export OS_CLOUD=<approved-cloud-profile>

openstack image list
openstack flavor show gp.0.4.8
openstack flavor show gp.0.8.32
openstack network show PUBLICNET
openstack keypair list
```

---

## Capacity Planning & Constraints

### 1. Cinder Block Storage Attachment Limits
OpenStack block storage attachment limits are often a hidden operational constraint. A worker hosting multiple Cinder-backed stateful services (such as Kafka, Loki, Harbor, or Prometheus) may reach its maximum hypervisor volume attachment ceiling before CPU or RAM is exhausted.

Before enabling a full service suite:
- Calculate the total expected volume count per worker node.
- If volume density is high, scale out additional worker nodes or use shared filesystem storage (e.g., Longhorn or NFS) where appropriate.

### 2. Nova Server-Group Quotas
Production topologies use server groups for soft anti-affinity across masters and worker pools. Check Nova quotas before starting:

```bash
openstack --os-cloud "$OS_CLOUD" server group list
openstack --os-cloud "$OS_CLOUD" quota show
```

If server-group quota is exhausted, inspect group membership and delete only empty, orphaned groups from previously decommissioned clusters:

```bash
openstack --os-cloud "$OS_CLOUD" server group delete <orphaned-group-id>
```

---

## Production Deployment Workflow

### 1. Initialize Cluster Blueprint

Set deployment environment variables:

```bash
export ORG=<organization>
export CLUSTER=<fresh-cluster-name>
export GH_ORG=<github-user-or-org>
export OS_CLOUD=<approved-cloud-profile>
export BASE_DOMAIN=<base-domain>
export S3_ENDPOINT=<s3-swift-endpoint>
```

Initialize the blueprint:

```bash
opencenter cluster init "$CLUSTER" \
  --org "$ORG" \
  --type openstack \
  --os-cloud "$OS_CLOUD" \
  --base-domain "$BASE_DOMAIN" \
  --yes
```

### 2. Configure GitOps Release Locking

Set `opencenter.gitops.base_repo.release` to the intended release tag (e.g. `2026.03-rc05`). Any explicit OpenTofu module source override must reference a tag, commit, or branch that exists in the remote repository. Verify custom refs with `git ls-remote` before proceeding:

```bash
git ls-remote https://github.com/<org>/opentofu-modules.git
```

Set the GitOps repository URL and auth token:

```bash
opencenter cluster set "$ORG/$CLUSTER" \
  opencenter.gitops.repository.url="https://github.com/$GH_ORG/$CLUSTER-gitops.git" \
  opencenter.gitops.auth.token.provider=github \
  "opencenter.gitops.auth.token.token=$GH_BOOTSTRAP_TOKEN"
```

### 3. Provision Stateful Service Storage

`storage apply` resolves each service's typed configuration and exits with `service "<name>" is not configured` for a service the cluster does not enable, so provision only the services that are enabled. Confirm the enabled set, then run the loop over that subset:

```bash
opencenter cluster service status --cluster "$ORG/$CLUSTER"

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

The accepted backends differ per service: `loki` takes `swift`, `s3`, or `none`; `tempo` takes `s3` only; `mimir` takes `swift` or `s3`; `harbor` takes `s3` or `filesystem`; `etcd-backup` and `velero` take `s3` or `none`. A service on `none` or `filesystem` provisions no bucket and no credentials, so leave it out of the loop.

### 4. Enable cert-manager

Configure automated DNS-01 ACME certificate issuance:

```bash
opencenter cluster service enable cert-manager \
  --param="email=$ADMIN_EMAIL" \
  --param="dns_provider=cloudflare" \
  --secret="cloudflare_api_token=$CLOUDFLARE_API_TOKEN" \
  --default \
  --force
```

`--default` pins the credential these secrets populate as the cluster default issuer, which is what the gateway `cert-manager.io/cluster-issuer` annotation resolves to.

### 5. Validate, Generate, and Encrypt Secrets

```bash
# Validate configuration against schema and rules
opencenter cluster validate "$ORG/$CLUSTER"

# Render manifests and synchronize SOPS-encrypted secrets
opencenter cluster generate "$ORG/$CLUSTER" --force
opencenter secrets sync "$ORG/$CLUSTER"
```

Verify that all secrets are encrypted (`ENC[...]`):

```bash
grep -rn "ENC\[" ~/.config/opencenter/clusters/gitops/$ORG/applications/overlays/$CLUSTER/
```

### 6. Publish GitOps Manifests

`cluster init` has already run `git init` in `~/.config/opencenter/clusters/gitops/$ORG`
and set `core.hooksPath`. Create the empty remote repository, commit, and push:

```bash
gh repo create "$GH_ORG/$CLUSTER-gitops" --private --disable-wiki

cd ~/.config/opencenter/clusters/gitops/$ORG
git branch -M main
git add -A
git commit -m "Deploy production cluster $ORG/$CLUSTER"
git push -u origin main
```

`cluster deploy` re-points `origin` to `opencenter.gitops.repository.url` (set in step 2)
and aborts when the two differ, so no manual `git remote add` is needed.

### 7. Execute Deployment

```bash
opencenter cluster deploy "$ORG/$CLUSTER" \
  --break-lock \
  --yes \
  --log-level info
```

Monitor bootstrap progress:

```bash
tail -f ~/.local/state/opencenter/logs/bootstrap/$ORG/$CLUSTER/*.log
```

---

## Post-Deployment Verification & Release Evidence

An OpenTofu apply completing successfully is only the infrastructure milestone. The cluster is ready for production traffic when all of the following criteria are satisfied:

1. **Compute Instances**: All expected OpenStack VMs are active in Nova.
2. **Kubernetes Nodes**: All nodes report `Ready` status.
3. **Cloud Provider Integration**: Every node possesses an OpenStack provider ID, and the CCM has removed the uninitialized taint.
4. **Networking**: Calico pods and nodes are healthy without IP pool exhaustion.
5. **GitOps Synchronization**: Flux `GitRepository` and `Kustomization` resources reconcile cleanly without errors.
6. **Platform Services**: All platform `HelmReleases` report `Ready`.
7. **Storage**: OpenStack CSI driver successfully provisions and mounts Cinder volumes.

Verification commands:

```bash
export KUBECONFIG="$HOME/.config/opencenter/clusters/secrets/$ORG/$CLUSTER/kubernetes/kubeconfig.yaml"

kubectl get nodes -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[-1].type,PROVIDER_ID:.spec.providerID,TAINTS:.spec.taints
kubectl get pods -A
kubectl get gitrepositories,kustomizations,helmreleases -A
kubectl get certificates,clusterissuers -A
```

---

## Release Evidence Collection

For each production deployment, archive the following release evidence:

- Organization and cluster identifier.
- CLI git commit and GitOps base release tag.
- OpenStack region, image ID, compute flavors, node counts, and volume types.
- Validation output log.
- Deployment bootstrap log path (`~/.local/state/opencenter/logs/bootstrap/<org>/<cluster>/`).
- OpenTofu and Kubespray exit status.
- Kubernetes node list with provider IDs.
- Flux Kustomization and HelmRelease status table.

> [!CAUTION]
> Never record passwords, application credential secrets, tokens, Age private keys, or SSH private keys in release evidence records.

---

## Production Release Checklist

```text
[ ] Regional image, flavors, network, keypair, and volume types verified
[ ] Nova server-group quota verified for all planned pools
[ ] Worker volume attachment density planned against hypervisor limits
[ ] Unique cluster name, FQDN, and empty GitOps repository prepared
[ ] GitOps release tag and custom module refs verified remotely (git ls-remote)
[ ] Storage provisioned for every enabled S3-backed service
[ ] Configuration offline validation passed with zero errors
[ ] Manifests generated and secrets verified SOPS-encrypted (ENC[...])
[ ] GitOps manifests committed and pushed to remote repository
[ ] Deployment executed and bootstrap logs monitored
[ ] Generated kubeconfig exported
[ ] Nodes verified Ready with OpenStack provider IDs and no uninitialized taints
[ ] Calico CNI, Flux GitOps, and cert-manager verified operational
[ ] Portal hostnames resolve correctly via external DNS
[ ] Release evidence archived without plain text secrets
```

---

## Related Documentation

- [Getting Started: Deploy an OpenStack Cluster](../getting-started/openstack-first-cluster.md)
- [Release Process Guidelines](../contributing/release-process.md)
- [Troubleshooting Deployment](../operations/troubleshoot-deployment.md)
