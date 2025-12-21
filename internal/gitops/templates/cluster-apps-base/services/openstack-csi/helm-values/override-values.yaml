secret:
  enabled: true
  hostMount: true
  create: true
  filename: cloud.conf
  name: cinder-csi-cloud-config
  data:
    cloud.conf: |-
      [Global]
      auth-url = https://keystone.api.{{ .OpenCenter.OpenStack.Region }}.rackspacecloud.com/v3
      application-credential-id = {{ .OpenCenter.Secrets.OpenStackCSI.AppCredsID }}
      application-credential-secret = {{ .OpenCenter.Secrets.OpenStackCSI.AppCredsSecret }}
      domain-name = {{ .OpenCenter.OpenStack.DomainName }}
      region = {{ .OpenCenter.OpenStack.Region }}
      tenant-name = {{ .OpenCenter.OpenStack.TenantName }}
      tls-insecure =  false