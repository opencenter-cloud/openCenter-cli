apiVersion: v1
kind: Secret
metadata:
  name: opencenter-cloudflare-credentials-secret
type: Opaque
stringData:
  api-token: {{ .GetCertManagerCloudflareAPIToken }}
