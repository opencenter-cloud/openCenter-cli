// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

// Package secretartifacts plans secret manifests without depending on a secret
// backend or a GitOps renderer.
package secretartifacts

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

const secretFilename = "secret.yaml"

// Dedicated secret filenames are Secrets the renderers emit with a filename of
// their own instead of the generic per-service secret.yaml, because the Helm
// chart or the Envoy SecurityPolicy references that exact Kubernetes Secret
// name. They carry credential material, so they are sync-owned: see
// DedicatedSecretFilename for why the planner has to know them.
const (
	GrafanaAdminPasswordSecretFilename = "grafana-admin-password.yaml"
	GrafanaOAuthSecretFilename         = "grafana-oauth-secret.yaml"
	GatewayOIDCSecretFilename          = "gateway-oidc-secret.yaml"
)

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
	credentialArtifacts, err := certManagerCredentialArtifacts(cfg)
	if err != nil {
		return nil, err
	}
	result = append(result, credentialArtifacts...)
	dedicatedArtifacts, err := dedicatedSecretArtifacts(cfg)
	if err != nil {
		return nil, err
	}
	// kube-prometheus-stack consumes Grafana's admin credential through the
	// dedicated grafana-admin-password.yaml. The generic secret.yaml carries the
	// same credential under the same Kubernetes identity, so the renderer omits it
	// and Kustomize never sees a duplicate resource; materializing both would have
	// sync write two Secrets that fight over one name.
	if grafanaAdminDedicatedSecretPlanned(dedicatedArtifacts) {
		result = dropGenericGrafanaSecret(result)
	}
	result = append(result, dedicatedArtifacts...)
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	if err := ValidateTargets(cfg, result); err != nil {
		return nil, err
	}
	return result, nil
}

// grafanaAdminDedicatedSecretPlanned reports whether the dedicated
// grafana-admin-password Secret is in the planned set.
func grafanaAdminDedicatedSecretPlanned(artifacts []Artifact) bool {
	want := "services/kube-prometheus-stack/" + GrafanaAdminPasswordSecretFilename
	for _, artifact := range artifacts {
		if artifact.Path == want {
			return true
		}
	}
	return false
}

// dropGenericGrafanaSecret removes the generic kube-prometheus-stack secret.yaml
// owned solely by the grafana logical service, which the dedicated admin Secret
// replaces. An artifact with any other owner is kept: a second credential owner
// on that target is not the admin password's to displace.
func dropGenericGrafanaSecret(artifacts []Artifact) []Artifact {
	generic := "services/kube-prometheus-stack/" + secretFilename
	kept := make([]Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		owners := artifact.OwnerNames()
		if artifact.Path == generic && len(owners) == 1 && owners[0] == "grafana" {
			continue
		}
		kept = append(kept, artifact)
	}
	return kept
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

// CertManagerCredentialArtifactPath returns the overlay-relative path of the
// SOPS-encrypted Secret holding one cert-manager DNS credential.
//
// The name is derived here, from the same resolved credential set the renderer
// iterates, so the materialized Secret and the ClusterIssuer's
// apiTokenSecretRef / accessKeyIDSecretRef can never disagree: a name computed
// independently on either side is how a plaintext credential ended up committed
// while sync encrypted only the legacy generic secret.yaml.
//
// provider must be "aws" or "cloudflare"; name is the credential map key, which
// validateCertManagerCredentialName already constrains to a DNS-1123 label.
func CertManagerCredentialArtifactPath(provider, name string) (string, error) {
	filename, err := CertManagerCredentialFilename(provider, name)
	if err != nil {
		return "", err
	}
	return path.Join("services", "cert-manager", filename), nil
}

// CertManagerCredentialFilename returns the Secret filename for one
// cert-manager DNS credential: opencenter-aws-credentials-secret-<name>.yaml or
// opencenter-cloudflare-credentials-secret-<name>.yaml.
//
// The name is validated as a DNS-1123 label rather than assumed. The renderer
// applies the same rule, but sync must not rely on the renderer having run: an
// unvalidated name would flow straight into an overlay-relative path that the
// ownership ledger later resolves for hashing and deletion.
func CertManagerCredentialFilename(provider, name string) (string, error) {
	prefix := certManagerCredentialPrefix(provider)
	if prefix == "" {
		return "", fmt.Errorf("unsupported cert-manager credential provider %q", provider)
	}
	if err := ValidateCertManagerCredentialName(name); err != nil {
		return "", err
	}
	return prefix + strings.TrimSpace(name) + ".yaml", nil
}

// ValidateCertManagerCredentialName enforces the DNS-1123 label rule the
// credential map keys must satisfy.
//
// A credential name is only ever safe to embed in a path once it is known to be
// a label, and the ownership ledger later resolves that path to hash and to
// delete. Exported so the renderer validates through this function instead of
// keeping a second, drift-prone copy of the contract.
func ValidateCertManagerCredentialName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("cert-manager credential name must not be empty")
	}
	if len(trimmed) > 63 {
		return fmt.Errorf("cert-manager credential name %q exceeds the 63-character DNS-1123 label limit", trimmed)
	}
	if !certManagerCredentialNamePattern.MatchString(trimmed) {
		return fmt.Errorf("cert-manager credential name %q must be a DNS-1123 label of lowercase letters, digits, and hyphens, starting and ending with a letter or digit", trimmed)
	}
	return nil
}

