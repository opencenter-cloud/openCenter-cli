package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestRenderClusterAppsCertManagerCloudflare(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("cloudflare-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "cloudflare-guided.sjc3.k8s.opencenter.cloud"

	// Use new map-based credentials
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"prod": {
			Enabled:  true,
			APIToken: "cf-token",
			DNSZones: []string{"cloudflare-guided.sjc3.k8s.opencenter.cloud"},
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = "cloudflare"
	certManager.Email = "ops@example.com"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")

	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	if !strings.Contains(kustomization, "./opencenter-cloudflare-credentials-secret-prod.yaml") {
		t.Fatalf("expected Cloudflare secret in kustomization:\n%s", kustomization)
	}
	if strings.Contains(kustomization, "./opencenter-aws-credentials-secret") {
		t.Fatalf("did not expect Route53 AWS secret in Cloudflare kustomization:\n%s", kustomization)
	}

	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-prod-issuer.yaml"))
	if !strings.Contains(issuer, "cloudflare:") {
		t.Fatalf("expected Cloudflare solver in issuer:\n%s", issuer)
	}
	if !strings.Contains(issuer, "apiTokenSecretRef") {
		t.Fatalf("expected Cloudflare token secret ref in issuer:\n%s", issuer)
	}

	secret := mustReadFile(t, filepath.Join(base, "opencenter-cloudflare-credentials-secret-prod.yaml"))
	if !strings.Contains(secret, "api-token: cf-token") {
		t.Fatalf("expected Cloudflare API token in secret:\n%s", secret)
	}
}

func TestRenderClusterAppsCertManagerDesignate(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("designate-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "designate-guided.sjc3.k8s.opencenter.cloud"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.api.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "sjc3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectID = "project-123"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectName = "project-name"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = "rackspace"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = "designate"
	certManager.Email = "ops@example.com"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")

	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	if !strings.Contains(kustomization, "./opencenter-openstack-designate-credentials-secret.yaml") {
		t.Fatalf("expected Designate secret in kustomization:\n%s", kustomization)
	}

	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-designate-issuer.yaml"))
	if !strings.Contains(issuer, "webhook:") {
		t.Fatalf("expected webhook solver in issuer:\n%s", issuer)
	}
	if !strings.Contains(issuer, "groupName: acme.syseleven.de") {
		t.Fatalf("expected Designate webhook group name in issuer:\n%s", issuer)
	}
	if !strings.Contains(issuer, "solverName: designatedns") {
		t.Fatalf("expected Designate webhook solver name in issuer:\n%s", issuer)
	}

	secret := mustReadFile(t, filepath.Join(base, "opencenter-openstack-designate-credentials-secret.yaml"))
	if !strings.Contains(secret, "OS_APPLICATION_CREDENTIAL_ID: app-cred-id") {
		t.Fatalf("expected Designate application credential id in secret:\n%s", secret)
	}
	if !strings.Contains(secret, "OS_APPLICATION_CREDENTIAL_SECRET: app-cred-secret") {
		t.Fatalf("expected Designate application credential secret in secret:\n%s", secret)
	}
}

func TestRenderClusterAppsLokiSwift(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("loki-swift-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Services["loki"] = &configservices.LokiConfig{
		BaseConfig:             configservices.BaseConfig{Enabled: true, Namespace: "observability"},
		StorageType:            "swift",
		BucketName:             "loki-container",
		SwiftAuthURL:           "https://identity.api.example.com/v3",
		SwiftRegion:            "SJC3",
		SwiftAuthVersion:       3,
		SwiftUsername:          "loki-svc",
		SwiftProjectName:       "loki-project",
		SwiftProjectDomainName: "rackspace",
		SwiftContainerName:     "loki-container",
		SwiftUserDomainName:    "rackspace",
		SwiftDomainName:        "rackspace",
	}
	cfg.Secrets.Loki.SwiftPassword = "swift-secret"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	overrideValues := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "loki", "helm-values", "override-values.yaml"))
	if !strings.Contains(overrideValues, "type: swift") {
		t.Fatalf("expected swift storage type in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "username: loki-svc") {
		t.Fatalf("expected swift username in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "password: swift-secret") {
		t.Fatalf("expected swift password in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "project_name: loki-project") {
		t.Fatalf("expected swift project_name in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "container_name: loki-container") {
		t.Fatalf("expected swift container name in Loki values:\n%s", overrideValues)
	}
	// OCTR-674: schemaConfig.object_store must match storage type.
	if !strings.Contains(overrideValues, "object_store: swift") {
		t.Fatalf("expected schemaConfig object_store: swift in Loki values:\n%s", overrideValues)
	}
	// OCTR-674: global.dnsService must be set to coredns.
	if !strings.Contains(overrideValues, "dnsService: coredns") {
		t.Fatalf("expected global.dnsService: coredns in Loki values:\n%s", overrideValues)
	}
}

func TestRenderMimirOverrideValues(t *testing.T) {
	cfg := newDefault("mimir-guided")
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	openstack.AuthURL = "https://identity.api.example.com/v3"
	openstack.Region = "SJC3"
	openstack.ProjectID = "project-id"
	openstack.ApplicationCredentialID = "app-cred-id"
	openstack.Domain = "rackspace"
	openstack.DomainName = "rackspace"
	openstack.UserDomainName = "rackspace"
	cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = "mimir-swift-secret"

	mimirValues := renderOverrideValues(t, cfg, "mimir")
	if !strings.Contains(mimirValues, "dnsService: coredns") {
		t.Fatalf("expected global.dnsService: coredns in Mimir values:\n%s", mimirValues)
	}
	if !strings.Contains(mimirValues, "backend: swift") || !strings.Contains(mimirValues, "application_credential_secret: mimir-swift-secret") {
		t.Fatalf("expected configured Swift storage in Mimir values:\n%s", mimirValues)
	}
	if !strings.Contains(mimirValues, "minio:\n    enabled: false") {
		t.Fatalf("expected bundled MinIO to be disabled in Mimir values:\n%s", mimirValues)
	}
	if strings.Contains(mimirValues, "backend: s3") || strings.Contains(mimirValues, "PLACEHOLDER") {
		t.Fatalf("did not expect S3 or placeholder storage credentials in Mimir values:\n%s", mimirValues)
	}
	// No external kafka-cluster, so there must be no Kafka ingest_storage wiring.
	// (The chart's bundled Kafka broker stays enabled in this case and gets a PVC
	// size override — see the bundled-Kafka assertion below — so we no longer
	// assert the absence of any "kafka:" key, only the ingest_storage wiring.)
	if strings.Contains(mimirValues, "ingest_storage:") {
		t.Fatalf("did not expect Kafka ingest storage when kafka-cluster is disabled:\n%s", mimirValues)
	}
	// The bundled Kafka broker (active when external kafka-cluster is disabled)
	// must get a >=10Gi PVC via the sub-chart's persistence.size key (Cinder min).
	if !strings.Contains(mimirValues, "kafka:\n    persistence:\n        size: 10Gi") {
		t.Fatalf("expected bundled Kafka PVC size override (persistence.size: 10Gi) when kafka-cluster is disabled:\n%s", mimirValues)
	}

	kafka := cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig)
	kafka.Enabled = true
	// kafka-cluster ignores the namespace field and always deploys to
	// kafka-system, so the Mimir address must point there regardless.
	kafka.Namespace = "strimzi"
	mimirValues = renderOverrideValues(t, cfg, "mimir")
	if !strings.Contains(mimirValues, "address: kafka-cluster-kafka-brokers.kafka-system.svc.cluster.local:9092") {
		t.Fatalf("expected Mimir Kafka address to point at kafka-system:\n%s", mimirValues)
	}
	if strings.Contains(mimirValues, "strimzi") {
		t.Fatalf("Mimir Kafka address must not follow the non-functional configured namespace:\n%s", mimirValues)
	}
	if !strings.Contains(mimirValues, "kafka:\n    enabled: false") {
		t.Fatalf("expected bundled Kafka disabled when external kafka-cluster is enabled:\n%s", mimirValues)
	}
}

func TestRenderLokiOverrideValuesAffinity(t *testing.T) {
	cfg := newDefault("loki-affinity-guided")
	lokiValues := renderOverrideValues(t, cfg, "loki")

	for _, component := range []string{"write", "read", "backend"} {
		// The affinity sub-block (not required to be the first key under the
		// component) must carry soft hostname anti-affinity for the component.
		expected := "affinity:\n        podAntiAffinity:\n            requiredDuringSchedulingIgnoredDuringExecution: []\n            preferredDuringSchedulingIgnoredDuringExecution:\n                - weight: 100\n                  podAffinityTerm:\n                      topologyKey: kubernetes.io/hostname\n                      labelSelector:\n                          matchLabels:\n                              app.kubernetes.io/name: loki\n                              app.kubernetes.io/instance: loki\n                              app.kubernetes.io/component: " + component
		if !strings.Contains(lokiValues, expected) {
			t.Fatalf("expected soft hostname anti-affinity for Loki %s:\n%s", component, lokiValues)
		}
	}
}

// TestRenderObservabilityPVCStorageClassPinned verifies loki/tempo/mimir pin an
// explicit storageClass on their PVCs (to the infra default), so PVCs never rely
// on the ambiguous cluster default during the bootstrap window (transient
// Longhorn default / Cinder SC not yet created) and never mis-bind permanently.
func TestRenderObservabilityPVCStorageClassPinned(t *testing.T) {
	cfg := newDefault("obs-storageclass-guided")
	sc := cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass
	if sc == "" {
		t.Fatal("test config must have a default storage class")
	}

	// Loki SimpleScalable components (write/read/backend) each pin
	// persistence.storageClass. Assert the pinned block appears once per component.
	lokiValues := renderOverrideValues(t, cfg, "loki")
	pinned := "persistence:\n        storageClass: " + sc
	if got := strings.Count(lokiValues, pinned); got < 3 {
		t.Fatalf("expected Loki to pin persistence.storageClass %q on write/read/backend (>=3 occurrences), got %d:\n%s", sc, got, lokiValues)
	}
	for _, component := range []string{"write", "read", "backend"} {
		if !strings.Contains(lokiValues, "app.kubernetes.io/component: "+component) {
			t.Fatalf("expected Loki %s component block present:\n%s", component, lokiValues)
		}
	}

	// Tempo pins the chart-wide global.storageClass.
	tempoValues := renderOverrideValues(t, cfg, "tempo")
	if !strings.Contains(tempoValues, "global:\n    storageClass: "+sc) {
		t.Fatalf("expected Tempo global.storageClass %q:\n%s", sc, tempoValues)
	}

	// Mimir pins storageClass per stateful component.
	mimirValues := renderOverrideValues(t, cfg, "mimir")
	for _, component := range []string{"ingester", "store_gateway", "compactor", "alertmanager"} {
		if !strings.Contains(mimirValues, component+":\n    persistentVolume:\n        size: 10Gi\n        storageClass: "+sc) {
			t.Fatalf("expected Mimir %s to pin storageClass %q:\n%s", component, sc, mimirValues)
		}
	}

	// kube-prometheus-stack pins storageClassName on Prometheus, Alertmanager, and
	// Grafana PVCs (they default to the infra SC when no per-component class is set).
	kpsValues := renderOverrideValues(t, cfg, "kube-prometheus-stack")
	if got := strings.Count(kpsValues, "storageClassName: "+sc); got < 3 {
		t.Fatalf("expected kube-prometheus-stack to pin storageClassName %q on Prometheus/Alertmanager/Grafana (>=3), got %d:\n%s", sc, got, kpsValues)
	}
}

// TestRenderKafkaClusterPVCStorageClassPinned verifies the kafka-cluster Strimzi
// persistent-claim volumes pin an explicit storage class, so early Kafka PVCs
// never mis-bind to Longhorn during the bootstrap window.
func TestRenderKafkaClusterPVCStorageClassPinned(t *testing.T) {
	cfg := newDefault("kafka-storageclass-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig).Enabled = true
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps: %v", err)
	}

	sc := cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass
	kafkaPVC := mustReadFile(t, filepath.Join(
		cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays",
		cfg.ClusterName(), "services", "kafka-cluster", "kafka-persistent.yaml"))

	// Both node pools (controller 10Gi, broker 20Gi) must carry the pinned class.
	if got := strings.Count(kafkaPVC, "class: "+sc); got < 2 {
		t.Fatalf("expected kafka-cluster to pin storage class %q on both persistent-claim volumes (>=2), got %d:\n%s", sc, got, kafkaPVC)
	}
}

func renderOverrideValues(t *testing.T, cfg v2.Config, serviceName string) string {
	t.Helper()

	spec, ok := newBuiltInRenderCatalog().Lookup(serviceName)
	if !ok || spec.OverrideValuesRenderer == nil {
		t.Fatalf("missing override-values renderer for %q", serviceName)
	}

	values, err := spec.OverrideValuesRenderer(cfg)
	if err != nil {
		t.Fatalf("render %s override-values: %v", serviceName, err)
	}
	return values
}

func TestRenderClusterAppsTempoS3(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("tempo-s3-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Services["tempo"] = &configservices.TempoConfig{
		BaseConfig:  configservices.BaseConfig{Enabled: true, Namespace: "observability"},
		StorageType: "s3",
		BucketName:  "tempo-container",
		// Fully-qualified endpoint; template must strip the scheme (OCTR: minio-go
		// rejects endpoints with a scheme/path).
		S3Endpoint: "https://s3.example.com",
		S3Region:   "SJC3",
	}
	cfg.Secrets.Tempo.AccessKey = "tempo-s3-access"
	cfg.Secrets.Tempo.SecretKey = "tempo-s3-secret"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	overrideValues := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "tempo", "helm-values", "override-values.yaml"))
	if !strings.Contains(overrideValues, "backend: s3") {
		t.Fatalf("expected s3 backend in Tempo values:\n%s", overrideValues)
	}
	// 5c: endpoint must have the scheme stripped (bare host, no https://).
	if !strings.Contains(overrideValues, "endpoint: s3.example.com") {
		t.Fatalf("expected scheme-stripped s3 endpoint in Tempo values:\n%s", overrideValues)
	}
	if strings.Contains(overrideValues, "endpoint: https://") {
		t.Fatalf("did not expect fully-qualified https endpoint in Tempo values:\n%s", overrideValues)
	}
	// 5a: usage-report must be disabled via the chart's top-level key.
	if !strings.Contains(overrideValues, "reportingEnabled: false") {
		t.Fatalf("expected reportingEnabled: false in Tempo values:\n%s", overrideValues)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestRenderClusterAppsCertManagerMultiCredential(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("multi-cred")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "multi-cred.sjc3.k8s.opencenter.cloud"

	// Configure multiple AWS and Cloudflare credentials
	cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
		"production": {
			Enabled:            true,
			AWSAccessKey:       "AKIAPROD123",
			AWSSecretAccessKey: "prodSecretKey",
			Region:             "us-east-1",
			DNSZones:           []string{"prod.example.com"},
		},
		"staging": {
			Enabled:            true,
			AWSAccessKey:       "AKIASTAGE456",
			AWSSecretAccessKey: "stageSecretKey",
			Region:             "us-west-2",
			DNSZones:           []string{"staging.example.com"},
		},
		"disabled-cred": {
			Enabled:            false,
			AWSAccessKey:       "SHOULD_NOT_APPEAR",
			AWSSecretAccessKey: "SHOULD_NOT_APPEAR",
		},
	}
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cf-main": {
			Enabled:  true,
			APIToken: "cf-main-token",
			DNSZones: []string{"cf.example.com"},
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.Email = "ops@example.com"
	certManager.Region = "us-east-1"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")

	// Verify kustomization includes all enabled credentials
	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	for _, expected := range []string{
		"./opencenter-aws-credentials-secret-production.yaml",
		"./opencenter-aws-credentials-secret-staging.yaml",
		"./opencenter-cloudflare-credentials-secret-cf-main.yaml",
		"./letsencrypt-production-issuer.yaml",
		"./letsencrypt-staging-issuer.yaml",
		"./letsencrypt-cf-main-issuer.yaml",
	} {
		if !strings.Contains(kustomization, expected) {
			t.Errorf("expected %q in kustomization:\n%s", expected, kustomization)
		}
	}

	// Verify disabled credential is NOT rendered
	if strings.Contains(kustomization, "disabled-cred") {
		t.Fatalf("disabled credential should not appear in kustomization:\n%s", kustomization)
	}

	// Verify AWS production secret
	prodSecret := mustReadFile(t, filepath.Join(base, "opencenter-aws-credentials-secret-production.yaml"))
	if !strings.Contains(prodSecret, "name: opencenter-aws-credentials-secret-production") {
		t.Fatalf("expected production secret name:\n%s", prodSecret)
	}
	if !strings.Contains(prodSecret, "access-key-id: AKIAPROD123") {
		t.Fatalf("expected production access key:\n%s", prodSecret)
	}

	// Verify AWS staging secret
	stageSecret := mustReadFile(t, filepath.Join(base, "opencenter-aws-credentials-secret-staging.yaml"))
	if !strings.Contains(stageSecret, "name: opencenter-aws-credentials-secret-staging") {
		t.Fatalf("expected staging secret name:\n%s", stageSecret)
	}

	// Verify Cloudflare secret
	cfSecret := mustReadFile(t, filepath.Join(base, "opencenter-cloudflare-credentials-secret-cf-main.yaml"))
	if !strings.Contains(cfSecret, "name: opencenter-cloudflare-credentials-secret-cf-main") {
		t.Fatalf("expected cloudflare secret name:\n%s", cfSecret)
	}
	if !strings.Contains(cfSecret, "api-token: cf-main-token") {
		t.Fatalf("expected cloudflare API token:\n%s", cfSecret)
	}

	// Verify production issuer references correct secret
	prodIssuer := mustReadFile(t, filepath.Join(base, "letsencrypt-production-issuer.yaml"))
	if !strings.Contains(prodIssuer, "name: letsencrypt-production") {
		t.Fatalf("expected production issuer name:\n%s", prodIssuer)
	}
	if !strings.Contains(prodIssuer, `name: "opencenter-aws-credentials-secret-production"`) {
		t.Fatalf("expected production secret ref in issuer:\n%s", prodIssuer)
	}
	if !strings.Contains(prodIssuer, "region: us-east-1") {
		t.Fatalf("expected production region in issuer:\n%s", prodIssuer)
	}

	// Verify staging issuer uses its own region
	stageIssuer := mustReadFile(t, filepath.Join(base, "letsencrypt-staging-issuer.yaml"))
	if !strings.Contains(stageIssuer, "region: us-west-2") {
		t.Fatalf("expected staging region in issuer:\n%s", stageIssuer)
	}

	// Verify cloudflare issuer
	cfIssuer := mustReadFile(t, filepath.Join(base, "letsencrypt-cf-main-issuer.yaml"))
	if !strings.Contains(cfIssuer, "cloudflare:") {
		t.Fatalf("expected cloudflare solver in issuer:\n%s", cfIssuer)
	}
	if !strings.Contains(cfIssuer, `name: "opencenter-cloudflare-credentials-secret-cf-main"`) {
		t.Fatalf("expected cloudflare secret ref in issuer:\n%s", cfIssuer)
	}

	// Verify disabled credential file does NOT exist
	disabledPath := filepath.Join(base, "opencenter-aws-credentials-secret-disabled-cred.yaml")
	if _, err := os.Stat(disabledPath); err == nil {
		t.Fatalf("disabled credential file should not exist: %s", disabledPath)
	}
}

func TestRenderClusterAppsCertManagerValidationFailsOnMissingSecrets(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("validation-test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "validation-test.example.com"

	// Enable an AWS credential but leave secrets empty
	cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
		"broken": {
			Enabled:            true,
			AWSAccessKey:       "",
			AWSSecretAccessKey: "",
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.Email = "ops@example.com"
	certManager.Region = "us-east-1"

	err := RenderClusterApps(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing secrets, got nil")
	}
	if !strings.Contains(err.Error(), "secrets.cert_manager.aws.broken.aws_access_key is required") {
		t.Fatalf("expected access key validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "secrets.cert_manager.aws.broken.aws_secret_access_key is required") {
		t.Fatalf("expected secret key validation error, got: %v", err)
	}
}

func TestRenderClusterAppsCertManagerValidationFailsOnMissingCloudflareToken(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("cf-validation-test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "cf-validation-test.example.com"

	// Enable a Cloudflare credential but leave token empty
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"missing-token": {
			Enabled:  true,
			APIToken: "",
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = "cloudflare"
	certManager.Email = "ops@example.com"

	err := RenderClusterApps(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing Cloudflare token, got nil")
	}
	if !strings.Contains(err.Error(), "secrets.cert_manager.cloudflare.missing-token.api_token is required") {
		t.Fatalf("expected Cloudflare token validation error, got: %v", err)
	}
}

func TestRenderClusterAppsCertManagerAWSSecretUsesStringData(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("aws-stringdata")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "aws-stringdata.example.com"

	cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
		"main": {
			Enabled:            true,
			AWSAccessKey:       "AKIAIOSFODNN7EXAMPLE",
			AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:             "us-east-1",
			DNSZones:           []string{"aws-stringdata.example.com"},
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.Email = "ops@example.com"
	certManager.Region = "us-east-1"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	secret := mustReadFile(t, filepath.Join(base, "opencenter-aws-credentials-secret-main.yaml"))

	// The secret MUST use stringData (plaintext) not data (base64-encoded).
	// Kubernetes stringData accepts raw values; data requires base64 encoding.
	// Using data with raw plaintext produces invalid secrets at apply time.
	if !strings.Contains(secret, "stringData:") {
		t.Fatalf("AWS credential secret must use 'stringData:' (not 'data:') for plaintext values.\nGot:\n%s", secret)
	}
	if strings.Contains(secret, "\ndata:\n") {
		t.Fatalf("AWS credential secret must NOT use 'data:' field with plaintext values.\nGot:\n%s", secret)
	}

	// The secret should include type: Opaque (like the Cloudflare template does)
	if !strings.Contains(secret, "type: Opaque") {
		t.Fatalf("AWS credential secret should declare 'type: Opaque'.\nGot:\n%s", secret)
	}

	// Verify the credential values are present as-is (not base64-encoded)
	if !strings.Contains(secret, "access-key-id: AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("expected plaintext access key in stringData.\nGot:\n%s", secret)
	}
	if !strings.Contains(secret, "secret-access-key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY") {
		t.Fatalf("expected plaintext secret key in stringData.\nGot:\n%s", secret)
	}
}

func TestRenderClusterAppsCertManagerCloudflareSecretUsesStringData(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("cf-stringdata")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "cf-stringdata.example.com"

	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"main": {
			Enabled:  true,
			APIToken: "cf-api-token-value-12345",
			DNSZones: []string{"cf-stringdata.example.com"},
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = "cloudflare"
	certManager.Email = "ops@example.com"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	secret := mustReadFile(t, filepath.Join(base, "opencenter-cloudflare-credentials-secret-main.yaml"))

	// Cloudflare secret should also use stringData (it already does — this is a regression guard)
	if !strings.Contains(secret, "stringData:") {
		t.Fatalf("Cloudflare credential secret must use 'stringData:' for plaintext values.\nGot:\n%s", secret)
	}
	if strings.Contains(secret, "\ndata:\n") {
		t.Fatalf("Cloudflare credential secret must NOT use 'data:' field with plaintext values.\nGot:\n%s", secret)
	}
	if !strings.Contains(secret, "type: Opaque") {
		t.Fatalf("Cloudflare credential secret should declare 'type: Opaque'.\nGot:\n%s", secret)
	}
}

func TestRenderClusterAppsCertManagerAWSIssuerHasSelectorAndRegion(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("issuer-selector")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "issuer-selector.sjc3.k8s.opencenter.cloud"

	cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
		"prod": {
			Enabled:            true,
			AWSAccessKey:       "AKIAEXAMPLE",
			AWSSecretAccessKey: "secretExampleKey",
			Region:             "us-east-1",
			DNSZones:           []string{"issuer-selector.sjc3.k8s.opencenter.cloud"},
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.Email = "mpk-support@rackspace.com"
	certManager.Region = "us-east-1"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-prod-issuer.yaml"))

	// Must be a ClusterIssuer
	if !strings.Contains(issuer, "kind: ClusterIssuer") {
		t.Fatalf("expected ClusterIssuer kind.\nGot:\n%s", issuer)
	}

	// Must have route53 solver with a valid (non-empty) region
	if !strings.Contains(issuer, "route53:") {
		t.Fatalf("expected route53 solver block.\nGot:\n%s", issuer)
	}
	if !strings.Contains(issuer, "region: us-east-1") {
		t.Fatalf("expected region 'us-east-1' in route53 solver.\nGot:\n%s", issuer)
	}

	// Must have a selector with dnsZones
	if !strings.Contains(issuer, "selector:") {
		t.Fatalf("expected 'selector:' block in issuer — cert-manager issuers without a selector match ALL certificates, which is unsafe in multi-tenant clusters.\nGot:\n%s", issuer)
	}
	if !strings.Contains(issuer, "dnsZones:") {
		t.Fatalf("expected 'dnsZones:' under selector.\nGot:\n%s", issuer)
	}
	if !strings.Contains(issuer, "- issuer-selector.sjc3.k8s.opencenter.cloud") {
		t.Fatalf("expected the configured DNS zone in the selector dnsZones list.\nGot:\n%s", issuer)
	}

	// Must reference the correct credential secret
	if !strings.Contains(issuer, `name: "opencenter-aws-credentials-secret-prod"`) {
		t.Fatalf("expected accessKeyIDSecretRef to reference the correct credential secret.\nGot:\n%s", issuer)
	}
}

func TestRenderClusterAppsCertManagerAWSIssuerMultipleZones(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("multi-zone")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "multi-zone.sjc3.k8s.opencenter.cloud"

	cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
		"main": {
			Enabled:            true,
			AWSAccessKey:       "AKIAMULTIZONE",
			AWSSecretAccessKey: "multiZoneSecret",
			Region:             "eu-west-1",
			DNSZones:           []string{"zone-a.example.com", "zone-b.example.com"},
		},
	}

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.Email = "ops@example.com"
	certManager.Region = "eu-west-1"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-main-issuer.yaml"))

	// Both DNS zones must be present in the selector
	if !strings.Contains(issuer, "- zone-a.example.com") {
		t.Fatalf("expected zone-a.example.com in selector dnsZones.\nGot:\n%s", issuer)
	}
	if !strings.Contains(issuer, "- zone-b.example.com") {
		t.Fatalf("expected zone-b.example.com in selector dnsZones.\nGot:\n%s", issuer)
	}

	// Region must match credential-level region
	if !strings.Contains(issuer, "region: eu-west-1") {
		t.Fatalf("expected region 'eu-west-1'.\nGot:\n%s", issuer)
	}
}

func TestGatewayRendersHostnamesWithClusterFQDN(t *testing.T) {
	cfg := newDefault("raxai")
	// Set the ClusterFQDN as it would be for a real cluster
	cfg.OpenCenter.Cluster.ClusterFQDN = "raxai.dev1.sjc3.k8s.opencenter.cloud"

	// When ClusterFQDN changes, service hostnames that derive from it must update.
	// This simulates the real scenario where a user configures their FQDN.
	keycloakSvc := cfg.OpenCenter.Services["keycloak"].(*configservices.KeycloakConfig)
	keycloakSvc.Hostname = "auth." + cfg.OpenCenter.Cluster.ClusterFQDN

	headlampSvc := cfg.OpenCenter.Services["headlamp"].(*configservices.HeadlampConfig)
	headlampSvc.Hostname = "dashboard." + cfg.OpenCenter.Cluster.ClusterFQDN

	files, err := gatewayOverlayFilesRenderer(cfg)
	if err != nil {
		t.Fatalf("gatewayOverlayFilesRenderer() error = %v", err)
	}

	gateway, ok := files["gateway.yaml"]
	if !ok {
		t.Fatal("expected gateway.yaml to be rendered")
	}

	// All hostnames must include the full ClusterFQDN (with cluster name "raxai").
	// They should be <service>.raxai.dev1.sjc3.k8s.opencenter.cloud
	// NOT <service>.dev1.sjc3.k8s.opencenter.cloud (missing cluster name)
	expectedHostnames := []string{
		"auth.raxai.dev1.sjc3.k8s.opencenter.cloud",
		"dashboard.raxai.dev1.sjc3.k8s.opencenter.cloud",
		"gitops.raxai.dev1.sjc3.k8s.opencenter.cloud",
		"prometheus.raxai.dev1.sjc3.k8s.opencenter.cloud",
		"alertmanager.raxai.dev1.sjc3.k8s.opencenter.cloud",
		"grafana.raxai.dev1.sjc3.k8s.opencenter.cloud",
		"harbor.raxai.dev1.sjc3.k8s.opencenter.cloud",
	}

	for _, expected := range expectedHostnames {
		if !strings.Contains(gateway, expected) {
			t.Errorf("expected hostname %q in gateway.yaml but not found.\nGot:\n%s", expected, gateway)
		}
	}

	// Verify that truncated FQDN (without cluster name) does NOT appear
	wrongAuthHostname := "auth.dev1.sjc3.k8s.opencenter.cloud"
	if strings.Contains(gateway, wrongAuthHostname) {
		t.Errorf("gateway.yaml must NOT contain truncated hostname %q (missing cluster name)", wrongAuthHostname)
	}
}

func TestRenderClusterAppsOpenStackCCMNamespace(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("ccm-ns-test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	// The openstack-ccm kustomization must target the openstack-ccm namespace,
	// not kube-system.
	kustomizationPath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "openstack-ccm")

	// Find the kustomization file that references the namespace
	entries, err := os.ReadDir(kustomizationPath)
	if err != nil {
		t.Fatalf("failed to read openstack-ccm overlay dir: %v", err)
	}

	var foundNamespace bool
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content := mustReadFile(t, filepath.Join(kustomizationPath, entry.Name()))
		if strings.Contains(content, "targetNamespace") {
			if !strings.Contains(content, "targetNamespace: openstack-ccm") {
				t.Errorf("expected targetNamespace: openstack-ccm in %s.\nGot:\n%s", entry.Name(), content)
			}
			foundNamespace = true
		}
	}

	if !foundNamespace {
		// Check the service config directly
		svc := cfg.OpenCenter.Services["openstack-ccm"].(*configservices.DefaultServiceConfig)
		if svc.Namespace != "openstack-ccm" {
			t.Fatalf("expected openstack-ccm service namespace to be 'openstack-ccm', got %q", svc.Namespace)
		}
	}
}

func TestRenderClusterAppsOpenStackCSINamespace(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("csi-ns-test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	// The openstack-csi kustomization must target the openstack-csi namespace,
	// not kube-system.
	svc := cfg.OpenCenter.Services["openstack-csi"].(*configservices.DefaultServiceConfig)
	if svc.Namespace != "openstack-csi" {
		t.Fatalf("expected openstack-csi service namespace to be 'openstack-csi', got %q", svc.Namespace)
	}

	// The namespace resource must carry the privileged pod-security label (OCTR-670).
	nsPath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "openstack-csi", "namespace", "namespace.yaml")
	nsDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, nsPath)))
	if err != nil {
		t.Fatalf("parse %s: %v", nsPath, err)
	}
	if len(nsDocs) != 1 {
		t.Fatalf("expected 1 document in namespace.yaml, got %d", len(nsDocs))
	}
	labels, _ := nestedValue(nsDocs[0], "metadata", "labels").(map[string]any)
	if labels == nil {
		t.Fatalf("namespace.yaml has no metadata.labels")
	}
	if got := labels["pod-security.kubernetes.io/enforce"]; got != "privileged" {
		t.Errorf("pod-security.kubernetes.io/enforce = %v, want \"privileged\"", got)
	}
}

func TestRenderClusterAppsOpenStackCSIStagesAndSecretSettings(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("csi-stages-test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	fluxPath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd", "openstack-csi.yaml")
	fluxDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, fluxPath)))
	if err != nil {
		t.Fatalf("parse %s: %v", fluxPath, err)
	}

	dependsOn := func(doc map[string]any) map[string]bool {
		t.Helper()

		spec, ok := doc["spec"].(map[string]any)
		if !ok {
			t.Fatalf("%s document has no spec map: %#v", fluxPath, doc)
		}
		rawDependencies, ok := spec["dependsOn"].([]any)
		if !ok {
			t.Fatalf("%s document has no dependsOn list: %#v", fluxPath, doc)
		}

		dependencies := make(map[string]bool, len(rawDependencies))
		for _, rawDependency := range rawDependencies {
			dependency, ok := rawDependency.(map[string]any)
			if !ok {
				t.Fatalf("%s document has malformed dependsOn entry: %#v", fluxPath, rawDependency)
			}
			name, ok := dependency["name"].(string)
			if !ok {
				t.Fatalf("%s document has dependsOn entry without a name: %#v", fluxPath, dependency)
			}
			dependencies[name] = true
		}
		return dependencies
	}

	stageDependencies := make(map[string]map[string]bool)
	for _, doc := range fluxDocs {
		metadata, ok := doc["metadata"].(map[string]any)
		if !ok || doc["kind"] != "Kustomization" {
			continue
		}
		name, ok := metadata["name"].(string)
		if ok {
			stageDependencies[name] = dependsOn(doc)
		}
	}

	baseDependencies, ok := stageDependencies["openstack-csi-base"]
	if !ok {
		t.Fatalf("openstack-csi-base was not rendered in %s", fluxPath)
	}
	for _, dependency := range []string{"sources", "openstack-csi-override"} {
		if !baseDependencies[dependency] {
			t.Errorf("openstack-csi-base must depend on %q, got %v", dependency, baseDependencies)
		}
	}

	overrideDependencies, ok := stageDependencies["openstack-csi-override"]
	if !ok {
		t.Fatalf("openstack-csi-override was not rendered in %s", fluxPath)
	}
	if !overrideDependencies["sources"] {
		t.Errorf("openstack-csi-override must depend on sources, got %v", overrideDependencies)
	}
	if overrideDependencies["openstack-csi-base"] {
		t.Errorf("openstack-csi-override must not depend on openstack-csi-base, got %v", overrideDependencies)
	}

	overridePath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "openstack-csi", "helm-values", "override-values.yaml")
	overrideDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, overridePath)))
	if err != nil {
		t.Fatalf("parse %s: %v", overridePath, err)
	}
	if len(overrideDocs) != 1 {
		t.Fatalf("expected one YAML document in %s, got %d", overridePath, len(overrideDocs))
	}
	secret, ok := overrideDocs[0]["secret"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no secret map: %#v", overridePath, overrideDocs[0])
	}
	for field, expected := range map[string]bool{"enabled": true, "hostMount": false, "create": true} {
		actual, ok := secret[field].(bool)
		if !ok || actual != expected {
			t.Errorf("%s secret.%s = %#v, want %t", overridePath, field, secret[field], expected)
		}
	}
}

