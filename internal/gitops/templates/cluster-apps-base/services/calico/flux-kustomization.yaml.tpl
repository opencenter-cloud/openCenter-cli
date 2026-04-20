{{- /* Only render Calico GitOps manifests when install_method is "helm" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "kubespray") "helm") }}
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: calico
  namespace: flux-system
spec:
  interval: 10m
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/calico
  prune: true
  wait: true
  sourceRef:
    kind: GitRepository
    name: flux-system
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: calico
      namespace: tigera-operator
{{- end }}
