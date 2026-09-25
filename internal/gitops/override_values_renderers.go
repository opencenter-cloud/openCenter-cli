// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// templateRenderer creates a renderer that executes a Go template against the config.
func templateRenderer(tmpl string) OverrideValuesRenderer {
	return func(cfg v2.Config) (string, error) {
		funcMap := sprig.TxtFuncMap()
		funcMap["objectStorageBackend"] = func(serviceName string) string {
			return v2.ResolveObjectStorageBackend(&cfg, serviceName)
		}
		funcMap["fqdn"] = clusterFQDNFunc(cfg)
		funcMap["kubePrometheusStack"] = func() (kubePrometheusStackRenderContract, error) {
			return resolveKubePrometheusStackContract(cfg)
		}
		funcMap["mimirStorage"] = func() (mimirStorageTemplateData, error) {
			return resolveMimirStorageTemplateData(cfg)
		}
		t, err := template.New("override-values").Funcs(funcMap).Parse(tmpl)
		if err != nil {
			return "", err
		}
		var buf strings.Builder
		if err := t.Execute(&buf, cfg); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
}

type mimirStorageTemplateData struct {
	Backend                string
	BucketName             string
	RulerBucketName        string
	AlertmanagerBucketName string
	S3Endpoint             string
	S3Region               string
	BucketLookupType       string
	S3Insecure             bool
	CredentialHash         string
	SwiftAuthURL           string
	SwiftRegion            string
	SwiftAuthVersion       int
	SwiftCredentialID      string
	SwiftUsername          string
	SwiftProjectName       string
	SwiftProjectDomainName string
	SwiftUserDomainName    string
	SwiftDomainName        string
	SwiftContainerName     string
}

func resolveMimirStorageTemplateData(cfg v2.Config) (mimirStorageTemplateData, error) {
	mimir, _ := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	backend := ""
	if mimir != nil {
		backend = strings.ToLower(strings.TrimSpace(mimir.StorageType))
	}
	if backend != "s3" {
		// Swift is the legacy Mimir backend. Keep it as the compatibility
		// default when an older configuration omits the typed backend.
		backend = "swift"
	}
	if backend == "swift" && strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) != "openstack" {
		return mimirStorageTemplateData{}, fmt.Errorf("Mimir Swift renderer: resolved Swift storage is only supported on OpenStack infrastructure")
	}

	bucket := ""
	if mimir != nil {
		bucket = strings.TrimSpace(mimir.BucketName)
	}
	if bucket == "" {
		bucket = fmt.Sprintf("%s-mimir", cfg.ClusterName())
	}
	rulerBucket := ""
	if mimir != nil {
		rulerBucket = strings.TrimSpace(mimir.RulerBucketName)
	}
	if rulerBucket == "" {
		rulerBucket = bucket + "-ruler"
	}
	alertmanagerBucket := ""
	if mimir != nil {
		alertmanagerBucket = strings.TrimSpace(mimir.AlertmanagerBucketName)
	}
	if alertmanagerBucket == "" {
		alertmanagerBucket = bucket + "-alertmanager"
	}

	accessKey, secretKey := cfg.GetMimirS3Credentials()
	endpoint, region := "", ""
	bucketLookupType := "auto"
	var insecure bool
	credentialHash := ""
	swiftCredentialSecret := ""
	swiftAuthURL, swiftRegion, swiftCredentialID := "", "", ""
	swiftUsername, swiftProjectName, swiftProjectDomain := "", "", ""
	swiftUserDomain, swiftDomain, swiftContainer := "", "", ""
	swiftAuthVersion := 3
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if mimir != nil {
		if backend == "s3" {
			var err error
			endpoint, err = v2.MimirS3EndpointHost(mimir.S3Endpoint)
			if err != nil {
				return mimirStorageTemplateData{}, fmt.Errorf("Mimir S3 renderer: %w", err)
			}
		}
		region = strings.TrimSpace(mimir.S3Region)
		if mimir.S3ForcePathStyle {
			bucketLookupType = "path"
		}
		insecure = mimir.S3Insecure
		swiftAuthURL = strings.TrimSpace(mimir.SwiftAuthURL)
		swiftRegion = strings.TrimSpace(mimir.SwiftRegion)
		if mimir.SwiftAuthVersion > 0 {
			swiftAuthVersion = mimir.SwiftAuthVersion
		}
		swiftCredentialID = strings.TrimSpace(mimir.SwiftApplicationCredentialID)
		swiftUsername = strings.TrimSpace(mimir.SwiftUsername)
		swiftProjectName = strings.TrimSpace(mimir.SwiftProjectName)
		swiftProjectDomain = strings.TrimSpace(mimir.SwiftProjectDomainName)
		swiftUserDomain = strings.TrimSpace(mimir.SwiftUserDomainName)
		swiftDomain = strings.TrimSpace(mimir.SwiftDomainName)
		swiftContainer = strings.TrimSpace(mimir.SwiftContainerName)
	}
	if openstack != nil {
		if swiftAuthURL == "" {
			swiftAuthURL = strings.TrimSpace(openstack.AuthURL)
		}
		if swiftRegion == "" {
			swiftRegion = strings.TrimSpace(openstack.Region)
		}
		if swiftCredentialID == "" {
			swiftCredentialID = cfg.GetMimirSwiftApplicationCredentialID()
		}
		if swiftProjectName == "" {
			swiftProjectName = strings.TrimSpace(openstack.ProjectName)
			if swiftProjectName == "" {
				swiftProjectName = strings.TrimSpace(openstack.TenantName)
			}
		}
		if swiftProjectDomain == "" {
			swiftProjectDomain = strings.TrimSpace(openstack.ProjectDomainName)
			if swiftProjectDomain == "" {
				swiftProjectDomain = strings.TrimSpace(openstack.DomainName)
			}
			if swiftProjectDomain == "" {
				swiftProjectDomain = strings.TrimSpace(openstack.Domain)
			}
		}
		if swiftUserDomain == "" {
			swiftUserDomain = strings.TrimSpace(openstack.UserDomainName)
		}
		if swiftDomain == "" {
			swiftDomain = strings.TrimSpace(openstack.DomainName)
			if swiftDomain == "" {
				swiftDomain = strings.TrimSpace(openstack.Domain)
			}
		}
	}
	if swiftUserDomain == "" {
		swiftUserDomain = swiftDomain
	}
	if swiftDomain == "" {
		swiftDomain = swiftUserDomain
	}
	if swiftContainer == "" && mimir != nil {
		swiftContainer = strings.TrimSpace(mimir.BucketName)
	}
	if swiftContainer == "" {
		swiftContainer = fmt.Sprintf("%s-mimir", cfg.ClusterName())
	}
	if backend == "swift" {
		resolvedID, resolvedSecret, err := cfg.ResolveMimirSwiftCredentials()
		if err != nil {
			return mimirStorageTemplateData{}, fmt.Errorf("Mimir Swift renderer: %w", err)
		}
		swiftCredentialID = resolvedID
		swiftCredentialSecret = resolvedSecret
	}
	if backend == "s3" {
		if strings.TrimSpace(accessKey) == "" || strings.EqualFold(strings.TrimSpace(accessKey), v2.PlaceholderSecret) {
			return mimirStorageTemplateData{}, fmt.Errorf("Mimir S3 renderer: S3 access key is missing or still a placeholder")
		}
		if strings.TrimSpace(secretKey) == "" || strings.EqualFold(strings.TrimSpace(secretKey), v2.PlaceholderSecret) {
			return mimirStorageTemplateData{}, fmt.Errorf("Mimir S3 renderer: S3 secret key is missing or still a placeholder")
		}
		credentialHash = credentialRotationHash(accessKey, secretKey)
	} else {
		idMissing := strings.TrimSpace(swiftCredentialID) == "" || strings.EqualFold(strings.TrimSpace(swiftCredentialID), v2.PlaceholderSecret)
		secretMissing := strings.TrimSpace(swiftCredentialSecret) == "" || strings.EqualFold(strings.TrimSpace(swiftCredentialSecret), v2.PlaceholderSecret)
		if idMissing != secretMissing {
			return mimirStorageTemplateData{}, fmt.Errorf("Mimir Swift renderer: application credential ID and secret must be configured together")
		}
		credentialHash = credentialRotationHash(swiftCredentialID, swiftCredentialSecret)
	}

	return mimirStorageTemplateData{
		Backend:                backend,
		BucketName:             bucket,
		RulerBucketName:        rulerBucket,
		AlertmanagerBucketName: alertmanagerBucket,
		S3Endpoint:             endpoint,
		S3Region:               region,
		BucketLookupType:       bucketLookupType,
		S3Insecure:             insecure,
		CredentialHash:         credentialHash,
		SwiftAuthURL:           swiftAuthURL,
		SwiftRegion:            swiftRegion,
		SwiftAuthVersion:       swiftAuthVersion,
		SwiftCredentialID:      swiftCredentialID,
		SwiftUsername:          swiftUsername,
		SwiftProjectName:       swiftProjectName,
		SwiftProjectDomainName: swiftProjectDomain,
		SwiftUserDomainName:    swiftUserDomain,
		SwiftDomainName:        swiftDomain,
		SwiftContainerName:     swiftContainer,
	}, nil
}

