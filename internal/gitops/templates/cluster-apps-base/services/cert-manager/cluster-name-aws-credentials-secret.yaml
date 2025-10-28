apiVersion: v1
data:
  access-key-id: {{ (index .OpenCenter.Services "cert-manager").AWSAccessKey | b64enc }}
  secret-access-key: {{ (index .OpenCenter.Services "cert-manager").AWSSecretAccessKey | b64enc }}
kind: Secret
metadata:
  name: opencenter-aws-credentials-secret
