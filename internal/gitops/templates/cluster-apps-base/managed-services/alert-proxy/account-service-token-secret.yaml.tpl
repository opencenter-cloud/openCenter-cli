apiVersion: v1
kind: Secret
metadata:
  name: account-service-token-secret
type: Opaque
data:
  account_service_token: {{ .Secrets.AlertProxy.AccountServiceToken | b64enc }}
