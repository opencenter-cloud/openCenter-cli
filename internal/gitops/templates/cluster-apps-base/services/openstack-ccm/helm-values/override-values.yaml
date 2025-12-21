cloudConfig:
  global:
    auth-url: https://keystone.api.{{ .OpenCenter.OpenStack.Region }}.rackspacecloud.com/v3 
    application-credential-id: {{ .OpenCenter.Secrets.Openstack-ccm.AppCredsID }}
    application-credential-secret: {{ .OpenCenter.Secrets.Openstack-ccm.AppCredsSecret }}
    domain-name: {{ .OpenCenter.OpenStack.DomainName  || defauult rackspace_cloud_domain }}
    region: {{ .OpenCenter.OpenStack.Region }}
    tenant-name: {{ .OpenCenter.OpenStack.TenantName }}
    tls-insecure: {{ .OpenCenter.OpenStack.Region | default false }}
  loadBalancer:
    floating-network-id: {{ .OpenCenter.OpenStack.FloatingNetworkID}}
    subnet-id: {{ .OpenCenter.OpenStack.SubnetID }}