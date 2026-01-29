---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: envoy-gateway-api-base
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
    name: opencenter-gateway-api
    namespace: flux-system
  path: applications/base/services/gateway-api
  targetNamespace: envoy-gateway-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: envoy-gateway-api
      namespace: envoy-gateway-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: envoy-gateway
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: envoy-gateway-api-override
  namespace: flux-system
spec:
  dependsOn:
    - name: envoy-gateway-api-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/gateway-api
  targetNamespace: envoy-gateway-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: envoy-gateway-api
      namespace: envoy-gateway-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: envoy-gateway
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
