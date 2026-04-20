{{- /* Only render Calico GitOps manifests when install_method is "helm" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "kubespray") "helm") }}
installation:
  enabled: true
  kubernetesProvider: ""
  calicoNetwork:
    bgp: Disabled
    ipPools:
      - cidr: "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods | default "10.42.0.0/16" }}"
        encapsulation: "{{ if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.VXLANMode | default "Always") "Always" }}VXLAN{{ else if eq .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.VXLANMode "CrossSubnet" }}VXLANCrossSubnet{{ else if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.IPIPMode | default "") "Always" }}IPIP{{ else if eq .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.IPIPMode "CrossSubnet" }}IPIPCrossSubnet{{ else }}VXLAN{{ end }}"
        natOutgoing: Enabled
        nodeSelector: all()
    nodeAddressAutodetectionV4:
      firstFound: true
  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  windowsDataplane: HNS
  {{- else }}
  windowsDataplane: Disabled
  {{- end }}
  serviceCIDRs:
    - "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices | default "10.43.0.0/16" }}"

{{- if .OpenCenter.Services.calico.KubeAPIServer }}
kubernetesServiceEndpoint:
  host: "{{ .OpenCenter.Services.calico.KubeAPIServer }}"
  port: "443"
{{- end }}
{{- end }}