func TestRenderClusterAppsGatewayDependsOnEnvoyGatewayAPIBase(t *testing.T) {
	spec, ok := newBuiltInRenderCatalog().Lookup("gateway")
	if !ok {
		t.Fatal("gateway is missing from the render catalog")
	}
	if !containsString(spec.ExtraDependencies, "envoy-gateway-api-base") {
		t.Fatalf("expected catalog gateway dependencies to contain envoy-gateway-api-base, got %v", spec.ExtraDependencies)
	}
	if containsString(spec.ExtraDependencies, "gateway-api-base") {
		t.Fatal("catalog gateway dependencies contain gateway-api-base, which does not exist")
	}
}

func TestGatewayClusterIssuerIsLetsencryptDefault(t *testing.T) {
	cfg := newDefault("mycluster")
	cfg.OpenCenter.Cluster.ClusterFQDN = "mycluster.dev1.sjc3.k8s.opencenter.cloud"

	files, err := gatewayOverlayFilesRenderer(cfg)
	if err != nil {
		t.Fatalf("gatewayOverlayFilesRenderer() error = %v", err)
	}

	gateway := files["gateway.yaml"]

	// The cluster-issuer annotation must reference letsencrypt-default,
	// not letsencrypt-<cluster-name>.
	if strings.Contains(gateway, "cluster-issuer: letsencrypt-mycluster") {
		t.Fatalf("rmpk-gateway cluster-issuer should be 'letsencrypt-default', not 'letsencrypt-mycluster'.\nGot:\n%s", gateway)
	}
	if !strings.Contains(gateway, "cluster-issuer: letsencrypt-default") {
		t.Fatalf("expected cluster-issuer annotation 'letsencrypt-default' in rmpk-gateway.\nGot:\n%s", gateway)
	}
}

