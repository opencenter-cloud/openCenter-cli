apiVersion: v1
kind: Secret
metadata:
  name: overseer-core-device-id-secret
type: Opaque
data:
  overseer_core_device_id: {{ .Secrets.AlertProxy.CoreDeviceId | b64enc }}
