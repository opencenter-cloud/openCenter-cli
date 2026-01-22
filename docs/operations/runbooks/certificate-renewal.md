# Certificate Renewal Runbook


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Certificate Overview](#certificate-overview)
- [Pre-Flight Checks](#pre-flight-checks)
- [Application Certificate Renewal](#application-certificate-renewal)
- [Kubernetes API Certificate Renewal](#kubernetes-api-certificate-renewal)
- [etcd Certificate Renewal](#etcd-certificate-renewal)
- [Kubelet Certificate Renewal](#kubelet-certificate-renewal)
- [Certificate Monitoring](#certificate-monitoring)
- [Troubleshooting](#troubleshooting)
- [Post-Renewal Verification](#post-renewal-verification)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
- [Certificate Renewal Schedule](#certificate-renewal-schedule)
- [Emergency Certificate Renewal](#emergency-certificate-renewal)
**doc_type: how-to**

Step-by-step procedures for renewing and rotating certificates in openCenter-managed Kubernetes clusters, including Kubernetes API certificates, etcd certificates, and application TLS certificates.

## Who This Is For

Operations teams and SREs responsible for certificate lifecycle management. Use this runbook when certificates are approaching expiration or when performing routine certificate rotation.

## Prerequisites

- Running openCenter cluster with cert-manager enabled
- Access to cluster configuration and SOPS keys
- `kubectl` access with cluster-admin permissions
- SSH access to control plane nodes
- Backup of cluster state (see [Disaster Recovery](../disaster-recovery.md))

## Certificate Overview

openCenter clusters use multiple certificate types:

**Kubernetes Certificates**:
- API server certificate (client and server)
- Kubelet certificates (client and server)
- Controller manager client certificate
- Scheduler client certificate
- Admin client certificate
- Service account signing key

**etcd Certificates**:
- etcd server certificate
- etcd peer certificate
- etcd client certificate (for API server)

**Application Certificates**:
- Ingress TLS certificates (managed by cert-manager)
- Service mesh certificates
- Application-specific certificates

**Certificate Lifetimes**:
- Kubernetes certificates: 1 year (default)
- etcd certificates: 1 year (default)
- Let's Encrypt certificates: 90 days
- Internal CA certificates: 10 years

## Pre-Flight Checks

### Check Certificate Expiration

Verify which certificates need renewal:

```bash
# Check all cert-manager certificates
kubectl get certificates -A

# Check certificate expiration dates
kubectl get certificates -A -o json | jq -r '.items[] | 
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    notAfter: .status.notAfter,
    renewalTime: .status.renewalTime
  }'

# Check Kubernetes API server certificate
echo | openssl s_client -connect api.cluster.example.com:6443 2>/dev/null | \
  openssl x509 -noout -dates

# Check etcd certificates (SSH to control plane node)
ssh ubuntu@control-plane-1
sudo openssl x509 -in /etc/kubernetes/pki/etcd/server.crt -noout -dates
sudo openssl x509 -in /etc/kubernetes/pki/etcd/peer.crt -noout -dates
```

**Expected output**:
```
NAMESPACE     NAME                    READY   SECRET                  AGE
istio-system  istio-gateway-cert      True    istio-gateway-tls       45d
production    app-tls-cert            True    app-tls-secret          30d

notBefore=Jan  1 00:00:00 2026 GMT
notAfter=Apr  1 00:00:00 2026 GMT
```

### Verify cert-manager Health

Ensure cert-manager is operational:

```bash
# Check cert-manager pods
kubectl get pods -n cert-manager

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager --tail=50

# Verify cert-manager webhook
kubectl get validatingwebhookconfigurations cert-manager-webhook

# Test cert-manager functionality
kubectl get clusterissuers
kubectl get issuers -A
```

### Create Backup

Backup certificates before renewal:

```bash
# Backup all certificate secrets
kubectl get secrets -A -l cert-manager.io/certificate-name -o yaml > \
  certificate-secrets-backup-$(date +%Y%m%d).yaml

# Encrypt backup
sops -e certificate-secrets-backup-$(date +%Y%m%d).yaml > \
  certificate-secrets-backup-$(date +%Y%m%d).enc.yaml

# Backup Kubernetes PKI certificates (SSH to control plane)
ssh ubuntu@control-plane-1
sudo tar czf /tmp/k8s-pki-backup-$(date +%Y%m%d).tar.gz \
  /etc/kubernetes/pki/

# Copy backup to local machine
scp ubuntu@control-plane-1:/tmp/k8s-pki-backup-$(date +%Y%m%d).tar.gz \
  ~/backups/
```

## Application Certificate Renewal

### Automatic Renewal (cert-manager)

cert-manager automatically renews certificates 30 days before expiration.

**Verify automatic renewal**:

```bash
# Check certificate renewal status
kubectl describe certificate -n production app-tls-cert

# Check cert-manager logs for renewal activity
kubectl logs -n cert-manager deployment/cert-manager | grep "Certificate.*renewed"

# Force immediate renewal (if needed)
kubectl annotate certificate -n production app-tls-cert \
  cert-manager.io/issue-temporary-certificate="true" --overwrite
```

### Manual Certificate Renewal

Force certificate renewal manually:

```bash
# Delete certificate to trigger renewal
kubectl delete certificate -n production app-tls-cert

# cert-manager will automatically recreate the certificate
# Monitor certificate creation
kubectl get certificate -n production app-tls-cert -w

# Verify new certificate
kubectl get secret -n production app-tls-secret -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -dates

# Check certificate is valid
kubectl get certificate -n production app-tls-cert -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# Should output: True
```

**Expected timeline**:
- Certificate deletion: Immediate
- New certificate request: Within 30 seconds
- ACME challenge: 1-2 minutes
- Certificate issuance: 2-5 minutes
- Total time: 3-7 minutes

### Update Certificate Issuer

If changing certificate issuer (e.g., staging to production Let's Encrypt):

```yaml
# Update certificate resource
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: app-tls-cert
  namespace: production
spec:
  secretName: app-tls-secret
  issuerRef:
    name: letsencrypt-prod  # Changed from letsencrypt-staging
    kind: ClusterIssuer
  dnsNames:
    - app.example.com
```

```bash
# Apply updated certificate
kubectl apply -f certificate.yaml

# Delete old secret to force renewal
kubectl delete secret -n production app-tls-secret

# Monitor renewal
kubectl get certificate -n production app-tls-cert -w
```

## Kubernetes API Certificate Renewal

### Check API Server Certificate

Verify API server certificate expiration:

```bash
# Check from external client
echo | openssl s_client -connect api.cluster.example.com:6443 2>/dev/null | \
  openssl x509 -noout -dates

# Check from control plane node
ssh ubuntu@control-plane-1
sudo openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -dates
```

### Renew API Server Certificate

Renew Kubernetes API server certificate:

```bash
# SSH to first control plane node
ssh ubuntu@control-plane-1

# Backup existing certificates
sudo cp -r /etc/kubernetes/pki /etc/kubernetes/pki.backup-$(date +%Y%m%d)

# Renew API server certificate
sudo kubeadm certs renew apiserver

# Verify new certificate
sudo openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -dates

# Restart API server
sudo crictl ps | grep kube-apiserver
sudo crictl stop <apiserver-container-id>
# Container will automatically restart

# Verify API server is healthy
kubectl get nodes
kubectl cluster-info
```

**Repeat for all control plane nodes**:

```bash
# Control plane node 2
ssh ubuntu@control-plane-2
sudo kubeadm certs renew apiserver
sudo crictl stop $(sudo crictl ps | grep kube-apiserver | awk '{print $1}')

# Control plane node 3
ssh ubuntu@control-plane-3
sudo kubeadm certs renew apiserver
sudo crictl stop $(sudo crictl ps | grep kube-apiserver | awk '{print $1}')
```

### Renew All Kubernetes Certificates

Renew all Kubernetes certificates at once:

```bash
# SSH to control plane node
ssh ubuntu@control-plane-1

# Check which certificates will be renewed
sudo kubeadm certs check-expiration

# Renew all certificates
sudo kubeadm certs renew all

# Restart control plane components
sudo crictl stop $(sudo crictl ps | grep kube-apiserver | awk '{print $1}')
sudo crictl stop $(sudo crictl ps | grep kube-controller-manager | awk '{print $1}')
sudo crictl stop $(sudo crictl ps | grep kube-scheduler | awk '{print $1}')

# Restart kubelet
sudo systemctl restart kubelet

# Verify cluster health
kubectl get nodes
kubectl get pods -n kube-system
```

**Expected output from check-expiration**:
```
CERTIFICATE                EXPIRES                  RESIDUAL TIME   CERTIFICATE AUTHORITY   EXTERNALLY MANAGED
admin.conf                 Jan 19, 2027 00:00 UTC   364d            ca                      no
apiserver                  Jan 19, 2027 00:00 UTC   364d            ca                      no
apiserver-etcd-client      Jan 19, 2027 00:00 UTC   364d            etcd-ca                 no
apiserver-kubelet-client   Jan 19, 2027 00:00 UTC   364d            ca                      no
controller-manager.conf    Jan 19, 2027 00:00 UTC   364d            ca                      no
etcd-healthcheck-client    Jan 19, 2027 00:00 UTC   364d            etcd-ca                 no
etcd-peer                  Jan 19, 2027 00:00 UTC   364d            etcd-ca                 no
etcd-server                Jan 19, 2027 00:00 UTC   364d            etcd-ca                 no
front-proxy-client         Jan 19, 2027 00:00 UTC   364d            front-proxy-ca          no
scheduler.conf             Jan 19, 2027 00:00 UTC   364d            ca                      no
```

### Update Kubeconfig

Update kubeconfig with new certificates:

```bash
# SSH to control plane node
ssh ubuntu@control-plane-1

# Backup current kubeconfig
cp ~/.kube/config ~/.kube/config.backup-$(date +%Y%m%d)

# Update admin kubeconfig
sudo kubeadm init phase kubeconfig admin

# Copy new kubeconfig
sudo cp /etc/kubernetes/admin.conf ~/.kube/config
sudo chown $(id -u):$(id -g) ~/.kube/config

# Verify kubectl works
kubectl get nodes
```

**Update local kubeconfig**:

```bash
# Copy new kubeconfig from control plane
scp ubuntu@control-plane-1:~/.kube/config ~/.kube/cluster-config

# Merge with existing kubeconfig
KUBECONFIG=~/.kube/config:~/.kube/cluster-config kubectl config view --flatten > ~/.kube/config.new
mv ~/.kube/config.new ~/.kube/config

# Verify access
kubectl get nodes
```

## etcd Certificate Renewal

### Check etcd Certificates

Verify etcd certificate expiration:

```bash
# SSH to control plane node
ssh ubuntu@control-plane-1

# Check etcd server certificate
sudo openssl x509 -in /etc/kubernetes/pki/etcd/server.crt -noout -dates

# Check etcd peer certificate
sudo openssl x509 -in /etc/kubernetes/pki/etcd/peer.crt -noout -dates

# Check etcd CA certificate
sudo openssl x509 -in /etc/kubernetes/pki/etcd/ca.crt -noout -dates
```

### Renew etcd Certificates

Renew etcd certificates:

```bash
# SSH to control plane node
ssh ubuntu@control-plane-1

# Backup etcd certificates
sudo cp -r /etc/kubernetes/pki/etcd /etc/kubernetes/pki/etcd.backup-$(date +%Y%m%d)

# Renew etcd server certificate
sudo kubeadm certs renew etcd-server

# Renew etcd peer certificate
sudo kubeadm certs renew etcd-peer

# Renew etcd healthcheck client certificate
sudo kubeadm certs renew etcd-healthcheck-client

# Verify new certificates
sudo openssl x509 -in /etc/kubernetes/pki/etcd/server.crt -noout -dates

# Restart etcd
sudo crictl stop $(sudo crictl ps | grep etcd | awk '{print $1}')

# Verify etcd health
sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health
```

**Repeat for all control plane nodes** (one at a time):

```bash
# Wait for etcd cluster to stabilize before proceeding to next node
sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list

# Proceed to next control plane node only when all members are healthy
```

## Kubelet Certificate Renewal

### Enable Kubelet Certificate Rotation

Configure automatic kubelet certificate rotation:

```yaml
# Update cluster configuration
opencenter:
  cluster:
    kubernetes:
      kubelet_rotate_server_certificates: true
```

```bash
# Apply configuration
openCenter cluster setup my-cluster --force

# Verify kubelet configuration on nodes
ssh ubuntu@worker-1
sudo cat /var/lib/kubelet/config.yaml | grep -A 2 rotateCertificates
```

**Expected output**:
```yaml
rotateCertificates: true
serverTLSBootstrap: true
```

### Manual Kubelet Certificate Renewal

Manually renew kubelet certificates if needed:

```bash
# SSH to node
ssh ubuntu@worker-1

# Backup kubelet certificates
sudo cp /var/lib/kubelet/pki/kubelet-client-current.pem \
  /var/lib/kubelet/pki/kubelet-client-current.pem.backup-$(date +%Y%m%d)

# Delete kubelet client certificate
sudo rm /var/lib/kubelet/pki/kubelet-client-current.pem

# Restart kubelet to trigger certificate renewal
sudo systemctl restart kubelet

# Verify new certificate
sudo openssl x509 -in /var/lib/kubelet/pki/kubelet-client-current.pem -noout -dates

# Check kubelet status
sudo systemctl status kubelet
kubectl get nodes
```

## Certificate Monitoring

### Set Up Expiration Alerts

Configure Prometheus alerts for certificate expiration:

```yaml
# certificate-expiry-alerts.yaml
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
            summary: "Certificate {{ $labels.name }} expiring in less than 7 days"
            description: "Certificate {{ $labels.name }} in namespace {{ $labels.namespace }} expires in {{ $value | humanizeDuration }}"
        
        - alert: CertificateExpired
          expr: |
            certmanager_certificate_expiration_timestamp_seconds - time() < 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Certificate {{ $labels.name }} has expired"
            description: "Certificate {{ $labels.name }} in namespace {{ $labels.namespace }} expired {{ $value | humanizeDuration }} ago"
        
        - alert: KubernetesCertificateExpiringSoon
          expr: |
            apiserver_client_certificate_expiration_seconds_count{job="apiserver"} > 0 and
            histogram_quantile(0.01, apiserver_client_certificate_expiration_seconds_bucket{job="apiserver"}) < 604800
          for: 1h
          labels:
            severity: warning
          annotations:
            summary: "Kubernetes client certificate expiring soon"
            description: "A client certificate for the API server will expire in less than 7 days"
```

```bash
# Apply alerts
kubectl apply -f certificate-expiry-alerts.yaml

# Verify alerts are loaded
kubectl get prometheusrules -n monitoring certificate-expiry
```

### Create Monitoring Dashboard

Create Grafana dashboard for certificate monitoring:

```json
{
  "dashboard": {
    "title": "Certificate Expiration",
    "panels": [
      {
        "title": "Certificates Expiring Soon",
        "targets": [
          {
            "expr": "(certmanager_certificate_expiration_timestamp_seconds - time()) / 86400",
            "legendFormat": "{{ namespace }}/{{ name }}"
          }
        ]
      },
      {
        "title": "Certificate Renewal Status",
        "targets": [
          {
            "expr": "certmanager_certificate_ready_status",
            "legendFormat": "{{ namespace }}/{{ name }}"
          }
        ]
      }
    ]
  }
}
```

### Automated Certificate Checks

Create cron job to check certificate expiration:

```bash
# Create certificate check script
cat > ~/check-certificates.sh <<'EOF'
#!/bin/bash
set -e

CLUSTER_NAME="my-cluster"
ALERT_DAYS=30

# Check cert-manager certificates
echo "Checking cert-manager certificates..."
kubectl get certificates -A -o json | jq -r '.items[] | 
  select(.status.notAfter != null) |
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    notAfter: .status.notAfter,
    daysRemaining: (((.status.notAfter | fromdateiso8601) - now) / 86400 | floor)
  } |
  select(.daysRemaining < '$ALERT_DAYS') |
  "WARNING: Certificate \(.namespace)/\(.name) expires in \(.daysRemaining) days"'

# Check Kubernetes API certificate
echo "Checking Kubernetes API certificate..."
EXPIRY=$(echo | openssl s_client -connect api.$CLUSTER_NAME.example.com:6443 2>/dev/null | \
  openssl x509 -noout -enddate | cut -d= -f2)
EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s)
NOW_EPOCH=$(date +%s)
DAYS_REMAINING=$(( ($EXPIRY_EPOCH - $NOW_EPOCH) / 86400 ))

if [ $DAYS_REMAINING -lt $ALERT_DAYS ]; then
  echo "WARNING: Kubernetes API certificate expires in $DAYS_REMAINING days"
fi

echo "Certificate check complete"
EOF

chmod +x ~/check-certificates.sh

# Add to crontab (run daily at 9 AM)
crontab -e
# Add line:
# 0 9 * * * /home/ubuntu/check-certificates.sh >> /var/log/cert-check.log 2>&1
```

## Troubleshooting

### Certificate Renewal Fails

**Symptoms**: cert-manager unable to renew certificate

**Diagnosis**:

```bash
# Check certificate status
kubectl describe certificate -n production app-tls-cert

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager --tail=100

# Check certificate request
kubectl get certificaterequest -n production

# Check ACME challenge
kubectl get challenges -n production
kubectl describe challenge -n production <challenge-name>
```

**Resolution**:

```bash
# If DNS challenge failing, verify DNS configuration
dig _acme-challenge.app.example.com TXT

# If HTTP challenge failing, verify ingress is accessible
curl -I http://app.example.com/.well-known/acme-challenge/test

# Delete failed certificate request
kubectl delete certificaterequest -n production <request-name>

# Trigger new certificate request
kubectl annotate certificate -n production app-tls-cert \
  cert-manager.io/issue-temporary-certificate="true" --overwrite
```

### API Server Certificate Mismatch

**Symptoms**: kubectl commands fail with certificate errors

**Diagnosis**:

```bash
# Check certificate on API server
echo | openssl s_client -connect api.cluster.example.com:6443 2>/dev/null | \
  openssl x509 -noout -text

# Check certificate in kubeconfig
kubectl config view --raw -o json | jq -r '.clusters[0].cluster."certificate-authority-data"' | \
  base64 -d | openssl x509 -noout -text

# Compare certificate fingerprints
```

**Resolution**:

```bash
# Update kubeconfig with new certificate
sudo kubeadm init phase kubeconfig admin
sudo cp /etc/kubernetes/admin.conf ~/.kube/config

# Or regenerate kubeconfig
kubectl config set-cluster my-cluster \
  --server=https://api.cluster.example.com:6443 \
  --certificate-authority=/etc/kubernetes/pki/ca.crt \
  --embed-certs=true
```

### etcd Certificate Issues

**Symptoms**: etcd cluster unhealthy after certificate renewal

**Diagnosis**:

```bash
# Check etcd health
sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health

# Check etcd logs
sudo journalctl -u etcd -n 100

# Verify certificate matches
sudo openssl x509 -in /etc/kubernetes/pki/etcd/server.crt -noout -modulus | openssl md5
sudo openssl rsa -in /etc/kubernetes/pki/etcd/server.key -noout -modulus | openssl md5
```

**Resolution**:

```bash
# Restore from backup if certificates are corrupted
sudo cp -r /etc/kubernetes/pki/etcd.backup-20260119 /etc/kubernetes/pki/etcd

# Restart etcd
sudo crictl stop $(sudo crictl ps | grep etcd | awk '{print $1}')

# Verify etcd recovery
sudo ETCDCTL_API=3 etcdctl endpoint health
```

## Post-Renewal Verification

### Verify Certificate Validity

Confirm all certificates are valid:

```bash
# Check cert-manager certificates
kubectl get certificates -A
# All should show READY=True

# Check Kubernetes API certificate
echo | openssl s_client -connect api.cluster.example.com:6443 2>/dev/null | \
  openssl x509 -noout -dates

# Verify kubectl access
kubectl get nodes
kubectl get pods -A

# Check application endpoints
curl -I https://app.example.com
```

### Test Certificate Functionality

Verify certificates work correctly:

```bash
# Test TLS handshake
openssl s_client -connect app.example.com:443 -servername app.example.com

# Test API server authentication
kubectl auth can-i get pods

# Test etcd connectivity
sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  member list
```

### Update Documentation

Document certificate renewal:

```markdown
# Certificate Renewal Log

**Date**: 2026-01-19
**Operator**: John Doe
**Certificates Renewed**:
- Kubernetes API server certificate (all control plane nodes)
- etcd certificates (all control plane nodes)
- Application TLS certificate (app.example.com)

**Issues Encountered**: None

**Next Renewal Due**: 2027-01-19
```

## Best Practices

### Certificate Management
- Monitor certificate expiration continuously
- Renew certificates at least 30 days before expiration
- Test certificate renewal in non-production first
- Always backup certificates before renewal
- Document certificate renewal procedures

### Automation
- Enable automatic certificate rotation where possible
- Use cert-manager for application certificates
- Configure kubelet certificate rotation
- Set up expiration alerts
- Automate certificate monitoring

### Security
- Protect certificate private keys (chmod 600)
- Store backups encrypted with SOPS
- Rotate certificates regularly (annually)
- Use strong key sizes (RSA 2048+ or ECDSA P-256+)
- Audit certificate access logs

### Operations
- Schedule certificate renewals during maintenance windows
- Renew control plane certificates one node at a time
- Verify cluster health after each renewal
- Keep certificate inventory up to date
- Train team on certificate renewal procedures

## Related Documentation

- **[Security Operations](../security.md)** - Certificate security practices
- **[Disaster Recovery](../disaster-recovery.md)** - Certificate backup procedures
- **[Monitoring](../monitoring.md)** - Certificate monitoring setup
- **[Configuration Reference](../../reference/configuration.md)** - cert-manager configuration

## Certificate Renewal Schedule

Recommended renewal schedule:

- **Application certificates**: Automatic (cert-manager)
- **Kubernetes certificates**: Annually or 90 days before expiration
- **etcd certificates**: Annually or 90 days before expiration
- **CA certificates**: Every 5-10 years (requires cluster rebuild)

## Emergency Certificate Renewal

If certificates have already expired:

1. **Backup cluster state immediately**
2. **SSH directly to control plane nodes** (kubectl won't work)
3. **Renew certificates using kubeadm**
4. **Restart control plane components**
5. **Update kubeconfig files**
6. **Verify cluster recovery**
7. **Document incident and update procedures**

See [Incident Response](../incident-response.md) for emergency procedures.
