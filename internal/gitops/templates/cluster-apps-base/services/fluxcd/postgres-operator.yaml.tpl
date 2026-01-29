---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: postgres-operator-base
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
    name: opencenter-postgres-operator
    namespace: flux-system
  path: applications/base/services/postgres-operator
  targetNamespace: postgres-operator
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: postgres-operator
      namespace: postgres-operator
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: postgres-operator
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: postgres-operator-override
  namespace: flux-system
spec:
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/postgres-operator
  targetNamespace: postgres-operator
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: postgres-operator
      namespace: postgres-operator
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: postgres-operator
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
