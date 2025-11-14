---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: kyverno-base
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
    name: opencenter-kyverno
    namespace: flux-system
  path: applications/base/services/kyverno/policy-engine
  targetNamespace: kyverno
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: kyverno
      namespace: kyverno
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: kyverno
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
