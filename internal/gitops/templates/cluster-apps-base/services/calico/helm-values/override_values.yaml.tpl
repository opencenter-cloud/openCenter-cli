installation:
  enabled: true
  kubernetesProvider: ""
  calicoNetwork:
    windowsDataplane: "Disabled"
    nodeAddressAutodetectionV4:
      interface: "enp3s0"
    ipPools:
      - cidr: "10.42.0.0/16"
        encapsulation: "VXLAN"
        natOutgoing: Enabled
  serviceCIDRs:
    - "10.43.0.0/16"

# Optionally configure the host and port used to access the Kubernetes API server.
kubernetesServiceEndpoint:
  host: "10.2.184.10"
  port: "443"
