loki:
  auth_enabled: true
  storage:
    bucketNames:
      chunks: stage-cluster-loki
      ruler: stage-cluster-loki
      admin: stage-cluster-loki
    type: swift
    swift:
      auth_url: https://keystone.api.sjc3.rackspacecloud.com/v3/
      auth_version: 3
      internal: false
      username: prat4036
      password: 5d5e874bbdbe4112a95aef520f97ce0b
      user_domain_name: rackspace_cloud_domain
      project_name: 981977_Flex
      project_domain_name: rackspace_cloud_domain
      region_name: SJC3
      container_name: stage-cluster-loki
      max_retries: 5
      connect_timeout: 10s
      request_timeout: 30s
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
  resources:
    requests:
      cpu: 100m
      memory: 500Mi
    limits:
      cpu: "1"
      memory: 1Gi
  persistence:
    enabled: true
    size: 20Gi
    storageClass: csi-cinder-sc-delete
  podAntiAffinityPreset: soft
read:
  replicas: 3
  resources:
    requests:
      cpu: 100m
      memory: 500Mi
    limits:
      cpu: "1"
      memory: 1Gi
  persistence:
    enabled: true
    size: 20Gi
    storageClass: csi-cinder-sc-delete
  podAntiAffinityPreset: soft
backend:
  replicas: 3
  resources:
    requests:
      cpu: 100m
      memory: 400Mi
    limits:
      cpu: "1"
      memory: 1Gi
  persistence:
    enabled: true
    size: 20Gi
    storageClass: csi-cinder-sc-delete
  podAntiAffinityPreset: soft
gateway:
  replicas: 2
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi
  ingress:
    enabled: false
chunksCache:
  enabled: true
  memcached:
    replicaCount: 3
    resources:
      requests:
        cpu: 100m
        memory: 512Mi
      limits:
        cpu: "1"
        memory: 1Gi
resultsCache:
  enabled: true
  memcached:
    replicaCount: 3
    resources:
      requests:
        cpu: 100m
        memory: 512Mi
      limits:
        cpu: "1"
        memory: 1Gi
