# Talos on OpenStack — Product Requirements Document


## Table of Contents

- [Background](#background)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Success Metrics](#success-metrics)
- [Personas & Key Jobs](#personas-key-jobs)
- [User Stories & Acceptance Criteria](#user-stories-acceptance-criteria)
- [Functional Requirements](#functional-requirements)
- [Non-Functional Requirements](#non-functional-requirements)
- [Dependencies](#dependencies)
- [Constraints & Assumptions](#constraints-assumptions)
- [Open Questions](#open-questions)
- [References](#references)
## Background
Organizations operating in multi-tenant OpenStack clouds need an automated, auditable way to deploy Talos-backed Kubernetes clusters. The previous `readme.md` captured a narrative of Zero Trust controls (immutability, attestation, Barbican-backed secrets, WireGuard access). This PRD distills that narrative into implementable requirements for the openCenter CLI Talos provider.

## Goals
1. Enable secure, immutable Kubernetes clusters using Talos Linux on OpenStack with minimal manual steps.
2. Enforce Zero Trust networking, cryptographic attestation, and defense-in-depth policies by default.
3. Provide a declarative, GitOps-friendly lifecycle powered by Pulumi (Go bindings) that integrates cleanly with GitOps workflows.
4. Surface verifiable evidence (attestation fingerprints, validation reports, audit logs) for compliance teams.

## Non-Goals
- Supporting traditional Linux distributions or SSH-based workflows.
- Acting as a general OpenStack orchestrator unrelated to Talos clusters.
- Delivering GUI/portal experiences; CLI + GitOps artifacts are the contract.
- Shipping CAPI/CAPO integrations in this iteration.

## Success Metrics
- 100% of generated Talos nodes boot from cryptographically verified Glance images.
- 0 instances require SSH or console access during lifecycle operations.
- Secrets at rest (Talos STATE, Cinder volumes, Kubernetes secrets) always show encrypted status during validation.
- Drift detection reconciles security group rules and load balancer policies within five minutes.

## Personas & Key Jobs
- **Platform Engineer** — needs to bootstrap production-ready Talos clusters quickly.
- **Security Engineer** — must guarantee Zero Trust controls and auditability.
- **Network Architect** — designs multi-zone Neutron topology and wants deterministic segmentation.
- **Compliance Officer** — requires traceable attestations and logs for reviews.

## User Stories & Acceptance Criteria
### US-1: Cluster Initialization
As a platform engineer, I want to initialize a Talos cluster on OpenStack with secure defaults so that I can deploy production-grade Kubernetes without manual hardening.
- CLI generates Talos machine configs with AppArmor, Seccomp, hardened sysctls, KubePrism, and disk encryption enabled.
- Pulumi stack configuration captures networks, routers, security groups, load balancers, and credential policies with least-privilege defaults.
- Glance image verification and Barbican-backed attestation keys enforced.

### US-2: Secure Bootstrapping
As a cluster administrator, I want to bootstrap Talos nodes without SSH so that I eliminate traditional remote access attack vectors.
- WireGuard bastion (UDP 51820) restricts Talos API (50000) and kube-apiserver access to trusted peers only.
- `talosctl apply-config` and `talosctl bootstrap` operations succeed exclusively via the VPN tunnel.
- CLI refuses to proceed if Talos API endpoints are exposed on public networks.

### US-3: Secrets Management
As a security engineer, I want Kubernetes secrets encrypted at rest using OpenStack Barbican so that sensitive data is protected by HSM-backed keys.
- Barbican KMS plugin deployed as static pod; kube-apiserver configured for envelope encryption.
- Talos STATE + EPHEMERAL partitions encrypted (LUKS) with keys sealed to vTPM or Barbican if vTPM unavailable.
- CLI validates encryption health and emits remediation if keys are out-of-sync.

### US-4: Network Segmentation
As a network architect, I want strict micro-segmentation between cluster zones so that lateral movement is prevented.
- Management, control, and data plane networks created with explicit routes and ACLs.
- Security groups follow default deny, allowing only enumerated ports (6443, 50000, etcd, overlay traffic).
- KubePrism handles internal control-plane load balancing to minimize Octavia exposure.

### US-5: Image Integrity
As a compliance officer, I want cryptographic verification of OS images so that supply-chain attacks are prevented.
- Talos images uploaded to Glance with signature metadata; Nova enforces verification before booting nodes.
- CLI surfaces signature fingerprints and fails fast when verification is disabled.

### US-6: Immutable Lifecycle
As a DevOps engineer, I want node updates via replacement rather than patching so that configuration drift is eliminated.
- Pulumi-managed node groups (control-plane + worker definitions) recreate instances when image/version changes occur; CLI displays planned replacements.
- Old nodes drained/cordoned automatically; logs shipped before termination.

### US-7: Observability & Audit
As a security analyst, I want comprehensive audit logs shipped off-node so that I can investigate incidents.
- Talos system logs and kube-apiserver audit logs stream to configured collector (Loki, Splunk, etc.).
    - CLI exposes `status` or `audit report` command summarizing log delivery health.

## Functional Requirements
| ID | Requirement |
| --- | --- |
| FR-1 | Provide `validate`, `init`, `apply`, `status`, `rotate`, and `destroy` commands for the Talos provider. |
| FR-2 | Generate GitOps-ready directory structure with Talos machine configs, Pulumi stack configuration, and `.sops.yaml` entries. |
| FR-3 | Execute Pulumi (Go) programs that provision the WireGuard bastion VM, Octavia load balancers, Neutron networks, routers, security groups, encrypted volumes, and vTPM-enabled Nova instances per blueprint. |
| FR-4 | Enforce Glance image signature checks and Barbican certificate usage; abort when verification disabled. |
| FR-5 | Configure Talos machine configs for disk encryption, KubePrism, system extensions, and log shipping. |
| FR-6 | Deploy Barbican KMS plugin manifests and configure kube-apiserver for envelope encryption. |
| FR-7 | Provide Pulumi preview/plan output describing changes (node replacements, network updates) before apply and integrate that information into CLI UX. |
| FR-8 | Run drift detection via Pulumi refresh plus targeted OpenStack checks comparing expected vs actual resources/security policies. |
| FR-9 | Manage Pulumi state in an S3-compatible backend (OpenStack Swift with EC2 credentials), including backend bootstrap, credential distribution, secrets-provider passphrase handling (stored with SOPS/Barbican), and health verification before operations. |
| FR-10 | Automatically select the appropriate load-balancer/vTPM strategy (Octavia vs HAProxy floating IP; hardware vTPM vs Barbican-managed software encryption) and surface the active mode in CLI outputs. |
| FR-11 | Emit machine-readable reports (JSON/YAML) plus human summaries for CI integration. |

## Non-Functional Requirements
- **Security**: Secrets never written unencrypted; CLI enforces SOPS usage and cleans temporary files.
- **Reliability**: Commands are idempotent; retries handle OpenStack eventual consistency.
- **Performance**: Validation and plan generation complete within five minutes for clusters up to 50 nodes.
- **Usability**: CLI outputs include remediation hints, architecture diagrams, and references to documentation sections.
- **Compliance**: Align with Zero Trust principles described in design and architecture docs.

## Dependencies
- OpenStack services: Nova, Neutron, Glance, Cinder, Octavia, Barbican, Keystone (with MFA + application credentials).
- Talos Linux images with proper signatures and drivers.
- WireGuard + `talosctl` binaries on operator machines/CI runners.
- SOPS + `.sops.yaml` policy preconfigured in repo.
- Pulumi CLI/runtime and Go SDK dependencies available where the CLI executes.
- Barbican-accessible storage for Pulumi secrets-provider passphrases generated per cluster.

## Constraints & Assumptions
- Operators can create a dedicated OpenStack project for each cluster.
- At least one Octavia flavor available; fallback designs documented separately if absent.
- Access to Barbican is mandatory; if unavailable, CLI halts with remediation instructions.
- GitOps pipelines (Flux/Argo) consume generated manifests but are not provisioned automatically in v1.

## Open Questions
- Which log collectors should be supported out of the box (Loki vs Splunk vs HTTP generic)?
- How should we expose metrics for drift detection—CLI command only or push to webhook?
- Are there regulatory requirements for specific cipher suites that exceed Talos defaults?

## References
- `design.md` for experience intent and assumptions.
- `architecture.md` for system components and integration flows.
