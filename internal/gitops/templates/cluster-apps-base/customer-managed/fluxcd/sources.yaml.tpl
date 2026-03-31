---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: customer-managed-sources
  namespace: flux-system
spec:
  interval: 10m
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/customer-managed/sources
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  wait: true
