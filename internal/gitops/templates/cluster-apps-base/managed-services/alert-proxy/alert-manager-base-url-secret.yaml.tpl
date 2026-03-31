apiVersion: v1
kind: Secret
metadata:
  name: alert-manager-url-secret
type: Opaque
data:
  alert_manager_url: {{ (index .OpenCenter.ManagedService "alert-proxy").AlertManagerBaseUrl | b64enc }}
