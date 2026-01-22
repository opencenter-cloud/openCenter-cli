# Upgrading Kubernetes Clusters


## Table of Contents

- [Task Summary](#task-summary)
- [Prerequisites](#prerequisites)
- [Check Version Compatibility](#check-version-compatibility)
- [Plan Rollback Procedure](#plan-rollback-procedure)
- [Update Configuration](#update-configuration)
- [Regenerate GitOps Repository](#regenerate-gitops-repository)
- [Monitor FluxCD Reconciliation](#monitor-fluxcd-reconciliation)
- [Verify Cluster Health](#verify-cluster-health)
- [Component Upgrade Order](#component-upgrade-order)
- [Rollback Procedure](#rollback-procedure)
- [Post-Upgrade Verification](#post-upgrade-verification)
- [Troubleshooting](#troubleshooting)
- [Related Documentation](#related-documentation)
- [See Also](#see-also)
**doc_type: how-to**

This guide walks you through upgrading a Kubernetes cluster's version without downtime. It covers planning, execution, verification, and rollback procedures.

## Task Summary

Upgrade the Kubernetes version of an existing cluster by updating the configuration, regenerating the GitOps repository, and letting FluxCD apply the changes. The upgrade happens in a rolling fashion: control plane nodes first, then worker nodes.

## Prerequisites

- Active cluster with working FluxCD reconciliation
- Backup completed (see [Backup and Restore](backup-restore.md))
- Maintenance window scheduled (optional but recommended)
- Access to cluster configuration file
- Git repository access for committing changes
- `kubectl` access to monitor the upgrade

## Check Version Compatibility

Before upgrading, verify the target version is compatible with your current version.

### Kubernetes Version Skew Policy

Kubernetes supports version skew between components:

- Control plane components can be one minor version apart
- Kubelet can be up to two minor versions behind the API server
- kubectl can be one minor version ahead or behind the API server

**Safe upgrade path**: Increment one minor version at a time. Don't skip versions.

**Example**:
- Current: 1.29.4
- Safe target: 1.30.x (any patch version)
- Unsafe target: 1.31.x (skips 1.30)

### Check Current Version

```bash
kubectl version --short
```

Look at the server version. If you're running 1.29.4, you can upgrade to any 1.30.x release.

### Review Release Notes

Read the Kubernetes release notes for your target version:

```
https://kubernetes.io/docs/setup/release/notes/
```

Look for:
- Deprecated APIs that your workloads might use
- Feature gate changes
- Breaking changes in core components
- Known issues or regressions

Pay attention to API deprecations. If your manifests use deprecated APIs, update them before upgrading. The API server will reject deprecated APIs after the deprecation period ends.

### Test in Non-Production

If you have a staging or development cluster, upgrade it first. Run your application test suite against the upgraded cluster. Watch for:

- API compatibility issues
- Performance regressions
- Unexpected behavior in core components
- Add-on compatibility (CNI, CSI, monitoring)

## Plan Rollback Procedure

Before starting the upgrade, document how to roll back if something goes wrong.

### Git Rollback

The simplest rollback is reverting the Git commit:

```bash
git revert HEAD
git push
```

FluxCD will detect the revert and roll back the cluster configuration. This works if the upgrade hasn't progressed far or if the issue is in the configuration, not the Kubernetes version itself.

### Manual Rollback

If Git revert doesn't work (for example, if nodes are stuck in an upgrade loop), you need manual intervention:

1. Suspend FluxCD reconciliation:
   ```bash
   flux suspend kustomization flux-system
   ```

2. Manually downgrade control plane nodes using your provider's tools (OpenStack, AWS, etc.)

3. Manually downgrade worker nodes

4. Resume FluxCD:
   ```bash
   flux resume kustomization flux-system
   ```

Document the manual rollback steps specific to your infrastructure provider before starting the upgrade.

## Update Configuration

Edit your cluster configuration file to change the Kubernetes version.

### Locate Configuration File

Find your cluster configuration:

```bash
openCenter cluster list
```

The configuration file is typically at:
```
~/.config/openCenter/clusters/<organization>/.cluster-name-config.yaml
```

### Edit Kubernetes Version

Open the configuration file and update the version field:

```yaml
opencenter:
  cluster:
    kubernetes:
      version: 1.31.4  # Change this to your target version
```

Use the full semantic version (major.minor.patch). Don't use version ranges or wildcards.

### Update Component Versions

Some components have version dependencies on Kubernetes. Check if you need to update:

- **Kubespray version**: The `kubespray_version` field must be compatible with your target Kubernetes version. Check the Kubespray compatibility matrix.
- **Network plugin**: Calico, Cilium, and Kube-OVN have minimum Kubernetes version requirements. Verify compatibility in their release notes.
- **CSI drivers**: OpenStack CSI and vSphere CSI may need updates for newer Kubernetes versions.

Example with Kubespray update:

```yaml
opencenter:
  cluster:
    kubernetes:
      version: 1.31.4
      kubespray_version: v2.25.0  # Updated for 1.31 compatibility
```

### Validate Configuration

Run validation before regenerating the repository:

```bash
openCenter cluster validate cluster-name
```

Fix any validation errors before proceeding. Common issues:

- Invalid version format (must be major.minor.patch)
- Incompatible component versions
- Missing required fields

## Regenerate GitOps Repository

Generate updated manifests from the new configuration.

### Render Command

```bash
openCenter cluster render cluster-name
```

This updates the GitOps repository with new manifests reflecting the version change. The command:

1. Loads your updated configuration
2. Renders templates with the new version
3. Writes updated files to the GitOps repository
4. Preserves your manual customizations (if any)

### Review Changes

Before committing, review what changed:

```bash
cd /path/to/gitops-repo
git diff
```

Look for:

- Updated Kubernetes version in infrastructure files (main.tf, inventory files)
- Changed container image tags for system components
- Modified configuration for version-specific features
- Unexpected changes (these might indicate a problem)

The diff should be focused on version-related changes. If you see unrelated changes, investigate before committing.

### Commit Changes

Commit the updated repository:

```bash
git add .
git commit -m "Upgrade Kubernetes to 1.31.4"
git push
```

Use a clear commit message that includes the target version. This makes rollback easier if you need to revert.

## Monitor FluxCD Reconciliation

FluxCD will detect the new commit and start applying changes.

### Watch Reconciliation Status

```bash
flux get kustomizations --watch
```

This shows the reconciliation status for all Kustomizations. Look for:

- **Ready**: Reconciliation succeeded
- **Progressing**: Reconciliation in progress
- **Failed**: Reconciliation failed (check logs)

Reconciliation can take 5-15 minutes depending on cluster size and the number of components being updated.

### Check for Errors

If a Kustomization shows "Failed", check the logs:

```bash
flux logs --kind=Kustomization --name=<kustomization-name>
```

Common errors:

- API version not available (deprecated API used in manifests)
- Resource conflicts (manual changes interfering with FluxCD)
- Timeout (large changes taking longer than the reconciliation timeout)

### Manual Reconciliation

If you don't want to wait for the automatic reconciliation interval, trigger a manual sync:

```bash
flux reconcile source git flux-system
flux reconcile kustomization flux-system
```

This forces FluxCD to check for new commits immediately.

## Verify Cluster Health

After FluxCD completes reconciliation, verify the cluster is healthy.

### Check Node Versions

```bash
kubectl get nodes -o wide
```

All nodes should show the new Kubernetes version in the VERSION column. If some nodes are still on the old version, the upgrade is still in progress or has stalled.

### Check Control Plane Components

```bash
kubectl get pods -n kube-system
```

All control plane pods should be running:

- kube-apiserver
- kube-controller-manager
- kube-scheduler
- etcd

If any are in CrashLoopBackOff or Error state, check their logs:

```bash
kubectl logs -n kube-system <pod-name>
```

### Check Pod Health

```bash
kubectl get pods --all-namespaces
```

Look for pods that are not Running or Completed. Investigate any pods in:

- CrashLoopBackOff
- Error
- ImagePullBackOff
- Pending (for more than a few minutes)

### Check Service Availability

Test critical services:

```bash
kubectl get svc --all-namespaces
```

Verify that services have endpoints:

```bash
kubectl get endpoints <service-name> -n <namespace>
```

If a service has no endpoints, its pods might not be ready.

### Run Application Tests

Run your application's health checks or smoke tests. Verify that:

- Applications respond to requests
- Database connections work
- External integrations function
- Performance is acceptable

## Component Upgrade Order

Kubernetes upgrades happen in a specific order to maintain cluster stability.

### Control Plane First

The control plane components upgrade before worker nodes:

1. **etcd**: Upgraded first (if managed by Kubespray)
2. **kube-apiserver**: Upgraded next
3. **kube-controller-manager**: Upgraded after API server
4. **kube-scheduler**: Upgraded after controller manager

During control plane upgrade, the API server may be briefly unavailable. Existing workloads continue running, but you can't make API calls (kubectl commands will fail).

### Worker Nodes Rolling Update

Worker nodes upgrade one at a time (or in small batches):

1. Node is cordoned (no new pods scheduled)
2. Pods are drained (evicted gracefully)
3. Node is upgraded
4. Node is uncordoned (ready for new pods)

This process repeats for each worker node. Workloads with multiple replicas experience no downtime because pods move to other nodes during the drain.

### Add-ons and Services Last

After all nodes are upgraded, add-ons and services update:

- CNI plugin (Calico, Cilium, Kube-OVN)
- CSI drivers (OpenStack CSI, vSphere CSI)
- Monitoring (Prometheus, Grafana)
- Ingress controllers
- Service mesh components

These updates happen through FluxCD reconciliation. Watch the Kustomization status to track progress.

## Rollback Procedure

If the upgrade fails or causes problems, roll back to the previous version.

### Git Revert (Preferred)

Revert the upgrade commit:

```bash
cd /path/to/gitops-repo
git revert HEAD
git push
```

FluxCD will detect the revert and roll back the configuration. This works if:

- The cluster is still responsive
- FluxCD is still running
- The issue is in the configuration, not the Kubernetes version

Wait for FluxCD to reconcile (5-10 minutes) and verify nodes return to the previous version:

```bash
kubectl get nodes -o wide
```

### Manual Intervention

If Git revert doesn't work, you need manual intervention.

#### Suspend FluxCD

Stop FluxCD from making changes:

```bash
flux suspend kustomization flux-system
```

This prevents FluxCD from interfering with manual rollback steps.

#### Downgrade Control Plane

Use your infrastructure provider's tools to downgrade control plane nodes. For OpenStack with Kubespray:

1. SSH to each control plane node
2. Run the Kubespray rollback playbook (if available)
3. Or manually downgrade kubelet, kubeadm, and kubectl packages

For managed Kubernetes (EKS, GKE, AKS), use the provider's rollback mechanism.

#### Downgrade Worker Nodes

Downgrade worker nodes one at a time:

1. Cordon the node:
   ```bash
   kubectl cordon <node-name>
   ```

2. Drain the node:
   ```bash
   kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
   ```

3. Downgrade the node (SSH and run rollback steps)

4. Uncordon the node:
   ```bash
   kubectl uncordon <node-name>
   ```

Repeat for each worker node.

#### Resume FluxCD

After manual rollback completes, resume FluxCD:

```bash
flux resume kustomization flux-system
```

Update the configuration file to match the rolled-back version and commit to Git. This ensures FluxCD doesn't try to upgrade again.

## Post-Upgrade Verification

After the upgrade completes successfully, perform thorough verification.

### Node Versions

All nodes should show the target version:

```bash
kubectl get nodes -o wide
```

If any node shows the old version, investigate why it didn't upgrade.

### Pod Health

All pods should be running:

```bash
kubectl get pods --all-namespaces | grep -v Running | grep -v Completed
```

This shows pods that are not in a healthy state. Investigate each one.

### Service Availability

Test critical services:

- API server responsiveness (kubectl commands should be fast)
- Ingress controller (external traffic routing)
- DNS resolution (CoreDNS pods healthy)
- Storage provisioning (create a test PVC)

### Application Functionality

Run your application's test suite or manual smoke tests. Verify:

- User-facing features work
- Background jobs run
- Database queries succeed
- External API calls work

### Performance Baseline

Compare performance metrics to pre-upgrade baselines:

- API server latency (check Prometheus metrics)
- Pod startup time
- Network throughput
- Storage I/O

Significant regressions might indicate a problem with the new version or configuration.

### Audit Logs

Check Kubernetes audit logs for unexpected errors or warnings:

```bash
kubectl logs -n kube-system <kube-apiserver-pod> | grep -i error
```

Look for:

- Deprecated API usage warnings
- Authentication failures
- Authorization denials
- Resource conflicts

## Troubleshooting

Common issues and solutions.

### Failed Upgrades

**Symptom**: Nodes stuck in "NotReady" state after upgrade.

**Cause**: Kubelet failed to start with the new version.

**Solution**:
1. SSH to the affected node
2. Check kubelet logs: `journalctl -u kubelet -n 100`
3. Look for configuration errors or missing dependencies
4. Fix the issue and restart kubelet: `systemctl restart kubelet`

### Stuck Nodes

**Symptom**: Some nodes remain on the old version after upgrade.

**Cause**: Upgrade process didn't reach those nodes, or upgrade failed silently.

**Solution**:
1. Check if the node is cordoned: `kubectl get nodes`
2. If cordoned, uncordon it: `kubectl uncordon <node-name>`
3. Manually trigger upgrade on the node (provider-specific)
4. Or drain, upgrade, and uncordon manually

### Version Skew

**Symptom**: Cluster components show different versions.

**Cause**: Partial upgrade or rollback left components out of sync.

**Solution**:
1. Identify which components are out of sync: `kubectl version` and `kubectl get nodes`
2. If control plane is newer than workers, wait for worker upgrade to complete
3. If workers are newer than control plane (shouldn't happen), roll back workers
4. If components within control plane are mismatched, restart the affected pods

### API Compatibility Issues

**Symptom**: Applications fail with "API version not found" errors.

**Cause**: Application uses deprecated APIs removed in the new Kubernetes version.

**Solution**:
1. Identify the deprecated API from the error message
2. Update application manifests to use the new API version
3. Redeploy the application
4. Or roll back the cluster if updating manifests is not feasible immediately

### Performance Degradation

**Symptom**: Cluster is slower after upgrade.

**Cause**: New version has different performance characteristics or configuration needs tuning.

**Solution**:
1. Check resource usage: `kubectl top nodes` and `kubectl top pods`
2. Look for resource-constrained components
3. Review Kubernetes release notes for performance-related changes
4. Adjust resource limits or configuration as needed
5. Or roll back if performance is unacceptable

### FluxCD Reconciliation Failures

**Symptom**: FluxCD shows "Failed" status for Kustomizations.

**Cause**: Manifests incompatible with new Kubernetes version, or resource conflicts.

**Solution**:
1. Check FluxCD logs: `flux logs`
2. Identify the failing resource
3. Fix the manifest or remove the resource
4. Commit the fix to Git
5. Or suspend the failing Kustomization: `flux suspend kustomization <name>`

## Related Documentation

- [How-To: Backup and Restore](backup-restore.md) - Create backups before upgrading
- [How-To: Update Cluster Configuration](update-cluster-config.md) - General configuration update workflow
- [Explanation: GitOps Workflow](../explanation/gitops-workflow.md) - How changes flow through Git
- [Reference: Configuration](../reference/configuration.md) - Configuration file structure

## See Also

- [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/)
- [Kubespray Compatibility Matrix](https://github.com/kubernetes-sigs/kubespray#supported-components)
- [FluxCD Troubleshooting](https://fluxcd.io/flux/cheatsheets/troubleshooting/)
