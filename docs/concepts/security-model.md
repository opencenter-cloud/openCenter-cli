---
id: security-model
title: "Security Model"
sidebar_label: Security Model
description: Defense-in-depth security architecture across all layers of the openCenter platform.
doc_type: explanation
audience: "security engineers, architects"
tags: [security, sops, encryption, rbac, kyverno]
---
# Security Model

**Purpose:** For security engineers, explains the security architecture and controls across all layers, covering threat model through compliance.

Understanding openCenter’s security model helps you assess risk, implement controls, and maintain compliance. This explanation covers the defense-in-depth approach across multiple layers.

## Security Architecture

openCenter implements **defense-in-depth** with security controls distributed across multiple layers:

```
Layer 5: Network Security (Optional)
    ├── Istio + mTLS (multi-tenant, zero-trust)
    └── Gateway API + NetworkPolicies (single-tenant)

Layer 4: Access Control
    ├── RBAC Manager (role-based access)
    └── Keycloak OIDC (identity management)

Layer 3: Secrets Management
    ├── SOPS Age encryption (Git)
    ├── FluxCD decryption (reconciliation)
    └── Kubernetes encryption at rest (etcd)

Layer 2: Platform Security
    ├── Kyverno policies (17 ClusterPolicies)
    ├── NetworkPolicies (platform services)
    └── Service hardening (security contexts)

Layer 1: Cluster Security
    ├── Pod Security Admission (baseline/restricted)
    ├── Admission controllers (PodSecurity, EventRateLimit)
    ├── Audit logging
    └── Encryption at rest
```

**Why this architecture:** Each layer provides independent security controls. Compromise of one layer doesn’t compromise others. Flexibility to choose appropriate controls for each environment.

**Evidence:** Session 1 A11, Ecosystem.md security architecture

## Threat Model

### Entry Points and Threats

**Entry Point 1: CLI User Input**

Threats:

* Command injection (malicious commands in configuration)
* Path traversal (access files outside workspace)
* XSS (malicious scripts in configuration)

Controls:

* InputValidator (sanitizes user input)
* CommandSanitizer (escapes shell commands)
* Path validation (restricts file access)

Evidence: `internal/security/input_validator.go`, `internal/security/command_sanitizer.go`

**Entry Point 2: Configuration Files**

Threats:

* Malicious YAML (code execution via templates)
* Secret exposure (plaintext secrets committed)
* Schema violations (invalid configuration)

Controls:

* Schema validation (JSON schema compliance)
* SOPS encryption (secrets encrypted before commit)
* Template sandboxing (limited template functions)

Evidence: `internal/config/validator.go`, `internal/sops/`

**Entry Point 3: Cloud Provider APIs**

Threats:

* Credential theft (stolen API keys)
* API abuse (excessive API calls)
* Quota exhaustion (resource exhaustion attacks)

Controls:

* Credential validation (verify before use)
* Rate limiting (limit API calls)
* Audit logging (track API usage)

Evidence: `internal/config/*_validator.go`, `internal/security/audit_logger.go`

**Entry Point 4: Git Repositories**

Threats:

* Malicious manifests (compromised GitOps repo)
* Secret exposure (plaintext secrets in Git)
* Supply chain attacks (compromised dependencies)

Controls:

* SSH key authentication (secure Git access)
* SOPS encryption (secrets encrypted in Git)
* Manifest validation (validate before apply)

Evidence: `internal/gitops/validators.go`, `.sops.yaml`

**Entry Point 5: Kubernetes API**

Threats:

* Unauthorized access (compromised credentials)
* Privilege escalation (container escape)
* Resource exhaustion (DoS attacks)

Controls:

* RBAC (role-based access control)
* Pod Security Admission (restrict privileged pods)
* Kyverno policies (enforce security policies)

Evidence: Ecosystem.md security layers

## Layer 1: Cluster Security

### Pod Security Admission

**Control:** Kubernetes-native pod security enforcement.

**Configuration:**

