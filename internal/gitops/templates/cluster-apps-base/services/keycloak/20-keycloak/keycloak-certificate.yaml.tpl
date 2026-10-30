apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: keycloak-tls
  namespace: keycloak
spec:
  secretName: {{ .OpenCenter.Services.keycloak.TLSSecretName | default "keycloak-tls-secret" }}
  duration: 8760h0m0s
  renewBefore: 360h0m0s
  dnsNames:
    - {{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}
    - keycloak.keycloak.svc.cluster.local
    - keycloak.keycloak.svc
    - keycloak
  issuerRef:
    name: {{ defaultIssuer (.OpenCenter.Services.keycloak.IssuerRef | default "") }}
    kind: ClusterIssuer
    group: cert-manager.io
