apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-issuer-prod
spec:
  acme:
    server: {{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}
    email: {{ .OpenCenter.Cluster.AdminEmail }}
    privateKeySecretRef:
      name: letsencrypt-dns01
    solvers:
      - dns01:
          route53:
            region: {{ (index .OpenCenter.Services "cert-manager").Region }}
            accessKeyIDSecretRef:
              name: {{ .OpenCenter.Cluster.ClusterName }}-aws-credentials-secret
              key: access-key-id
            secretAccessKeySecretRef:
              name: {{ .OpenCenter.Cluster.ClusterName }}-aws-credentials-secret
              key: secret-access-key
        selector:
          dnsZones:
            - {{ .OpenCenter.Cluster.ClusterFQDN }}
