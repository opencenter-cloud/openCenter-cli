apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .OpenCenter.Cluster.ClusterName }}
spec:
  acme:
    server: {{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}
    email: {{ (index .OpenCenter.Services "cert-manager").LetsEncryptEmail | default "mpk-support@rackspace.com" }}
    privateKeySecretRef:
      name: letsencrypt-dns01
    solvers:
      - dns01:
          route53:
            region: {{ (index .OpenCenter.Services "cert-manager").Region }}
            accessKeyIDSecretRef:
              name: "opencenter-aws-credentials-secret"
              key: access-key-id
            secretAccessKeySecretRef:
              name: "opencenter-aws-credentials-secret"
              key: secret-access-key
        selector:
          dnsZones:
            - {{ .OpenCenter.Cluster.ClusterFQDN }}