// certManagerCredentialNamePattern is the DNS-1123 label rule the credential
// map keys must satisfy.
var certManagerCredentialNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// certManagerCredentialProviders maps a DNS provider to the filename prefix of
// the Secret holding its credential. It is the single source of truth: the
// builder, the ownership-state admission test, and the renderer's own filename
// choice all read it, so a sync-materialized Secret and the ClusterIssuer's
// secretRef cannot drift apart.
var certManagerCredentialProviders = []struct {
	provider string
	prefix   string
}{
	{provider: "aws", prefix: "opencenter-aws-credentials-secret-"},
	{provider: "cloudflare", prefix: "opencenter-cloudflare-credentials-secret-"},
}

// certManagerCredentialPrefix returns the Secret filename prefix for a
// provider, or "" when the provider has no credential Secret.
func certManagerCredentialPrefix(provider string) string {
	for _, entry := range certManagerCredentialProviders {
		if strings.EqualFold(strings.TrimSpace(provider), entry.provider) {
			return entry.prefix
		}
	}
	return ""
}

// IsCertManagerCredentialFilename reports whether a basename is one this
// planner would construct for a credential Secret. It is the ownership-state
// admission test: the accepted set is derived from the same prefix list and
// label pattern the builder uses, so the two cannot drift apart.
func IsCertManagerCredentialFilename(base string) bool {
	if !strings.HasSuffix(base, ".yaml") {
		return false
	}
	stem := strings.TrimSuffix(base, ".yaml")
	for _, entry := range certManagerCredentialProviders {
		name := strings.TrimPrefix(stem, entry.prefix)
		if name != stem && certManagerCredentialNamePattern.MatchString(name) {
			return true
		}
	}
	return false
}

