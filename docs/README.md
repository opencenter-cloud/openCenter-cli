# Documentation

This folder is the source for the openCenter CLI documentation. Pages are
written in Markdown with YAML frontmatter and follow the
[Diátaxis](https://diataxis.fr/) framework: every page is one of
`tutorial`, `how-to`, `reference`, or `explanation`.

## Layout

Pages are organised by **lifecycle category**, not by Diátaxis type:

| Directory | Default `doc_type` | Purpose |
|---|---|---|
| `getting-started/` | `tutorial` | First-cluster walkthroughs |
| `operations/` | `how-to` | Day-2 task guides |
| `reference/` | `reference` | CLI, schema, services, flags |
| `concepts/` | `explanation` | Architecture and rationale |
| `providers/` | `reference` | Per-provider guides |
| `contributing/` | `explanation` | Developer docs |
| `release/` | `reference` | Release notes |

## Complete Site Map

### Getting Started (Tutorials)

| Page | Description |
|------|-------------|
| [getting-started](getting-started/getting-started.md) | End-to-end first cluster walkthrough |
| [kind-local-development](getting-started/kind-local-development.md) | Local cluster with Kind |
| [openstack-first-cluster](getting-started/openstack-first-cluster.md) | First cluster on OpenStack |
| [vmware-deployment](getting-started/vmware-deployment.md) | Deploy on VMware |
| [multi-cluster-setup](getting-started/multi-cluster-setup.md) | Multiple clusters in one org |

### Operations (How-To Guides)

| Page | Description |
|------|-------------|
| [validate-configuration](operations/validate-configuration.md) | Validate cluster config |
| [manage-secrets](operations/manage-secrets.md) | SOPS encryption lifecycle |
| [customize-services](operations/customize-services.md) | Enable/disable/configure services |
| [configure-networking](operations/configure-networking.md) | Network and DNS setup |
| [add-worker-pools](operations/add-worker-pools.md) | Add worker node groups |
| [backup-and-restore](operations/backup-and-restore.md) | Velero backup/restore |
| [upgrade-kubernetes](operations/upgrade-kubernetes.md) | Kubernetes version upgrades |
| [migrate-clusters](operations/migrate-clusters.md) | Cluster migration |
| [troubleshoot-deployment](operations/troubleshoot-deployment.md) | Deployment debugging |
| [integrate-ci-cd](operations/integrate-ci-cd.md) | CI/CD pipeline integration |
| [create-install-cli-plugin](operations/create-install-cli-plugin.md) | CLI plugin authoring |
| [flux-bootstrap-methods](operations/flux-bootstrap-methods.md) | Flux bootstrap options |
| [create-kind-cluster](operations/create-kind-cluster.md) | Kind cluster creation |
| [create-openstack-cluster](operations/create-openstack-cluster.md) | OpenStack cluster creation |
| [deploy-openstack-cluster](operations/deploy-openstack-cluster.md) | OpenStack cluster deployment |

### Reference

#### Configuration & CLI

| Page | Description |
|------|-------------|
| [cli-commands](reference/cli-commands.md) | Full CLI command tree |
| [configuration-schema](reference/configuration-schema.md) | Cluster config YAML schema |
| [configuration-precedence](reference/configuration-precedence.md) | Config override order |
| [default-values](reference/default-values.md) | Default config values |
| [environment-variables](reference/environment-variables.md) | Env var reference |
| [exit-codes](reference/exit-codes.md) | CLI exit codes |
| [file-locations](reference/file-locations.md) | Config/key file paths |
| [validation-rules](reference/validation-rules.md) | Validation rule catalog |
| [mise-tasks](reference/mise-tasks.md) | Development task runner |
| [audit-key](reference/audit-key.md) | Audit signing key |
| [providers](reference/providers.md) | Infrastructure providers |
| [opencenter/](reference/opencenter/) | Auto-generated per-command reference |

#### Platform Services (per-service docs)

| Page | Category | Description |
|------|----------|-------------|
| [services/index](reference/services/index.md) | — | Service matrix and overview |
| [services/calico](reference/services/calico.md) | Networking | Calico CNI |
| [services/gateway-api](reference/services/gateway-api.md) | Networking | Gateway API CRDs |
| [services/gateway](reference/services/gateway.md) | Networking | Envoy gateway |
| [services/metallb](reference/services/metallb.md) | Networking | Bare-metal LB |
| [services/cert-manager](reference/services/cert-manager.md) | Security | TLS certificates |
| [services/keycloak](reference/services/keycloak.md) | Security | Identity/OIDC |
| [services/kyverno](reference/services/kyverno.md) | Security | Policy engine |
| [services/rbac-manager](reference/services/rbac-manager.md) | Security | RBAC management |
| [services/sealed-secrets](reference/services/sealed-secrets.md) | Security | Encrypted secrets |
| [services/openstack-ccm](reference/services/openstack-ccm.md) | Cloud | OpenStack CCM |
| [services/openstack-csi](reference/services/openstack-csi.md) | Storage | Cinder CSI |
| [services/vsphere-csi](reference/services/vsphere-csi.md) | Storage | vSphere CSI |
| [services/longhorn](reference/services/longhorn.md) | Storage | Distributed storage |
| [services/external-snapshotter](reference/services/external-snapshotter.md) | Storage | Volume snapshots |
| [services/kube-prometheus-stack](reference/services/kube-prometheus-stack.md) | Observability | Prometheus + Grafana |
| [services/loki](reference/services/loki.md) | Observability | Log aggregation |
| [services/tempo](reference/services/tempo.md) | Observability | Distributed tracing |
| [services/mimir](reference/services/mimir.md) | Observability | Long-term metrics |
| [services/opentelemetry-kube-stack](reference/services/opentelemetry-kube-stack.md) | Observability | OTel collectors |
| [services/alert-proxy](reference/services/alert-proxy.md) | Observability | Alert forwarding |
| [services/fluxcd](reference/services/fluxcd.md) | GitOps | Continuous delivery |
| [services/weave-gitops](reference/services/weave-gitops.md) | GitOps | GitOps dashboard |
| [services/velero](reference/services/velero.md) | Backup | Disaster recovery |
| [services/etcd-backup](reference/services/etcd-backup.md) | Backup | etcd snapshots |
| [services/headlamp](reference/services/headlamp.md) | Management | K8s dashboard |
| [services/olm](reference/services/olm.md) | Management | Operator Lifecycle |
| [services/postgres-operator](reference/services/postgres-operator.md) | Management | PostgreSQL operator |
| [services/harbor](reference/services/harbor.md) | Management | Container registry |
| [services/kafka-cluster](reference/services/kafka-cluster.md) | Management | Apache Kafka |

### Concepts (Explanations)

| Page | Description |
|------|-------------|
| [architecture](concepts/architecture.md) | System architecture overview |
| [reference-architecture](concepts/reference-architecture.md) | Target cluster architecture |
| [gitops-workflow](concepts/gitops-workflow.md) | GitOps model and FluxCD |
| [configuration-lifecycle](concepts/configuration-lifecycle.md) | Config from init to deploy |
| [security-model](concepts/security-model.md) | Security design and SOPS |
| [services-templates](concepts/services-templates.md) | Template rendering system |
| [drift-detection](concepts/drift-detection.md) | Infrastructure drift |
| [plugin-internal-services](concepts/plugin-internal-services.md) | Internal plugin system |
| [plugin-external-cli](concepts/plugin-external-cli.md) | External CLI plugins |
| [provider-comparison](concepts/provider-comparison.md) | Provider feature matrix |

### Providers

| Page | Description |
|------|-------------|
| [README](providers/README.md) | Provider overview |
| [vmware](providers/vmware.md) | VMware provider guide |
| [vmware-quick-start](providers/vmware-quick-start.md) | VMware quick start |
| [vmware-terraform-template](providers/vmware-terraform-template.md) | VMware Terraform |

### Contributing

| Page | Description |
|------|-------------|
| [contributing](contributing/contributing.md) | Contribution guide |
| [development-setup](contributing/development-setup.md) | Dev environment setup |
| [code-structure](contributing/code-structure.md) | Package layout |
| [testing-guide](contributing/testing-guide.md) | Testing approach |
| [adding-providers](contributing/adding-providers.md) | New provider guide |
| [adding-services](contributing/adding-services.md) | New service guide |
| [build-system](contributing/build-system.md) | Build and release |
| [release-process](contributing/release-process.md) | Release workflow |
| [validation](contributing/validation.md) | Validation system |
| [services](contributing/services.md) | Service internals |
| [rendering-contract](contributing/rendering-contract.md) | Template rendering rules |
| [descriptor-condition-schema](contributing/descriptor-condition-schema.md) | Descriptor conditions |
| [services-rendering-options](contributing/services-rendering-options.md) | Rendering variants |
| [services-rendering-parity-plan](contributing/services-rendering-parity-plan.md) | Parity tracker |
| [per-service-file-variance](contributing/per-service-file-variance.md) | File variance analysis |
| [overlay-security-policy](contributing/overlay-security-policy.md) | Overlay security |
| [cluster-init-details](contributing/cluster-init-details.md) | Init internals |
| [cluster-deploy-openstack](contributing/cluster-deploy-openstack.md) | Deploy internals |
| [kind-cluster-verification](contributing/kind-cluster-verification.md) | Kind verification |
| [repo-cleanup-audit](contributing/repo-cleanup-audit.md) | Cleanup tracker |

### Release Notes

| Page | Description |
|------|-------------|
| [1.0.0-rc01](release/1.0.0-rc01.md) | Release candidate 1 |

### Other

| Page | Description |
|------|-------------|
| [index](index.md) | Documentation home |
| [glossary](glossary.md) | Term definitions |

## Non-Published Content

- [`CODEMAPS/`](CODEMAPS/) — Architecture maps for the development workflow.
  Not part of the published site. See [CODEMAPS/INDEX.md](CODEMAPS/INDEX.md).

## Editing Rules

- Every page must start with YAML frontmatter: `id`, `title`,
  `sidebar_label`, `description`, `doc_type`, `audience`, `tags`.
- Pick exactly one `doc_type` per file. Split mixed content and cross-link.
- Start the body with a `**Purpose:**` line naming the audience and scope.
- Place pages in the lifecycle directory matching the reader's task.
- Refresh the per-command reference under `reference/opencenter/`
  with `go run -tags tools ./cmd/docs` when the Cobra tree changes.

## Tooling

- `hack/scripts/audit_doc_frontmatter.py` — verify frontmatter rules (CI-safe).
- `hack/scripts/convert_adoc_to_md.py` — convert legacy `.adoc` to Markdown.
