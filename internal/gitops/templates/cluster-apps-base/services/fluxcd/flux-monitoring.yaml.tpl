---
# The flux-system PodMonitor needs monitoring.coreos.com/v1 CRDs from
# kube-prometheus-stack. Applying it via a Kustomization with dependsOn
# ensures the CRDs land first.
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: flux-monitoring
  namespace: flux-system
spec:
  dependsOn:
    - name: kube-prometheus-stack-base
  interval: {{ .OpenCenter.GitOps.Flux.Interval | default "5m" }}
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/fluxcd/fluxcd-configs
  prune: {{ .OpenCenter.GitOps.Flux.Prune | default true }}
