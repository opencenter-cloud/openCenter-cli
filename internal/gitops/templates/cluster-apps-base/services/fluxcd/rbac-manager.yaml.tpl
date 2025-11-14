---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: rbac-manager-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: kube-prometheus-stack-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-rbac-manager
    namespace: flux-system
  path: applications/base/services/rbac-manager
  targetNamespace: rbac-system
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: rbac-manager
      namespace: rbac-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: rbac-manager
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
