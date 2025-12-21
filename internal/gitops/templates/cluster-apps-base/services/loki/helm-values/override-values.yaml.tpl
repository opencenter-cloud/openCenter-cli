global:
    dnsService: coredns
loki:
    auth_enabled: true
    schemaConfig:
        configs:
            - from: "2025-01-01"
              store: tsdb
              object_store: s3
              schema: v13
              index:
                prefix: index_
                period: 24h
    storage:
        bucketNames:
            chunks: {{ .OpenCenter.Cluster.Name }}-loki
            ruler: {{ .OpenCenter.Cluster.Name }}-loki
            admin: {{ .OpenCenter.Cluster.Name }}-loki
        type: s3
        s3:
            s3: null
            endpoint: https://swift.api.{{ .OpenCenter.Cluster.ClusterRegion }}.rackspacecloud.com/v1/AUTH_ccfd4502116e41fd970e9bb6ebdcbbc6/
            region: null
            secretAccessKey: {{ .Secrets.Loki.AWSAccesKey }} 
            accessKeyId: {{ .Secrets.Loki.AWSAccesKeyID }}
            signatureVersion: null
            s3ForcePathStyle: false
            insecure: false
            http_config: {}
            # -- Check https://grafana.com/docs/loki/latest/configure/#s3_storage_config for more info on how to provide a backoff_config
            backoff_config: {}
            disable_dualstack: false
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
