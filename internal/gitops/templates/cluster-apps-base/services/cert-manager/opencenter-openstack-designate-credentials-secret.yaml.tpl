{{- $openstack := .OpenCenter.Infrastructure.Cloud.OpenStack -}}
apiVersion: v1
kind: Secret
metadata:
  name: opencenter-openstack-designate-credentials-secret
type: Opaque
stringData:
  OS_AUTH_URL: {{ if $openstack }}{{ $openstack.AuthURL }}{{ end }}
  OS_REGION_NAME: {{ if $openstack }}{{ $openstack.Region }}{{ end }}
  OS_DOMAIN_NAME: {{ if $openstack }}{{ $openstack.Domain | default $openstack.DomainName }}{{ end }}
  OS_PROJECT_ID: {{ if $openstack }}{{ $openstack.ProjectID }}{{ end }}
  OS_PROJECT_NAME: {{ if $openstack }}{{ $openstack.ProjectName | default $openstack.TenantName }}{{ end }}
  OS_APPLICATION_CREDENTIAL_ID: {{ if $openstack }}{{ $openstack.ApplicationCredentialID }}{{ end }}
  OS_APPLICATION_CREDENTIAL_SECRET: {{ if $openstack }}{{ $openstack.ApplicationCredentialSecret }}{{ end }}
