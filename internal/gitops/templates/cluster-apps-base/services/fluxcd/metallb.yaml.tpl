---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: metallb-base
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
    name: opencenter-metallb
    namespace: flux-system
  path: applications/base/services/metallb
  targetNamespace: metallb-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: metallb
      namespace: metallb-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: metallb
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: metallb-override
  namespace: flux-system
spec:
  dependsOn:
    - name: metallb-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/dev-cluster/services/metallb
  targetNamespace: metallb-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: metallb
      namespace: metallb-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: metallb
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