func credentialRotationHash(values ...string) string {
	hash := sha256.New()
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// staticRenderer returns a renderer that always produces the same content.
func staticRenderer(content string) OverrideValuesRenderer {
	return func(_ v2.Config) (string, error) {
		return content, nil
	}
}

// --- Templates (moved from .tpl files) ---

const openstackCCMTemplate = `cloudConfig:
  global:
    auth-url: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}
    application-credential-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID }}
    application-credential-secret: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret }}
    domain-name: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Domain | default "default" }}
    region: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}
    tenant-name: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.TenantName }}
    tls-insecure: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
  loadBalancer:
    floating-network-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingNetworkID }}
    subnet-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.SubnetID }}
`

const openstackCSITemplate = `secret:
  enabled: true
  hostMount: false
  create: true
  filename: cloud.conf
  name: cinder-csi-cloud-config
  data:
    cloud.conf: |-
      [Global]
      auth-url = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}
      application-credential-id = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID }}
      application-credential-secret = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret }}
      domain-name = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Domain }}
      region = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}
      tenant-name = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.TenantName }}
      tls-insecure = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
`

const vsphereCsiTemplate = `global:
  config:
    existingSecret: "vsphere-csi"
    global:
      cluster-id: "{{ .OpenCenter.Meta.Name }}"
    csidriver:
      enabled: true
    storageclass:
      enabled: true
      name: "{{ .OpenCenter.Infrastructure.Storage.DefaultStorageClass }}"
      storagepolicyname: ""
      expansion: true
      default: true
      reclaimPolicy: Delete
      volumebindingmode: "Immediate"
      datastoreurl: {{ .Secrets.VSphereCsi.Datastoreurl }}
vsphere-cpi:
  enabled: true
  global:
    config:
      existingConfig:
        enabled: true
        type: Secret
        name: "vsphere-cpi-secret"
      secretsInline: false
controller:
  config:
    block-volume-snapshot: true
  replicaCount: 3
  snapshotter:
    image:
      registry: {{ (index .OpenCenter.Services "vsphere-csi").Image.Repository | default "registry.k8s.io" }}
      repository: sig-storage/csi-snapshotter
      tag: {{ (index .OpenCenter.Services "vsphere-csi").Image.Tag | default "v8.2.0" }}
      pullPolicy: IfNotPresent
    args:
      - "--v=4"
      - "--kube-api-qps=100"
      - "--kube-api-burst=100"
      - "--timeout=300s"
      - "--csi-address=$(ADDRESS)"
      - "--leader-election"
      - "--leader-election-lease-duration=120s"
      - "--leader-election-renew-deadline=60s"
      - "--leader-election-retry-period=30s"
snapshot:
  controller:
    enabled: true
    replicaCount: 1
`

type veleroTemplateData struct {
	BackupStorageLocationName string
	Provider                  string
	Bucket                    string
	Region                    string
	S3Endpoint                string
	S3ForcePathStyle          bool
	S3Insecure                bool
	CredentialsExistingSecret string
	PluginEnabled             bool
	PluginName                string
	PluginImage               string
	VSphereSnapshotClass      bool
}

func veleroRenderer(cfg v2.Config) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	storageType := v2.ResolveVeleroStorageBackend(&cfg)
	bucket := ""
	region := ""
	s3Endpoint := ""
	s3ForcePathStyle := false
	s3Insecure := false
	if service, ok := cfg.OpenCenter.Services["velero"].(*services.VeleroConfig); ok && service != nil {
		if configured := strings.ToLower(strings.TrimSpace(service.StorageType)); configured != "" {
			storageType = configured
		}
		bucket = strings.TrimSpace(service.BackupBucket)
		region = strings.TrimSpace(service.Region)
		s3Endpoint = strings.TrimSpace(service.S3Endpoint)
		s3ForcePathStyle = service.S3ForcePathStyle
		s3Insecure = service.S3Insecure
	}

	if storageType == "none" {
		return "---\nconfiguration: {}\nbackupsEnabled: false\n...\n", nil
	}

	data := veleroTemplateData{
		BackupStorageLocationName: "default",
		Bucket:                    bucket,
		Region:                    region,
		S3Endpoint:                s3Endpoint,
		S3ForcePathStyle:          s3ForcePathStyle,
		S3Insecure:                s3Insecure,
		CredentialsExistingSecret: "velero-cloud-credentials",
		VSphereSnapshotClass:      provider == "vmware" || provider == "vsphere",
	}

	switch storageType {
	case "swift":
		data.Provider = "community.openstack.org/openstack"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-openstack"
		data.PluginImage = "lirt/velero-plugin-for-openstack:v0.6.0"
	case "gcs":
		data.Provider = "velero.io/gcp"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-gcp"
		data.PluginImage = "velero/velero-plugin-for-gcp:v1.8.2"
	case "azure":
		data.Provider = "velero.io/azure"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-azure"
		data.PluginImage = "velero/velero-plugin-for-microsoft-azure:v1.10.1"
	default:
		data.Provider = "velero.io/aws"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-aws"
		data.PluginImage = "velero/velero-plugin-for-aws:v1.10.0"
	}

	if data.Region == "" {
		if provider == "openstack" && cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
			data.Region = strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
		}
		if data.Region == "" {
			data.Region = strings.TrimSpace(cfg.OpenCenter.Meta.Region)
		}
	}
	if data.Bucket == "" {
		data.Bucket = cfg.OpenCenter.Meta.Name + "-velero"
	}

	t, err := template.New("velero-values").Funcs(sprig.TxtFuncMap()).Parse(veleroTemplate)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const veleroTemplate = `---
configuration:
  features: EnableCSI
  defaultSnapshotMoveData: false
  defaultVolumesToFsBackup: false
  backupStorageLocation:
    - name: {{ .BackupStorageLocationName }}
      provider: {{ .Provider }}
      default: true
      bucket: {{ .Bucket }}
      config:
        region: {{ .Region }}
{{- if eq .Provider "velero.io/aws" }}
        s3Url: {{ .S3Endpoint }}
        s3ForcePathStyle: {{ .S3ForcePathStyle }}
        insecureSkipTLSVerify: {{ .S3Insecure }}
{{- end }}
  volumeSnapshotLocation: []
{{- if eq .Provider "velero.io/aws" }}
credentials:
  useSecret: true
  existingSecret: {{ .CredentialsExistingSecret }}
{{- end }}
{{- if .PluginEnabled }}
initContainers:
  - name: {{ .PluginName }}
    image: {{ .PluginImage }}
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
{{- end }}
snapshotsEnabled: true
backupsEnabled: true
deployNodeAgent: false
{{- if .VSphereSnapshotClass }}
extraObjects:
  - apiVersion: snapshot.storage.k8s.io/v1
    kind: VolumeSnapshotClass
    metadata:
      name: velero-vsphere-snapshot-class
      labels:
        velero.io/csi-volumesnapshot-class: "true"
    driver: csi.vsphere.vmware.com
    deletionPolicy: Delete
{{- end }}
`

