global:
    dnsService: coredns
alertmanager:
    enabled: false
metaMonitoring:
    dashboards:
        enabled: true
    serviceMonitor:
        enabled: true
    prometheusRule:
        enabled: true
        mimirAlerts: true
        mimirRules: true
kafka:
    enabled: false
mimir:
    structuredConfig:
        blocks_storage:
            backend: s3
            s3:
                bucket_name: {{ .OpenCenter.Cluster.ClusterName }}-mimir
                endpoint: swift.api.{{ .OpenCenter.Meta.Region }}.rackspacecloud.com
                access_key_id: {{ .Secrets.Global.AWS.Application.AccessKey | default "PLACEHOLDER-MIMIR-ACCESS-KEY" }}
                secret_access_key: {{ .Secrets.Global.AWS.Application.SecretAccessKey | default "PLACEHOLDER-MIMIR-SECRET-KEY" }}
        ingest_storage:
            kafka:
                address: kafka-cluster-kafka-brokers.kafka-system.svc.cluster.local:9092
                topic: mimir-ingest
                auto_create_topic_enabled: true
                auto_create_topic_default_partitions: 1000
        limits:
            ingestion_rate: 100000
            ingestion_burst_size: 500000
            max_global_series_per_user: 2000000
            compactor_blocks_retention_period: 14d
compactor:
    persistentVolume:
        storageClassName: {{ .OpenCenter.Storage.DefaultStorageClass }}
        size: 20Gi
distributor:
    replicas: 2
ingester:
    persistentVolume:
        storageClassName: {{ .OpenCenter.Storage.DefaultStorageClass }}
        size: 15Gi
    replicas: 3
    topologySpreadConstraints: {}
    affinity:
        podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
                - labelSelector:
                    matchExpressions:
                        - key: app.kubernetes.io/component
                          operator: In
                          values:
                            - ingester
                  topologyKey: kubernetes.io/hostname
    zoneAwareReplication:
        topologyKey: kubernetes.io/hostname
admin-cache:
    enabled: true
    replicas: 2
chunks-cache:
    enabled: true
    replicas: 2
    allocatedMemory: 500
index-cache:
    enabled: true
    replicas: 3
metadata-cache:
    enabled: true
results-cache:
    enabled: true
minio:
    enabled: false
overrides_exporter:
    replicas: 1
querier:
    replicas: 1
query_frontend:
    replicas: 2
ruler:
    enabled: false
store_gateway:
    persistentVolume:
        storageClassName: {{ .OpenCenter.Storage.DefaultStorageClass }}
        size: 15Gi
    replicas: 3
    topologySpreadConstraints: {}
    affinity:
        podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
                - labelSelector:
                    matchExpressions:
                        - key: app.kubernetes.io/component
                          operator: In
                          values:
                            - store-gateway
                  topologyKey: kubernetes.io/hostname
    zoneAwareReplication:
        topologyKey: kubernetes.io/hostname
gateway:
    replicas: 2