func TestRenderHarborOverrideValuesUsesEC2S3Credentials(t *testing.T) {
	cfg := newDefault("harbor-s3-guided")
	cfg.Secrets.Harbor = v2.HarborSecrets{
		AdminPassword:     "harbor-admin",
		RegistryPassword:  "harbor-registry",
		DatabasePassword:  "harbor-database",
		S3AccessKeyID:     "ec2-access-key",
		S3SecretAccessKey: "ec2-secret-key",
	}
	cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig).S3Endpoint = "https://s3.example"

	values, err := templateRenderer(harborTemplate)(cfg)
	if err != nil {
		t.Fatalf("render Harbor override-values: %v", err)
	}
	for _, expected := range []string{
		"type: s3",
		"region: " + cfg.OpenCenter.Meta.Region,
		"accesskey: ec2-access-key",
		"secretkey: ec2-secret-key",
		"regionendpoint: https://s3.example",
	} {
		if !strings.Contains(values, expected) {
			t.Fatalf("expected %q in Harbor values:\n%s", expected, values)
		}
	}
	for _, forbidden := range []string{"CHANGEME", "PLACEHOLDER", "Global.AWS.Application"} {
		if strings.Contains(values, forbidden) {
			t.Fatalf("did not expect %q in Harbor values:\n%s", forbidden, values)
		}
	}
}

