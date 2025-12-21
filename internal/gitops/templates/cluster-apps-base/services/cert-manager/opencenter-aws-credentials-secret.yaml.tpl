apiVersion: v1
data:
  access-key-id: {{ .Secrets.CertManager.AWSAccessKey }}
  secret-access-key: {{ .Secrets.CertManager.AWSSecretAccessKey }}
kind: Secret
metadata:
  name: opencenter-aws-credentials-secret
