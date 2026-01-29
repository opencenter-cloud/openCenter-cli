---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: sealed-secrets-base
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
    name: opencenter-sealed-secrets
    namespace: flux-system
  path: applications/base/services/sealed-secrets
  targetNamespace: sealed-secrets
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: sealed-secrets
      namespace: sealed-secrets
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: sealed-secrets
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: sealed-secrets-override
  namespace: flux-system
spec:
  dependsOn:
    - name: sealed-secrets-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/sealed-secrets
  targetNamespace: sealed-secrets
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: sealed-secrets
      namespace: sealed-secrets
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: sealed-secrets
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
