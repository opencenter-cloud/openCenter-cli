---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: longhorn-base
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
    name: opencenter-longhorn
    namespace: flux-system
  path: applications/base/services/longhorn
  targetNamespace: longhorn-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: longhorn
      namespace: longhorn-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: longhorn
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: longhorn-override
  namespace: flux-system
spec:
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/longhorn
  targetNamespace: longhorn-system
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: longhorn
      namespace: longhorn-system
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: longhorn
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
