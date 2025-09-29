---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: kube-prometheus-stack-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-kube-prometheus-stack
    namespace: flux-system
  path: applications/base/services/kube-prometheus-stack
  targetNamespace: observability
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: kube-prometheus-stack
      namespace: observability
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: kube-prometheus-stack
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: kube-prometheus-stack-override
  namespace: flux-system
spec:
  dependsOn:
    - name: envoy-gateway-api-base
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/kube-prometheus-stack
  targetNamespace: observability
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: kube-prometheus-stack
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
