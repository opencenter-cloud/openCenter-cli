---
# Renders to clusters/<cluster>/services.yaml.
#
# Bridges the flux-system reconciliation path (./clusters/<cluster>) to the
# per-service Flux Kustomization overlays under applications/overlays/<cluster>/
# services/fluxcd. Without this file, `flux bootstrap` sets up nothing beyond
# the source-controller and the root Kustomization - none of the services in
# the overlay get reconciled.
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: services
  namespace: flux-system
spec:
  interval: {{ .OpenCenter.GitOps.Flux.Interval | default "5m" }}
  prune: {{ .OpenCenter.GitOps.Flux.Prune | default true }}
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/fluxcd
  sourceRef:
    kind: GitRepository
    name: flux-system
