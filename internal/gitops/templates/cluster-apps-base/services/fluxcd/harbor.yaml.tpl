---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: harbor-override
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: harbor-namespace
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/harbor
  targetNamespace: harbor
  prune: true
  wait: true
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: harbor
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: harbor-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: kube-prometheus-stack-base
      namespace: flux-system
    - name: harbor-override
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-harbor
    namespace: flux-system
  path: applications/base/services/harbor
  targetNamespace: harbor
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: harbor
      namespace: harbor
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: harbor
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
