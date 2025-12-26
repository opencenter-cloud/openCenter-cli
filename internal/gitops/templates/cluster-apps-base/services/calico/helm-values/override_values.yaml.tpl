installation:
  enabled: true
  kubernetesProvider: ""
  calicoNetwork:
  windowsDataplane: "Disabled"
  nodeAddressAutodetectionV4:
  interface: "enp3s0"
  ipPools:
  - cidr: "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods }}"
    encapsulation: "VXLAN"
    natOutgoing: Enabled
  serviceCIDRs:
  - "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices }}"

# Optionally configure the host and port used to access the Kubernetes API server.
{{- if .OpenCenter.Services.calico.KubeAPIServer }}
kubernetesServiceEndpoint:
  host: "{{ .OpenCenter.Services.calico.KubeAPIServer }}"
  port: "443"
{{- end }}
