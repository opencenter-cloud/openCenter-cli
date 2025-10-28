---
cluster:
    name: {{ .ClusterName }}
cloudConfig:
    global:
        auth-url: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}
        application-credential-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID }}
        application-credential-secret: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret }}
        domain-name: "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Domain |default "Default" }}
        region: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}
        tenant-name: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.TenantName |default "Default" }}
        tls-insecure: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
    loadBalancer:
        floating-network-id:  {{ .OpenCenter.Infrastructure.Cloud.OpenStack.FloatingNetworkId }}
        subnet-id:  {{ .OpenCenter.Infrastructure.Cloud.OpenStack.SubnetId }}
