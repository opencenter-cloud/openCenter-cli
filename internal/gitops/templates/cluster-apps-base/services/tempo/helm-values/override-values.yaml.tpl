global:
    storageClass: {{ .OpenCenter.Services.CSI.StorageClass }}
storage:
    trace:
        backend: s3
        s3:
            bucket: {{ OpenCenter.Cluster.Name }}-tempo
            endpoint: swift.api.{{ .OpenCenter.OpenStack.Region }}.rackspacecloud.com
            access_key: {{ Secrets.Tempo.AccessKey }}
            secret_key: {{ Secrets.Tempo.SecretKey }}
            region: {{ .OpenCenter.Services.Tempo.Region }}
            insecure: false
multitenancyEnabled: true
ingester:
    persistence:
        size: 50Gi
