{{- $tempo := index .OpenCenter.Services "tempo" -}}
{{- $storageType := $tempo.StorageType | default "s3" -}}
{{- $bucketName := $tempo.BucketName | default (printf "%s-tempo" .OpenCenter.Meta.Name) -}}
global:
    storageClass: {{ $tempo.StorageClass | default .OpenCenter.Infrastructure.Storage.DefaultStorageClass }}
storage:
    trace:
        backend: {{ $storageType }}
{{- if eq $storageType "swift" }}
        swift:
            auth_version: {{ $tempo.SwiftAuthVersion | default 3 }}
            auth_url: {{ $tempo.SwiftAuthURL }}
            region: {{ $tempo.SwiftRegion | default .OpenCenter.Meta.Region }}
            application_credential_id: {{ $tempo.SwiftApplicationCredentialID }}
            application_credential_secret: {{ .GetTempoSwiftApplicationCredentialSecret }}
            user_domain_name: {{ $tempo.SwiftUserDomainName }}
            domain_name: {{ $tempo.SwiftDomainName }}
            container_name: {{ $tempo.SwiftContainerName | default $bucketName }}
{{- else }}
        s3:
            bucket: {{ $bucketName }}
            endpoint: {{ $tempo.S3Endpoint | default (printf "swift.api.%s.rackspacecloud.com" .OpenCenter.Meta.Region) }}
            access_key: {{ .GetTempoS3AccessKey }}
            secret_key: {{ .GetTempoS3SecretKey }}
            region: {{ $tempo.S3Region | default .OpenCenter.Meta.Region }}
            forcepathstyle: {{ $tempo.S3ForcePathStyle }}
            insecure: {{ $tempo.S3Insecure }}
{{- end }}
multitenancyEnabled: true
ingester:
    persistence:
        size: {{ $tempo.VolumeSize | default 50 }}Gi
