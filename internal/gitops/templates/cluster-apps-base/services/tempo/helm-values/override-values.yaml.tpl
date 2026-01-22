global:
    storageClass: {{ (index .OpenCenter.Services "tempo").StorageClass | default .OpenCenter.Storage.DefaultStorageClass }}
storage:
    trace:
        backend: {{ (index .OpenCenter.Services "tempo").StorageType | default "s3" }}
        s3:
            bucket: {{ (index .OpenCenter.Services "tempo").BucketName | default (printf "%s-tempo" .OpenCenter.Meta.Name) }}
            endpoint: {{ (index .OpenCenter.Services "tempo").S3Endpoint | default (printf "swift.api.%s.rackspacecloud.com" .OpenCenter.Meta.Region) }}
            access_key: {{ .GetTempoS3AccessKey }}
            secret_key: {{ .GetTempoS3SecretKey }}
            region: {{ (index .OpenCenter.Services "tempo").S3Region | default .OpenCenter.Meta.Region }}
            insecure: {{ (index .OpenCenter.Services "tempo").S3Insecure | default .OpenCenter.Meta.Region }}
multitenancyEnabled: true
ingester:
    persistence:
        size: {{ (index .OpenCenter.Services "tempo").VolumeSize | default 50 }}Gi
