apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .ClusterName }}
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: {{ (index .OpenCenter.Services "cert-manager").Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01
    solvers:
      - dns01:
          route53:
            region: us-east-1
            accessKeyIDSecretRef:
              name: "{{ .ClusterName }}-aws-credentials-secret"
              key: access-key-id
            secretAccessKeySecretRef:
              name: "{{ .ClusterName }}-aws-credentials-secret"
              key: secret-access-key
        selector:
          dnsZones:
            - "{{ .ClusterName }}.k8s.opencenter.cloud"