```yaml
# Kubespray inventory: k8s_hardening.yml
kube_apiserver_enable_admission_plugins:
  - PodSecurity
  - EventRateLimit
  - AlwaysPullImages

kube_pod_security_default_enforce: baseline
kube_pod_security_default_audit: restricted
kube_pod_security_default_warn: restricted
```

**Policies:**

* **Baseline (Enforced):** Prevents known privilege escalations
  * No privileged containers
  * No host namespaces (PID, IPC, network)
  * No host paths
  * No host ports
  * Limited capabilities
* **Restricted (Audit/Warn):** Hardened security
  * Run as non-root
  * Read-only root filesystem
  * Drop all capabilities
  * Seccomp profile required

**Exemptions:** `trivy-temp`, `tigera-operator`, `kube-system` (system components)

**Why this design:** Baseline prevents most attacks while allowing legitimate workloads. Restricted is too strict for some services (audit/warn mode provides visibility).

**Evidence:** Ecosystem.md Layer 1 security, Session 1 A11

### Admission Controllers

**PodSecurity:** Enforces Pod Security Standards

**EventRateLimit:** Prevents event flooding (DoS protection)

**AlwaysPullImages:** Forces image pull (prevents local image tampering)

**Why these controllers:** PodSecurity is the modern replacement for PodSecurityPolicy. EventRateLimit prevents API server overload. AlwaysPullImages ensures image integrity.

**Evidence:** Ecosystem.md Layer 1 security

### Audit Logging

**Control:** Comprehensive API server audit logging.

**Configuration:**

```yaml
# Kubespray: k8s_hardening.yml
kube_apiserver_enable_audit_log: true
kube_audit_log_path: /var/log/kubernetes/audit.log
kube_audit_log_maxage: 30
kube_audit_log_maxbackup: 10
kube_audit_log_maxsize: 100
```

**Logged Events:**

* Authentication attempts
* Authorization decisions
* Resource modifications
* Secret access

**Retention:** 30 days (configurable)

**Why this design:** Audit logs provide forensic evidence for security incidents. 30-day retention balances storage cost with compliance requirements.

**Evidence:** Ecosystem.md Layer 1 security

### Encryption at Rest

**Control:** Kubernetes encryption of secrets in etcd.

**Configuration:**

```yaml
# Kubespray: k8s_hardening.yml
kube_encrypt_secret_data: true
```

**Encrypted Resources:**

* Secrets
* ConfigMaps (optional)

**Why this design:** Protects secrets if etcd is compromised. Encryption key stored separately from etcd data.

**Evidence:** Ecosystem.md Layer 1 security

## Layer 2: Platform Security

### Kyverno Policies

**Control:** Policy engine for Kubernetes resources.

**Default Policies (17 ClusterPolicies):**

1. **disallow-privileged-containers:** Block privileged containers
2. **disallow-host-namespaces:** Block host PID/IPC/network
3. **disallow-host-path:** Block host path volumes
4. **require-run-as-nonroot:** Require non-root user
5. **restrict-seccomp:** Require seccomp profile
6. **restrict-volume-types:** Limit volume types
7. **disallow-capabilities:** Drop dangerous capabilities
8. **require-ro-rootfs:** Require read-only root filesystem
9. **disallow-host-ports:** Block host port binding
10. **require-probes:** Require liveness/readiness probes
11. **restrict-image-registries:** Limit allowed registries
12. **require-labels:** Require standard labels
13. **disallow-latest-tag:** Block :latest image tag
14. **require-requests-limits:** Require resource limits
15. **disallow-default-namespace:** Block default namespace
16. **require-network-policy:** Require NetworkPolicy
17. **disallow-deprecated-apis:** Block deprecated APIs

**Enforcement Modes:**

* **Enforce:** Block non-compliant resources
* **Audit:** Log violations, allow resources
* **Warn:** Warn users, allow resources

**Why this design:** Kyverno provides fine-grained policy control beyond Pod Security Admission. Policies are declarative (GitOps-friendly). Enforcement modes allow gradual rollout.

