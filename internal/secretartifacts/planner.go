// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

// Package secretartifacts plans secret manifests without depending on a secret
// backend or a GitOps renderer.
package secretartifacts

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

const secretFilename = "secret.yaml"

type Owner struct {
	LogicalService string
	Payload        map[string]interface{}
}

// Artifact describes one materialized secret manifest. Multiple logical owners
// may share one physical target and are merged deterministically.
type Artifact struct {
	// LogicalService is retained for compatibility and is the first owner in
	// deterministic order. Consumers should use Owners for complete identity.
	LogicalService string
	TargetService  string
	Path           string
	Payload        map[string]interface{}
	Owners         []string
	SourcePayloads map[string]map[string]interface{}
}

func (a Artifact) OwnerNames() []string {
	if len(a.Owners) > 0 {
		return append([]string(nil), a.Owners...)
	}
	if a.LogicalService != "" {
		return []string{a.LogicalService}
	}
	return nil
}

// Plan returns non-empty secret artifacts, grouped by physical target path.
func Plan(cfg *v2.Config) ([]Artifact, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	type source struct {
		name string
		data any
	}
	// keycloak is intentionally omitted: its admin/client secrets are never
	// consumed by any rendered manifest (admin bootstrap comes from the realm-import,
	// DB creds from postgres-operator), so materializing services/keycloak/secret.yaml
	// only created an orphaned artifact.
	fixed := []source{
		{"cert-manager", certManagerPayload(cfg)}, {"loki", lokiPayload(cfg)},
		{"mimir", mimirPayload(cfg)},
		{"headlamp", cfg.Secrets.Headlamp},
		{"weave-gitops", cfg.Secrets.WeaveGitOps}, {"grafana", grafanaPayload(cfg)},
		{"tempo", cfg.Secrets.Tempo}, {"alert-proxy", cfg.Secrets.AlertProxy},
		{"vsphere-csi", cfg.Secrets.VSphereCsi},
		{"etcd-backup", etcdBackupPayload(cfg)},
		{"harbor", harborPayload(cfg)},
		{"velero", veleroPayload(cfg)},
	}
	sources := append([]source(nil), fixed...)
	keys := make([]string, 0, len(cfg.Secrets.ServiceSecrets))
	for raw := range cfg.Secrets.ServiceSecrets {
		keys = append(keys, raw)
	}
	sort.Strings(keys)
	seenServices := make(map[string]string)
	for _, raw := range keys {
		service := normalizeServiceName(raw)
		if previous, exists := seenServices[service]; exists && previous != raw {
			return nil, fmt.Errorf("service_secrets keys %q and %q normalize to the same logical service %q", previous, raw, service)
		}
		seenServices[service] = raw
		if err := validateService(service); err != nil {
			return nil, fmt.Errorf("service_secrets %q: %w", raw, err)
		}
		// Canonical Harbor and Tempo secrets are supplied by the typed config
		// blocks above. Do not add a second owner for the same target artifact;
		// this also prevents filesystem/managed modes from reintroducing legacy
		// S3 keys through service_secrets.
		// Loki service_secrets remains a compatibility input when no typed final
		// values payload can be planned. Once the canonical payload exists, omit
		// the legacy owner so the artifact remains a single values.yaml entry.
		if service == "harbor" || service == "tempo" || (service == "loki" && lokiPayload(cfg) != nil) {
			continue
		}
		sources = append(sources, source{service, cfg.Secrets.ServiceSecrets[raw]})
	}

	byPath := make(map[string]*Artifact)
	for _, source := range sources {
		if (source.name == "loki" && v2.ResolveObjectStorageBackend(cfg, "loki") == "none") ||
			(source.name == "etcd-backup" && v2.ResolveObjectStorageBackend(cfg, "etcd-backup") == "none") ||
			(source.name == "velero" && v2.ResolveVeleroStorageBackend(cfg) == "none") {
			continue
		}
		payload, err := normalize(source.data)
		if err != nil {
			return nil, fmt.Errorf("normalize %s secrets: %w", source.name, err)
		}
		if len(payload) == 0 {
			continue
		}
		if err := validateService(source.name); err != nil {
			return nil, fmt.Errorf("secret service %q: %w", source.name, err)
		}
		target := source.name
		if source.name == "grafana" {
			target = "kube-prometheus-stack"
		}
		relPath := path.Join(targetRoot(cfg, target), target, secretFilename)
		artifact := byPath[relPath]
		if artifact == nil {
			artifact = &Artifact{TargetService: target, Path: relPath, Payload: map[string]interface{}{}, SourcePayloads: map[string]map[string]interface{}{}}
			byPath[relPath] = artifact
		}
		if _, exists := artifact.SourcePayloads[source.name]; exists {
			// A repeated logical owner is allowed only when the entire payload is
			// identical; this avoids map-order-dependent overwrites.
			if !reflect.DeepEqual(artifact.SourcePayloads[source.name], payload) {
				return nil, fmt.Errorf("conflicting duplicate secret owner %q for target %q", source.name, target)
			}
			continue
		}
		artifact.SourcePayloads[source.name] = payload
		artifact.Owners = append(artifact.Owners, source.name)
		for key, value := range payload {
			canonical := canonicalKey(key)
			for existingKey, existing := range artifact.Payload {
				if canonicalKey(existingKey) != canonical {
					continue
				}
				if !reflect.DeepEqual(existing, value) {
					return nil, fmt.Errorf("conflicting secret key %q for target %q (owners include %q)", canonical, target, source.name)
				}
				goto nextKey
			}
			artifact.Payload[key] = value
		nextKey:
		}
	}

	paths := make([]string, 0, len(byPath))
	for rel, artifact := range byPath {
		sort.Strings(artifact.Owners)
		if len(artifact.Owners) == 0 {
			continue
		}
		artifact.LogicalService = artifact.Owners[0]
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	result := make([]Artifact, 0, len(paths))
	for _, rel := range paths {
		artifact := byPath[rel]
		// Skip artifacts targeting disabled services from fixed legacy blocks.
		// Dynamic service_secrets entries are validated strictly by ValidateTargets;
		// fixed blocks are filtered here so secrets sync never materializes files
		// that cluster generate would later reject.
		if !isTargetServiceEnabled(cfg, artifact.TargetService) {
			continue
		}
		result = append(result, *artifact)
	}
	if err := ValidateTargets(cfg, result); err != nil {
		return nil, err
	}
	return result, nil
}

// grafanaPayload keeps older or hand-edited cluster configurations compatible
// with kube-prometheus-stack's existingSecret contract. The chart requires
// both admin-user and admin-password keys, while legacy configurations can
// contain only the password.
func grafanaPayload(cfg *v2.Config) map[string]interface{} {
	password := cfg.Secrets.Grafana.AdminPassword
	if strings.TrimSpace(password) == "" {
		return nil
	}
	user := cfg.Secrets.Grafana.AdminUser
	if strings.TrimSpace(user) == "" {
		user = "admin"
	}
	return map[string]interface{}{
		"admin_user":     user,
		"admin_password": password,
	}
}

func etcdBackupPayload(cfg *v2.Config) map[string]interface{} {
	if v2.ResolveObjectStorageBackend(cfg, "etcd-backup") == "none" {
		return nil
	}
	service, _ := cfg.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig)
	if service == nil && strings.TrimSpace(cfg.Secrets.EtcdBackup.AccessKeyID) == "" && strings.TrimSpace(cfg.Secrets.EtcdBackup.SecretAccessKey) == "" {
		return nil
	}
	payload := map[string]interface{}{
		"ETCDCTL_API": "3", "ETCDCTL_ENDPOINTS": "https://127.0.0.1:2379",
		"ETCDCTL_CACERT": "/etc/kubernetes/ssl/etcd/ca.crt", "ETCDCTL_CERT": "/etc/kubernetes/ssl/etcd/server.crt",
		"ETCDCTL_KEY": "/etc/kubernetes/ssl/etcd/server.key",
		"ACCESS_KEY":  cfg.Secrets.EtcdBackup.AccessKeyID, "SECRET_KEY": cfg.Secrets.EtcdBackup.SecretAccessKey,
	}
	if service != nil {
		endpoint := strings.TrimSpace(service.S3Endpoint)
		if endpoint == "" {
			endpoint = strings.TrimSpace(service.S3Host)
		}
		payload["S3_HOST"] = endpoint
		payload["S3_REGION"] = service.S3Region
		payload["S3_BUCKET_NAME"] = service.S3BucketName
	}
	return payload
}

