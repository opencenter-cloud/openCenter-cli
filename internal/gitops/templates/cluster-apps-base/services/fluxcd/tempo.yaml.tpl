---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: tempo-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: observability-namespace
      namespace: flux-system
    - name: kube-prometheus-stack-base
      namespace: flux-system
    - name: kube-prometheus-stack-override
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-observability
    namespace: flux-system
  path: applications/base/services/observability/tempo
  targetNamespace: observability
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: tempo
      namespace: observability
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: tempo
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: tempo-override
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
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/stage-cluster/services/tempo
  targetNamespace: observability
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: tempo
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