**Trade-offs:** More policies mean more complexity. But policies prevent common misconfigurations.

**Evidence:** Ecosystem.md Layer 2 security, Session 1 A11

### NetworkPolicies

**Control:** Network isolation for platform services.

**Default Policies:**

```yaml
# FluxCD namespace isolation
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: flux-system
  namespace: flux-system
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: flux-system
  egress:
  - to:
    - namespaceSelector: {}
```

**Why this design:** Default-deny network policies prevent lateral movement. Services explicitly allow required traffic.

**Trade-offs:** NetworkPolicies can break legitimate traffic if misconfigured. But they provide strong network isolation.

**Evidence:** Ecosystem.md Layer 2 security

### Service Hardening

**Control:** Security contexts for all platform services.

**Example:**

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
  seccompProfile:
    type: RuntimeDefault
```

**Why this design:** Defense-in-depth. Even if Pod Security Admission is bypassed, service-level security contexts provide protection.

**Evidence:** Ecosystem.md Layer 2 security

## Layer 3: Secrets Management

### SOPS Age Encryption

**Control:** Secrets encrypted with Age before commit to Git.

**Workflow:**

```
1. User: Create secret in plaintext
2. SOPS: Encrypt with Age public key
3. Git: Commit encrypted secret
4. FluxCD: Decrypt with Age private key
5. Kubernetes: Store in etcd (encrypted at rest)
```

**Key Management:**

* **Generation:** `opencenter secrets keys generate`
* **Storage:** `~/.config/opencenter/clusters/<org>/secrets/age/<cluster>-key.txt`
* **Rotation:** 90-day lifecycle (recommended)
* **Backup:** Secure, offline location

**Why this design:** Age is simpler than GPG (no key servers, no expiration by default). SOPS integrates with FluxCD. Secrets safe in Git.

**Trade-offs:** Age keys must be managed separately. Key rotation requires re-encrypting all secrets. But this is simpler than alternatives (Vault, Sealed Secrets).

**Evidence:** `internal/sops/manager.go`, Session 1 A11, Ecosystem.md

### Dual Encryption

**Control:** Secrets encrypted twice (Git + etcd).

**Layers:**

1. **In Git:** SOPS Age encryption (secrets safe to commit)
2. **In Cluster:** Kubernetes encryption at rest (etcd encrypted)

**Why this design:** Defense-in-depth. Compromise of Git doesn’t expose secrets (encrypted). Compromise of etcd doesn’t expose secrets (encrypted).

**Evidence:** Ecosystem.md dual encryption

### Key Lifecycle

**Generation:**

```bash
opencenter secrets keys generate
```

**Rotation (90-day lifecycle):**

```bash
opencenter secrets keys rotate --cluster my-cluster --type age
```

**Revocation:**

```bash
opencenter secrets keys revoke --cluster my-cluster --key abc123
```

**Why 90 days:** Balance between security (frequent rotation) and operational overhead (re-encryption).

**Evidence:** `internal/secrets/rotation.go`, Session 1 A11

## Layer 4: Access Control

### RBAC Manager

**Control:** Declarative RBAC management with Keycloak integration.

**Default Policies:**

```yaml
apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: cluster-admins
spec:
  rbacBindings:
  - name: cluster-admins
    subjects:
    - kind: Group
      name: cluster-admins
    roleBindings:
    - clusterRole: cluster-admin
