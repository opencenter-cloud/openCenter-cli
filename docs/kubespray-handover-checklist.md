# Kubespray Cluster Handover Checklist

> **Enterprise-Grade Production Readiness Validation**  
> Version: 1.0 | Last Updated: 2025-11-19

## Executive Summary

This checklist validates that a Kubespray-managed Kubernetes cluster is production-ready, fully reproducible, and meets day-2 operational standards. Each section includes validation procedures, evidence artifacts, and acceptance criteria suitable for automation via CLI commands.

**Cluster Name:** `<CLUSTER_NAME>`  
**Kubespray Version:** `<KUBESPRAY_VERSION>`  
**Kubespray Commit SHA:** `<KUBESPRAY_COMMIT_SHA>`  
**Deployment Date:** `<DEPLOYMENT_DATE>`  
**Handover Date:** `<HANDOVER_DATE>`

---

## Table of Contents

1. [Cluster Configuration Validation](#1-cluster-configuration-validation)
2. [Kubespray Deployment Validation](#2-kubespray-deployment-validation)
3. [Infrastructure & Topology](#3-infrastructure--topology)
4. [Control Plane & etcd](#4-control-plane--etcd)
5. [Node OS Baseline](#5-node-os-baseline)
6. [Networking](#6-networking)
7. [Security & Certificates](#7-security--certificates)
8. [Post-Install Conformance](#8-post-install-conformance)
9. [Managed Services Validation](#9-managed-services-validation)
10. [Day-2 Readiness Gates](#10-day-2-readiness-gates)
11. [Sign-Off & Acceptance](#11-sign-off--acceptance)

---

## 1. Cluster Configuration Validation

**Purpose:** Validate that the cluster configuration file accurately reflects the deployed cluster and all enabled services. This section establishes the baseline for all subsequent validation by comparing the declared configuration against actual cluster state.

**Reference Documentation:**
- [Cluster Configuration File Reference](./cluster-config.md) - Complete documentation of the configuration file structure

### 1.1 Configuration File Validation

**Purpose:** Validate that the cluster configuration file exists, is valid, and matches the deployed cluster.

**Prerequisites:**
- Access to cluster configuration file (`.<cluster-name>-config.yaml`)
- openCenter CLI installed
- yq or similar YAML parser installed

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate config \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Locate and verify cluster configuration file:**
   ```bash
   # Configuration file location (organization-based structure)
   CONFIG_FILE=~/.config/openCenter/clusters/<organization>/.<cluster-name>-config.yaml
   
   # Verify file exists and is readable
   ls -la $CONFIG_FILE
   
   # Check file permissions (should be 0600)
   stat -c "%a %n" $CONFIG_FILE  # Linux
   stat -f "%Lp %N" $CONFIG_FILE  # macOS
   ```

2. **Validate configuration against JSON schema:**
   ```bash
   # Using openCenter CLI
   openCenter cluster validate <CLUSTER_NAME>
   
   # Or using external validator (ajv-cli)
   openCenter cluster schema --out cluster.schema.json
   ajv validate -s cluster.schema.json -d $CONFIG_FILE
   ```

3. **Extract and verify cluster metadata:**
   ```bash
   # Parse configuration file
   echo "=== Cluster Metadata ==="
   yq eval '.opencenter.meta' $CONFIG_FILE
   
   echo "=== Cluster Name ==="
   yq eval '.opencenter.cluster.cluster_name' $CONFIG_FILE
   
   echo "=== Kubernetes Version ==="
   yq eval '.opencenter.cluster.kubernetes.version' $CONFIG_FILE
   
   echo "=== Infrastructure Provider ==="
   yq eval '.opencenter.infrastructure.provider' $CONFIG_FILE
   ```

4. **Verify infrastructure provider configuration:**
   ```bash
   # Check provider-specific settings
   PROVIDER=$(yq eval '.opencenter.infrastructure.provider' $CONFIG_FILE)
   echo "Provider: $PROVIDER"
   
   # For OpenStack
   if [ "$PROVIDER" = "openstack" ]; then
     echo "=== OpenStack Configuration ==="
     yq eval '.opencenter.infrastructure.cloud.openstack' $CONFIG_FILE
   fi
   
   # For AWS
   if [ "$PROVIDER" = "aws" ]; then
     echo "=== AWS Configuration ==="
     yq eval '.opencenter.infrastructure.cloud.aws' $CONFIG_FILE
   fi
   
   # For baremetal
   if [ "$PROVIDER" = "baremetal" ]; then
     echo "=== Baremetal Configuration ==="
     yq eval '.opencenter.cluster.kubernetes.master_nodes' $CONFIG_FILE
     yq eval '.opencenter.cluster.kubernetes.worker_nodes' $CONFIG_FILE
   fi
   ```

5. **List all enabled services from configuration:**
   ```bash
   # Extract all enabled services
   echo "=== Enabled Services ==="
   yq eval '.opencenter.services | to_entries | .[] | select(.value.enabled == true) | .key' $CONFIG_FILE | tee enabled-services.txt
   
   # Count enabled services
   SERVICE_COUNT=$(cat enabled-services.txt | wc -l)
   echo "Total enabled services: $SERVICE_COUNT"
   ```

6. **Verify CNI plugin configuration (only one should be enabled):**
   ```bash
   # Check which CNI is enabled
   echo "=== CNI Plugin Configuration ==="
   yq eval '.opencenter.cluster.kubernetes.network_plugin' $CONFIG_FILE
   
   # Verify only one CNI is enabled
   CNI_COUNT=$(yq eval '.opencenter.cluster.kubernetes.network_plugin | to_entries | .[] | select(.value.enabled == true) | .key' $CONFIG_FILE | wc -l)
   
   if [ "$CNI_COUNT" -ne 1 ]; then
     echo "❌ ERROR: Multiple or no CNI plugins enabled (found: $CNI_COUNT)"
     exit 1
   else
     CNI_NAME=$(yq eval '.opencenter.cluster.kubernetes.network_plugin | to_entries | .[] | select(.value.enabled == true) | .key' $CONFIG_FILE)
     echo "✅ Single CNI plugin enabled: $CNI_NAME"
   fi
   ```

7. **Verify GitOps configuration:**
   ```bash
   # Check GitOps settings
   echo "=== GitOps Configuration ==="
   yq eval '.opencenter.gitops' $CONFIG_FILE
   
   # Verify GitOps directory exists
   GITOPS_DIR=$(yq eval '.opencenter.gitops.git_dir' $CONFIG_FILE)
   if [ -d "$GITOPS_DIR" ]; then
     echo "✅ GitOps directory exists: $GITOPS_DIR"
     ls -la $GITOPS_DIR
   else
     echo "❌ GitOps directory not found: $GITOPS_DIR"
   fi
   ```

8. **Check secrets configuration:**
   ```bash
   # Verify SOPS key path
   echo "=== Secrets Configuration ==="
   SOPS_KEY=$(yq eval '.secrets.sops_age_key_file' $CONFIG_FILE)
   if [ -f "$SOPS_KEY" ]; then
     echo "✅ SOPS key exists: $SOPS_KEY"
   else
     echo "❌ SOPS key not found: $SOPS_KEY"
   fi
   
   # Verify SSH key paths
   SSH_PRIVATE=$(yq eval '.secrets.ssh_key.private' $CONFIG_FILE)
   SSH_PUBLIC=$(yq eval '.secrets.ssh_key.public' $CONFIG_FILE)
   
   if [ -f "$SSH_PRIVATE" ] && [ -f "$SSH_PUBLIC" ]; then
     echo "✅ SSH keys exist"
     ls -la $SSH_PRIVATE $SSH_PUBLIC
   else
     echo "❌ SSH keys not found"
   fi
   ```

9. **Validate node counts and topology:**
   ```bash
   # Extract node configuration
   echo "=== Node Configuration ==="
   MASTER_COUNT=$(yq eval '.opencenter.cluster.kubernetes.master_count' $CONFIG_FILE)
   WORKER_COUNT=$(yq eval '.opencenter.cluster.kubernetes.worker_count' $CONFIG_FILE)
   
   echo "Control Plane Nodes (configured): $MASTER_COUNT"
   echo "Worker Nodes (configured): $WORKER_COUNT"
   echo "Total Nodes (configured): $((MASTER_COUNT + WORKER_COUNT))"
   
   # Verify odd number for HA
   if [ $MASTER_COUNT -gt 1 ] && [ $((MASTER_COUNT % 2)) -eq 0 ]; then
     echo "⚠️  WARNING: Control plane count should be odd for HA (found: $MASTER_COUNT)"
   fi
   ```

10. **Compare configuration with actual cluster state:**
    ```bash
    echo "=== Configuration vs Actual State ==="
    
    # Compare node count
    ACTUAL_NODES=$(kubectl get nodes --no-headers | wc -l)
    EXPECTED_NODES=$((MASTER_COUNT + WORKER_COUNT))
    
    if [ "$ACTUAL_NODES" -eq "$EXPECTED_NODES" ]; then
      echo "✅ Node count matches: Expected $EXPECTED_NODES, Found $ACTUAL_NODES"
    else
      echo "❌ Node count mismatch: Expected $EXPECTED_NODES, Found $ACTUAL_NODES"
    fi
    
    # Compare Kubernetes version
    CONFIGURED_VERSION=$(yq eval '.opencenter.cluster.kubernetes.version' $CONFIG_FILE)
    ACTUAL_VERSION=$(kubectl version --short 2>/dev/null | grep Server | awk '{print $3}' | sed 's/v//')
    
    if [[ "$ACTUAL_VERSION" == "$CONFIGURED_VERSION"* ]]; then
      echo "✅ Kubernetes version matches: $CONFIGURED_VERSION"
    else
      echo "❌ Version mismatch: Expected $CONFIGURED_VERSION, Found $ACTUAL_VERSION"
    fi
    
    # Compare CNI plugin
    CONFIGURED_CNI=$(yq eval '.opencenter.cluster.kubernetes.network_plugin | to_entries | .[] | select(.value.enabled == true) | .key' $CONFIG_FILE)
    
    case $CONFIGURED_CNI in
      calico)
        kubectl get pods -n kube-system -l k8s-app=calico-node &>/dev/null && echo "✅ Calico CNI deployed" || echo "❌ Calico CNI not found"
        ;;
      cilium)
        kubectl get pods -n kube-system -l k8s-app=cilium &>/dev/null && echo "✅ Cilium CNI deployed" || echo "❌ Cilium CNI not found"
        ;;
      kube-ovn)
        kubectl get pods -n kube-system -l app=kube-ovn-cni &>/dev/null && echo "✅ Kube-OVN CNI deployed" || echo "❌ Kube-OVN CNI not found"
        ;;
    esac
    ```

**Evidence Artifacts:**
- [ ] `cluster-config.yaml` - Full cluster configuration file
- [ ] `config-validation.txt` - Schema validation results
- [ ] `enabled-services.txt` - List of enabled services
- [ ] `config-vs-actual.txt` - Configuration vs actual state comparison
- [ ] `config-metadata.json` - Extracted metadata

**Acceptance Criteria:**
- ✅ Configuration file exists and is readable (permissions 0600)
- ✅ Configuration validates against JSON schema
- ✅ All required fields populated
- ✅ Infrastructure provider correctly configured
- ✅ Only one CNI plugin enabled
- ✅ Node counts match actual cluster
- ✅ Kubernetes version matches actual cluster
- ✅ GitOps directory exists and accessible
- ✅ SOPS and SSH keys exist at configured paths
- ✅ Enabled services list documented

**Risks & Pitfalls:**
- ⚠️ Configuration drift after manual cluster changes
- ⚠️ Missing or incorrect service configurations
- ⚠️ Secrets paths pointing to non-existent files
- ⚠️ Multiple CNI plugins enabled causing conflicts
- ⚠️ Node count mismatch indicating scaling without config update

---

### 1.2 Service Configuration Matrix

**Purpose:** Create a comprehensive matrix of configured vs deployed services for validation. This matrix serves as the checklist for Section 9 (Managed Services Validation).

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate service-matrix \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Generate service matrix from configuration:**
   ```bash
   cat > service-matrix.sh <<'EOF'
   #!/bin/bash
   CONFIG_FILE=$1
   
   echo "Service,Configured,Deployed,Status"
   
   # Check each service from the configuration
   for service in calico cert-manager etcd-backup external-snapshotter fluxcd \
                  gateway gateway-api headlamp keycloak kube-prometheus-stack \
                  kyverno loki olm openstack-ccm openstack-csi postgres-operator \
                  prometheus rbac-manager sources velero vsphere-csi weave-gitops; do
     
     ENABLED=$(yq eval ".opencenter.services.$service.enabled" $CONFIG_FILE 2>/dev/null)
     
     if [ "$ENABLED" = "true" ]; then
       # Check if deployed (namespace or pods exist)
       case $service in
         fluxcd)
           DEPLOYED=$(kubectl get ns flux-system 2>/dev/null && echo "true" || echo "false")
           ;;
         cert-manager)
           DEPLOYED=$(kubectl get ns cert-manager 2>/dev/null && echo "true" || echo "false")
           ;;
         kube-prometheus-stack)
           DEPLOYED=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus 2>/dev/null | grep -q Running && echo "true" || echo "false")
           ;;
         keycloak)
           DEPLOYED=$(kubectl get pods -n keycloak 2>/dev/null | grep -q Running && echo "true" || echo "false")
           ;;
         headlamp)
           DEPLOYED=$(kubectl get pods -n headlamp 2>/dev/null | grep -q Running && echo "true" || echo "false")
           ;;
         velero)
           DEPLOYED=$(kubectl get ns velero 2>/dev/null && echo "true" || echo "false")
           ;;
         *)
           DEPLOYED=$(kubectl get pods --all-namespaces -l app.kubernetes.io/name=$service 2>/dev/null | grep -q Running && echo "true" || echo "false")
           ;;
       esac
       
       if [ "$DEPLOYED" = "true" ]; then
         STATUS="✅ Match"
       else
         STATUS="❌ Missing"
       fi
     else
       ENABLED="false"
       DEPLOYED="N/A"
       STATUS="⊘ Disabled"
     fi
     
     echo "$service,$ENABLED,$DEPLOYED,$STATUS"
   done
   EOF
   
   chmod +x service-matrix.sh
   ./service-matrix.sh $CONFIG_FILE | column -t -s,
   ```

2. **Validate critical services are enabled:**
   ```bash
   # Critical services that should typically be enabled
   CRITICAL_SERVICES="fluxcd cert-manager gateway-api"
   
   echo "=== Critical Services Check ==="
   for service in $CRITICAL_SERVICES; do
     ENABLED=$(yq eval ".opencenter.services.$service.enabled" $CONFIG_FILE)
     if [ "$ENABLED" != "true" ]; then
       echo "⚠️  WARNING: Critical service $service is not enabled"
     else
       echo "✅ Critical service $service is enabled"
     fi
   done
   ```

3. **Check service-specific configuration for enabled services:**
   ```bash
   # For each enabled service, verify its configuration
   echo "=== Service Configurations ==="
   while read service; do
     echo ""
     echo "=== $service Configuration ==="
     yq eval ".opencenter.services.$service" $CONFIG_FILE
   done < enabled-services.txt
   ```

4. **Verify service dependencies:**
   ```bash
   # Check for common service dependencies
   echo "=== Service Dependencies Check ==="
   
   # If Keycloak is enabled, check OIDC configuration
   KEYCLOAK_ENABLED=$(yq eval ".opencenter.services.keycloak.enabled" $CONFIG_FILE)
   if [ "$KEYCLOAK_ENABLED" = "true" ]; then
     echo "Keycloak enabled, checking OIDC configuration..."
     yq eval '.opencenter.cluster.kubernetes.oidc' $CONFIG_FILE
   fi
   
   # If Loki is enabled, check storage configuration
   LOKI_ENABLED=$(yq eval ".opencenter.services.loki.enabled" $CONFIG_FILE)
   if [ "$LOKI_ENABLED" = "true" ]; then
     echo "Loki enabled, checking storage configuration..."
     yq eval '.opencenter.services.loki | pick(["swift_auth_url", "loki_bucket_name", "loki_storage_class"])' $CONFIG_FILE
   fi
   
   # If Velero is enabled, check backup configuration
   VELERO_ENABLED=$(yq eval ".opencenter.services.velero.enabled" $CONFIG_FILE)
   if [ "$VELERO_ENABLED" = "true" ]; then
     echo "Velero enabled, checking backup configuration..."
     yq eval '.opencenter.services.velero | pick(["velero_backup_bucket", "velero_region"])' $CONFIG_FILE
   fi
   ```

**Evidence Artifacts:**
- [ ] `service-matrix.csv` - Service configuration matrix
- [ ] `service-configs.yaml` - All service configurations
- [ ] `critical-services-check.txt` - Critical services validation
- [ ] `service-dependencies.txt` - Service dependency validation

**Acceptance Criteria:**
- ✅ All enabled services are deployed
- ✅ No unexpected services running
- ✅ Critical services enabled and healthy
- ✅ Service configurations match requirements
- ✅ Service dependencies satisfied

**Risks & Pitfalls:**
- ⚠️ Services enabled in config but not deployed
- ⚠️ Services deployed but not in configuration (shadow IT)
- ⚠️ Missing service dependencies
- ⚠️ Incomplete service configurations

---

## 2. Kubespray Deployment Validation

### 1.1 Inventory & Configuration

**Purpose:** Validate that the Kubespray inventory accurately reflects the deployed cluster topology.

**Prerequisites:**
- Access to Kubespray inventory directory
- Original deployment artifacts preserved
- Git repository with inventory tracked

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate kubespray-inventory \
  --cluster <CLUSTER_NAME> \
  --inventory-path <KUBESPRAY_INVENTORY>
```

**Manual Validation Steps:**

1. **Verify inventory structure:**
   ```bash
   tree <KUBESPRAY_INVENTORY>
   # Expected: hosts.yaml, group_vars/, host_vars/
   ```

2. **Validate host groups match deployed topology:**
   ```bash
   ansible-inventory -i <KUBESPRAY_INVENTORY>/hosts.yaml --list | jq '.all.children'
   # Verify: kube_control_plane, etcd, kube_node, k8s_cluster
   ```

3. **Check control plane node count:**
   ```bash
   ansible-inventory -i <KUBESPRAY_INVENTORY>/hosts.yaml --list | \
     jq '.kube_control_plane.hosts | length'
   # Expected: Odd number (1, 3, 5) for HA
   ```

4. **Validate etcd topology:**
   ```bash
   ansible-inventory -i <KUBESPRAY_INVENTORY>/hosts.yaml --list | \
     jq '.etcd.hosts'
   # Expected: Same as control plane or dedicated etcd nodes
   ```

5. **Verify critical group_vars:**
   ```bash
   cat <KUBESPRAY_INVENTORY>/group_vars/k8s_cluster/k8s-cluster.yml | \
     grep -E "kube_version|kube_network_plugin|container_manager"
   ```

6. **Check scaling configuration:**
   ```bash
   cat <KUBESPRAY_INVENTORY>/group_vars/k8s_cluster/addons.yml | \
     grep -E "metrics_server_enabled|ingress_nginx_enabled"
   ```

**Evidence Artifacts:**
- [ ] `inventory-structure.txt` - Full inventory tree output
- [ ] `host-groups.json` - Ansible inventory JSON dump
- [ ] `group-vars-snapshot.tar.gz` - All group_vars and host_vars
- [ ] `inventory-diff.txt` - Git diff from initial to final state

**Acceptance Criteria:**
- ✅ Inventory contains all deployed nodes
- ✅ Host groups match actual cluster topology
- ✅ Control plane count is odd (HA requirement)
- ✅ etcd nodes are properly distributed
- ✅ All critical variables are documented
- ✅ Inventory is version-controlled in Git

**Risks & Pitfalls:**
- ⚠️ Inventory drift after manual node additions
- ⚠️ Undocumented variable overrides in host_vars
- ⚠️ Missing IP address reservations for future scaling

---

### 1.2 Deployment Integrity

**Purpose:** Confirm Kubespray Ansible playbooks executed successfully without errors.

**Prerequisites:**
- Ansible execution logs preserved
- Access to deployment automation system
- Playbook run history available

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate kubespray-deployment \
  --cluster <CLUSTER_NAME> \
  --ansible-log <ANSIBLE_LOG_PATH>
```

**Manual Validation Steps:**

1. **Check Ansible playbook exit codes:**
   ```bash
   grep -E "PLAY RECAP|failed=" <ANSIBLE_LOG_PATH>
   # Expected: failed=0 for all hosts
   ```

2. **Validate no stderr warnings:**
   ```bash
   grep -i "warn\|error\|fatal" <ANSIBLE_LOG_PATH> | \
     grep -v "ok:" | wc -l
   # Expected: 0 or documented exceptions
   ```

3. **Verify all tasks completed:**
   ```bash
   grep "TASK \[" <ANSIBLE_LOG_PATH> | wc -l
   # Compare with expected task count
   ```

4. **Check for skipped critical tasks:**
   ```bash
   grep "skipping:" <ANSIBLE_LOG_PATH> | \
     grep -E "etcd|apiserver|kubelet"
   # Expected: None for core components
   ```

5. **Run idempotency check (dry-run):**
   ```bash
   ansible-playbook -i <KUBESPRAY_INVENTORY>/hosts.yaml \
     cluster.yml --check --diff > idempotency-check.log 2>&1
   ```

6. **Validate no configuration drift:**
   ```bash
   grep "changed:" idempotency-check.log
   # Expected: changed=0 for all hosts
   ```

**Evidence Artifacts:**
- [ ] `ansible-playbook.log` - Full deployment log
- [ ] `play-recap.txt` - Final task summary
- [ ] `idempotency-check.log` - Dry-run results
- [ ] `deployment-timeline.json` - Task execution times

**Acceptance Criteria:**
- ✅ All playbook runs completed with exit code 0
- ✅ No failed tasks in PLAY RECAP
- ✅ Idempotency check shows zero changes
- ✅ No unexpected warnings or errors
- ✅ Deployment logs archived and accessible

**Risks & Pitfalls:**
- ⚠️ Transient network errors masked by retries
- ⚠️ Non-idempotent tasks causing drift
- ⚠️ Missing log rotation causing disk pressure

---

### 1.3 Reproducibility Validation

**Purpose:** Ensure the cluster can be recreated using the same inventory and versions.

**Prerequisites:**
- Complete inventory and variable files
- Kubespray version pinned
- Infrastructure-as-Code for underlying VMs/nodes

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate kubespray-reproducibility \
  --cluster <CLUSTER_NAME> \
  --inventory-path <KUBESPRAY_INVENTORY> \
  --kubespray-commit <KUBESPRAY_COMMIT_SHA>
```

**Manual Validation Steps:**

1. **Verify Kubespray version pinning:**
   ```bash
   cd <KUBESPRAY_REPO>
   git rev-parse HEAD
   # Expected: Matches <KUBESPRAY_COMMIT_SHA>
   ```

2. **Check requirements.txt pinning:**
   ```bash
   cat <KUBESPRAY_REPO>/requirements.txt | grep -v "^#"
   # Expected: All versions pinned (no >= or ~=)
   ```

3. **Validate Kubernetes version lock:**
   ```bash
   grep "kube_version:" <KUBESPRAY_INVENTORY>/group_vars/k8s_cluster/k8s-cluster.yml
   # Expected: Exact version (e.g., v1.28.5)
   ```

4. **Check container image tags:**
   ```bash
   grep -r "image:" <KUBESPRAY_INVENTORY>/group_vars/ | \
     grep -v "latest"
   # Expected: All images use specific tags
   ```

5. **Verify infrastructure state:**
   ```bash
   # For Terraform/Tofu-managed infrastructure
   cd <INFRA_REPO>
   terraform show -json | jq '.values.root_module.resources[] | select(.type=="openstack_compute_instance_v2")'
   ```

6. **Document external dependencies:**
   ```bash
   cat > reproducibility-manifest.yaml <<EOF
   kubespray_version: <KUBESPRAY_COMMIT_SHA>
   kubernetes_version: <KUBE_VERSION>
   container_runtime: containerd-<VERSION>
   network_plugin: <CNI_PLUGIN>-<VERSION>
   infrastructure_provider: <PROVIDER>
   EOF
   ```

**Evidence Artifacts:**
- [ ] `reproducibility-manifest.yaml` - Version manifest
- [ ] `kubespray-commit.txt` - Git commit SHA
- [ ] `requirements-frozen.txt` - Python dependencies
- [ ] `container-images.txt` - All image references with tags

**Acceptance Criteria:**
- ✅ Kubespray commit SHA documented
- ✅ All versions explicitly pinned
- ✅ No "latest" tags in use
- ✅ Infrastructure state captured
- ✅ Reproducibility manifest complete

**Risks & Pitfalls:**
- ⚠️ Upstream image repositories changing/deleting tags
- ⚠️ Ansible Galaxy role version drift
- ⚠️ OS package repository changes

---

## 2. Infrastructure & Topology

### 2.1 Node Inventory

**Purpose:** Validate all nodes are accounted for and properly labeled.

**Prerequisites:**
- kubectl access to cluster
- Node SSH access for verification

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate infrastructure \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **List all nodes:**
   ```bash
   kubectl get nodes -o wide
   ```

2. **Verify node count matches inventory:**
   ```bash
   INVENTORY_COUNT=$(ansible-inventory -i <KUBESPRAY_INVENTORY>/hosts.yaml --list | \
     jq '.k8s_cluster.hosts | length')
   CLUSTER_COUNT=$(kubectl get nodes --no-headers | wc -l)
   [ "$INVENTORY_COUNT" -eq "$CLUSTER_COUNT" ] && echo "✅ Match" || echo "❌ Mismatch"
   ```

3. **Check node roles:**
   ```bash
   kubectl get nodes --show-labels | \
     grep -E "node-role.kubernetes.io/(control-plane|master|worker)"
   ```

4. **Validate node readiness:**
   ```bash
   kubectl get nodes -o json | \
     jq -r '.items[] | select(.status.conditions[] | select(.type=="Ready" and .status!="True")) | .metadata.name'
   # Expected: Empty output
   ```

5. **Check node resource capacity:**
   ```bash
   kubectl top nodes
   kubectl describe nodes | grep -A 5 "Allocated resources"
   ```

**Evidence Artifacts:**
- [ ] `nodes-list.txt` - Full node inventory
- [ ] `node-labels.yaml` - All node labels
- [ ] `node-capacity.json` - Resource capacity report

**Acceptance Criteria:**
- ✅ All nodes present and Ready
- ✅ Node count matches inventory
- ✅ Roles correctly assigned
- ✅ No resource pressure warnings

---

## 3. Control Plane & etcd

### 3.1 etcd Cluster Health

**Purpose:** Validate etcd cluster health, quorum, and backup capability.

**Prerequisites:**
- etcdctl installed
- etcd certificates available
- SSH access to etcd nodes

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate etcd \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check etcd cluster health:**
   ```bash
   ETCDCTL_API=3 etcdctl \
     --endpoints=https://127.0.0.1:2379 \
     --cacert=/etc/ssl/etcd/ssl/ca.pem \
     --cert=/etc/ssl/etcd/ssl/node-$(hostname).pem \
     --key=/etc/ssl/etcd/ssl/node-$(hostname)-key.pem \
     endpoint health --cluster
   ```

2. **Verify etcd member list:**
   ```bash
   ETCDCTL_API=3 etcdctl member list -w table
   # Expected: All members started and healthy
   ```

3. **Check etcd quorum:**
   ```bash
   MEMBER_COUNT=$(ETCDCTL_API=3 etcdctl member list | wc -l)
   QUORUM=$((MEMBER_COUNT / 2 + 1))
   echo "Quorum requires: $QUORUM members"
   ```

4. **Validate etcd metrics:**
   ```bash
   curl -k https://127.0.0.1:2379/metrics | \
     grep -E "etcd_server_has_leader|etcd_server_leader_changes_seen_total"
   ```

5. **Test etcd snapshot:**
   ```bash
   ETCDCTL_API=3 etcdctl snapshot save /tmp/etcd-snapshot-$(date +%Y%m%d).db
   ETCDCTL_API=3 etcdctl snapshot status /tmp/etcd-snapshot-$(date +%Y%m%d).db -w table
   ```

6. **Verify certificate expiration:**
   ```bash
   for cert in /etc/ssl/etcd/ssl/*.pem; do
     echo "=== $cert ==="
     openssl x509 -in $cert -noout -enddate 2>/dev/null || echo "Not a cert"
   done
   ```

**Evidence Artifacts:**
- [ ] `etcd-health.txt` - Cluster health output
- [ ] `etcd-members.txt` - Member list
- [ ] `etcd-snapshot.db` - Test snapshot file
- [ ] `etcd-cert-expiry.txt` - Certificate expiration dates

**Acceptance Criteria:**
- ✅ All etcd members healthy
- ✅ Quorum maintained
- ✅ No leader changes in last 24h
- ✅ Snapshot creation successful
- ✅ Certificates valid for >90 days

**Risks & Pitfalls:**
- ⚠️ Split-brain scenarios during network partitions
- ⚠️ Disk I/O latency causing leader elections
- ⚠️ Certificate expiration without rotation

---

### 3.2 Control Plane Components

**Purpose:** Validate kube-apiserver, controller-manager, and scheduler health.

**Prerequisites:**
- kubectl access
- SSH access to control plane nodes

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate control-plane \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check API server health:**
   ```bash
   kubectl get --raw='/readyz?verbose' | grep -E "readyz check passed|failed"
   kubectl get --raw='/livez?verbose'
   ```

2. **Verify all control plane pods:**
   ```bash
   kubectl get pods -n kube-system -l tier=control-plane -o wide
   # Expected: All Running
   ```

3. **Check API server metrics:**
   ```bash
   kubectl get --raw /metrics | grep -E "apiserver_request_total|apiserver_request_duration"
   ```

4. **Validate controller-manager leader election:**
   ```bash
   kubectl get lease -n kube-system kube-controller-manager -o yaml | \
     grep holderIdentity
   ```

5. **Check scheduler leader election:**
   ```bash
   kubectl get lease -n kube-system kube-scheduler -o yaml | \
     grep holderIdentity
   ```

6. **Verify component status (deprecated but useful):**
   ```bash
   kubectl get componentstatuses
   ```

7. **Test API server load balancer (if applicable):**
   ```bash
   # For Kubespray nginx/haproxy LB
   curl -k https://<LB_VIP>:6443/healthz
   ```

**Evidence Artifacts:**
- [ ] `apiserver-readyz.txt` - API server readiness
- [ ] `control-plane-pods.yaml` - Pod status
- [ ] `leader-elections.txt` - Current leaders
- [ ] `lb-health.txt` - Load balancer health

**Acceptance Criteria:**
- ✅ API server readyz returns 200
- ✅ All control plane pods Running
- ✅ Leader elections stable
- ✅ Load balancer healthy (if used)
- ✅ No API server errors in logs

**Risks & Pitfalls:**
- ⚠️ Load balancer single point of failure
- ⚠️ API server rate limiting misconfigured
- ⚠️ Controller-manager leader flapping

---

## 4. Node OS Baseline

### 4.1 Operating System Validation

**Purpose:** Validate OS versions, kernel modules, and system configuration.

**Prerequisites:**
- SSH access to all nodes
- Ansible inventory

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate node-os \
  --cluster <CLUSTER_NAME> \
  --inventory-path <KUBESPRAY_INVENTORY>
```

**Manual Validation Steps:**

1. **Check OS versions across all nodes:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "cat /etc/os-release | grep PRETTY_NAME"
   ```

2. **Verify kernel versions:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "uname -r"
   # Expected: Consistent versions
   ```

3. **Validate required kernel modules:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "lsmod | grep -E 'br_netfilter|overlay|ip_vs'"
   ```

4. **Check sysctl settings:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "sysctl net.bridge.bridge-nf-call-iptables net.ipv4.ip_forward"
   # Expected: Both = 1
   ```

5. **Verify swap is disabled:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "swapon --show"
   # Expected: Empty output
   ```

6. **Check SELinux/AppArmor status:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "getenforce || aa-status"
   ```

7. **Validate time synchronization:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "timedatectl status | grep 'System clock synchronized'"
   ```

**Evidence Artifacts:**
- [ ] `os-versions.txt` - OS release info
- [ ] `kernel-versions.txt` - Kernel versions
- [ ] `kernel-modules.txt` - Loaded modules
- [ ] `sysctl-settings.txt` - Kernel parameters

**Acceptance Criteria:**
- ✅ All nodes running same OS version
- ✅ Kernel versions consistent
- ✅ Required modules loaded
- ✅ Swap disabled on all nodes
- ✅ Time synchronized across cluster

**Risks & Pitfalls:**
- ⚠️ Kernel version drift after updates
- ⚠️ Missing kernel modules for CNI
- ⚠️ Time skew causing certificate issues

---

### 4.2 Container Runtime

**Purpose:** Validate containerd configuration and health.

**Prerequisites:**
- SSH access to nodes
- crictl installed

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate container-runtime \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check containerd version:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "containerd --version"
   ```

2. **Verify containerd service status:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "systemctl status containerd | grep Active"
   ```

3. **Validate containerd config:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "cat /etc/containerd/config.toml | grep -A 5 'plugins.\"io.containerd.grpc.v1.cri\"'"
   ```

4. **Check runtime class:**
   ```bash
   kubectl get runtimeclass
   ```

5. **Test crictl functionality:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "crictl ps | head -5"
   ```

6. **Verify image pull capability:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "crictl pull registry.k8s.io/pause:3.9"
   ```

**Evidence Artifacts:**
- [ ] `containerd-version.txt` - Runtime version
- [ ] `containerd-config.toml` - Configuration file
- [ ] `crictl-ps.txt` - Running containers

**Acceptance Criteria:**
- ✅ containerd running on all nodes
- ✅ Versions consistent
- ✅ Configuration matches Kubespray template
- ✅ Image pull successful

---

## 5. Networking

### 5.1 CNI Plugin Validation

**Purpose:** Validate network plugin (Cilium/Calico/Weave) health and connectivity.

**Prerequisites:**
- kubectl access
- Network plugin CLI tools (cilium, calicoctl)

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate networking \
  --cluster <CLUSTER_NAME> \
  --cni-plugin <CNI_PLUGIN>
```

**Manual Validation Steps (Cilium Example):**

1. **Check CNI plugin pods:**
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o wide
   # Expected: One pod per node, all Running
   ```

2. **Verify Cilium status:**
   ```bash
   cilium status --wait
   ```

3. **Run Cilium connectivity test:**
   ```bash
   cilium connectivity test --test-namespace cilium-test
   ```

4. **Check pod-to-pod connectivity:**
   ```bash
   kubectl run test-pod-1 --image=nicolaka/netshoot --restart=Never -- sleep 3600
   kubectl run test-pod-2 --image=nicolaka/netshoot --restart=Never -- sleep 3600
   POD1_IP=$(kubectl get pod test-pod-1 -o jsonpath='{.status.podIP}')
   kubectl exec test-pod-2 -- ping -c 3 $POD1_IP
   ```

5. **Validate pod-to-service connectivity:**
   ```bash
   kubectl create deployment nginx --image=nginx --replicas=2
   kubectl expose deployment nginx --port=80
   kubectl run test-curl --image=curlimages/curl --restart=Never -- \
     curl -s http://nginx.default.svc.cluster.local
   ```

6. **Check cross-node connectivity:**
   ```bash
   kubectl get pods -o wide | grep test-pod
   # Ensure pods on different nodes can communicate
   ```

7. **Verify MTU settings:**
   ```bash
   kubectl exec -n kube-system ds/cilium -- cilium status | grep MTU
   ```

8. **Check IPAM allocation:**
   ```bash
   kubectl get ippools  # For Calico
   # OR
   cilium bpf ipam list  # For Cilium
   ```

**Evidence Artifacts:**
- [ ] `cni-pods.yaml` - CNI plugin pod status
- [ ] `connectivity-test.log` - Full connectivity test output
- [ ] `pod-to-pod-ping.txt` - Ping test results
- [ ] `ipam-status.txt` - IP allocation status

**Acceptance Criteria:**
- ✅ CNI pods running on all nodes
- ✅ Connectivity tests pass
- ✅ Pod-to-pod communication works
- ✅ Pod-to-service communication works
- ✅ Cross-node traffic flows
- ✅ MTU correctly configured
- ✅ No IP exhaustion

**Risks & Pitfalls:**
- ⚠️ MTU mismatch causing packet fragmentation
- ⚠️ IP pool exhaustion
- ⚠️ Network policy blocking legitimate traffic
- ⚠️ Overlay network performance issues

---

### 5.2 DNS & Service Discovery

**Purpose:** Validate CoreDNS and service discovery.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate dns \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check CoreDNS pods:**
   ```bash
   kubectl get pods -n kube-system -l k8s-app=kube-dns
   ```

2. **Verify DNS resolution:**
   ```bash
   kubectl run test-dns --image=busybox --restart=Never -- \
     nslookup kubernetes.default.svc.cluster.local
   kubectl logs test-dns
   ```

3. **Test external DNS:**
   ```bash
   kubectl run test-external-dns --image=busybox --restart=Never -- \
     nslookup google.com
   kubectl logs test-external-dns
   ```

4. **Check CoreDNS ConfigMap:**
   ```bash
   kubectl get configmap -n kube-system coredns -o yaml
   ```

5. **Verify DNS metrics:**
   ```bash
   kubectl get --raw /api/v1/namespaces/kube-system/services/kube-dns:metrics/proxy/metrics | \
     grep coredns_dns_request_count_total
   ```

**Evidence Artifacts:**
- [ ] `coredns-pods.yaml` - CoreDNS status
- [ ] `dns-resolution-test.txt` - DNS test results
- [ ] `coredns-config.yaml` - CoreDNS configuration

**Acceptance Criteria:**
- ✅ CoreDNS pods healthy
- ✅ Internal DNS resolution works
- ✅ External DNS resolution works
- ✅ No DNS timeout errors

---

## 6. Security & Certificates

### 6.1 Certificate Validation

**Purpose:** Validate Kubespray-generated certificates and kubeconfigs.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate certificates \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check API server certificate:**
   ```bash
   echo | openssl s_client -connect <API_SERVER>:6443 2>/dev/null | \
     openssl x509 -noout -dates -subject -issuer
   ```

2. **Verify certificate expiration dates:**
   ```bash
   kubeadm certs check-expiration
   # Run on control plane node
   ```

3. **Validate kubeconfig files:**
   ```bash
   # Admin kubeconfig
   kubectl config view --kubeconfig=/etc/kubernetes/admin.conf
   
   # Verify certificate in kubeconfig
   kubectl config view --kubeconfig=/etc/kubernetes/admin.conf --raw | \
     grep client-certificate-data | awk '{print $2}' | base64 -d | \
     openssl x509 -noout -dates
   ```

4. **Check kubelet certificates:**
   ```bash
   ansible -i <KUBESPRAY_INVENTORY>/hosts.yaml all -m shell \
     -a "ls -la /var/lib/kubelet/pki/"
   ```

5. **Verify certificate rotation configuration:**
   ```bash
   kubectl get cm -n kube-system kubeadm-config -o yaml | \
     grep -A 5 certificatesDir
   ```

6. **Test certificate rotation capability:**
   ```bash
   # Document the process, don't execute in production
   cat > cert-rotation-test.md <<EOF
   # Certificate Rotation Test Plan
   1. Backup current certificates
   2. Run: kubeadm certs renew all
   3. Restart control plane components
   4. Verify cluster functionality
   EOF
   ```

**Evidence Artifacts:**
- [ ] `cert-expiration.txt` - All certificate expiry dates
- [ ] `kubeconfig-validation.txt` - Kubeconfig verification
- [ ] `cert-rotation-plan.md` - Rotation procedure

**Acceptance Criteria:**
- ✅ All certificates valid for >90 days
- ✅ Certificate chain complete
- ✅ Kubeconfigs functional
- ✅ Rotation procedure documented

**Risks & Pitfalls:**
- ⚠️ Certificate expiration causing cluster outage
- ⚠️ Missing CA certificate backup
- ⚠️ Rotation procedure not tested

---

### 6.2 RBAC & Security Policies

**Purpose:** Validate RBAC configuration and security policies.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate security \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Audit cluster roles:**
   ```bash
   kubectl get clusterroles | grep -v "system:"
   ```

2. **Check cluster role bindings:**
   ```bash
   kubectl get clusterrolebindings -o json | \
     jq -r '.items[] | select(.subjects[]?.name=="system:anonymous") | .metadata.name'
   # Expected: Empty or documented
   ```

3. **Verify Pod Security Standards:**
   ```bash
   kubectl get ns -o yaml | grep -A 3 "pod-security.kubernetes.io"
   ```

4. **Check for privileged pods:**
   ```bash
   kubectl get pods --all-namespaces -o json | \
     jq -r '.items[] | select(.spec.containers[].securityContext.privileged==true) | "\(.metadata.namespace)/\(.metadata.name)"'
   ```

5. **Validate service account tokens:**
   ```bash
   kubectl get serviceaccounts --all-namespaces
   kubectl get secrets --all-namespaces | grep "service-account-token"
   ```

6. **Check network policies:**
   ```bash
   kubectl get networkpolicies --all-namespaces
   ```

7. **Audit admission controllers:**
   ```bash
   kubectl exec -n kube-system kube-apiserver-<NODE> -- \
     kube-apiserver --help | grep enable-admission-plugins
   ```

**Evidence Artifacts:**
- [ ] `rbac-audit.yaml` - RBAC configuration
- [ ] `privileged-pods.txt` - List of privileged workloads
- [ ] `network-policies.yaml` - Network policy dump
- [ ] `admission-controllers.txt` - Enabled admission controllers

**Acceptance Criteria:**
- ✅ No anonymous access
- ✅ Privileged pods documented and justified
- ✅ Pod Security Standards enforced
- ✅ Network policies in place
- ✅ Admission controllers enabled (PodSecurity, NodeRestriction)

---

### 6.3 Secrets Management

**Purpose:** Validate secrets are properly encrypted and managed.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate secrets \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check encryption at rest:**
   ```bash
   kubectl get secrets -n kube-system -o yaml | head -20
   # Verify encryption provider in API server config
   ```

2. **Verify encryption configuration:**
   ```bash
   cat /etc/kubernetes/manifests/kube-apiserver.yaml | \
     grep encryption-provider-config
   ```

3. **Test secret encryption:**
   ```bash
   kubectl create secret generic test-secret --from-literal=key=value
   ETCDCTL_API=3 etcdctl get /registry/secrets/default/test-secret | hexdump -C
   # Should show encrypted data, not plaintext
   ```

4. **Audit secret access:**
   ```bash
   kubectl get secrets --all-namespaces -o json | \
     jq -r '.items[] | "\(.metadata.namespace)/\(.metadata.name)"' | wc -l
   ```

5. **Check for hardcoded secrets in manifests:**
   ```bash
   kubectl get all --all-namespaces -o yaml | \
     grep -i "password\|token\|key" | grep -v "serviceaccount"
   ```

**Evidence Artifacts:**
- [ ] `encryption-config.yaml` - Encryption configuration
- [ ] `secret-audit.txt` - Secret inventory
- [ ] `encryption-test.txt` - Encryption verification

**Acceptance Criteria:**
- ✅ Secrets encrypted at rest
- ✅ No plaintext secrets in etcd
- ✅ No hardcoded secrets in manifests
- ✅ Encryption keys rotated regularly

---

## 7. Post-Install Conformance

### 7.1 Kubernetes Conformance Tests

**Purpose:** Run official Kubernetes conformance suite.

**Prerequisites:**
- Sonobuoy installed
- Sufficient cluster resources

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate conformance \
  --cluster <CLUSTER_NAME> \
  --suite sonobuoy
```

**Manual Validation Steps:**

1. **Install Sonobuoy:**
   ```bash
   wget https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.57.1/sonobuoy_0.57.1_linux_amd64.tar.gz
   tar -xzf sonobuoy_0.57.1_linux_amd64.tar.gz
   sudo mv sonobuoy /usr/local/bin/
   ```

2. **Run conformance tests:**
   ```bash
   sonobuoy run --mode=certified-conformance --wait
   ```

3. **Check test status:**
   ```bash
   sonobuoy status
   ```

4. **Retrieve results:**
   ```bash
   results=$(sonobuoy retrieve)
   sonobuoy results $results
   ```

5. **Extract detailed results:**
   ```bash
   sonobuoy results $results --mode=detailed > conformance-detailed.txt
   ```

6. **Check for failures:**
   ```bash
   sonobuoy results $results --mode=detailed | grep -E "failed|error"
   ```

7. **Cleanup:**
   ```bash
   sonobuoy delete --wait
   ```

**Evidence Artifacts:**
- [ ] `sonobuoy-results.tar.gz` - Full test results
- [ ] `conformance-summary.txt` - Test summary
- [ ] `conformance-detailed.txt` - Detailed results
- [ ] `conformance-failures.txt` - Any failures

**Acceptance Criteria:**
- ✅ All conformance tests pass
- ✅ No critical failures
- ✅ Results archived for certification

**Risks & Pitfalls:**
- ⚠️ Tests consuming excessive resources
- ⚠️ Timeouts on slow clusters
- ⚠️ Network policy blocking test pods

---

### 7.2 Kubespray Health Checks

**Purpose:** Run Kubespray's built-in cluster health playbooks.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate kubespray-health \
  --cluster <CLUSTER_NAME> \
  --inventory-path <KUBESPRAY_INVENTORY>
```

**Manual Validation Steps:**

1. **Run cluster health playbook:**
   ```bash
   cd <KUBESPRAY_REPO>
   ansible-playbook -i <KUBESPRAY_INVENTORY>/hosts.yaml \
     cluster-health.yml > cluster-health.log 2>&1
   ```

2. **Verify all health checks passed:**
   ```bash
   grep "PLAY RECAP" cluster-health.log
   # Expected: failed=0 for all hosts
   ```

3. **Run upgrade simulation (dry-run):**
   ```bash
   ansible-playbook -i <KUBESPRAY_INVENTORY>/hosts.yaml \
     upgrade-cluster.yml --check --diff > upgrade-check.log 2>&1
   ```

4. **Check for configuration drift:**
   ```bash
   grep "changed:" upgrade-check.log
   ```

**Evidence Artifacts:**
- [ ] `cluster-health.log` - Health check results
- [ ] `upgrade-check.log` - Upgrade simulation

**Acceptance Criteria:**
- ✅ All health checks pass
- ✅ No unexpected drift
- ✅ Upgrade path validated

---

### 7.3 Network & Ingress Tests

**Purpose:** Validate end-to-end network and ingress functionality.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate ingress \
  --cluster <CLUSTER_NAME> \
  --ingress-controller <INGRESS_CONTROLLER>
```

**Manual Validation Steps:**

1. **Deploy test application:**
   ```bash
   kubectl create deployment echo --image=ealen/echo-server:latest
   kubectl expose deployment echo --port=80
   ```

2. **Create ingress resource:**
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: echo-ingress
   spec:
     ingressClassName: <INGRESS_CLASS>
     rules:
     - host: echo.test.local
       http:
         paths:
         - path: /
           pathType: Prefix
           backend:
             service:
               name: echo
               port:
                 number: 80
   EOF
   ```

3. **Test ingress connectivity:**
   ```bash
   INGRESS_IP=$(kubectl get ingress echo-ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   curl -H "Host: echo.test.local" http://$INGRESS_IP/
   ```

4. **Verify TLS termination (if configured):**
   ```bash
   curl -k -H "Host: echo.test.local" https://$INGRESS_IP/
   ```

5. **Test load balancing:**
   ```bash
   kubectl scale deployment echo --replicas=3
   for i in {1..10}; do
     curl -s -H "Host: echo.test.local" http://$INGRESS_IP/ | grep hostname
   done
   ```

**Evidence Artifacts:**
- [ ] `ingress-test.txt` - Ingress test results
- [ ] `ingress-config.yaml` - Ingress configuration

**Acceptance Criteria:**
- ✅ Ingress controller healthy
- ✅ HTTP routing works
- ✅ TLS termination works (if configured)
- ✅ Load balancing distributes traffic

---

## 8. Managed Services Validation

### 8.1 Flux (GitOps)

**Purpose:** Validate Flux GitOps controllers and reconciliation.

**Prerequisites:**
- Flux CLI installed
- Git repository access
- Flux deployed in cluster

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate flux \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check Flux installation:**
   ```bash
   flux check
   ```

2. **Verify Flux components:**
   ```bash
   kubectl get pods -n flux-system
   ```

3. **List GitRepository sources:**
   ```bash
   flux get sources git
   ```

4. **Check Kustomization status:**
   ```bash
   flux get kustomizations
   ```

5. **Verify reconciliation:**
   ```bash
   flux reconcile source git flux-system
   flux reconcile kustomization flux-system
   ```

6. **Test drift detection:**
   ```bash
   # Manually modify a resource
   kubectl scale deployment -n flux-system source-controller --replicas=0
   # Wait for Flux to reconcile
   sleep 60
   kubectl get deployment -n flux-system source-controller
   # Expected: Replicas restored
   ```

7. **Check Flux logs:**
   ```bash
   flux logs --all-namespaces --since=1h
   ```

**Evidence Artifacts:**
- [ ] `flux-check.txt` - Flux health check
- [ ] `flux-sources.yaml` - Git sources
- [ ] `flux-kustomizations.yaml` - Kustomizations
- [ ] `flux-drift-test.txt` - Drift detection test

**Acceptance Criteria:**
- ✅ All Flux components healthy
- ✅ Git sources syncing
- ✅ Kustomizations applied
- ✅ Drift detection working
- ✅ No reconciliation errors

**Risks & Pitfalls:**
- ⚠️ Git authentication failures
- ⚠️ Kustomization conflicts
- ⚠️ Webhook not configured for fast updates

---

### 8.2 cert-manager

**Purpose:** Validate certificate management and issuance.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate cert-manager \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check cert-manager pods:**
   ```bash
   kubectl get pods -n cert-manager
   ```

2. **Verify ClusterIssuers:**
   ```bash
   kubectl get clusterissuers
   kubectl describe clusterissuer <ISSUER_NAME>
   ```

3. **Test certificate issuance:**
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: test-cert
     namespace: default
   spec:
     secretName: test-cert-tls
     issuerRef:
       name: <ISSUER_NAME>
       kind: ClusterIssuer
     dnsNames:
     - test.example.com
   EOF
   ```

4. **Check certificate status:**
   ```bash
   kubectl get certificate test-cert -o yaml
   kubectl describe certificate test-cert
   ```

5. **Verify certificate secret:**
   ```bash
   kubectl get secret test-cert-tls -o yaml
   ```

6. **Check cert-manager logs:**
   ```bash
   kubectl logs -n cert-manager deployment/cert-manager --tail=100
   ```

7. **Validate webhook:**
   ```bash
   kubectl get validatingwebhookconfigurations | grep cert-manager
   ```

**Evidence Artifacts:**
- [ ] `cert-manager-pods.yaml` - Pod status
- [ ] `clusterissuers.yaml` - Issuer configuration
- [ ] `test-certificate.yaml` - Test certificate
- [ ] `cert-manager-logs.txt` - Recent logs

**Acceptance Criteria:**
- ✅ cert-manager pods healthy
- ✅ ClusterIssuers ready
- ✅ Test certificate issued successfully
- ✅ Webhook functional
- ✅ No issuance errors

**Risks & Pitfalls:**
- ⚠️ ACME rate limits
- ⚠️ DNS01 challenge failures
- ⚠️ Webhook blocking certificate creation

---

### 8.3 external-dns

**Purpose:** Validate automatic DNS record management.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate external-dns \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check external-dns pods:**
   ```bash
   kubectl get pods -n external-dns
   ```

2. **Verify external-dns configuration:**
   ```bash
   kubectl get deployment -n external-dns external-dns -o yaml | \
     grep -A 10 "args:"
   ```

3. **Create test service with annotation:**
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: v1
   kind: Service
   metadata:
     name: test-external-dns
     annotations:
       external-dns.alpha.kubernetes.io/hostname: test-dns.example.com
   spec:
     type: LoadBalancer
     ports:
     - port: 80
     selector:
       app: test
   EOF
   ```

4. **Check external-dns logs:**
   ```bash
   kubectl logs -n external-dns deployment/external-dns --tail=50
   ```

5. **Verify DNS record creation:**
   ```bash
   # Check with DNS provider CLI or API
   dig test-dns.example.com
   ```

6. **Test DNS propagation:**
   ```bash
   nslookup test-dns.example.com
   ```

**Evidence Artifacts:**
- [ ] `external-dns-pods.yaml` - Pod status
- [ ] `external-dns-config.yaml` - Configuration
- [ ] `dns-records.txt` - Created DNS records
- [ ] `external-dns-logs.txt` - Recent logs

**Acceptance Criteria:**
- ✅ external-dns pods healthy
- ✅ DNS records created automatically
- ✅ DNS propagation successful
- ✅ No permission errors

**Risks & Pitfalls:**
- ⚠️ DNS provider API rate limits
- ⚠️ Incorrect DNS zone configuration
- ⚠️ Stale DNS records not cleaned up

---

### 8.4 Ingress Controller (<INGRESS_CONTROLLER>)

**Purpose:** Validate ingress controller deployment and functionality.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate ingress-controller \
  --cluster <CLUSTER_NAME> \
  --controller <INGRESS_CONTROLLER>
```

**Manual Validation Steps (NGINX Ingress Example):**

1. **Check ingress controller pods:**
   ```bash
   kubectl get pods -n ingress-nginx
   ```

2. **Verify LoadBalancer service:**
   ```bash
   kubectl get svc -n ingress-nginx ingress-nginx-controller
   ```

3. **Check ingress class:**
   ```bash
   kubectl get ingressclass
   ```

4. **Test basic routing:**
   ```bash
   # Already covered in section 7.3
   ```

5. **Verify metrics endpoint:**
   ```bash
   kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 10254:10254
   curl http://localhost:10254/metrics
   ```

6. **Check admission webhook:**
   ```bash
   kubectl get validatingwebhookconfigurations | grep ingress-nginx
   ```

7. **Test rate limiting (if configured):**
   ```bash
   for i in {1..100}; do
     curl -s -o /dev/null -w "%{http_code}\n" -H "Host: echo.test.local" http://$INGRESS_IP/
   done | sort | uniq -c
   ```

**Evidence Artifacts:**
- [ ] `ingress-controller-pods.yaml` - Pod status
- [ ] `ingress-controller-config.yaml` - Configuration
- [ ] `ingress-metrics.txt` - Metrics sample
- [ ] `rate-limit-test.txt` - Rate limiting test

**Acceptance Criteria:**
- ✅ Ingress controller pods healthy
- ✅ LoadBalancer IP assigned
- ✅ Routing functional
- ✅ Metrics exposed
- ✅ Admission webhook working

**Risks & Pitfalls:**
- ⚠️ LoadBalancer IP not assigned
- ⚠️ SSL passthrough misconfigured
- ⚠️ Resource limits too low

---

### 8.5 OpenTelemetry Collectors

**Purpose:** Validate OTel collector deployment and data flow.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate otel \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check OTel collector pods:**
   ```bash
   kubectl get pods -n observability -l app.kubernetes.io/name=opentelemetry-collector
   ```

2. **Verify collector configuration:**
   ```bash
   kubectl get configmap -n observability otel-collector-config -o yaml
   ```

3. **Check collector receivers:**
   ```bash
   kubectl logs -n observability deployment/otel-collector | grep "receiver"
   ```

4. **Test trace ingestion:**
   ```bash
   # Deploy sample app with OTel instrumentation
   kubectl apply -f https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/main/tests/e2e/smoke-tracegen/00-install.yaml
   ```

5. **Verify metrics export:**
   ```bash
   kubectl port-forward -n observability svc/otel-collector 8888:8888
   curl http://localhost:8888/metrics
   ```

6. **Check backend connectivity:**
   ```bash
   kubectl logs -n observability deployment/otel-collector | grep -E "exporter|backend"
   ```

**Evidence Artifacts:**
- [ ] `otel-collector-pods.yaml` - Pod status
- [ ] `otel-collector-config.yaml` - Configuration
- [ ] `otel-metrics.txt` - Collector metrics
- [ ] `otel-logs.txt` - Recent logs

**Acceptance Criteria:**
- ✅ OTel collectors healthy
- ✅ Receivers accepting data
- ✅ Exporters sending to backends
- ✅ No data loss errors

**Risks & Pitfalls:**
- ⚠️ Memory pressure from high cardinality
- ⚠️ Backend authentication failures
- ⚠️ Sampling rate too aggressive

---

### 8.6 Prometheus & Grafana

**Purpose:** Validate metrics collection and visualization.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate monitoring \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check Prometheus pods:**
   ```bash
   kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus
   ```

2. **Verify Prometheus targets:**
   ```bash
   kubectl port-forward -n monitoring svc/prometheus 9090:9090
   # Open http://localhost:9090/targets
   ```

3. **Test Prometheus queries:**
   ```bash
   curl -s 'http://localhost:9090/api/v1/query?query=up' | jq '.data.result | length'
   ```

4. **Check ServiceMonitors:**
   ```bash
   kubectl get servicemonitors --all-namespaces
   ```

5. **Verify Grafana deployment:**
   ```bash
   kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana
   ```

6. **Test Grafana datasource:**
   ```bash
   kubectl port-forward -n monitoring svc/grafana 3000:3000
   # Login and check Prometheus datasource
   ```

7. **Verify alerting rules:**
   ```bash
   kubectl get prometheusrules -n monitoring
   curl -s 'http://localhost:9090/api/v1/rules' | jq '.data.groups[].rules[] | select(.type=="alerting")'
   ```

8. **Check Alertmanager:**
   ```bash
   kubectl get pods -n monitoring -l app.kubernetes.io/name=alertmanager
   kubectl port-forward -n monitoring svc/alertmanager 9093:9093
   ```

**Evidence Artifacts:**
- [ ] `prometheus-targets.json` - Scrape targets
- [ ] `prometheus-rules.yaml` - Alert rules
- [ ] `grafana-dashboards.json` - Dashboard list
- [ ] `alertmanager-config.yaml` - Alert routing

**Acceptance Criteria:**
- ✅ Prometheus scraping all targets
- ✅ No target down errors
- ✅ Grafana datasource connected
- ✅ Alert rules loaded
- ✅ Alertmanager routing configured

**Risks & Pitfalls:**
- ⚠️ High cardinality metrics causing OOM
- ⚠️ Retention period too short
- ⚠️ Alert fatigue from noisy rules

---

### 8.7 Loki (Logging)

**Purpose:** Validate log aggregation and querying.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate logging \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check Loki pods:**
   ```bash
   kubectl get pods -n logging -l app.kubernetes.io/name=loki
   ```

2. **Verify Promtail daemonset:**
   ```bash
   kubectl get daemonset -n logging promtail
   kubectl get pods -n logging -l app.kubernetes.io/name=promtail
   ```

3. **Test log ingestion:**
   ```bash
   kubectl port-forward -n logging svc/loki 3100:3100
   curl -s 'http://localhost:3100/loki/api/v1/labels'
   ```

4. **Query logs:**
   ```bash
   curl -G -s 'http://localhost:3100/loki/api/v1/query_range' \
     --data-urlencode 'query={namespace="kube-system"}' | jq '.data.result | length'
   ```

5. **Verify Grafana Loki datasource:**
   ```bash
   # In Grafana, check Loki datasource and run test query
   ```

6. **Check log retention:**
   ```bash
   kubectl get configmap -n logging loki-config -o yaml | grep retention
   ```

**Evidence Artifacts:**
- [ ] `loki-pods.yaml` - Pod status
- [ ] `promtail-status.yaml` - Log shipper status
- [ ] `loki-labels.json` - Available log labels
- [ ] `loki-config.yaml` - Configuration

**Acceptance Criteria:**
- ✅ Loki pods healthy
- ✅ Promtail running on all nodes
- ✅ Logs being ingested
- ✅ Queries returning results
- ✅ Retention configured

**Risks & Pitfalls:**
- ⚠️ Storage exhaustion from high log volume
- ⚠️ Query performance issues
- ⚠️ Missing logs from certain namespaces

---

### 8.8 Tempo (Tracing)

**Purpose:** Validate distributed tracing backend.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate tracing \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check Tempo pods:**
   ```bash
   kubectl get pods -n observability -l app.kubernetes.io/name=tempo
   ```

2. **Verify Tempo configuration:**
   ```bash
   kubectl get configmap -n observability tempo-config -o yaml
   ```

3. **Test trace ingestion:**
   ```bash
   kubectl port-forward -n observability svc/tempo 3200:3200
   curl -s http://localhost:3200/ready
   ```

4. **Query traces:**
   ```bash
   # Using Grafana Tempo datasource
   kubectl port-forward -n monitoring svc/grafana 3000:3000
   # Navigate to Explore > Tempo
   ```

5. **Check OTel to Tempo flow:**
   ```bash
   kubectl logs -n observability deployment/otel-collector | grep tempo
   ```

6. **Verify storage backend:**
   ```bash
   kubectl get pvc -n observability | grep tempo
   ```

**Evidence Artifacts:**
- [ ] `tempo-pods.yaml` - Pod status
- [ ] `tempo-config.yaml` - Configuration
- [ ] `tempo-traces-sample.json` - Sample traces
- [ ] `tempo-storage.txt` - Storage status

**Acceptance Criteria:**
- ✅ Tempo pods healthy
- ✅ Traces being ingested
- ✅ Queries returning results
- ✅ Storage backend configured
- ✅ Grafana datasource working

**Risks & Pitfalls:**
- ⚠️ Storage growth from trace retention
- ⚠️ Query latency on large trace volumes
- ⚠️ Missing traces due to sampling

---

### 8.9 Mimir/VictoriaMetrics (Long-term Metrics)

**Purpose:** Validate long-term metrics storage.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate metrics-storage \
  --cluster <CLUSTER_NAME> \
  --backend <mimir|victoriametrics>
```

**Manual Validation Steps (Mimir Example):**

1. **Check Mimir components:**
   ```bash
   kubectl get pods -n mimir
   # Expected: distributor, ingester, querier, compactor, store-gateway
   ```

2. **Verify Prometheus remote write:**
   ```bash
   kubectl get secret -n monitoring prometheus-remote-write-config -o yaml
   ```

3. **Test query endpoint:**
   ```bash
   kubectl port-forward -n mimir svc/mimir-query-frontend 8080:8080
   curl -s 'http://localhost:8080/prometheus/api/v1/query?query=up' | jq '.data.result | length'
   ```

4. **Check ingestion rate:**
   ```bash
   curl -s 'http://localhost:8080/prometheus/api/v1/query?query=cortex_ingester_ingested_samples_total' | jq
   ```

5. **Verify compaction:**
   ```bash
   kubectl logs -n mimir deployment/mimir-compactor --tail=50
   ```

6. **Check object storage:**
   ```bash
   # Verify S3/GCS bucket configuration
   kubectl get configmap -n mimir mimir-config -o yaml | grep -A 5 "storage:"
   ```

**Evidence Artifacts:**
- [ ] `mimir-pods.yaml` - Component status
- [ ] `mimir-config.yaml` - Configuration
- [ ] `mimir-metrics.txt` - Ingestion metrics
- [ ] `mimir-storage.txt` - Storage backend info

**Acceptance Criteria:**
- ✅ All Mimir components healthy
- ✅ Prometheus remote write configured
- ✅ Metrics being ingested
- ✅ Queries returning historical data
- ✅ Compaction running
- ✅ Object storage accessible

**Risks & Pitfalls:**
- ⚠️ Object storage costs
- ⚠️ Ingestion rate limits
- ⚠️ Query performance degradation

---

### 8.10 Velero (Backup & Restore)

**Purpose:** Validate cluster backup and restore capability.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate backup \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Check Velero installation:**
   ```bash
   velero version
   kubectl get pods -n velero
   ```

2. **Verify backup storage location:**
   ```bash
   velero backup-location get
   kubectl get backupstoragelocation -n velero
   ```

3. **Create test backup:**
   ```bash
   kubectl create namespace velero-test
   kubectl create deployment nginx --image=nginx -n velero-test
   kubectl create configmap test-config --from-literal=key=value -n velero-test
   
   velero backup create test-backup --include-namespaces velero-test --wait
   ```

4. **Verify backup completion:**
   ```bash
   velero backup describe test-backup
   velero backup logs test-backup
   ```

5. **Test restore:**
   ```bash
   kubectl delete namespace velero-test
   velero restore create --from-backup test-backup --wait
   ```

6. **Verify restore:**
   ```bash
   kubectl get all -n velero-test
   kubectl get configmap -n velero-test test-config
   ```

7. **Check scheduled backups:**
   ```bash
   velero schedule get
   ```

8. **Verify backup retention:**
   ```bash
   velero backup get
   # Check TTL on backups
   ```

**Evidence Artifacts:**
- [ ] `velero-version.txt` - Velero version
- [ ] `backup-locations.yaml` - Storage locations
- [ ] `test-backup-describe.txt` - Backup details
- [ ] `test-restore-log.txt` - Restore log
- [ ] `backup-schedules.yaml` - Scheduled backups

**Acceptance Criteria:**
- ✅ Velero pods healthy
- ✅ Backup storage accessible
- ✅ Test backup successful
- ✅ Test restore successful
- ✅ Scheduled backups configured
- ✅ Retention policy set

**Risks & Pitfalls:**
- ⚠️ Backup storage quota exceeded
- ⚠️ PV snapshots not supported
- ⚠️ Restore conflicts with existing resources

---

## 9. Day-2 Readiness Gates

### 9.1 Backup & Restore Success

**Purpose:** Validate end-to-end backup and restore capability.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate day2-backup \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Full cluster backup:**
   ```bash
   velero backup create full-cluster-backup --wait
   ```

2. **Verify etcd snapshot:**
   ```bash
   ETCDCTL_API=3 etcdctl snapshot save /backup/etcd-snapshot-$(date +%Y%m%d).db
   ETCDCTL_API=3 etcdctl snapshot status /backup/etcd-snapshot-$(date +%Y%m%d).db -w table
   ```

3. **Document restore procedure:**
   ```bash
   cat > disaster-recovery-plan.md <<EOF
   # Disaster Recovery Plan
   
   ## Cluster Restore Procedure
   1. Restore etcd from snapshot
   2. Restore cluster resources with Velero
   3. Verify control plane health
   4. Verify workload health
   5. Verify data integrity
   
   ## RTO: <TARGET_RTO>
   ## RPO: <TARGET_RPO>
   EOF
   ```

4. **Test partial restore:**
   ```bash
   # Delete a namespace and restore it
   kubectl delete namespace <TEST_NAMESPACE>
   velero restore create --from-backup full-cluster-backup \
     --include-namespaces <TEST_NAMESPACE> --wait
   ```

**Evidence Artifacts:**
- [ ] `full-cluster-backup.txt` - Backup details
- [ ] `etcd-snapshot.db` - etcd snapshot
- [ ] `disaster-recovery-plan.md` - DR procedure
- [ ] `restore-test-results.txt` - Restore test results

**Acceptance Criteria:**
- ✅ Full cluster backup successful
- ✅ etcd snapshot created
- ✅ Restore procedure documented
- ✅ Partial restore tested
- ✅ RTO/RPO targets defined

---

### 9.2 GitOps Drift Detection

**Purpose:** Validate Flux detects and remediates configuration drift.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate day2-gitops-drift \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Introduce manual drift:**
   ```bash
   kubectl scale deployment -n <NAMESPACE> <DEPLOYMENT> --replicas=0
   kubectl annotate deployment -n <NAMESPACE> <DEPLOYMENT> manual-change=true
   ```

2. **Monitor Flux reconciliation:**
   ```bash
   flux logs --follow --level=info
   ```

3. **Verify drift remediation:**
   ```bash
   # Wait for reconciliation interval
   sleep 300
   kubectl get deployment -n <NAMESPACE> <DEPLOYMENT>
   # Expected: Replicas restored, annotation removed
   ```

4. **Check reconciliation metrics:**
   ```bash
   kubectl port-forward -n flux-system svc/source-controller 8080:8080
   curl -s http://localhost:8080/metrics | grep gotk_reconcile
   ```

5. **Test Git source update:**
   ```bash
   # Make a change in Git repo
   # Verify Flux picks it up within reconciliation interval
   flux reconcile source git flux-system
   ```

**Evidence Artifacts:**
- [ ] `drift-test-before.yaml` - State before drift
- [ ] `drift-test-after.yaml` - State after remediation
- [ ] `flux-reconciliation-logs.txt` - Flux logs
- [ ] `reconciliation-metrics.txt` - Metrics

**Acceptance Criteria:**
- ✅ Drift detected within reconciliation interval
- ✅ Drift automatically remediated
- ✅ Git changes applied automatically
- ✅ No reconciliation errors

---

### 9.3 Certificate Rotation

**Purpose:** Validate certificate rotation capability.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate day2-cert-rotation \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Document current certificate expiry:**
   ```bash
   kubeadm certs check-expiration > certs-before-rotation.txt
   ```

2. **Perform certificate rotation (on test cluster):**
   ```bash
   # Backup certificates
   sudo cp -r /etc/kubernetes/pki /etc/kubernetes/pki.backup
   
   # Rotate certificates
   sudo kubeadm certs renew all
   ```

3. **Restart control plane components:**
   ```bash
   sudo systemctl restart kubelet
   kubectl delete pod -n kube-system -l component=kube-apiserver
   kubectl delete pod -n kube-system -l component=kube-controller-manager
   kubectl delete pod -n kube-system -l component=kube-scheduler
   ```

4. **Verify cluster functionality:**
   ```bash
   kubectl get nodes
   kubectl get pods --all-namespaces
   ```

5. **Check new certificate expiry:**
   ```bash
   kubeadm certs check-expiration > certs-after-rotation.txt
   ```

6. **Document rotation procedure:**
   ```bash
   cat > cert-rotation-procedure.md <<EOF
   # Certificate Rotation Procedure
   
   ## Prerequisites
   - Backup current certificates
   - Maintenance window scheduled
   - Rollback plan ready
   
   ## Steps
   1. Rotate certificates with kubeadm
   2. Restart control plane components
   3. Verify cluster health
   4. Update kubeconfigs
   
   ## Rollback
   - Restore from backup
   - Restart components
   EOF
   ```

**Evidence Artifacts:**
- [ ] `certs-before-rotation.txt` - Pre-rotation expiry
- [ ] `certs-after-rotation.txt` - Post-rotation expiry
- [ ] `cert-rotation-procedure.md` - Rotation procedure
- [ ] `rotation-test-log.txt` - Test execution log

**Acceptance Criteria:**
- ✅ Rotation procedure documented
- ✅ Test rotation successful (on test cluster)
- ✅ Cluster functional after rotation
- ✅ New certificates valid for 1 year
- ✅ Rollback procedure documented

---

### 9.4 DNS Propagation Validation

**Purpose:** Validate DNS records are properly propagated.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate day2-dns \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **List all external DNS records:**
   ```bash
   kubectl get ingress --all-namespaces -o json | \
     jq -r '.items[].spec.rules[].host' | sort -u > dns-records.txt
   ```

2. **Verify DNS resolution:**
   ```bash
   while read domain; do
     echo "=== $domain ==="
     dig +short $domain
     nslookup $domain
   done < dns-records.txt
   ```

3. **Check DNS propagation globally:**
   ```bash
   # Use external service like whatsmydns.net or dnschecker.org
   for domain in $(cat dns-records.txt); do
     echo "Check: https://dnschecker.org/#A/$domain"
   done
   ```

4. **Verify cert-manager DNS01 challenges:**
   ```bash
   kubectl get challenges --all-namespaces
   kubectl describe challenge <CHALLENGE_NAME>
   ```

5. **Test DNS failover (if configured):**
   ```bash
   # Document DNS failover configuration
   # Test by simulating primary DNS failure
   ```

**Evidence Artifacts:**
- [ ] `dns-records.txt` - All DNS records
- [ ] `dns-resolution-test.txt` - Resolution test results
- [ ] `dns-propagation-check.txt` - Global propagation status

**Acceptance Criteria:**
- ✅ All DNS records resolve correctly
- ✅ Global propagation verified
- ✅ DNS01 challenges working
- ✅ TTL values appropriate

---

### 9.5 Node Failure Simulation

**Purpose:** Validate cluster resilience to node failures.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate day2-node-failure \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Deploy test workload:**
   ```bash
   kubectl create deployment resilience-test --image=nginx --replicas=5
   kubectl expose deployment resilience-test --port=80
   ```

2. **Verify pod distribution:**
   ```bash
   kubectl get pods -l app=resilience-test -o wide
   ```

3. **Simulate worker node failure:**
   ```bash
   # Cordon and drain a worker node
   kubectl cordon <WORKER_NODE>
   kubectl drain <WORKER_NODE> --ignore-daemonsets --delete-emptydir-data
   ```

4. **Monitor pod rescheduling:**
   ```bash
   watch kubectl get pods -l app=resilience-test -o wide
   ```

5. **Verify service availability:**
   ```bash
   kubectl run test-curl --image=curlimages/curl --restart=Never -- \
     curl -s http://resilience-test.default.svc.cluster.local
   ```

6. **Simulate control plane node failure (HA clusters):**
   ```bash
   # Power off one control plane node
   # Verify API server still accessible
   kubectl get nodes
   ```

7. **Restore node:**
   ```bash
   kubectl uncordon <WORKER_NODE>
   ```

8. **Verify cluster recovery:**
   ```bash
   kubectl get nodes
   kubectl get pods --all-namespaces | grep -v Running
   ```

**Evidence Artifacts:**
- [ ] `node-failure-timeline.txt` - Event timeline
- [ ] `pod-rescheduling-log.txt` - Pod movements
- [ ] `service-availability-test.txt` - Service test results
- [ ] `cluster-recovery-status.txt` - Final state

**Acceptance Criteria:**
- ✅ Pods rescheduled within 5 minutes
- ✅ Service remained available
- ✅ No data loss
- ✅ Control plane HA maintained
- ✅ Cluster fully recovered

---

### 9.6 End-to-End Observability Check

**Purpose:** Validate complete observability stack integration.

**Validation Procedure:**

```bash
# CLI Command Proposal
openCenter cluster validate day2-observability \
  --cluster <CLUSTER_NAME>
```

**Manual Validation Steps:**

1. **Deploy instrumented test application:**
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/open-telemetry/opentelemetry-demo/main/kubernetes/opentelemetry-demo.yaml
   ```

2. **Generate test traffic:**
   ```bash
   kubectl port-forward -n otel-demo svc/frontend 8080:8080
   # Generate load with curl or browser
   ```

3. **Verify metrics in Prometheus:**
   ```bash
   # Query for application metrics
   curl -s 'http://prometheus:9090/api/v1/query?query=http_requests_total' | jq
   ```

4. **Verify logs in Loki:**
   ```bash
   # Query for application logs
   curl -G -s 'http://loki:3100/loki/api/v1/query_range' \
     --data-urlencode 'query={namespace="otel-demo"}' | jq
   ```

5. **Verify traces in Tempo:**
   ```bash
   # Search for traces in Grafana
   # Verify trace spans and service graph
   ```

6. **Check Grafana dashboards:**
   ```bash
   # Verify dashboards show:
   # - Application metrics
   # - Application logs
   # - Distributed traces
   # - Service dependencies
   ```

7. **Test alerting:**
   ```bash
   # Trigger a test alert
   kubectl scale deployment -n otel-demo frontend --replicas=0
   # Verify alert fires in Alertmanager
   ```

**Evidence Artifacts:**
- [ ] `observability-metrics.png` - Metrics dashboard screenshot
- [ ] `observability-logs.png` - Logs dashboard screenshot
- [ ] `observability-traces.png` - Traces view screenshot
- [ ] `observability-alerts.txt` - Alert test results

**Acceptance Criteria:**
- ✅ Metrics flowing to Prometheus
- ✅ Logs flowing to Loki
- ✅ Traces flowing to Tempo
- ✅ Grafana dashboards functional
- ✅ Alerts firing correctly
- ✅ End-to-end correlation working

---

## 10. Sign-Off & Acceptance

### 10.1 Handover Checklist Summary

**Cluster Information:**
- Cluster Name: `<CLUSTER_NAME>`
- Kubernetes Version: `<KUBE_VERSION>`
- Kubespray Version: `<KUBESPRAY_VERSION>`
- Node Count: `<NODE_COUNT>` (Control Plane: `<CP_COUNT>`, Workers: `<WORKER_COUNT>`)
- CNI Plugin: `<CNI_PLUGIN>`
- Ingress Controller: `<INGRESS_CONTROLLER>`

**Validation Status:**

| Category | Status | Evidence | Notes |
|----------|--------|----------|-------|
| Kubespray Deployment | ☐ Pass ☐ Fail | | |
| Infrastructure & Topology | ☐ Pass ☐ Fail | | |
| Control Plane & etcd | ☐ Pass ☐ Fail | | |
| Node OS Baseline | ☐ Pass ☐ Fail | | |
| Networking | ☐ Pass ☐ Fail | | |
| Security & Certificates | ☐ Pass ☐ Fail | | |
| Conformance Tests | ☐ Pass ☐ Fail | | |
| Flux (GitOps) | ☐ Pass ☐ Fail | | |
| cert-manager | ☐ Pass ☐ Fail | | |
| external-dns | ☐ Pass ☐ Fail | | |
| Ingress Controller | ☐ Pass ☐ Fail | | |
| OpenTelemetry | ☐ Pass ☐ Fail | | |
| Prometheus & Grafana | ☐ Pass ☐ Fail | | |
| Loki | ☐ Pass ☐ Fail | | |
| Tempo | ☐ Pass ☐ Fail | | |
| Mimir/VictoriaMetrics | ☐ Pass ☐ Fail | | |
| Velero | ☐ Pass ☐ Fail | | |
| Day-2 Backup & Restore | ☐ Pass ☐ Fail | | |
| Day-2 GitOps Drift | ☐ Pass ☐ Fail | | |
| Day-2 Cert Rotation | ☐ Pass ☐ Fail | | |
| Day-2 DNS Propagation | ☐ Pass ☐ Fail | | |
| Day-2 Node Failure | ☐ Pass ☐ Fail | | |
| Day-2 Observability | ☐ Pass ☐ Fail | | |

---

### 10.2 Outstanding Items

**Items Requiring Further Research:**

1. [ ] Item: `<DESCRIPTION>`
   - Impact: `<HIGH|MEDIUM|LOW>`
   - Owner: `<OWNER>`
   - Target Date: `<DATE>`

2. [ ] Item: `<DESCRIPTION>`
   - Impact: `<HIGH|MEDIUM|LOW>`
   - Owner: `<OWNER>`
   - Target Date: `<DATE>`

**Items Requiring Custom Development:**

1. [ ] Item: `<DESCRIPTION>`
   - Justification: `<REASON>`
   - Effort Estimate: `<HOURS/DAYS>`
   - Owner: `<OWNER>`

---

### 10.3 Known Limitations

1. **Limitation:** `<DESCRIPTION>`
   - Impact: `<DESCRIPTION>`
   - Mitigation: `<MITIGATION_STRATEGY>`
   - Acceptance: ☐ Accepted ☐ Requires Resolution

2. **Limitation:** `<DESCRIPTION>`
   - Impact: `<DESCRIPTION>`
   - Mitigation: `<MITIGATION_STRATEGY>`
   - Acceptance: ☐ Accepted ☐ Requires Resolution

---

### 10.4 Operational Runbooks

**Required Runbooks:**

- [ ] Cluster Upgrade Procedure
- [ ] Node Addition/Removal Procedure
- [ ] Certificate Rotation Procedure
- [ ] Disaster Recovery Procedure
- [ ] Backup & Restore Procedure
- [ ] Incident Response Procedure
- [ ] Scaling Procedure
- [ ] Monitoring & Alerting Guide
- [ ] Troubleshooting Guide

**Runbook Location:** `<RUNBOOK_REPOSITORY_URL>`

---

### 10.5 Training & Knowledge Transfer

**Training Sessions Completed:**

- [ ] Cluster Architecture Overview
- [ ] Day-2 Operations Training
- [ ] GitOps Workflow Training
- [ ] Monitoring & Alerting Training
- [ ] Incident Response Training
- [ ] Disaster Recovery Training

**Documentation Provided:**

- [ ] Architecture Diagrams
- [ ] Network Topology Diagrams
- [ ] Runbook Documentation
- [ ] API Documentation
- [ ] Troubleshooting Guides

---

### 10.6 Support & Escalation

**Support Contacts:**

| Role | Name | Email | Phone | Availability |
|------|------|-------|-------|--------------|
| Primary SRE | `<NAME>` | `<EMAIL>` | `<PHONE>` | `<HOURS>` |
| Secondary SRE | `<NAME>` | `<EMAIL>` | `<PHONE>` | `<HOURS>` |
| Platform Lead | `<NAME>` | `<EMAIL>` | `<PHONE>` | `<HOURS>` |
| Vendor Support | `<NAME>` | `<EMAIL>` | `<PHONE>` | `<HOURS>` |

**Escalation Path:**

1. Level 1: On-call SRE
2. Level 2: Platform Lead
3. Level 3: Vendor Support
4. Level 4: Emergency Response Team

---

### 10.7 Final Sign-Off

**Provider Sign-Off:**

I certify that the cluster `<CLUSTER_NAME>` has been deployed according to specifications, all validation checks have been completed, and the cluster is ready for production use.

- Name: `<PROVIDER_NAME>`
- Title: `<PROVIDER_TITLE>`
- Signature: `___________________________`
- Date: `<DATE>`

**Customer Sign-Off:**

I acknowledge receipt of the cluster `<CLUSTER_NAME>`, have reviewed the handover documentation, and accept the cluster for production use.

- Name: `<CUSTOMER_NAME>`
- Title: `<CUSTOMER_TITLE>`
- Signature: `___________________________`
- Date: `<DATE>`

---

## Appendix A: CLI Command Reference

### Proposed CLI Commands for Automation

```bash
# Full validation suite
openCenter cluster validate all --cluster <CLUSTER_NAME>

# Individual validation commands
openCenter cluster validate kubespray-inventory --cluster <CLUSTER_NAME>
openCenter cluster validate kubespray-deployment --cluster <CLUSTER_NAME>
openCenter cluster validate kubespray-reproducibility --cluster <CLUSTER_NAME>
openCenter cluster validate infrastructure --cluster <CLUSTER_NAME>
openCenter cluster validate etcd --cluster <CLUSTER_NAME>
openCenter cluster validate control-plane --cluster <CLUSTER_NAME>
openCenter cluster validate node-os --cluster <CLUSTER_NAME>
openCenter cluster validate container-runtime --cluster <CLUSTER_NAME>
openCenter cluster validate networking --cluster <CLUSTER_NAME>
openCenter cluster validate dns --cluster <CLUSTER_NAME>
openCenter cluster validate certificates --cluster <CLUSTER_NAME>
openCenter cluster validate security --cluster <CLUSTER_NAME>
openCenter cluster validate secrets --cluster <CLUSTER_NAME>
openCenter cluster validate conformance --cluster <CLUSTER_NAME>
openCenter cluster validate kubespray-health --cluster <CLUSTER_NAME>
openCenter cluster validate ingress --cluster <CLUSTER_NAME>
openCenter cluster validate flux --cluster <CLUSTER_NAME>
openCenter cluster validate cert-manager --cluster <CLUSTER_NAME>
openCenter cluster validate external-dns --cluster <CLUSTER_NAME>
openCenter cluster validate ingress-controller --cluster <CLUSTER_NAME>
openCenter cluster validate otel --cluster <CLUSTER_NAME>
openCenter cluster validate monitoring --cluster <CLUSTER_NAME>
openCenter cluster validate logging --cluster <CLUSTER_NAME>
openCenter cluster validate tracing --cluster <CLUSTER_NAME>
openCenter cluster validate metrics-storage --cluster <CLUSTER_NAME>
openCenter cluster validate backup --cluster <CLUSTER_NAME>
openCenter cluster validate day2-backup --cluster <CLUSTER_NAME>
openCenter cluster validate day2-gitops-drift --cluster <CLUSTER_NAME>
openCenter cluster validate day2-cert-rotation --cluster <CLUSTER_NAME>
openCenter cluster validate day2-dns --cluster <CLUSTER_NAME>
openCenter cluster validate day2-node-failure --cluster <CLUSTER_NAME>
openCenter cluster validate day2-observability --cluster <CLUSTER_NAME>

# Generate handover report
openCenter cluster handover-report --cluster <CLUSTER_NAME> --output handover-report.pdf
```

---

## Appendix B: Evidence Artifact Checklist

All evidence artifacts should be collected and archived in a structured format:

```
handover-artifacts/
├── 01-kubespray-deployment/
│   ├── inventory-structure.txt
│   ├── host-groups.json
│   ├── ansible-playbook.log
│   ├── idempotency-check.log
│   └── reproducibility-manifest.yaml
├── 02-infrastructure/
│   ├── nodes-list.txt
│   ├── node-labels.yaml
│   └── node-capacity.json
├── 03-control-plane-etcd/
│   ├── etcd-health.txt
│   ├── etcd-snapshot.db
│   ├── apiserver-readyz.txt
│   └── control-plane-pods.yaml
├── 04-node-os/
│   ├── os-versions.txt
│   ├── kernel-versions.txt
│   └── containerd-config.toml
├── 05-networking/
│   ├── cni-pods.yaml
│   ├── connectivity-test.log
│   └── dns-resolution-test.txt
├── 06-security/
│   ├── cert-expiration.txt
│   ├── rbac-audit.yaml
│   └── encryption-config.yaml
├── 07-conformance/
│   ├── sonobuoy-results.tar.gz
│   ├── conformance-summary.txt
│   └── cluster-health.log
├── 08-managed-services/
│   ├── flux-check.txt
│   ├── cert-manager-pods.yaml
│   ├── prometheus-targets.json
│   └── velero-backup-test.txt
├── 09-day2-readiness/
│   ├── full-cluster-backup.txt
│   ├── drift-test-results.txt
│   ├── cert-rotation-procedure.md
│   └── node-failure-timeline.txt
└── 10-documentation/
    ├── architecture-diagrams/
    ├── runbooks/
    └── handover-checklist.md
```

---

## Appendix C: Automation Implementation Notes

### CLI Command Implementation Strategy

1. **Command Structure:**
   - Use Cobra for CLI framework (already in use)
   - Add `cluster validate` subcommand group
   - Each validation as a separate subcommand

2. **Validation Framework:**
   ```go
   type Validator interface {
       Name() string
       Validate(ctx context.Context, cluster string) (*ValidationResult, error)
   }
   
   type ValidationResult struct {
       Status      ValidationStatus  // Pass, Fail, Warning
       Evidence    []Artifact
       Errors      []error
       Suggestions []string
   }
   ```

3. **Evidence Collection:**
   - Automatically collect artifacts during validation
   - Store in structured directory format
   - Generate checksums for integrity

4. **Report Generation:**
   - Support multiple output formats (Markdown, PDF, JSON)
   - Include all evidence artifacts
   - Generate executive summary

5. **Integration Points:**
   - kubectl for Kubernetes API access
   - Ansible for node-level checks
   - etcdctl for etcd validation
   - Flux CLI for GitOps checks
   - Velero CLI for backup validation

---

## Document Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-11-19 | `<AUTHOR>` | Initial version |

---

**End of Handover Checklist**
