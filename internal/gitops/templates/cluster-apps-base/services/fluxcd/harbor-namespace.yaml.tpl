---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: harbor-namespace
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 60m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-harbor
    namespace: flux-system
  path: applications/base/services/harbor/namespace
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: harbor
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