func TestLokiTempoRenderedBackendMatchesSharedProviderAwareResolver(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		explicit string
		want     string
	}{
		{name: "openstack omitted", provider: "openstack", want: "swift"},
		{name: "generic omitted", provider: "kind", want: "s3"},
		{name: "openstack explicit s3", provider: "openstack", explicit: "s3", want: "s3"},
		{name: "generic explicit swift", provider: "kind", explicit: "swift", want: "swift"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustNewGitOpsTestConfig("backend-agreement", tt.provider)
			cfg.OpenCenter.Services["loki"].(*configservices.LokiConfig).StorageType = tt.explicit
			cfg.OpenCenter.Services["tempo"].(*configservices.TempoConfig).StorageType = tt.explicit

			for _, serviceName := range []string{"loki", "tempo"} {
				resolved := v2.ResolveObjectStorageBackend(&cfg, serviceName)
				if resolved != tt.want {
					t.Fatalf("%s resolver = %q, want %q", serviceName, resolved, tt.want)
				}
				values := renderOverrideValues(t, cfg, serviceName)
				needle := "type: " + resolved
				if serviceName == "tempo" {
					needle = "backend: " + resolved
				}
				if !strings.Contains(values, needle) {
					t.Fatalf("%s rendered backend disagrees with resolver %q:\n%s", serviceName, resolved, values)
				}
				if resolved == "s3" && strings.Contains(values, "rackspacecloud.com") {
					t.Fatalf("%s S3 rendering used a Rackspace-derived fallback endpoint:\n%s", serviceName, values)
				}
			}
		})
	}
}
