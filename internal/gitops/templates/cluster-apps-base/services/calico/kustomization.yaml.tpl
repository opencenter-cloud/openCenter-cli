{{- /* Only render Calico GitOps manifests when install_method is "helm" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "kubespray") "helm") }}
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: tigera-operator
resources:
  - namespace.yaml
  - helmrepository.yaml
  - helmrelease.yaml
{{- end }}
