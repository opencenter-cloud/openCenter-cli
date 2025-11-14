---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: external-snapshotter-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-external-snapshotter
  targetNamespace: external-snapshotter
  path: applications/base/services/external-snapshotter
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: external-snapshotter
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