func harborPayload(cfg *v2.Config) map[string]interface{} {
	credentials := cfg.Secrets.Harbor
	if anyBlank(credentials.AdminPassword, credentials.RegistryPassword, credentials.DatabasePassword) {
		return nil
	}
	values := map[string]interface{}{
		"harborAdminPassword": credentials.AdminPassword,
		"registry": map[string]interface{}{
			"credentials": map[string]interface{}{"password": credentials.RegistryPassword},
		},
		"database": map[string]interface{}{
			"internal": map[string]interface{}{"password": credentials.DatabasePassword},
		},
	}
	if v2.ResolveObjectStorageBackend(cfg, "harbor") == "s3" {
		accessKey, secretKey := cfg.GetHarborS3Credentials()
		values["persistence"] = map[string]interface{}{
			"imageChartStorage": map[string]interface{}{
				"s3": map[string]interface{}{
					"accesskey": accessKey,
					"secretkey": secretKey,
				},
			},
		}
	}
	return chartValuesPayload(values)
}

func lokiPayload(cfg *v2.Config) map[string]interface{} {
	backend := v2.ResolveObjectStorageBackend(cfg, "loki")
	// Keep the legacy inline renderer's portable S3 fallback unchanged while
	// allowing the final-values Secret to represent an explicitly configured
	// Loki Swift backend.
	if service, ok := cfg.OpenCenter.Services["loki"].(*services.LokiConfig); ok && strings.EqualFold(strings.TrimSpace(service.StorageType), "swift") {
		backend = "swift"
	}
	switch backend {
	case "s3":
		accessKey, secretKey := cfg.GetLokiS3Credentials()
		if anyBlank(accessKey, secretKey) {
			return nil
		}
		return chartValuesPayload(map[string]interface{}{
			"loki": map[string]interface{}{
				"storage": map[string]interface{}{
					"s3": map[string]interface{}{
						"accessKeyId":     accessKey,
						"secretAccessKey": secretKey,
					},
				},
			},
		})
	case "swift":
		password := cfg.GetLokiSwiftPassword()
		if strings.TrimSpace(password) == "" {
			return nil
		}
		return chartValuesPayload(map[string]interface{}{
			"loki": map[string]interface{}{
				"storage": map[string]interface{}{
					"swift": map[string]interface{}{"password": password},
				},
			},
		})
	default:
		return nil
	}
}

