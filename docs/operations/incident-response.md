# Incident Response


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Incident Classification](#incident-classification)
- [Incident Response Workflow](#incident-response-workflow)
- [Common Incident Types](#common-incident-types)
- [Escalation Procedures](#escalation-procedures)
- [Communication Guidelines](#communication-guidelines)
- [Post-Incident Review](#post-incident-review)
- [Incident Response Checklist](#incident-response-checklist)
- [Related Documentation](#related-documentation)
- [Emergency Procedures](#emergency-procedures)
- [Training and Preparedness](#training-and-preparedness)
**doc_type: how-to**

Procedures for responding to and resolving incidents in openCenter-managed Kubernetes clusters. This guide covers incident classification, response procedures, escalation paths, and post-incident reviews.

## Who This Is For

On-call engineers, SREs, and operations teams responsible for incident response and resolution. Use this guide when responding to alerts, outages, or degraded service conditions.

## Prerequisites

- On-call access to cluster and monitoring systems
- `kubectl` access with appropriate RBAC permissions
- Access to logging and monitoring dashboards
- Incident management system credentials
- Escalation contact list

## Incident Classification

### Severity Levels

**SEV1 - Critical**
- Complete service outage affecting all users
- Data loss or corruption
- Security breach or compromise
- Control plane failure

**Response Time**: Immediate  
**Escalation**: Immediate to senior engineer and management  
**Communication**: Every 30 minutes

**SEV2 - High**
- Partial service outage affecting subset of users
- Significant performance degradation
- Failed node or component
- Certificate expiration imminent

**Response Time**: Within 15 minutes  
**Escalation**: Within 30 minutes if not resolved  
**Communication**: Every hour

**SEV3 - Medium**
- Minor service degradation
- Non-critical component failure
- Resource capacity warnings
- Elevated error rates

**Response Time**: Within 1 hour  
**Escalation**: Within 4 hours if not resolved  
**Communication**: Daily updates

**SEV4 - Low**
- Informational alerts
- Planned maintenance needed
- Documentation issues
- Non-urgent improvements

**Response Time**: Next business day  
**Escalation**: Not required  
**Communication**: As needed

## Incident Response Workflow

### 1. Acknowledge and Assess

```bash
# Acknowledge alert in PagerDuty/monitoring system
# Document incident start time

# Quick cluster health check
kubectl get nodes
kubectl get pods -A | grep -v "Running\|Completed"
kubectl top nodes

# Check recent events
kubectl get events -A --sort-by='.lastTimestamp' | tail -20

# Review monitoring dashboards
# - Grafana: https://grafana.cluster.example.com
# - Prometheus: https://prometheus.cluster.example.com
```

### 2. Classify Severity

Determine incident severity based on impact:

```bash
# Check service availability
curl -I https://app.example.com/health

# Check affected users/services
kubectl get pods -A -o wide | grep -v "Running"

# Review error rates in logs
kubectl logs -n production deployment/app --tail=100 | grep ERROR
```

### 3. Initiate Response

Create incident ticket and notify stakeholders:

```markdown
**Incident**: [Brief Description]
**Severity**: SEV[1-4]
**Start Time**: [Timestamp]
**Affected Services**: [List]
**Impact**: [User/Business Impact]
**Responder**: [Your Name]
**Status**: Investigating
```

### 4. Investigate and Diagnose

Follow diagnostic procedures based on incident type (see sections below).

### 5. Implement Fix

Apply remediation steps, document all actions taken.

### 6. Verify Resolution

Confirm service restoration and monitor for recurrence.

### 7. Post-Incident Review

Schedule and conduct post-incident review within 48 hours.

## Common Incident Types

### Control Plane Failure

**Symptoms**:
- kubectl commands timeout or fail
- API server unreachable
- etcd cluster unhealthy

**Diagnosis**:

```bash
# Check control plane pod status
kubectl get pods -n kube-system | grep -E "apiserver|controller|scheduler|etcd"

# Check control plane node status
kubectl get nodes -l node-role.kubernetes.io/control-plane

# SSH to control plane node
ssh -i ~/.config/openCenter/secrets/ssh/cluster-key ubuntu@control-plane-1

# Check kubelet status
sudo systemctl status kubelet

# Check API server logs
sudo journalctl -u kube-apiserver -n 100

# Check etcd health
sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health
```

**Resolution**:

```bash
# Restart kubelet if unhealthy
sudo systemctl restart kubelet

# Restart API server pod (if accessible)
kubectl delete pod -n kube-system kube-apiserver-control-plane-1

# If etcd is unhealthy, restore from backup
# See: docs/operations/disaster-recovery.md

# Verify control plane recovery
kubectl get componentstatuses
kubectl cluster-info
```

**Escalation**: If control plane doesn't recover within 15 minutes, escalate to SEV1.

### Node Failure

**Symptoms**:
- Node shows NotReady status
- Pods on node are Pending or Unknown
- Node unreachable via SSH

**Diagnosis**:

```bash
# Check node status
kubectl get nodes
kubectl describe node <node-name>

# Check node conditions
kubectl get node <node-name> -o json | jq '.status.conditions'

# Try SSH to node
ssh -i ~/.config/openCenter/secrets/ssh/cluster-key ubuntu@<node-ip>

# If SSH works, check kubelet
sudo systemctl status kubelet
sudo journalctl -u kubelet -n 100

# Check node resources
kubectl top node <node-name>
```

**Resolution**:

```bash
# If kubelet is down, restart it
ssh ubuntu@<node-ip>
sudo systemctl restart kubelet

# If node is unrecoverable, drain and replace
kubectl drain <node-name> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --force \
  --timeout=300s

# Follow node replacement runbook
# See: docs/operations/runbooks/node-replacement.md
```

**Escalation**: If multiple nodes fail simultaneously, escalate to SEV1.

### Pod CrashLoopBackOff

**Symptoms**:
- Pods repeatedly restarting
- Application unavailable or degraded
- High restart count

**Diagnosis**:

```bash
# Identify crashing pods
kubectl get pods -A | grep CrashLoopBackOff

# Check pod logs
kubectl logs -n <namespace> <pod-name> --previous
kubectl logs -n <namespace> <pod-name>

# Check pod events
kubectl describe pod -n <namespace> <pod-name>

# Check resource limits
kubectl get pod -n <namespace> <pod-name> -o json | \
  jq '.spec.containers[].resources'

# Check liveness/readiness probes
kubectl get pod -n <namespace> <pod-name> -o json | \
  jq '.spec.containers[] | {livenessProbe, readinessProbe}'
```

**Resolution**:

```bash
# If configuration issue, fix and redeploy
kubectl apply -f fixed-deployment.yaml

# If resource issue, increase limits
kubectl set resources deployment -n <namespace> <deployment> \
  --limits=cpu=2,memory=2Gi \
  --requests=cpu=1,memory=1Gi

# If image issue, rollback to previous version
kubectl rollout undo deployment -n <namespace> <deployment>

# Verify resolution
kubectl rollout status deployment -n <namespace> <deployment>
```

### Storage Issues

**Symptoms**:
- PVCs stuck in Pending
- Pods can't mount volumes
- Storage full errors

**Diagnosis**:

```bash
# Check PVC status
kubectl get pvc -A

# Check PV status
kubectl get pv

# Describe problematic PVC
kubectl describe pvc -n <namespace> <pvc-name>

# Check storage class
kubectl get storageclass

# Check CSI driver pods
kubectl get pods -n kube-system | grep csi

# Check volume usage
kubectl exec -n <namespace> <pod-name> -- df -h
```

**Resolution**:

```bash
# If PVC pending due to no available PV, create PV or expand storage

# If volume full, expand PVC
kubectl patch pvc -n <namespace> <pvc-name> \
  -p '{"spec":{"resources":{"requests":{"storage":"200Gi"}}}}'

# If CSI driver issue, restart driver pods
kubectl delete pod -n kube-system -l app=csi-cinder-controllerplugin

# Verify resolution
kubectl get pvc -n <namespace> <pvc-name> -w
```

### Network Connectivity Issues

**Symptoms**:
- Pods can't reach services
- DNS resolution failures
- Intermittent connection timeouts

**Diagnosis**:

```bash
# Test DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default

# Test service connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- /bin/bash
# Inside pod:
curl http://service-name.namespace.svc.cluster.local

# Check CoreDNS pods
kubectl get pods -n kube-system -l k8s-app=kube-dns

# Check CNI plugin pods
kubectl get pods -n kube-system | grep calico

# Check network policies
kubectl get networkpolicies -A

# Check service endpoints
kubectl get endpoints -n <namespace> <service-name>
```

**Resolution**:

```bash
# If CoreDNS issue, restart pods
kubectl delete pod -n kube-system -l k8s-app=kube-dns

# If CNI issue, restart CNI pods
kubectl delete pod -n kube-system -l k8s-app=calico-node

# If network policy blocking, review and update
kubectl describe networkpolicy -n <namespace> <policy-name>

# Verify resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default
```

### Certificate Expiration

**Symptoms**:
- TLS handshake failures
- kubectl authentication errors
- Webhook failures

**Diagnosis**:

```bash
# Check certificate expiration
kubectl get certificates -A

# Check specific certificate
kubectl describe certificate -n <namespace> <cert-name>

# Check certificate dates
kubectl get secret -n <namespace> <cert-secret> -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -dates

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager
```

**Resolution**:

```bash
# Force certificate renewal
kubectl delete certificate -n <namespace> <cert-name>
# cert-manager will automatically recreate

# If cert-manager issue, restart
kubectl rollout restart deployment -n cert-manager cert-manager

# Follow certificate renewal runbook
# See: docs/operations/runbooks/certificate-renewal.md

# Verify new certificate
kubectl get certificate -n <namespace> <cert-name>
```

### High Resource Usage

**Symptoms**:
- Nodes at high CPU/memory utilization
- Pods being evicted
- Performance degradation

**Diagnosis**:

```bash
# Check node resource usage
kubectl top nodes

# Check pod resource usage
kubectl top pods -A --sort-by=cpu
kubectl top pods -A --sort-by=memory

# Check for resource-intensive pods
kubectl get pods -A -o json | jq -r '.items[] | 
  select(.status.phase=="Running") | 
  {namespace: .metadata.namespace, name: .metadata.name, 
   cpu: .spec.containers[].resources.requests.cpu,
   memory: .spec.containers[].resources.requests.memory}'

# Check for evicted pods
kubectl get pods -A --field-selector=status.phase=Failed | grep Evicted
```

**Resolution**:

```bash
# Scale down non-critical workloads
kubectl scale deployment -n <namespace> <deployment> --replicas=1

# Add more nodes if capacity issue
# Update cluster configuration and apply

# Right-size pod resources
kubectl set resources deployment -n <namespace> <deployment> \
  --limits=cpu=1,memory=1Gi \
  --requests=cpu=500m,memory=512Mi

# Implement horizontal pod autoscaling
kubectl autoscale deployment -n <namespace> <deployment> \
  --cpu-percent=70 --min=2 --max=10
```

## Escalation Procedures

### When to Escalate

Escalate when:
- Incident severity is SEV1 or SEV2
- Unable to diagnose root cause within 30 minutes
- Fix requires expertise beyond your level
- Multiple systems affected
- Security incident suspected

### Escalation Contacts

**Platform Team**:
- Primary: On-call engineer (PagerDuty)
- Secondary: Platform team lead
- Slack: #platform-oncall

**Security Team**:
- Email: security@example.com
- Slack: #security-incidents
- Phone: [Emergency Security Hotline]

**Management**:
- SEV1: Notify VP Engineering immediately
- SEV2: Notify Engineering Manager within 30 minutes

### Escalation Template

```markdown
**Escalation Request**

**Incident ID**: INC-2026-001
**Severity**: SEV2
**Duration**: 45 minutes
**Current Status**: Investigating

**Summary**: Control plane node unresponsive, API server intermittently unavailable

**Actions Taken**:
- Restarted kubelet on affected node
- Checked etcd health (healthy)
- Reviewed API server logs (no obvious errors)

**Current Impact**: 
- 30% of API requests failing
- kubectl commands intermittently timeout
- User-facing services degraded

**Assistance Needed**: 
- Deep dive into API server performance
- Possible etcd performance issue
- May need to failover control plane

**Responder**: John Doe
**Contact**: john.doe@example.com, Slack: @johndoe
```

## Communication Guidelines

### Status Updates

Provide regular updates based on severity:

- **SEV1**: Every 30 minutes
- **SEV2**: Every hour
- **SEV3**: Every 4 hours or daily
- **SEV4**: As needed

### Update Template

```markdown
**Incident Update** - [Timestamp]

**Status**: [Investigating | Identified | Monitoring | Resolved]

**Summary**: [Brief description of current situation]

**Impact**: [Current user/service impact]

**Actions**: [What has been done since last update]

**Next Steps**: [What will be done next]

**ETA**: [Estimated time to resolution, if known]
```

### Communication Channels

- **Internal**: Slack #incidents channel
- **External**: Status page (status.example.com)
- **Stakeholders**: Email to affected teams
- **Management**: Direct message for SEV1/SEV2

## Post-Incident Review

### Schedule Review

Schedule post-incident review within 48 hours of resolution:

- **SEV1**: Within 24 hours
- **SEV2**: Within 48 hours
- **SEV3**: Within 1 week
- **SEV4**: Optional

### Review Agenda

1. **Incident Timeline**
   - When was incident detected?
   - When was it acknowledged?
   - When was root cause identified?
   - When was it resolved?

2. **Root Cause Analysis**
   - What was the root cause?
   - Why did it happen?
   - What were contributing factors?

3. **Response Evaluation**
   - What went well?
   - What could be improved?
   - Were runbooks helpful?
   - Was escalation appropriate?

4. **Action Items**
   - Preventive measures
   - Monitoring improvements
   - Documentation updates
   - Training needs

### Post-Incident Report Template

```markdown
# Post-Incident Review: [Incident Title]

**Incident ID**: INC-2026-001
**Date**: 2026-01-19
**Severity**: SEV2
**Duration**: 2 hours 15 minutes
**Responders**: John Doe, Jane Smith

## Executive Summary

[Brief description of incident, impact, and resolution]

## Timeline

| Time | Event |
|------|-------|
| 14:00 | Alert triggered: API server high latency |
| 14:05 | Incident acknowledged by on-call engineer |
| 14:15 | Root cause identified: etcd disk I/O saturation |
| 14:30 | Mitigation applied: etcd compaction |
| 15:45 | Service fully restored |
| 16:15 | Incident closed |

## Root Cause

[Detailed explanation of what caused the incident]

## Impact

- **Users Affected**: ~500 users
- **Services Affected**: API server, kubectl access
- **Business Impact**: Unable to deploy changes for 2 hours
- **Data Loss**: None

## Resolution

[Description of how the incident was resolved]

## What Went Well

- Quick detection via monitoring alerts
- Clear runbook for etcd maintenance
- Effective communication with stakeholders

## What Could Be Improved

- Earlier detection of disk I/O issues
- Automated etcd compaction
- Better documentation of etcd performance tuning

## Action Items

| Action | Owner | Due Date | Status |
|--------|-------|----------|--------|
| Implement automated etcd compaction | John Doe | 2026-01-26 | Open |
| Add disk I/O monitoring alerts | Jane Smith | 2026-01-22 | Open |
| Update etcd runbook with performance tuning | John Doe | 2026-01-20 | Complete |
| Conduct etcd training session | Platform Team | 2026-02-01 | Open |

## Lessons Learned

[Key takeaways and insights from the incident]
```

## Incident Response Checklist

### During Incident

- [ ] Acknowledge alert and create incident ticket
- [ ] Classify severity and notify stakeholders
- [ ] Begin investigation and document findings
- [ ] Implement fix or mitigation
- [ ] Verify resolution and monitor for recurrence
- [ ] Provide regular status updates
- [ ] Close incident ticket with summary

### After Incident

- [ ] Schedule post-incident review
- [ ] Write post-incident report
- [ ] Identify and assign action items
- [ ] Update runbooks and documentation
- [ ] Share lessons learned with team
- [ ] Implement preventive measures

## Related Documentation

- **[Disaster Recovery](disaster-recovery.md)** - Backup and restore procedures
- **[Monitoring](monitoring.md)** - Alerting and observability
- **[Security Operations](security.md)** - Security incident response
- **[Runbooks](runbooks/)** - Operational procedures
- **[Troubleshooting](../how-to/troubleshooting.md)** - Common issues and solutions

## Emergency Procedures

### Complete Cluster Failure

If entire cluster is unresponsive:

1. Check cloud provider status page
2. Verify network connectivity to cluster
3. SSH to control plane nodes directly
4. Check control plane component status
5. Restore from backup if necessary (see Disaster Recovery)
6. Escalate to SEV1 immediately

### Data Loss Suspected

If data loss or corruption suspected:

1. Stop all write operations immediately
2. Isolate affected components
3. Preserve evidence for forensic analysis
4. Notify security team
5. Assess backup availability
6. Escalate to SEV1 and notify management

### Security Breach

If security breach suspected:

1. Isolate affected systems immediately
2. Preserve logs and evidence
3. Notify security team immediately
4. Do not remediate until security team approves
5. Follow security incident response procedures
6. Escalate to SEV1 and notify management

## Training and Preparedness

### Incident Response Drills

Conduct regular incident response drills:

- **Monthly**: Tabletop exercise with common scenarios
- **Quarterly**: Full incident simulation with on-call rotation
- **Annually**: Disaster recovery drill with complete cluster rebuild

### On-Call Preparation

Before going on-call:

- [ ] Review recent incidents and resolutions
- [ ] Test access to all systems and tools
- [ ] Review runbooks and escalation procedures
- [ ] Verify contact information is current
- [ ] Ensure laptop and phone are charged
- [ ] Review monitoring dashboards and alerts

### Knowledge Base

Maintain incident response knowledge base:

- Common incident patterns and resolutions
- Troubleshooting decision trees
- Escalation contact list
- Runbook index
- Post-incident reports archive
