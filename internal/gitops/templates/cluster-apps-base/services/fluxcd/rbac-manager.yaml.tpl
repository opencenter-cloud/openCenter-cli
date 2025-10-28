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
  interval: 5m
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
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: rbac-manager-override
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: rbac-manager-base
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/rbac-manager
  targetNamespace: flux-system
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: rbac-manager
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
