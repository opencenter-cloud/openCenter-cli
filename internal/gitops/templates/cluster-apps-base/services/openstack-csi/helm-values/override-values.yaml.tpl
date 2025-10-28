---
secret:
    enabled: true
    hostMount: true
    create: true
    filename: "cloud.conf"
    name: "{{ .ClusterName }}-cloud-config"
    data:
        cloud.conf: |
            [Global]
            auth-url = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}
            application-credential-id = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID }}
            application-credential-secret = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret }}
            region = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}
            tls-insecure = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
