---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: opentelemetry-kube-stack-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: observability-namespace
      namespace: flux-system
    - name: loki-override
      namespace: flux-system
    - name: loki-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-observability
    namespace: flux-system
  path: applications/base/services/observability/opentelemetry-kube-stack
  targetNamespace: observability
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: opentelemetry-kube-stack
      namespace: observability
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: opentelemetry-kube-stack
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: opentelemetry-kube-stack-override
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
  path: ./applications/overlays/stage-cluster/services/opentelemetry-kube-stack
  targetNamespace: observability
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: opentelemetry-kube-stack
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