const lokiTemplate = `{{- $loki := index .OpenCenter.Services "loki" -}}
{{- $storageType := objectStorageBackend "loki" -}}
{{- $bucketName := $loki.BucketName | default (printf "%s-loki" .OpenCenter.Meta.Name) -}}
{{- $storageClass := $loki.StorageClass | default .OpenCenter.Infrastructure.Storage.DefaultStorageClass -}}
{{- if eq $storageType "none" }}
deploymentMode: SingleBinary
singleBinary:
    replicas: 1
    persistence:
        enabled: true
        size: {{ $loki.VolumeSize | default 10 }}Gi
        storageClass: {{ $storageClass }}
read:
    replicas: 0
write:
    replicas: 0
backend:
    replicas: 0
{{- end }}
global:
    dnsService: coredns
loki:
{{- if eq $storageType "none" }}
    commonConfig:
        replication_factor: 1
{{- end }}
    storage:
{{- if eq $storageType "none" }}
        type: filesystem
        filesystem:
            chunks_directory: /var/loki/chunks
            rules_directory: /var/loki/rules
{{- else }}
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
            username: {{ $loki.SwiftUsername }}
            password: {{ .GetLokiSwiftPassword }}
            project_name: {{ $loki.SwiftProjectName }}
            project_domain_name: {{ $loki.SwiftProjectDomainName | default $loki.SwiftDomainName }}
            user_domain_name: {{ $loki.SwiftUserDomainName }}
            domain_name: {{ $loki.SwiftDomainName }}
            container_name: {{ $loki.SwiftContainerName | default $bucketName }}
{{- else }}
        s3:
            s3: null
            endpoint: {{ $loki.S3Endpoint }}
            region: {{ $loki.S3Region | default .OpenCenter.Meta.Region }}
            secretAccessKey: {{ .GetLokiS3SecretKey }}
            accessKeyId: {{ .GetLokiS3AccessKey }}
            signatureVersion: null
            s3ForcePathStyle: {{ $loki.S3ForcePathStyle }}
            insecure: {{ $loki.S3Insecure }}
            http_config: {}
            backoff_config: {}
            disable_dualstack: false
{{- end }}
{{ end }}
    schemaConfig:
        configs:
            - from: "2024-04-01"
              store: tsdb
              object_store: {{ if eq $storageType "none" }}filesystem{{ else }}{{ $storageType }}{{ end }}
              schema: v13
              index:
                  prefix: loki_index_
                  period: 24h
{{- if ne $storageType "none" }}
write:
    # Pin storageClass so PVCs never rely on the ambiguous cluster default during
    # the bootstrap window (transient Longhorn default / Cinder SC not yet created).
    persistence:
        storageClass: {{ $storageClass }}
    affinity:
        podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution: []
            preferredDuringSchedulingIgnoredDuringExecution:
                - weight: 100
                  podAffinityTerm:
                      topologyKey: kubernetes.io/hostname
                      labelSelector:
                          matchLabels:
                              app.kubernetes.io/name: loki
                              app.kubernetes.io/instance: loki
                              app.kubernetes.io/component: write
read:
    persistence:
        storageClass: {{ $storageClass }}
    affinity:
        podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution: []
            preferredDuringSchedulingIgnoredDuringExecution:
                - weight: 100
                  podAffinityTerm:
                      topologyKey: kubernetes.io/hostname
                      labelSelector:
                          matchLabels:
                              app.kubernetes.io/name: loki
                              app.kubernetes.io/instance: loki
                              app.kubernetes.io/component: read
backend:
    persistence:
        storageClass: {{ $storageClass }}
    affinity:
        podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution: []
            preferredDuringSchedulingIgnoredDuringExecution:
                - weight: 100
                  podAffinityTerm:
                      topologyKey: kubernetes.io/hostname
                      labelSelector:
                          matchLabels:
                              app.kubernetes.io/name: loki
                              app.kubernetes.io/instance: loki
                              app.kubernetes.io/component: backend
{{- end }}
`

