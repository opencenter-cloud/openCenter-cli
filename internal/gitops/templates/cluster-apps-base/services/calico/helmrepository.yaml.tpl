{{- /* Only render Calico GitOps manifests when install_method is "helm" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "kubespray") "helm") }}
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: projectcalico
  namespace: flux-system
spec:
  interval: 1h
  url: https://docs.tigera.io/calico/charts
{{- end }}
