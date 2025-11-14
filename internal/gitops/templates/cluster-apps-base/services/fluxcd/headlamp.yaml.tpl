---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: headlamp-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-headlamp
    namespace: flux-system
  path: applications/base/services/headlamp
  targetNamespace: headlamp
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: headlamp
      namespace: headlamp
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: headlamp
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: headlamp-override
  namespace: flux-system
spec:
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  path: ./applications/overlays/dev-cluster/services/headlamp
  targetNamespace: headlamp
  prune: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: headlamp
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