const tempoTemplate = `{{- $tempo := index .OpenCenter.Services "tempo" -}}
{{- $storageType := objectStorageBackend "tempo" -}}
{{- $bucketName := $tempo.BucketName | default (printf "%s-tempo" .OpenCenter.Meta.Name) -}}
{{- $storageClass := $tempo.StorageClass | default .OpenCenter.Infrastructure.Storage.DefaultStorageClass -}}
# Pin the storage class explicitly so PVCs never rely on the ambiguous cluster
# default. During bootstrap there is a window where Longhorn is (transiently)
# the cluster default and the Cinder default SC may not exist yet; an unpinned
# StatefulSet PVC created in that window binds to the wrong backend permanently.
global:
    storageClass: {{ $storageClass }}
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
            endpoint: {{ $tempo.S3Endpoint | trimPrefix "https://" | trimPrefix "http://" }}
            access_key: {{ .GetTempoS3AccessKey }}
            secret_key: {{ .GetTempoS3SecretKey }}
            region: {{ $tempo.S3Region | default .OpenCenter.Meta.Region }}
            forcepathstyle: {{ $tempo.S3ForcePathStyle }}
            insecure: {{ $tempo.S3Insecure }}
{{- end }}
reportingEnabled: false
`

const mimirTemplate = `{{- $mimir := mimirStorage -}}
{{- $storageClass := .OpenCenter.Infrastructure.Storage.DefaultStorageClass -}}
global:
    dnsService: coredns
    podAnnotations:
        opencenter.io/mimir-credentials-hash: {{ $mimir.CredentialHash | quote }}
{{- if eq $mimir.Backend "s3" }}
    extraEnv:
        - name: MIMIR_S3_ACCESS_KEY_ID
          valueFrom:
              secretKeyRef:
                  name: opencenter-mimir-secret
                  key: s3-access-key-id
        - name: MIMIR_S3_SECRET_ACCESS_KEY
          valueFrom:
              secretKeyRef:
                  name: opencenter-mimir-secret
                  key: s3-secret-access-key
{{- else }}
    extraEnv:
        - name: MIMIR_SWIFT_APPLICATION_CREDENTIAL_SECRET
          valueFrom:
              secretKeyRef:
                  name: opencenter-mimir-secret
                  key: swift-application-credential-secret
{{- end }}
minio:
    enabled: false
{{- if (index .OpenCenter.Services "kafka-cluster").Enabled }}
# External kafka-cluster provides ingest storage, so disable the chart's
# bundled demo Kafka broker (chart default is kafka.enabled: true).
kafka:
    enabled: false
{{- else }}
# No external kafka-cluster, so the chart's own bundled Kafka broker stays
# enabled. Its chart-default PVC size (5Gi) is below this region's Cinder
# minimum (10Gi for the "Standard" volume type), same class of bug as the
# ingester/store_gateway/compactor/alertmanager fix below. Unlike those
# (Mimir's own components, which use persistentVolume.size), the bundled
# Kafka is a vendored sub-chart using the persistence.size/storageClassName
# convention instead.
kafka:
    persistence:
        size: 10Gi
        storageClassName: {{ $storageClass }}
{{- end }}
mimir:
    structuredConfig:
        # The chart's own usage_stats block only sets installation_mode; usage
        # reporting stays enabled by default and initializes its own bucket
        # client separately from blocks_storage, crashing every component
        # ("unable to find the expected container <cluster>-mimir") unless
        # explicitly disabled here.
        usage_stats:
            enabled: false
        blocks_storage:
{{- if eq $mimir.Backend "s3" }}
            backend: s3
            s3:
                bucket_name: {{ $mimir.BucketName | quote }}
                endpoint: {{ $mimir.S3Endpoint | quote }}
                region: {{ $mimir.S3Region | default .OpenCenter.Meta.Region | quote }}
                access_key_id: "${MIMIR_S3_ACCESS_KEY_ID}"
                secret_access_key: "${MIMIR_S3_SECRET_ACCESS_KEY}"
                bucket_lookup_type: {{ $mimir.BucketLookupType | quote }}
                insecure: {{ $mimir.S3Insecure }}
                http: {}
        ruler_storage:
            backend: s3
            s3:
                bucket_name: {{ $mimir.RulerBucketName | quote }}
                endpoint: {{ $mimir.S3Endpoint | quote }}
                region: {{ $mimir.S3Region | default .OpenCenter.Meta.Region | quote }}
                access_key_id: "${MIMIR_S3_ACCESS_KEY_ID}"
                secret_access_key: "${MIMIR_S3_SECRET_ACCESS_KEY}"
                bucket_lookup_type: {{ $mimir.BucketLookupType | quote }}
                insecure: {{ $mimir.S3Insecure }}
                http: {}
        alertmanager_storage:
            backend: s3
            s3:
                bucket_name: {{ $mimir.AlertmanagerBucketName | quote }}
                endpoint: {{ $mimir.S3Endpoint | quote }}
                region: {{ $mimir.S3Region | default .OpenCenter.Meta.Region | quote }}
                access_key_id: "${MIMIR_S3_ACCESS_KEY_ID}"
                secret_access_key: "${MIMIR_S3_SECRET_ACCESS_KEY}"
                bucket_lookup_type: {{ $mimir.BucketLookupType | quote }}
                insecure: {{ $mimir.S3Insecure }}
                http: {}
{{- else }}
            backend: swift
            swift:
                container_name: {{ $mimir.SwiftContainerName | quote }}
                auth_version: {{ $mimir.SwiftAuthVersion }}
                auth_url: {{ $mimir.SwiftAuthURL | quote }}
                region_name: {{ $mimir.SwiftRegion | default .OpenCenter.Meta.Region | quote }}
                application_credential_id: {{ $mimir.SwiftCredentialID | quote }}
                application_credential_secret: "${MIMIR_SWIFT_APPLICATION_CREDENTIAL_SECRET}"
                username: {{ $mimir.SwiftUsername | quote }}
                project_name: {{ $mimir.SwiftProjectName | quote }}
                project_domain_name: {{ $mimir.SwiftProjectDomainName | quote }}
                user_domain_name: {{ $mimir.SwiftUserDomainName | quote }}
                domain_name: {{ $mimir.SwiftDomainName | quote }}
{{- end }}
{{- if (index .OpenCenter.Services "kafka-cluster").Enabled }}
        ingest_storage:
            kafka:
                # kafka-cluster always deploys to the kafka-system namespace
                # (hardcoded in its kustomization/flux templates); the
                # kafka-cluster.namespace config field is not honored there.
                address: kafka-cluster-kafka-brokers.kafka-system.svc.cluster.local:9092
                topic: mimir-ingest
                auto_create_topic_enabled: true
                auto_create_topic_default_partitions: 1000
{{- end }}
# Chart defaults (1-2Gi) are below this region's Cinder minimum volume size
# (10Gi for the "Standard" volume type), which fails PVC provisioning outright.
# storageClass is pinned explicitly so PVCs never rely on the ambiguous cluster
# default (a bootstrap window can leave Longhorn as the transient default, and
# the Cinder default SC may not exist yet); an unpinned StatefulSet PVC created
# then binds to the wrong backend permanently.
ingester:
    persistentVolume:
        size: 10Gi
        storageClass: {{ $storageClass }}
store_gateway:
    persistentVolume:
        size: 10Gi
        storageClass: {{ $storageClass }}
compactor:
    persistentVolume:
        size: 10Gi
        storageClass: {{ $storageClass }}
alertmanager:
    persistentVolume:
        size: 10Gi
        storageClass: {{ $storageClass }}
`

