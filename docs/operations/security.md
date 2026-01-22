# Security Operations


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Security Architecture Overview](#security-architecture-overview)
- [Enable Security Features](#enable-security-features)
- [Secrets Management](#secrets-management)
- [Certificate Management](#certificate-management)
- [Vulnerability Scanning](#vulnerability-scanning)
- [Access Control and RBAC](#access-control-and-rbac)
- [Compliance and Audit Logging](#compliance-and-audit-logging)
- [Network Security](#network-security)
- [Security Incident Response](#security-incident-response)
- [Security Best Practices](#security-best-practices)
- [Related Documentation](#related-documentation)
- [Next Steps](#next-steps)
**doc_type: how-to**

Security operations procedures for openCenter-managed Kubernetes clusters, covering vulnerability management, certificate rotation, secrets management, compliance validation, and incident response.

## Who This Is For

Security engineers, compliance officers, and operations teams responsible for maintaining cluster security posture. Use this guide to implement security controls, perform security audits, and respond to security incidents.

## Prerequisites

- Running openCenter cluster with security features enabled
- Access to cluster configuration and SOPS keys
- `kubectl` access with appropriate RBAC permissions
- Understanding of Kubernetes security concepts

## Security Architecture Overview

openCenter implements defense-in-depth security:

- **Secrets Management** - SOPS with Age encryption for sensitive data
- **Certificate Management** - cert-manager with automated renewal
- **Access Control** - RBAC with OIDC integration (Keycloak)
- **Policy Enforcement** - Kyverno for admission control
- **Security Hardening** - OS and Kubernetes hardening enabled by default
- **Audit Logging** - Kubernetes audit logs for compliance
- **Network Security** - Network policies and security groups

## Enable Security Features

### Enable Security Hardening

Security hardening is enabled by default in openCenter:

```yaml
opencenter:
  cluster:
    networking:
      security:
        os_hardening: true  # OS-level security hardening
    kubernetes:
      security:
        k8s_hardening: true  # Kubernetes security hardening
        pod_security_exemptions:
          - trivy-temp
          - tigera-operator
          - kube-system
```

OS hardening includes:
- Kernel parameter tuning for security
- Firewall rules (iptables/nftables)
- SSH hardening
- Audit daemon configuration
- File system permissions

Kubernetes hardening includes:
- Pod Security Standards enforcement
- Restricted security contexts
- Network policy defaults
- RBAC least privilege
- API server security flags

### Enable Policy Enforcement

Kyverno provides admission control and policy enforcement:

```yaml
opencenter:
  services:
    kyverno:
      enabled: true  # Enabled by default
```

Kyverno enforces policies for:
- Required security contexts
- Image pull policy enforcement
- Resource limit requirements
- Label and annotation standards
- Namespace isolation

### Enable Certificate Management

cert-manager automates certificate lifecycle:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      email: security@example.com
      region: us-east-1
      letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory

secrets:
  cert_manager:
    aws_access_key: "your-access-key"  # For Route53 DNS validation
    aws_secret_access_key: "your-secret-key"
```

## Secrets Management

### Rotate SOPS Age Keys

Rotate encryption keys annually or after security incidents:

```bash
# Generate new Age key
openCenter sops keygen my-cluster --rotate

# Update .sops.yaml with new key
# Old key: age1old...
# New key: age1new...

# Re-encrypt all secrets with new key
cd ~/.config/openCenter/clusters/myorg/my-cluster
find . -name "*.enc.yaml" -o -name "*-secret.yaml" | while read file; do
  sops updatekeys "$file"
done

# Verify re-encryption
sops -d infrastructure/clusters/my-cluster/secrets/openstack-credentials.yaml

# Commit changes
git add .
git commit -m "security: Rotate SOPS Age keys"
git push

# Archive old key securely
mv ~/.config/openCenter/secrets/age/my-cluster-key.txt \
   ~/.config/openCenter/secrets/age/my-cluster-key-$(date +%Y%m%d).txt.old
```

### Rotate Kubernetes Secrets

Rotate service account tokens and application secrets:

```bash
# Rotate service account token
kubectl delete secret -n default my-service-token
kubectl create token my-service-account -n default --duration=8760h > token.txt

# Update secret in SOPS-encrypted file
sops infrastructure/clusters/my-cluster/secrets/app-credentials.yaml
# Edit the secret value
# Save and exit

# Apply updated secret
kubectl apply -f infrastructure/clusters/my-cluster/secrets/app-credentials.yaml
```

### Rotate SSH Keys

Rotate SSH keys for cluster access:

```bash
# Generate new SSH key pair
ssh-keygen -t ed25519 -f ~/.config/openCenter/secrets/ssh/my-cluster-new -C "my-cluster-$(date +%Y%m%d)"

# Update cluster configuration
sops ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml
# Update ssh_key.private and ssh_key.public paths

# Deploy new key to nodes (requires cluster access)
# This is provider-specific - for OpenStack:
openstack server add security group <server-id> <security-group>

# Test new key
ssh -i ~/.config/openCenter/secrets/ssh/my-cluster-new ubuntu@<node-ip>

# Remove old key after verification
rm ~/.config/openCenter/secrets/ssh/my-cluster
```

## Certificate Management

### Monitor Certificate Expiration

Check certificate expiration dates:

```bash
# Check all certificates in cluster
kubectl get certificates -A

# Check specific certificate details
kubectl describe certificate -n istio-system istio-gateway-cert

# Check certificate expiration with openssl
kubectl get secret -n istio-system istio-gateway-cert -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -dates
```

Set up alerts for certificate expiration:

```yaml
# certificate-expiry-alert.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: certificate-expiry
  namespace: monitoring
spec:
  groups:
    - name: certificates
      interval: 1h
      rules:
        - alert: CertificateExpiringSoon
          expr: |
            certmanager_certificate_expiration_timestamp_seconds - time() < 604800
          for: 1h
          labels:
            severity: warning
          annotations:
            summary: "Certificate {{ $labels.name }} expiring soon"
            description: "Certificate {{ $labels.name }} in namespace {{ $labels.namespace }} expires in less than 7 days"
        
        - alert: CertificateExpired
          expr: |
            certmanager_certificate_expiration_timestamp_seconds - time() < 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Certificate {{ $labels.name }} has expired"
            description: "Certificate {{ $labels.name }} in namespace {{ $labels.namespace }} has expired"
```

### Manual Certificate Renewal

Force certificate renewal if needed:

```bash
# Delete certificate to trigger renewal
kubectl delete certificate -n istio-system istio-gateway-cert

# cert-manager will automatically recreate and renew

# Verify new certificate
kubectl get certificate -n istio-system istio-gateway-cert
kubectl describe certificate -n istio-system istio-gateway-cert
```

### Backup Certificates

Backup certificates for disaster recovery:

```bash
# Export all certificates
kubectl get certificates -A -o yaml > certificates-backup-$(date +%Y%m%d).yaml

# Export certificate secrets
kubectl get secrets -A -l cert-manager.io/certificate-name -o yaml > \
  certificate-secrets-backup-$(date +%Y%m%d).yaml

# Encrypt backup
sops -e certificates-backup-$(date +%Y%m%d).yaml > \
  certificates-backup-$(date +%Y%m%d).enc.yaml

# Store securely
mv certificates-backup-$(date +%Y%m%d).enc.yaml \
  ~/.config/openCenter/backups/
```

## Vulnerability Scanning

### Scan Container Images

Use Trivy for container image scanning:

```bash
# Install Trivy
brew install trivy  # macOS
# OR
apt-get install trivy  # Ubuntu

# Scan image for vulnerabilities
trivy image ghcr.io/rackerlabs/my-app:latest

# Scan with severity filter
trivy image --severity HIGH,CRITICAL ghcr.io/rackerlabs/my-app:latest

# Generate JSON report
trivy image -f json -o report.json ghcr.io/rackerlabs/my-app:latest
```

### Scan Kubernetes Manifests

Scan manifests for security issues:

```bash
# Scan Kubernetes YAML files
trivy config infrastructure/clusters/my-cluster/

# Scan with specific checks
trivy config --severity HIGH,CRITICAL infrastructure/clusters/my-cluster/

# Check for misconfigurations
trivy config --policy-namespaces user infrastructure/clusters/my-cluster/
```

### Continuous Scanning

Integrate scanning into CI/CD pipeline:

```yaml
# .github/workflows/security-scan.yml
name: Security Scan
on: [push, pull_request]

jobs:
  trivy-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'config'
          scan-ref: 'infrastructure/'
          format: 'sarif'
          output: 'trivy-results.sarif'
      
      - name: Upload Trivy results to GitHub Security
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-results.sarif'
```

## Access Control and RBAC

### Audit RBAC Permissions

Review RBAC configuration regularly:

```bash
# List all ClusterRoles
kubectl get clusterroles

# List all ClusterRoleBindings
kubectl get clusterrolebindings

# Check permissions for specific user
kubectl auth can-i --list --as=user@example.com

# Check specific permission
kubectl auth can-i delete pods --as=user@example.com -n production

# Audit who can perform privileged actions
kubectl get clusterrolebindings -o json | \
  jq -r '.items[] | select(.roleRef.name=="cluster-admin") | .metadata.name'
```

### Implement Least Privilege

Create role with minimal permissions:

```yaml
# developer-role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: developer
  namespace: development
rules:
  - apiGroups: ["", "apps"]
    resources: ["pods", "deployments", "services"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: developer-binding
  namespace: development
subjects:
  - kind: User
    name: developer@example.com
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: developer
  apiGroup: rbac.authorization.k8s.io
```

### Enable OIDC Authentication

Configure Keycloak for OIDC authentication:

```yaml
opencenter:
  services:
    keycloak:
      enabled: true
      hostname: auth.my-cluster.region.k8s.opencenter.cloud
      realm: opencenter
      client_id: kubernetes
      frontend_url: https://auth.my-cluster.region.k8s.opencenter.cloud
  cluster:
    kubernetes:
      oidc:
        enabled: true
        kube_oidc_url: https://auth.my-cluster.region.k8s.opencenter.cloud/realms/opencenter
        kube_oidc_client_id: kubernetes
        kube_oidc_username_claim: sub
        kube_oidc_username_prefix: "oidc:"
        kube_oidc_groups_claim: groups
        kube_oidc_groups_prefix: "oidc:"

secrets:
  keycloak:
    client_secret: "your-client-secret"
    admin_password: "your-admin-password"
```

## Compliance and Audit Logging

### Enable Kubernetes Audit Logging

Audit logs track all API server requests:

```yaml
# audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  # Log all requests at Metadata level
  - level: Metadata
    omitStages:
      - RequestReceived
  
  # Log pod changes at Request level
  - level: Request
    resources:
      - group: ""
        resources: ["pods"]
    verbs: ["create", "update", "patch", "delete"]
  
  # Log secret access at Metadata level (don't log secret data)
  - level: Metadata
    resources:
      - group: ""
        resources: ["secrets"]
  
  # Don't log read-only requests to certain resources
  - level: None
    resources:
      - group: ""
        resources: ["events", "nodes/status", "pods/status"]
    verbs: ["get", "list", "watch"]
```

### Query Audit Logs

Audit logs are stored in Loki (if enabled):

```logql
# All audit logs
{job="kube-apiserver-audit"}

# Failed authentication attempts
{job="kube-apiserver-audit"} | json | responseStatus_code="401"

# Secret access
{job="kube-apiserver-audit"} | json | objectRef_resource="secrets"

# Privileged operations
{job="kube-apiserver-audit"} | json | verb=~"create|update|delete" | user_username!~"system:.*"

# Access by specific user
{job="kube-apiserver-audit"} | json | user_username="user@example.com"
```

### Generate Compliance Reports

Create compliance report for audit:

```bash
# Export audit logs for date range
kubectl logs -n kube-system kube-apiserver-<node> --since=24h > audit-$(date +%Y%m%d).log

# Filter for security-relevant events
grep -E "secrets|serviceaccounts|roles|rolebindings" audit-$(date +%Y%m%d).log > \
  security-audit-$(date +%Y%m%d).log

# Generate summary report
cat security-audit-$(date +%Y%m%d).log | \
  jq -r '[.user.username, .verb, .objectRef.resource, .responseStatus.code] | @csv' | \
  sort | uniq -c > audit-summary-$(date +%Y%m%d).csv
```

## Network Security

### Implement Network Policies

Restrict pod-to-pod communication:

```yaml
# default-deny-all.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
---
# allow-frontend-to-backend.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend-to-backend
  namespace: production
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: frontend
      ports:
        - protocol: TCP
          port: 8080
```

### Audit Network Policies

Review network policy coverage:

```bash
# List all network policies
kubectl get networkpolicies -A

# Check if namespace has default deny policy
kubectl get networkpolicy -n production default-deny-all

# Test network connectivity
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- /bin/bash
# Inside pod:
curl http://backend-service.production.svc.cluster.local:8080
```

## Security Incident Response

### Detect Compromised Pod

Signs of compromise:
- Unexpected network connections
- High CPU/memory usage
- Modified binaries or configuration
- Unauthorized privilege escalation

Investigate suspicious pod:

```bash
# Check pod events
kubectl describe pod -n production suspicious-pod

# Check pod logs
kubectl logs -n production suspicious-pod --previous
kubectl logs -n production suspicious-pod

# Check running processes
kubectl exec -n production suspicious-pod -- ps aux

# Check network connections
kubectl exec -n production suspicious-pod -- netstat -tulpn

# Check file modifications
kubectl exec -n production suspicious-pod -- find / -mtime -1 -type f
```

### Isolate Compromised Pod

Quarantine the pod immediately:

```bash
# Apply network policy to isolate pod
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: isolate-suspicious-pod
  namespace: production
spec:
  podSelector:
    matchLabels:
      app: suspicious-app
  policyTypes:
    - Ingress
    - Egress
  # No ingress or egress rules = deny all
EOF

# Scale down deployment
kubectl scale deployment -n production suspicious-app --replicas=0

# Preserve pod for forensics
kubectl label pod -n production suspicious-pod quarantine=true
kubectl annotate pod -n production suspicious-pod incident-id=INC-2024-001
```

### Forensic Analysis

Collect evidence for analysis:

```bash
# Export pod specification
kubectl get pod -n production suspicious-pod -o yaml > pod-spec.yaml

# Export pod logs
kubectl logs -n production suspicious-pod --all-containers=true > pod-logs.txt

# Capture pod filesystem (if possible)
kubectl cp production/suspicious-pod:/var/log ./forensics/var-log/

# Export events
kubectl get events -n production --field-selector involvedObject.name=suspicious-pod > events.txt

# Create incident report
cat > incident-report.md <<EOF
# Security Incident Report

**Incident ID**: INC-2024-001
**Date**: $(date)
**Severity**: High

## Summary
Suspicious activity detected in pod suspicious-pod

## Evidence
- Pod specification: pod-spec.yaml
- Pod logs: pod-logs.txt
- Events: events.txt
- Filesystem: forensics/

## Actions Taken
1. Pod isolated with network policy
2. Deployment scaled to zero
3. Evidence collected
4. Security team notified

## Next Steps
- Analyze collected evidence
- Determine root cause
- Implement preventive measures
- Update security policies
EOF
```

### Remediation

After incident analysis:

```bash
# Remove compromised resources
kubectl delete pod -n production suspicious-pod
kubectl delete deployment -n production suspicious-app

# Rotate secrets
sops infrastructure/clusters/my-cluster/secrets/app-credentials.yaml
# Update compromised credentials

# Redeploy from known-good image
kubectl apply -f infrastructure/clusters/my-cluster/apps/production/app-deployment.yaml

# Verify deployment
kubectl rollout status deployment -n production app

# Update security policies
kubectl apply -f security-policies/enhanced-network-policy.yaml
```

## Security Best Practices

### Secrets Management
- Rotate SOPS keys annually
- Use separate keys per environment
- Never commit unencrypted secrets
- Audit secret access regularly
- Implement secret scanning in CI/CD

### Access Control
- Implement least privilege RBAC
- Use OIDC for user authentication
- Audit RBAC permissions quarterly
- Remove unused service accounts
- Enable MFA for administrative access

### Certificate Management
- Monitor certificate expiration
- Automate renewal with cert-manager
- Backup certificates regularly
- Use short-lived certificates where possible
- Rotate CA certificates annually

### Vulnerability Management
- Scan images before deployment
- Update base images regularly
- Patch critical vulnerabilities within 7 days
- Maintain inventory of deployed images
- Subscribe to security advisories

### Network Security
- Implement default-deny network policies
- Segment namespaces by trust level
- Use service mesh for mTLS
- Monitor network traffic patterns
- Restrict egress to known destinations

### Compliance
- Enable audit logging
- Retain logs per compliance requirements
- Generate regular compliance reports
- Document security controls
- Conduct periodic security assessments

## Related Documentation

- **[Disaster Recovery](disaster-recovery.md)** - Backup and recovery procedures
- **[Monitoring](monitoring.md)** - Security monitoring and alerting
- **[Configuration Reference](../reference/configuration.md)** - Security configuration options
- **[Troubleshooting](../how-to/troubleshooting.md)** - Security issue resolution

## Next Steps

- Enable all security features in your cluster configuration
- Set up automated vulnerability scanning
- Configure audit log retention and analysis
- Establish security incident response procedures
- Schedule regular security audits and penetration testing
