{{- $loki := index .OpenCenter.Services "loki" -}}
{{- $storageType := $loki.StorageType | default "s3" -}}
{{- $bucketName := $loki.BucketName | default (printf "%s-loki" .OpenCenter.Meta.Name) -}}
global:
    dnsService: coredns
loki:
    auth_enabled: true
    schemaConfig:
        configs:
            - from: "2025-01-01"
              store: tsdb
              object_store: {{ $storageType }}
              schema: v13
              index:
                prefix: index_
                period: 24h
    storage:
        bucketNames:
            chunks: {{ $bucketName }}
            ruler: {{ $bucketName }}
            admin: {{ $bucketName }}
        type: {{ $storageType }}
{{- if eq $storageType "swift" }}
        swift:
            auth_version: {{ $loki.SwiftAuthVersion | default 3 }}
            auth_url: {{ $loki.SwiftAuthURL }}
            region_name: {{ $loki.SwiftRegion | default .OpenCenter.Meta.Region }}
            application_credential_id: {{ $loki.SwiftApplicationCredentialID }}
            application_credential_secret: {{ .GetLokiSwiftApplicationCredentialSecret }}
            user_domain_name: {{ $loki.SwiftUserDomainName }}
            domain_name: {{ $loki.SwiftDomainName }}
            container_name: {{ $loki.SwiftContainerName | default $bucketName }}
{{- else }}
        s3:
            s3: null
            endpoint: {{ $loki.S3Endpoint | default (printf "https://swift.api.%s.rackspacecloud.com" .OpenCenter.Meta.Region) }}
            region: {{ $loki.S3Region | default .OpenCenter.Meta.Region }}
            secretAccessKey: {{ .GetLokiS3SecretKey }}
            accessKeyId: {{ .GetLokiS3AccessKey }}
            signatureVersion: null
            s3ForcePathStyle: {{ $loki.S3ForcePathStyle }}
            insecure: {{ $loki.S3Insecure }}
            http_config: {}
            # -- Check https://grafana.com/docs/loki/latest/configure/#s3_storage_config for more info on how to provide a backoff_config
            backoff_config: {}
            disable_dualstack: false
{{- end }}
    # Local pathing used by the charted components
    storage_config:
        tsdb_shipper:
            active_index_directory: /var/loki/index
            cache_location: /var/loki/index-cache
    # Scraping (Prometheus)
    serviceMonitor:
        enabled: true
write:
    replicas: 3
    persistence:
        enabled: true
        size: 100Gi
    podAntiAffinityPreset: soft
read:
    replicas: 3
    persistence:
        enabled: true
        size: 50Gi
    podAntiAffinityPreset: soft
backend:
    replicas: 3
    persistence:
        enabled: true
        size: 50Gi
    podAntiAffinityPreset: soft
gateway:
    replicas: 2
    ingress:
        enabled: false
chunksCache:
    enabled: true
    memcached:
        replicaCount: 3
resultsCache:
    enabled: true
    memcached:
        replicaCount: 3