const otelTemplate = `collectors:
  daemon:
    config:
      exporters:
        otlphttp/loki:
          endpoint: http://observability-loki-gateway.observability.svc.cluster.local/otlp
          headers:
            X-Scope-OrgID: "default"
          compression: gzip
          timeout: 30s
          retry_on_failure:
            enabled: true
            initial_interval: 1s
            max_interval: 10s
            max_elapsed_time: 0s
          sending_queue:
            enabled: true
            num_consumers: 10
            queue_size: 2000
        otlp/tempo:
          endpoint: observability-tempo-distributor.observability.svc.cluster.local:4317
          headers:
            X-Scope-OrgID: "default"
          tls:
            insecure: true
          compression: gzip
          timeout: 30s
          retry_on_failure:
            enabled: true
            initial_interval: 1s
            max_interval: 10s
            max_elapsed_time: 0s
          sending_queue:
            enabled: true
            num_consumers: 10
            queue_size: 2000
`

const headlampTemplate = `config:
    oidc:
        enabled: true
        externalSecret:
            enabled: false
        secret:
            create: true
        clientID: opencenter
        clientSecret: {{ .Secrets.Headlamp.OIDCClientSecret }}
        issuerURL: https://{{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" fqdn) }}/realms/opencenter
        scopes: openid profile email groups
        callbackURL: https://{{ (index .OpenCenter.Services "headlamp").Hostname | default (printf "headlamp.%s" fqdn) }}/oidc-callback
    pluginsDir: /build/plugins
initContainers:
    - command:
        - /bin/sh
        - -c
        - mkdir -p /build/plugins && cp -r /plugins/* /build/plugins/ && chown -R 100:101 /build
      image: ghcr.io/headlamp-k8s/headlamp-plugin-flux:latest
      imagePullPolicy: Always
      name: headlamp-plugins
      securityContext:
        runAsNonRoot: false
        privileged: false
        runAsUser: 0
        runAsGroup: 0
      volumeMounts:
        - mountPath: /build/plugins
          name: headlamp-plugins
volumeMounts:
    - mountPath: /build/plugins
      name: headlamp-plugins
volumes:
    - name: headlamp-plugins
      emptyDir: {}
`

