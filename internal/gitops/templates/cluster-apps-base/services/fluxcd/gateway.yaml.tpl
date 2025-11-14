---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: gateway
  namespace: flux-system
spec:
  dependsOn:
    - name: cert-manager-base
      namespace: flux-system
    - name: envoy-gateway-api-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  path: ./applications/overlays/stage-cluster/services/gateway/
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: gateway
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
