{{- $loki := index .OpenCenter.Services "loki" }}
{{- $storageType := $loki.LokiStorageType | default "swift" }}
loki:
  storage:
    bucketNames:
      chunks: {{ $loki.LokiBucketName }}
      ruler: {{ $loki.LokiBucketName }}
      admin: {{ $loki.LokiBucketName }}
    type: {{ $storageType }}
{{- if eq $storageType "swift" }}
    swift:
      auth_url: {{ $loki.SwiftAuthURL }}
      auth_version: {{ $loki.SwiftAuthVersion | default 3 }}
      internal: false
{{- if $loki.SwiftApplicationCredentialID }}
      # Using OpenStack Application Credentials (recommended)
      application_credential_id: {{ $loki.SwiftApplicationCredentialID | quote }}
      application_credential_secret: {{ .Secrets.Loki.SwiftApplicationCredentialSecret | quote }}
{{- else }}
      # Using legacy username/password authentication (deprecated)
      username: {{ $loki.SwiftUsername | quote }}
      password: {{ .Secrets.Loki.SwiftPassword | quote }}
      user_domain_name: {{ $loki.SwiftDomainName }}
      project_name: {{ $loki.SwiftProjectName }}
      project_domain_name: {{ $loki.SwiftDomainName }}
{{- end }}
      region_name: {{ $loki.SwiftRegion }}
      container_name: {{ $loki.SwiftContainerName | default $loki.LokiBucketName }}
{{- if $loki.SwiftUserDomainName }}
      user_domain_name: {{ $loki.SwiftUserDomainName }}
{{- end }}
      max_retries: 5
      connect_timeout: 10s
      request_timeout: 30s
{{- else if eq $storageType "s3" }}
    s3:
{{- if $loki.LokiS3Endpoint }}
      # Custom S3-compatible endpoint (MinIO, Ceph, DigitalOcean Spaces, etc.)
      endpoint: {{ $loki.LokiS3Endpoint }}
      s3ForcePathStyle: {{ $loki.LokiS3ForcePathStyle | default true }}
{{- if $loki.LokiS3Insecure }}
      insecure: true
{{- end }}
{{- else }}
      # Standard AWS S3
      s3: s3://{{ $loki.LokiS3Region }}/{{ $loki.LokiBucketName }}
      s3ForcePathStyle: false
{{- end }}
{{- if or .Secrets.Loki.S3AccessKeyID .Secrets.Loki.S3SecretAccessKey }}
      # Static credentials (prefer IAM roles when possible)
      accessKeyId: {{ .Secrets.Loki.S3AccessKeyID | quote }}
      secretAccessKey: {{ .Secrets.Loki.S3SecretAccessKey | quote }}
{{- end }}
      region: {{ $loki.LokiS3Region }}
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
  resources:
    requests:
      cpu: 100m
      memory: 500Mi
    limits:
      cpu: "1"
      memory: 1Gi
  persistence:
    enabled: true
    size: {{ $loki.LokiVolumeSize }}Gi
    storageClass: {{ $loki.LokiStorageClass }}
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
    size: {{ $loki.LokiVolumeSize }}Gi
    storageClass: {{ $loki.LokiStorageClass }}
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
    size: {{ $loki.LokiVolumeSize }}Gi
    storageClass: {{ $loki.LokiStorageClass }}
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