const harborTemplate = `{{- $harbor := index .OpenCenter.Services "harbor" -}}
{{- $storageClass := $harbor.StorageClass | default .OpenCenter.Infrastructure.Storage.DefaultStorageClass -}}
{{- $storageType := lower ($harbor.StorageType | default "s3") -}}
externalURL: https://{{ $harbor.Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
logLevel: info
expose:
    type: clusterIP
persistence:
    enabled: true
    resourcePolicy: keep
    persistentVolumeClaim:
        # The registry PVC is the image backend in filesystem mode.
        registry:
            size: {{ $harbor.RegistryVolumeSize | default 100 }}Gi
            storageClass: {{ $storageClass }}
        jobservice:
            jobLog:
                size: {{ $harbor.JobserviceVolumeSize | default 10 }}Gi
                storageClass: {{ $storageClass }}
        database:
            size: {{ $harbor.DatabaseVolumeSize | default 10 }}Gi
            storageClass: {{ $storageClass }}
        redis:
            size: {{ $harbor.RedisVolumeSize | default 10 }}Gi
            storageClass: {{ $storageClass }}
        trivy:
            size: {{ $harbor.TrivyVolumeSize | default 10 }}Gi
            storageClass: {{ $storageClass }}
    # The registry PVC is the image backend in filesystem mode.
    imageChartStorage:
{{- if eq $storageType "filesystem" }}
        type: filesystem
        filesystem:
            rootdirectory: /storage
{{- else }}
        type: s3
        s3:
            region: {{ .OpenCenter.Meta.Region }}
            bucket: {{ $harbor.S3Bucket | default (printf "%s-harbor" .OpenCenter.Cluster.ClusterName) }}
            accesskey: {{ .GetHarborS3AccessKey }}
            secretkey: {{ .GetHarborS3SecretKey }}
            regionendpoint: {{ $harbor.S3Endpoint }}
            v4auth: true
            secure: true
            rootdirectory: images
{{- end }}
harborAdminPassword: {{ .Secrets.Harbor.AdminPassword | quote }}
metrics:
    enabled: true
    serviceMonitor:
        enabled: true
cache:
    enabled: true
    expireHours: 24
portal:
    replicas: 1
core:
    replicas: 1
jobservice:
    replicas: 1
registry:
    replicas: 1
    credentials:
        username: harbor-registry
        password: {{ .Secrets.Harbor.RegistryPassword | quote }}
        htpasswdString: ""
trivy:
    replicas: 1
database:
    internal:
        password: {{ .Secrets.Harbor.DatabasePassword | quote }}
exporter:
    replicas: 1
`

