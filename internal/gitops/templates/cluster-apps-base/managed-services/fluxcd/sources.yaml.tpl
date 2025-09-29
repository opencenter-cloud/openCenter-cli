---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: managed-services-sources
  namespace: flux-system
spec:
  interval: 1m
  path: ./applications/overlays/{{ .ClusterName }}/managed-services/sources
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  wait: true