func chartValuesPayload(values map[string]interface{}) map[string]interface{} {
	data, err := yaml.Marshal(values)
	if err != nil {
		// All values above are strings/maps, so this is unreachable unless the
		// payload is changed to contain an unsupported value type.
		return nil
	}
	return map[string]interface{}{"values.yaml": string(data)}
}

func anyBlank(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

// mimirPayload contains only the credentials consumed by Mimir's effective
// backend. The values renderer consumes this artifact through secretKeyRef;
// neither backend is written into Helm values as plaintext.
func mimirPayload(cfg *v2.Config) map[string]interface{} {
	switch v2.ResolveObjectStorageBackend(cfg, "mimir") {
	case "s3":
		accessKey, secretKey := cfg.GetMimirS3AccessKey(), cfg.GetMimirS3SecretKey()
		if strings.TrimSpace(accessKey) == "" && strings.TrimSpace(secretKey) == "" {
			return nil
		}
		return map[string]interface{}{
			"s3-access-key-id":     accessKey,
			"s3-secret-access-key": secretKey,
		}
	case "swift":
		_, secret, err := cfg.ResolveMimirSwiftCredentials()
		if err != nil || strings.TrimSpace(secret) == "" {
			return nil
		}
		return map[string]interface{}{"swift-application-credential-secret": secret}
	default:
		return nil
	}
}

func veleroPayload(cfg *v2.Config) map[string]interface{} {
	if v2.ResolveVeleroStorageBackend(cfg) == "none" {
		return nil
	}
	access, secret := cfg.Secrets.Velero.AccessKeyID, cfg.Secrets.Velero.SecretAccessKey
	if strings.TrimSpace(access) == "" && strings.TrimSpace(secret) == "" {
		return nil
	}
	return map[string]interface{}{"cloud": fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n", access, secret)}
}

// certManagerPayload emits only the flat, string-valued legacy cert-manager
// secret fields into the generic services/cert-manager/secret.yaml. The
// multi-credential AWS/Cloudflare maps (CertManagerSecrets.AWS/.Cloudflare) are
// deliberately excluded: they are nested structures that are NOT valid
// stringData (Kubernetes requires flat string values) and are already rendered
// as flat per-credential Secrets by the cert-manager renderer
// (opencenter-aws-credentials-secret-<name>, etc.). Marshaling the whole
// CertManagerSecrets struct here previously produced a nested stringData.aws map
// that failed dry-run.
func certManagerPayload(cfg *v2.Config) map[string]interface{} {
	payload := map[string]interface{}{}
	//lint:ignore SA1019 legacy flat cert-manager field is intentionally retained for migration compatibility.
	//nolint:staticcheck // SA1019: preserve legacy cert-manager payload compatibility.
	if v := strings.TrimSpace(cfg.Secrets.CertManager.AWSAccessKey); v != "" {
		payload["aws_access_key"] = v
	}
	if v := strings.TrimSpace(cfg.Secrets.CertManager.AWSSecretAccessKey); v != "" {
		payload["aws_secret_access_key"] = v
	}
	if v := strings.TrimSpace(cfg.Secrets.CertManager.CloudflareAPIToken); v != "" {
		payload["cloudflare_api_token"] = v
	}
	if len(payload) == 0 {
		return nil
	}
	return payload
}

func normalizeServiceName(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "_", "-")
}
func canonicalKey(key string) string { return strings.ReplaceAll(strings.TrimSpace(key), "_", "-") }

func normalize(rawSecrets any) (map[string]interface{}, error) {
	if rawSecrets == nil {
		return nil, nil
	}
	if values, ok := rawSecrets.(map[string]any); ok {
		return filterNonEmptySecrets(values), nil
	}
	data, err := yaml.Marshal(rawSecrets)
	if err != nil {
		return nil, err
	}
	values := make(map[string]any)
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return filterNonEmptySecrets(values), nil
}

func filterNonEmptySecrets(values map[string]any) map[string]interface{} {
	filtered := make(map[string]interface{}, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				filtered[key] = typed
			}
		case nil:
		default:
			filtered[key] = value
		}
	}
	return filtered
}

