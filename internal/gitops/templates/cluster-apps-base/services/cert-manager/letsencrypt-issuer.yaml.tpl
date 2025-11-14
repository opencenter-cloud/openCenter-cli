apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-issuer-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: mpk-support@rackspace.com
    privateKeySecretRef:
      name: letsencrypt-dns01
    solvers:
      - dns01:
          route53:
            region: us-east-1
            accessKeyIDSecretRef:
              name: opencenter-dev-aws-credentials-secret
              key: access-key-id
            secretAccessKeySecretRef:
              name: opencenter-dev-aws-credentials-secret
              key: secret-access-key
        selector:
          dnsZones:
            - stage.sjc3.k8s.opencenter.cloud
