# Talos on OpenStack — Architecture Document


## Table of Contents

- [Scope](#scope)
- [System Context](#system-context)
- [Component Responsibilities](#component-responsibilities)
- [Network & Security Architecture](#network-security-architecture)
- [Control Flows](#control-flows)
- [Availability & Scalability Considerations](#availability-scalability-considerations)
- [Observability & Audit](#observability-audit)
- [Pulumi State Management](#pulumi-state-management)
- [Fallback Strategies](#fallback-strategies)
- [Dependencies & Assumptions](#dependencies-assumptions)
## Scope
This document captures the Pulumi-based system blueprint that implements the Talos provider for openCenter CLI. It translates the Zero Trust narrative from `readme.md` into concrete components, responsibilities, and integration points that AI agents must implement without relying on Terraform or Cluster API (CAPI/CAPO).

## System Context
```
+---------------------------+        +-------------------+
|   Developer / CI Runner   |        |   GitOps Repo     |
| - runs opencenter CLI     |<------>| - stores configs  |
+-------------+-------------+        +---------+---------+
              |                                 ^
              v                                 |
+-------------+---------------------------------+----------------+
|      openCenter Talos Provider CLI + Pulumi Run-Time           |
| - validate OpenStack prerequisites                              |
| - generate Talos + Pulumi blueprints                            |
| - execute Go Pulumi programs to manage OpenStack + Talos        |
+-------------+---------------------------------+----------------+
              |
              v
+-------------+--------------------------------------------------+
|   OpenStack APIs (Keystone, Barbican, Glance, Neutron, etc.)   |
| - Dedicated project per cluster (created via Pulumi)           |
| - Networks/LBs/instances/vTPM resources                        |
+-------------+--------------------------------------------------+
              |
              v
+-------------+---------------------------+
|   Talos Linux Cluster (control+workers) |
| - Talos API (mTLS via WireGuard)        |
| - etcd quorum + Kubernetes              |
| - Barbican KMS integration              |
+----------------------------------------+
```

## Component Responsibilities
1. **openCenter Talos Commands & Pulumi Runtime**
   - Collects OpenStack credentials and validates Keystone MFA, service catalog, quotas, and Barbican/Octavia availability.
   - Generates blueprints: Talos machine configs, Pulumi stack configuration (YAML) inspired by `third-party/openstack-flex-examples/pulumi/talos-cluster-ts` and `third-party/liberty-infrastructure`, WireGuard assets, and `.sops.yaml` entries.
   - Executes Pulumi programs written in Go to create/update/delete the dedicated OpenStack project, networks, security groups, bastion, load balancers, boot volumes with vTPM, and Talos nodes.
   - Surfaces plan/apply feedback (diffs, attestation fingerprints) and handles drift remediation via `pulumi preview`/`pulumi up`/`pulumi refresh` equivalents embedded in CLI commands.

2. **Pulumi Program Modules**
   - Encapsulate reusable stacks (project bootstrap, networking, compute, bastion, security artifacts) and expose strongly typed Go functions for CLI orchestration.
   - Enforce security defaults: network segmentation, default-deny security groups, Barbican key management, Glance signature enforcement, disk encryption with vTPM fallbacks.
   - Emit structured outputs consumed by GitOps repos (Kubernetes API endpoints, WireGuard public keys, attestation hashes).

3. **OpenStack Services**
   - **Keystone**: enforces MFA and least-privilege application credentials; Pulumi module provisions a dedicated project/application credential pair per cluster.
   - **Glance**: stores signed Talos images with signature metadata verifying via Nova.
   - **Barbican**: stores TLS certificates, disk encryption keys, Kubernetes KMS master keys, and WireGuard secrets.
   - **Octavia**: provides external API access and (optionally) internal load balancers when KubePrism is not sufficient.
   - **Neutron**: delivers the three-zone network fabric and routing policies.
   - **Cinder/Nova**: provide encrypted persistent volumes, vTPM-backed boot disks, and anti-affinity groups.

4. **Talos Cluster**
   - Hosts Kubernetes control plane and worker nodes with immutable OS, AppArmor, Seccomp, locked Talos API, and log forwarding sidecars.
   - Runs Barbican KMS plugin for Kubernetes secrets, WireGuard/KubeSpan overlays, kube-apiserver audit policy, and centralized logging exporters.

## Network & Security Architecture
- **Subnets**: `mgmt_subnet` (bastion/WireGuard), `control_subnet` (Talos control-plane), `data_subnet` (workers). Routes prevent internet ingress except through bastion/LB segments.
- **Security Groups**: default deny with explicit ingress for Kubernetes API 6443 (Octavia VIP only), Talos API 50000 (WireGuard peers), etcd 2379-2380 (control-plane SG), Kubelet 10250 (control↔worker), and overlay traffic UDP 51820-51821 / 8472.
- **WireGuard Bastion**: Pulumi provisions a hardened VM with MFA-enforced access and logging. All Talos/Kubernetes API calls traverse the VPN.
- **Cryptographic Chain**: Pulumi ensures Glance images carry signature metadata; Nova verifies using Barbican certificates before boot. Talos node configs embed TLS bundles generated via `talosctl` and stored encrypted with SOPS + Barbican references. vTPM-backed disks protect STATE/EPHEMERAL partitions; fallback keys stored in Barbican if vTPM absent.

## Control Flows
1. **Validation Flow**
   - CLI queries Keystone for required services, tests Barbican secret creation, checks Octavia health, and verifies quota availability.
   - Emits pass/fail report with remediation for missing capabilities (e.g., enabling image signature validation).

2. **Blueprint Generation Flow**
   - CLI collects topology, AZ mapping, capacity targets, and integrations (log sinks, audit endpoints).
   - Renders Talos machine configs, Pulumi stack YAML, and documentation referencing security controls. Assets include references to reusable Pulumi modules mirroring the examples in `third-party/liberty-infrastructure`.

3. **Pulumi Apply Flow**
   - CLI runs Pulumi preview to display diff (project creation, networks, load balancers, bastion, Talos nodes).
   - On approval, CLI executes Pulumi up using Go bindings, standing up OpenStack resources, uploading Barbican secrets, registering vTPM-enabled Nova servers, and generating WireGuard peers.
   - CLI connects through WireGuard to push Talos machine configs, bootstraps etcd, enables KubePrism, and prints attestation fingerprints for compliance.

4. **Operations Flow**
   - `opencenter talos status` runs Pulumi refresh to detect drift (e.g., modified security group rules) and provides remediation steps.
   - Rotation commands adjust desired node counts/image IDs in Pulumi stack YAML, trigger Pulumi up to recreate nodes (cordon/drain orchestrated through Talos API), and verify workload stability.
   - Audit/export integrations stream Talos + Kubernetes logs to configured collectors; health reported through CLI.

## Availability & Scalability Considerations
- Pulumi enforces spreading control-plane nodes across at least three Nova AZs with anti-affinity policies and health-monitored Octavia VIPs.
- Bastion/WireGuard VM defined as cattle (Pulumi-managed) with snapshot + IaC metadata for rapid recreation.
- Scaling events modify Pulumi stack variables; CLI exposes helpers to adjust worker pools, after which Pulumi up adds/removes Nova instances with Talos configs applied via automation.

## Observability & Audit
- Pulumi stack includes log collector destinations; CLI ensures Talos configs route system + audit logs externally.
- Drift detection uses Pulumi refresh plus targeted OpenStack API checks for image signatures, Barbican policies, and network ACLs.
- Outputs include attestation fingerprints, bastion public keys, WireGuard peer lists, and audit sink status for GitOps recording.

## Pulumi State Management
- State stored in an S3-compatible backend backed by OpenStack Swift (using EC2-compatible credentials).
- CLI bootstraps a dedicated Swift container per cluster/project, configures Pulumi to point at the `s3://<container>/<prefix>` endpoint, and injects credentials via environment variables or Pulumi config secrets.
- Versioning and server-side encryption enabled on the Swift container to protect history.
- Pulumi secrets safeguarded via the passphrase-based secrets provider; the passphrase itself is generated per cluster, stored with SOPS, and ultimately protected by Barbican so that state blobs remain encrypted even if Swift SSE is disabled.
- Per-environment stacks: each environment/cluster receives its own Pulumi stack name and Swift prefix so failures/drift are isolated; CLI scaffolding ensures the correct stack is selected before preview/apply.
- Default backend path recorded in blueprint so CI/CD runners and humans share a single state source; CLI validates reachability before preview/apply and ensures the secrets provider passphrase is available before running Pulumi.

## Fallback Strategies
- **Load Balancing (No Octavia)**: When Octavia is unavailable, Pulumi deploys a pair of HAProxy instances inside the management subnet, fronted by a Neutron floating IP. Health checks and firewall rules mimic Octavia defaults, and the CLI flags the environment as \"HAProxy fallback\" in outputs so operators know managed load balancing is absent.
- **Disk Encryption (No vTPM)**: If the selected flavors do not expose vTPM, the Pulumi program provisions Barbican-managed encryption keys and injects them into Talos machine configs so STATE/EPHEMERAL partitions rely on remote key wrapping. CLI validation highlights the reduced assurance level and recommends migrating to vTPM flavors when available.

## Dependencies & Assumptions
- OpenStack tenant exposes Nova, Neutron, Cinder, Glance, Barbican, Octavia, Keystone (with MFA + application credentials) APIs.
- Pulumi CLI/runtime available to the openCenter binary; Go Pulumi SDK leveraged for inline programs.
- Talos images with required drivers are available in Glance and carry signature metadata.
- WireGuard and `talosctl` binaries exist on operator workstation/CI runner.
