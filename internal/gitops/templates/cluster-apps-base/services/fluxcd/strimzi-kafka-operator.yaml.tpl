---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: strimzi-kafka-operator-base
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
    name: opencenter-strimzi-kafka-operator
    namespace: flux-system
  path: applications/base/services/strimzi-kafka-operator
  targetNamespace: kafka-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: strimzi-kafka-operator
      namespace: kafka-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: envoy-gateway
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
