---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: olm
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
    name: opencenter-olm-config
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/olm
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: olm
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