func validateService(service string) error {
	if strings.TrimSpace(service) == "" || service == "." || service == ".." || strings.ContainsAny(service, `/\\`) {
		return fmt.Errorf("invalid service name")
	}
	return nil
}

// ValidateTargets verifies that materialized non-empty artifacts have a
// configured, enabled target when service topology is declared.
func ValidateTargets(cfg *v2.Config, artifacts []Artifact) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	for _, artifact := range artifacts {
		if len(cfg.OpenCenter.Services) == 0 && len(cfg.OpenCenter.ManagedServices) == 0 && len(cfg.OpenCenter.LegacyManaged) == 0 {
			continue
		}
		var found, enabled bool
		for raw, value := range cfg.OpenCenter.Services {
			if normalizeServiceName(raw) == artifact.TargetService {
				found = true
				enabled = enabled || serviceEnabled(value)
			}
		}
		managed := cfg.OpenCenter.ManagedServices
		if len(managed) == 0 {
			managed = cfg.OpenCenter.LegacyManaged
		}
		for raw, value := range managed {
			if normalizeServiceName(raw) == artifact.TargetService {
				found = true
				enabled = enabled || serviceEnabled(value)
			}
		}
		if !found {
			return fmt.Errorf("secret artifact %q targets missing service %q", artifact.Path, artifact.TargetService)
		}
		if !enabled {
			// Fixed legacy blocks may exist as placeholders for an optional
			// managed service; only explicit service_secrets must fail closed here.
			explicit := false
			for raw := range cfg.Secrets.ServiceSecrets {
				if normalizeServiceName(raw) == artifact.TargetService {
					explicit = true
				}
			}
			if !explicit {
				continue
			}
			return fmt.Errorf("secret artifact %q targets disabled service %q", artifact.Path, artifact.TargetService)
		}
	}
	return nil
}

func serviceEnabled(value any) bool {
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return true
	}
	field := v.FieldByName("Enabled")
	return !field.IsValid() || field.Kind() != reflect.Bool || field.Bool()
}

func targetRoot(cfg *v2.Config, target string) string {
	if cfg != nil && len(cfg.OpenCenter.Services) > 0 {
		if _, ok := cfg.OpenCenter.Services[target]; ok {
			return "services"
		}
	}
	managed := cfg.OpenCenter.ManagedServices
	if len(managed) == 0 {
		managed = cfg.OpenCenter.LegacyManaged
	}
	if _, ok := managed[target]; ok {
		return "managed-services"
	}
	return "services"
}

// isTargetServiceEnabled checks whether the target service is enabled in the
// cluster configuration. Returns true if the service is not found (conservative
// default — let ValidateTargets handle missing-service errors downstream).
func isTargetServiceEnabled(cfg *v2.Config, target string) bool {
	if cfg == nil {
		return true
	}
	if len(cfg.OpenCenter.Services) == 0 && len(cfg.OpenCenter.ManagedServices) == 0 && len(cfg.OpenCenter.LegacyManaged) == 0 {
		return true
	}
	for raw, value := range cfg.OpenCenter.Services {
		if normalizeServiceName(raw) == target {
			return serviceEnabled(value)
		}
	}
	managed := cfg.OpenCenter.ManagedServices
	if len(managed) == 0 {
		managed = cfg.OpenCenter.LegacyManaged
	}
	for raw, value := range managed {
		if normalizeServiceName(raw) == target {
			return serviceEnabled(value)
		}
	}
	// Service not found in config — don't filter it here, let ValidateTargets
	// handle the "targets missing service" error.
	return true
}