```

**Why this design:** RBACDefinition CRDs are GitOps-friendly. Keycloak groups map to Kubernetes roles. Centralized access management.

**Evidence:** Ecosystem.md Layer 4 security

### Keycloak OIDC

**Control:** Identity and access management with OIDC.

**Integration:**

```yaml
# Kubernetes API server
kube_oidc_url: "https://auth.example.com/realms/opencenter"
kube_oidc_client_id: "kubernetes"
kube_oidc_username_claim: "sub"
kube_oidc_groups_claim: "groups"
```

**Features:**

* Single sign-on (SSO)
* Multi-factor authentication (MFA)
* Group-based access control
* Audit trail (authentication logs)

**Why this design:** Centralized identity management. No need to manage kubeconfig files. Group-based access simplifies RBAC.

**Evidence:** Ecosystem.md Layer 4 security

## Layer 5: Network Security

### Option A: Istio + mTLS

**Use Case:** Multi-tenant, regulated, zero-trust environments.

**Features:**

* Mutual TLS (mTLS) between services
* Traffic encryption
* Service-to-service authentication
* Fine-grained authorization

**Trade-offs:** More complexity, higher resource usage. But provides strong security for sensitive workloads.

**Evidence:** Ecosystem.md Layer 5 security

### Option B: Gateway API + NetworkPolicies

**Use Case:** Single-tenant, trusted network environments.

**Features:**

* Gateway API for ingress
* NetworkPolicies for isolation
* TLS termination at gateway

**Trade-offs:** Less security than Istio. But simpler and lower resource usage.

**Evidence:** Ecosystem.md Layer 5 security

## Supply Chain Security

### Dependency Management

**Control:** Pinned versions with integrity checks.

**Go Modules:**

```go
// go.mod
module github.com/opencenter-cloud/opencenter-cli

go 1.25.2

require (
    github.com/spf13/cobra v1.8.0
    gopkg.in/yaml.v3 v3.0.1
    // ... (pinned versions)
)
```

**Why this design:** `go.sum` provides integrity checks. Pinned versions ensure reproducible builds.

**Evidence:** `go.mod`, `go.sum`, Session 1 A11

### Image Security

**Control:** Harbor registry with scanning.

**Features:**

* Vulnerability scanning (Trivy)
* Image signing (Cosign)
* Admission control (block vulnerable images)

**Why this design:** Catch vulnerabilities before deployment. Signed images prevent tampering.

**Evidence:** Ecosystem.md infrastructure services

### SBOM Generation

**Control:** Software Bill of Materials for compliance.

**Status:** VERIFY: Check if SBOM generation is automated

**Why this design:** Compliance requirements (SBOM mandates). Vulnerability tracking (know what’s deployed).

**Evidence:** Session 1 A11

## Audit and Compliance

### CLI Audit Logging

**Control:** Comprehensive audit log with HMAC signatures.

**Logged Operations:**

* Cluster initialization
* Configuration changes
* Secret operations (generation, rotation, revocation, decryption)
* Deployment actions
* Drift detection
* Input validation failures

**Integrity:** Each log entry is signed with an HMAC-SHA256 key stored at `~/.config/opencenter/audit/audit.key`. The key is a 32-byte random value generated on first use. Signatures cover timestamp, event type, actor, resource, action, and result fields. See [Audit Signing Key](reference/audit-key.md) for the full reference.

**Retention:** 30 days, 100 MB max file size with automatic rotation.

**Why this design:** Forensic evidence for security incidents. HMAC signatures ensure log integrity -- any post-hoc modification of a log entry invalidates its signature.

**Evidence:** `internal/security/audit_logger.go`, Session 1 A11

### Kubernetes Audit Logging

**Control:** API server audit logs.

**Logged Events:**

* Authentication attempts
* Authorization decisions
* Resource modifications
* Secret access

**Retention:** 30 days (configurable)

**Why this design:** Compliance requirements (audit trail). Security incident investigation.

**Evidence:** Ecosystem.md Layer 1 security

### Compliance Frameworks

**Supported:**

* CIS Kubernetes Benchmark (via Kyverno policies)
* Pod Security Standards (baseline/restricted)
* NIST 800-190 (container security)

**Evidence:** Session 1 A11

## Security Best Practices

### 1. Rotate Keys Regularly

**Practice:** Rotate Age keys every 90 days, SSH keys every 180 days.

**Rationale:** Limit exposure window if keys are compromised.

**Evidence:** `internal/secrets/rotation.go`

### 2. Use Least Privilege

**Practice:** Grant minimum required permissions.

**Example:** Use viewer role for read-only access, not cluster-admin.

**Rationale:** Limit blast radius of compromised credentials.

### 3. Enable All Security Layers

**Practice:** Use all 5 security layers (cluster, platform, secrets, access, network).

**Rationale:** Defense-in-depth. Compromise of one layer doesn’t compromise others.

### 4. Monitor Security Events

**Practice:** Review audit logs regularly.

**Commands:**

```bash
# Review cluster status and drift
opencenter cluster status my-cluster
opencenter cluster drift detect my-cluster

