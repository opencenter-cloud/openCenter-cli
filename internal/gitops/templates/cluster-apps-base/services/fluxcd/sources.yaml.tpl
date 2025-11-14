---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: sources
  namespace: flux-system
spec:
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  path: ./applications/overlays/stage-cluster/services/sources
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: sources
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
