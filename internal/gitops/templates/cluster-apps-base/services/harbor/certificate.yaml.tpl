apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: harbor-tls
  namespace: harbor
spec:
  secretName: harbor-tls
  dnsNames:
    - {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
  issuerRef:
    name: letsencrypt-{{ .OpenCenter.Cluster.ClusterName }}
    kind: ClusterIssuer