// certManagerCredentialArtifacts plans one Secret artifact per credential the
// cert-manager renderer will emit an issuer for.
//
// cfg.ResolvedCertManagerAWSCredentials / ...CloudflareCredentials are the
// single source of truth for which credentials are in force: enabled named map
// entries verbatim, plus the dns_provider-derived credential when none is
// enabled. Planning from that set is what makes `secrets sync` encrypt the
// credentials `cluster configure` writes; Plan's fixed source table cannot
// carry them because their targets are per-credential paths, not the one
// services/<service>/secret.yaml route.
//
// A credential whose secret material is absent or still the init placeholder is
// skipped, matching the renderer, which emits no issuer for it. Materializing
// one anyway would write a Secret whose values readiness already reports as
// missing, and sync would then prune it once the real value arrives.
func certManagerCredentialArtifacts(cfg *v2.Config) ([]Artifact, error) {
	if !isTargetServiceEnabled(cfg, "cert-manager") {
		return nil, nil
	}
	type credential struct {
		provider string
		name     string
		payload  map[string]interface{}
	}
	var credentials []credential
	for _, name := range sortedCredentialNames(cfg.ResolvedCertManagerAWSCredentials()) {
		cred := cfg.ResolvedCertManagerAWSCredentials()[name]
		if isPlaceholderSecret(cred.AWSAccessKey) || isPlaceholderSecret(cred.AWSSecretAccessKey) {
			// The renderer emits no issuer for a credential that is not fully
			// filled in, so materializing a Secret for it would create an orphan.
			continue
		}
		credentials = append(credentials, credential{provider: "aws", name: name, payload: map[string]interface{}{
			"aws-access-key":        strings.TrimSpace(cred.AWSAccessKey),
			"aws-secret-access-key": strings.TrimSpace(cred.AWSSecretAccessKey),
		}})
	}
	for _, name := range sortedCredentialNames(cfg.ResolvedCertManagerCloudflareCredentials()) {
		cred := cfg.ResolvedCertManagerCloudflareCredentials()[name]
		if isPlaceholderSecret(cred.APIToken) {
			continue
		}
		credentials = append(credentials, credential{provider: "cloudflare", name: name, payload: map[string]interface{}{
			"api-token": strings.TrimSpace(cred.APIToken),
		}})
	}

	artifacts := make([]Artifact, 0, len(credentials))
	for _, entry := range credentials {
		relPath, err := CertManagerCredentialArtifactPath(entry.provider, entry.name)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{
			LogicalService: entry.name,
			TargetService:  "cert-manager",
			Path:           relPath,
			Payload:        entry.payload,
			Owners:         []string{entry.name},
			SourcePayloads: map[string]map[string]interface{}{entry.name: entry.payload},
		})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func sortedCredentialNames[V any](credentials map[string]V) []string {
	names := make([]string, 0, len(credentials))
	for name := range credentials {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// isPlaceholderSecret reports whether a credential value carries no real secret
// material: empty, or the init placeholder.
func isPlaceholderSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, v2.PlaceholderSecret)
}

// DedicatedSecretFilename reports whether an overlay-relative path is a Secret
// manifest the renderers emit with a dedicated filename instead of the generic
// per-service secret.yaml, and returns the directory that owns it.
//
// The renderers and the planner must agree on exactly one set: a name listed
// here but not rendered produces an orphan that sync prunes, and a rendered
// name missing from here stays committed as plaintext because sync never claims
// the path. The renderers consult this function instead of keeping their own copy.
func DedicatedSecretFilename(relative string) (dir string, filename string, ok bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(filepath.ToSlash(relative))), "/")
	if len(parts) != 3 || parts[0] != "services" {
		return "", "", false
	}
	// The directory is part of the match, not just the filename: a file named
	// gateway-oidc-secret.yaml somewhere else is not the rendered manifest, and
	// claiming it would let sync own a path no consumer reads.
	switch parts[1] + "/" + parts[2] {
	case "kube-prometheus-stack/" + GrafanaAdminPasswordSecretFilename,
		"kube-prometheus-stack/" + GrafanaOAuthSecretFilename:
		return "kube-prometheus-stack", parts[2], true
	case "oidc-security-policies/" + GatewayOIDCSecretFilename:
		// The Envoy SecurityPolicies and their Secret live in their own
		// services/oidc-security-policies layer (service-keycloak descriptor),
		// which is not a service, so the directory is fixed rather than derived
		// from the owning service's namespace.
		return "oidc-security-policies", parts[2], true
	}
	return "", "", false
}

