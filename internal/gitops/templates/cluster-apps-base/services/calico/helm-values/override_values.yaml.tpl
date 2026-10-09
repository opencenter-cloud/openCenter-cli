{{- /* Only render Calico GitOps manifests when install_method is "helm" (default) */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "helm") "helm") }}
{{- $calico := .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico }}
{{- $autodetectMode := $calico.CalicoInterfaceAutodetect | trim | lower | default "first-found" }}
installation:
  enabled: true
  kubernetesProvider: ""
  # OpenStack nodes using an external cloud provider remain tainted until the
  # GitOps-managed CCM initializes them. Calico must become ready before Flux
  # can install that CCM, so every operator-managed control-plane workload
  # must tolerate the temporary bootstrap taint.
  {{- if eq .OpenCenter.Infrastructure.Provider "openstack" }}
  controlPlaneTolerations:
    - key: node.cloudprovider.kubernetes.io/uninitialized
      operator: Exists
      effect: NoSchedule
  {{- end }}
  calicoNetwork:
    bgp: Disabled
    ipPools:
      - cidr: "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods | default "10.42.0.0/16" }}"
        encapsulation: "{{ if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.VXLANMode | default "Always") "Always" }}VXLAN{{ else if eq .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.VXLANMode "CrossSubnet" }}VXLANCrossSubnet{{ else if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.IPIPMode | default "") "Always" }}IPIP{{ else if eq .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.IPIPMode "CrossSubnet" }}IPIPCrossSubnet{{ else }}VXLAN{{ end }}"
        natOutgoing: Enabled
        nodeSelector: all()
    nodeAddressAutodetectionV4:
      {{- if eq $autodetectMode "interface" }}
      interface: "{{ $calico.CNIIface | default "" | trim }}"
      {{- else if eq $autodetectMode "cidr" }}
      cidrs:
        - "{{ $calico.AutodetectCIDR | default "" | trim }}"
      {{- else if eq $autodetectMode "first-found" }}
      firstFound: true
      {{- else }}
      {{- fail (printf "unsupported calico_interface_autodetect mode %q; expected first-found, interface, or cidr" $autodetectMode) }}
      {{- end }}
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