# Kubernetes audit log
kubectl logs -n kube-system kube-apiserver-<node>
```

**Rationale:** Detect security incidents early.

### 5. Keep Services Updated

**Practice:** Update platform services regularly.

**Workflow:**

```
1. Test updates in dev
2. Update gitops-base tag
3. Deploy to prod
```

**Rationale:** Security patches for vulnerabilities.

## Common Misconceptions

### "SOPS encryption is optional"

**Reality:** SOPS encryption is required for secrets. No option to disable. This is intentional (security by default).

### "Pod Security Admission replaces Kyverno"

**Reality:** Pod Security Admission provides baseline security. Kyverno provides fine-grained policies. Both are needed for defense-in-depth.

### "Secrets are decrypted in Git"

**Reality:** Secrets stay encrypted in Git. FluxCD decrypts in-memory during reconciliation. Decrypted secrets never touch disk.

### "RBAC is enough for access control"

**Reality:** RBAC controls what users can do. OIDC controls who users are. Both are needed for complete access control.

### "Network policies are optional"

**Reality:** NetworkPolicies are optional but recommended. Without them, all pods can communicate (flat network).

## Threat Scenarios

### Scenario 1: Compromised Git Repository

**Attack:** Attacker gains write access to GitOps repository.

**Impact:** Attacker can modify manifests, deploy malicious workloads.

**Mitigations:**

* SSH key authentication (no passwords)
* Branch protection (require PR reviews)
* SOPS encryption (secrets not exposed)
* Kyverno policies (block malicious manifests)
* Audit logging (detect unauthorized changes)

### Scenario 2: Stolen Age Key

**Attack:** Attacker steals Age private key.

**Impact:** Attacker can decrypt secrets in Git.

**Mitigations:**

* Key rotation (limit exposure window)
* Key backup (secure, offline location)
* Access control (limit key access)
* Audit logging (detect key usage)

### Scenario 3: Container Escape

**Attack:** Attacker escapes container to host.

**Impact:** Attacker gains host access, can compromise other containers.

**Mitigations:**

* Pod Security Admission (prevent privileged containers)
* Kyverno policies (enforce security contexts)
* OS hardening (kernel security modules)
* Network policies (limit lateral movement)

### Scenario 4: Credential Theft

**Attack:** Attacker steals Kubernetes credentials.

**Impact:** Attacker gains cluster access.

**Mitigations:**

* RBAC (limit permissions)
* OIDC (centralized authentication)
* Audit logging (detect unauthorized access)
* Key rotation (limit exposure window)

## Further Reading

* [Architecture](concepts/architecture.md) - System design and components
* [GitOps Workflow](concepts/gitops-workflow.md) - Repository structure and reconciliation
* [Manage Secrets](operations/manage-secrets.md) - SOPS and secrets management
* [Customize Services](operations/customize-services.md) - Security service configuration

---

## Evidence

This explanation is based on:

* Security architecture: Session 1 A11, Ecosystem.md security
* Threat model: Session 1 A11
* Layer 1 security: Ecosystem.md, Kubespray hardening
* Layer 2 security: Ecosystem.md, Kyverno policies
* Layer 3 security: `internal/sops/manager.go`, Ecosystem.md
* Layer 4 security: Ecosystem.md RBAC
* Layer 5 security: Ecosystem.md network security
* Audit logging: `internal/security/audit_logger.go`