const kubePrometheusStackTemplate = `---
{{ $kps := index .OpenCenter.Services "kube-prometheus-stack" -}}
{{- $contract := kubePrometheusStack -}}
{{- $defaultSC := .OpenCenter.Infrastructure.Storage.DefaultStorageClass -}}
{{- $webhookURL := $kps.WebhookURL | trim -}}
fullnameOverride: {{ $contract.ReleaseName }}
alertmanager:
  enabled: true
  service:
    enabled: true
  alertmanagerSpec:
    externalUrl: https://{{ $kps.AlertmanagerHostname | default (printf "alertmanager.%s" fqdn) }}
    # Pin the PVC storage class (see prometheusSpec.storageSpec note).
    storage:
      volumeClaimTemplate:
        spec:
          storageClassName: {{ $kps.AlertmanagerStorageClass | default $defaultSC }}
  config:
    global:
      resolve_timeout: 5m
    inhibit_rules:
      - source_matchers: [severity = critical]
        target_matchers: [severity =~ warning|info]
        equal: [namespace, alertname]
      - source_matchers: [severity = warning]
        target_matchers: [severity = info]
        equal: [namespace, alertname]
      - source_matchers: [alertname = InfoInhibitor]
        target_matchers: [severity = info]
        equal: [namespace]
      - target_matchers: [alertname = InfoInhibitor]
    route:
      receiver: "null"
      group_by: [namespace, alertname]
      group_wait: 30s
      group_interval: 60s
      repeat_interval: 12h
      routes:
        - receiver: "null"
          matchers: [alertname = "Watchdog"]
{{- if $webhookURL }}
        - receiver: warning_alerts_receiver
          continue: false
          matchers: [severity =~ "warning"]
{{- end }}
        - receiver: alert_proxy_receiver
          continue: false
          matchers: [severity =~ "critical"]
    receivers:
      - name: "null"
{{- if $webhookURL }}
      - name: warning_alerts_receiver
        msteamsv2_configs:
          - send_resolved: true
            webhook_url: {{ $webhookURL | quote }}
{{- end }}
      - name: alert_proxy_receiver
        webhook_configs:
          - url: http://rackspace-alert-proxy.rackspace.svc.cluster.local/alert/process
            send_resolved: true
prometheus:
  enabled: true
  service:
    enabled: true
  prometheusSpec:
    externalUrl: https://{{ $kps.PrometheusHostname | default (printf "prometheus.%s" fqdn) }}
    externalLabels:
      cluster: {{ .OpenCenter.Meta.Name }}
      region: {{ .OpenCenter.Meta.Region }}
      customer: {{ .OpenCenter.Meta.Organization }}
    # Pin the PVC storage class so the Prometheus TSDB volume never relies on the
    # ambiguous cluster default during the bootstrap window (transient Longhorn
    # default / Cinder SC not yet created), which would bind it permanently.
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: {{ $kps.PrometheusStorageClass | default $defaultSC }}
grafana:
  enabled: true
  service:
    enabled: true
  # Grafana has its own fullname helper, so pin its override explicitly too.
  fullnameOverride: {{ $contract.GrafanaServiceName }}
  # Grafana's PVC (when persistence is enabled) must likewise pin its class.
  persistence:
    storageClassName: {{ $kps.GrafanaStorageClass | default $defaultSC }}
  admin:
    existingSecret: "grafana-admin-password"
    userKey: admin-user
    passwordKey: admin-password
  datasources:
    datasources.yaml:
      apiVersion: 1
      datasources:
        - name: Loki
          uid: loki-default
          type: loki
          access: proxy
          url: http://observability-loki-gateway.observability.svc.cluster.local
          isDefault: false
          jsonData:
            httpHeaderName1: "X-Scope-OrgID"
            maxLines: 1000
          secureJsonData:
            httpHeaderValue1: "default"
          editable: true
        - name: Tempo
          uid: tempo-default
          type: tempo
          access: proxy
          url: http://observability-tempo-query-frontend.observability.svc.cluster.local:3200
          isDefault: false
          jsonData:
            httpHeaderName1: x-scope-orgid
            maxLines: 1000
            pdcInjected: false
            tracesToLogsV2:
              customQuery: false
              datasourceUid: loki-default
              filterBySpanID: true
              filterByTraceID: true
            tracesToMetrics:
              datasourceUid: prometheus
          secureJsonData:
            httpHeaderValue1: "default"
          editable: true
`
