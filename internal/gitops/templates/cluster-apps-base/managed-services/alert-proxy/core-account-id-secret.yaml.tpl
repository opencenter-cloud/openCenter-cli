apiVersion: v1
kind: Secret
metadata:
  name: core-account-id-secret
type: Opaque
data:
  core_account_number: {{ .Secrets.AlertProxy.CoreAccountNumber | b64enc }}