// dedicatedSecretArtifacts plans the Secrets the renderers emit outside the
// generic services/<service>/secret.yaml route.
//
// These three manifests carry real credentials interpolated straight out of the
// cluster config, and the renderers cannot encrypt them: the staged-tree
// encryption hook is scoped to helm-values/override-values.yaml. Before this
// planner claimed them, `cluster validate` failed on plaintext Secret material
// that `secrets sync` -- the documented remedy -- reported as `unchanged` and
// never opened, so the operator had no way to pass validation.
//
// Each artifact reproduces the renderer's filename, Secret name, namespace and
// key set exactly: sync regenerates the whole file, so a divergence would
// silently rename or relocate a Secret that a Helm chart or an Envoy
// SecurityPolicy resolves by exact identity. The gates below mirror the
// renderer gates, so a planned artifact always has a rendered file to land in.
func dedicatedSecretArtifacts(cfg *v2.Config) ([]Artifact, error) {
	type dedicated struct {
		// dir is the overlay directory the renderer writes into, and the only
		// thing the artifact path needs. targetService is the enabled service the
		// gate consults; it is deliberately not reported as the artifact's target,
		// because oidc-security-policies is a layer rather than a service and
		// ValidateTargets would reject it as a missing one.
		dir           string
		filename      string
		owner         string
		targetService string
		payload       map[string]interface{}
	}
	var planned []dedicated

	if password := strings.TrimSpace(cfg.Secrets.Grafana.AdminPassword); password != "" && declaresService(cfg, "kube-prometheus-stack") && isTargetServiceEnabled(cfg, "kube-prometheus-stack") {
		// Same gate as kubePrometheusStackOverlayFilesRenderer: an empty admin
		// password must not produce a Secret, or Flux reconciles it over the one
		// applied during bootstrap and locks the operator out of Grafana.
		user := cfg.Secrets.Grafana.AdminUser
		if strings.TrimSpace(user) == "" {
			user = "admin"
		}
		planned = append(planned, dedicated{
			dir:           "kube-prometheus-stack",
			filename:      GrafanaAdminPasswordSecretFilename,
			owner:         "grafana-admin",
			targetService: "kube-prometheus-stack",
			payload:       map[string]interface{}{"admin-user": user, "admin-password": password},
		})
	}

	if cfg.OpenCenter.Identity.OIDC.Enabled && oidcClientSecretsConfigured(cfg) &&
		declaresService(cfg, "kube-prometheus-stack") && isTargetServiceEnabled(cfg, "kube-prometheus-stack") {
		planned = append(planned, dedicated{
			dir:           "kube-prometheus-stack",
			filename:      GrafanaOAuthSecretFilename,
			owner:         "grafana-oauth",
			targetService: "kube-prometheus-stack",
			payload: map[string]interface{}{
				"client_id":     "grafana-oauth",
				"client_secret": cfg.Secrets.Keycloak.GrafanaClientSecret,
			},
		})
		// The SecurityPolicy layer renders only when keycloak AND
		// kube-prometheus-stack are enabled, on top of the OIDC gate above.
		if isTargetServiceEnabled(cfg, "keycloak") {
			planned = append(planned, dedicated{
				dir:           "oidc-security-policies",
				filename:      GatewayOIDCSecretFilename,
				owner:         "gateway-oidc",
				targetService: "keycloak",
				payload:       map[string]interface{}{"client-secret": cfg.Secrets.Keycloak.ClientSecret},
			})
		}
	}

	artifacts := make([]Artifact, 0, len(planned))
	for _, entry := range planned {
		if !isTargetServiceEnabled(cfg, entry.targetService) {
			continue
		}
		artifacts = append(artifacts, Artifact{
			LogicalService: entry.owner,
			TargetService:  entry.dir,
			Path:           path.Join("services", entry.dir, entry.filename),
			Payload:        entry.payload,
			Owners:         []string{entry.owner},
			SourcePayloads: map[string]map[string]interface{}{entry.owner: entry.payload},
		})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

// oidcClientSecretsConfigured reports whether both realm client secrets are real
// credentials. It is the planner's copy of the oidcClientSecretsConfigured
// template function that gates the OIDC SecurityPolicy manifests; the gitops
// package owns the template helper and cannot import this package, so the two
// are kept in sync by TestOIDCClientSecretsGateMatchesRenderer.
func oidcClientSecretsConfigured(cfg *v2.Config) bool {
	return !isPlaceholderSecret(cfg.Secrets.Keycloak.ClientSecret) &&
		!isPlaceholderSecret(cfg.Secrets.Keycloak.GrafanaClientSecret)
}

// declaresService reports whether the configuration names a service in its
// topology at all, enabled or not. The overlay renderers are reached through the
// service's descriptor, so a service absent from the map renders nothing and a
// Secret planned for it would be an orphan sync later prunes.
func declaresService(cfg *v2.Config, service string) bool {
	if cfg == nil {
		return false
	}
	for raw := range cfg.OpenCenter.Services {
		if normalizeServiceName(raw) == service {
			return true
		}
	}
	managed := cfg.OpenCenter.ManagedServices
	if len(managed) == 0 {
		managed = cfg.OpenCenter.LegacyManaged
	}
	for raw := range managed {
		if normalizeServiceName(raw) == service {
			return true
		}
	}
	return false
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
		// A dedicated Secret lives in the layer that renders it, and for
		// gateway-oidc-secret that layer is services/oidc-security-policies -- a
		// descriptor layer, not a service. Its enablement gate is checked in
		// dedicatedSecretArtifacts, so no service remains to resolve here.
		if _, _, dedicated := DedicatedSecretFilename(artifact.Path); dedicated {
			continue
		}
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
