---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: weave-gitops-base
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
    name: opencenter-weave-gitops
    namespace: flux-system
  path: applications/base/services/weave-gitops
  targetNamespace: flux-system
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: weave-gitops
      namespace: flux-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: weave-gitops
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: weave-gitops-override
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: envoy-gateway-api-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/stage-cluster/services/weave-gitops
  targetNamespace: flux-system
  prune: true
  wait: true
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: weave-gitops
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
