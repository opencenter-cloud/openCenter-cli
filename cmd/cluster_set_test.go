package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestClusterSetUpdatesExplicitField(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "set-cluster", "opencenter")
	cfg.OpenCenter.Meta.Env = "dev"
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"set-cluster", "opencenter.meta.env=prod"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v", err)
	}

	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "env: prod") {
		t.Fatalf("expected config to contain env: prod, got:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "Updated cluster configuration set-cluster") {
		t.Fatalf("expected update summary, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Next: opencenter cluster validate set-cluster") {
		t.Fatalf("expected validate next step, got:\n%s", out.String())
	}
}

func TestClusterSetUpdatesNTPServersFromCommaDelimitedValue(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "set-ntp", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"set-ntp", "opencenter.infrastructure.networking.ntp_servers=time-a.example,time-b.example"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set ntp_servers failed: %v", err)
	}

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	got := updated.OpenCenter.Infrastructure.Networking.NTPServers
	if want := []string{"time-a.example", "time-b.example"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ntp_servers = %#v, want %#v", got, want)
	}
}

func TestClusterSetUpdatesCpServerGroupAffinity(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "set-cp-affinity", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"set-cp-affinity", "opencenter.infrastructure.compute.cp_server_group_affinity=soft-anti-affinity"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	got := updated.OpenCenter.Infrastructure.Compute.CpServerGroupAffinity
	if want := []string{"soft-anti-affinity"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cp_server_group_affinity = %#v, want %#v", got, want)
	}
}

func TestClusterSetUpdatesWorkerGroupAffinity(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "set-worker-affinity", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"set-worker-affinity", "opencenter.infrastructure.compute.worker_group_affinity=soft-anti-affinity"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	got := updated.OpenCenter.Infrastructure.Compute.WorkerGroupAffinity
	if want := []string{"soft-anti-affinity"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("worker_group_affinity = %#v, want %#v", got, want)
	}
}

func TestClusterSetDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "dry-run-set", "opencenter")
	cfg.OpenCenter.Meta.Env = "dev"
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	root := newClusterSetRootForTest()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"cluster", "set", "dry-run-set", "opencenter.meta.env=prod", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster set --dry-run failed: %v", err)
	}

	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "env: prod") {
		t.Fatalf("dry-run wrote config:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "Would update cluster configuration dry-run-set") {
		t.Fatalf("expected dry-run summary, got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Next:") {
		t.Fatalf("dry-run should not print next step, got:\n%s", out.String())
	}
}

func TestClusterSetUpdatesKindDisableDefaultCNIByPath(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, _ := saveKindConfigForCommandTest(t, dir, "set-kind-cni", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"set-kind-cni", "opencenter.infrastructure.kind.disable_default_cni=true"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated, err := loadCanonicalConfig("set-kind-cni")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	if updated.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if !updated.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be true after cluster set")
	}
}

func TestClusterSetUpdatesNestedServiceFieldByPath(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, _ := saveKindConfigForCommandTest(t, dir, "set-service-enabled", "opencenter")
	if cfg.OpenCenter.Services == nil {
		cfg.OpenCenter.Services = make(v2.ServiceMap)
	}
	cfg.OpenCenter.Services["test-service"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: false},
	}
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"set-service-enabled", "opencenter.services.test-service.enabled=true"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("cluster set failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated, err := loadCanonicalConfig("set-service-enabled")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	service, ok := updated.OpenCenter.Services["test-service"].(*services.DefaultServiceConfig)
	if !ok {
		t.Fatalf("test-service config type = %T, want *services.DefaultServiceConfig", updated.OpenCenter.Services["test-service"])
	}
	if !service.Enabled {
		t.Fatal("expected test-service to be enabled after cluster set")
	}
}

func TestClusterSetRejectsMissingAssignmentAfterClusterName(t *testing.T) {
	cmd := newClusterSetCmd()
	cmd.SetArgs([]string{"set-cluster"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cluster set to reject missing assignment")
	}
	if !strings.Contains(err.Error(), "at least one path=value assignment is required after cluster name") {
		t.Fatalf("expected missing assignment error, got: %v", err)
	}
}

func TestClusterSetSwitchesTokenToSSH(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	_, clusterPaths := saveKindConfigForCommandTest(t, dir, "token-to-ssh", "opencenter")

	privateKey := filepath.Join(dir, "keys", "id_ed25519")
	publicKey := privateKey + ".pub"
	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"token-to-ssh",
		"opencenter.gitops.repository.url=ssh://git@github.com/acme/platform.git",
		"opencenter.gitops.auth.token=null",
		"opencenter.gitops.auth.ssh.private_key=" + privateKey,
		"opencenter.gitops.auth.ssh.public_key=" + publicKey,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set token-to-ssh failed: %v", err)
	}

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	if updated.OpenCenter.GitOps.Repository.URL != "ssh://git@github.com/acme/platform.git" {
		t.Fatalf("repository URL = %q", updated.OpenCenter.GitOps.Repository.URL)
	}
	if updated.OpenCenter.GitOps.Auth.Token != nil {
		t.Fatalf("expected token auth to be nil, got %#v", updated.OpenCenter.GitOps.Auth.Token)
	}
	if updated.OpenCenter.GitOps.Auth.SSH == nil {
		t.Fatal("expected SSH auth to be allocated")
	}
	if updated.OpenCenter.GitOps.Auth.SSH.PrivateKey != privateKey {
		t.Fatalf("private key = %q, want %q", updated.OpenCenter.GitOps.Auth.SSH.PrivateKey, privateKey)
	}
	if updated.OpenCenter.GitOps.Auth.SSH.PublicKey != publicKey {
		t.Fatalf("public key = %q, want %q", updated.OpenCenter.GitOps.Auth.SSH.PublicKey, publicKey)
	}
}

func TestClusterSetFailureAfterAssignmentDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "failed-set", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	if err := os.Remove(clusterPaths.ConfigPath + ".backup"); err != nil {
		t.Fatalf("remove setup backup: %v", err)
	}
	before, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read original config: %v", err)
	}

	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"failed-set",
		"opencenter.gitops.repository.url=ssh://git@github.com/acme/platform.git",
		"opencenter.gitops.auth.token=not-json",
	})

	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid pointer JSON error, got: %v", err)
	}

	after, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config after failed set: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("failed set changed config:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if _, err := os.Stat(clusterPaths.ConfigPath + ".backup"); !os.IsNotExist(err) {
		t.Fatalf("failed set created backup, stat error = %v", err)
	}
}

func TestClusterSetDryRunPointerSwitchDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "dry-run-pointer", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	if err := os.Remove(clusterPaths.ConfigPath + ".backup"); err != nil {
		t.Fatalf("remove setup backup: %v", err)
	}
	before, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read original config: %v", err)
	}

	privateKey := filepath.Join(dir, "keys", "id_ed25519")
	cmd := newClusterSetRootForTest()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"cluster", "set", "dry-run-pointer",
		"opencenter.gitops.repository.url=ssh://git@github.com/acme/platform.git",
		"opencenter.gitops.auth.token=null",
		"opencenter.gitops.auth.ssh.private_key=" + privateKey,
		"opencenter.gitops.auth.ssh.public_key=" + privateKey + ".pub",
		"--dry-run",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set --dry-run pointer switch failed: %v", err)
	}

	after, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config after dry-run: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("dry-run changed config:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if _, err := os.Stat(clusterPaths.ConfigPath + ".backup"); !os.IsNotExist(err) {
		t.Fatalf("dry-run created backup, stat error = %v", err)
	}
}

func TestClusterSetStringNullRemainsLiteral(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "literal-null", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"literal-null", "opencenter.meta.env=null"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set string null failed: %v", err)
	}

	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var updated v2.Config
	if err := yaml.Unmarshal(data, &updated); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if updated.OpenCenter.Meta.Env != "null" {
		t.Fatalf("env = %q, want literal null", updated.OpenCenter.Meta.Env)
	}
}

func TestSetReflectValueAssignsNonNullPointer(t *testing.T) {
	token := &v2.GitOpsTokenAuth{Provider: "github", Token: "original"}

	err := setReflectValue(reflect.ValueOf(&token).Elem(), `{"provider":"github","token":"replacement"}`)
	if err != nil {
		t.Fatalf("expected pointer assignment to succeed, got: %v", err)
	}
	if token.Token != "replacement" {
		t.Fatalf("token = %#v, want replacement pointer value", token)
	}
}

func newClusterSetRootForTest() *cobra.Command {
	root := &cobra.Command{
		Use: "opencenter",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)

	cluster := &cobra.Command{Use: "cluster"}
	cluster.AddCommand(newClusterSetCmd())
	root.AddCommand(cluster)
	return root
}
// saveOCTR812ClusterConfigForCommandTest writes a kind cluster config that
// reproduces the OCTR-812 local setup: Mimir and Harbor are enabled, but the
// storage backend they would resolve to is Swift on a non-OpenStack provider.
// The point of the fixture is to make load-time validation fail for Mimir Swift
// if cross-service catchall behavior regresses, while still allowing the
// cluster set command itself to update unrelated service keys.
func saveOCTR812ClusterConfigForCommandTest(t *testing.T, dir, clusterName, organization string) (v2.Config, *paths.ClusterPaths) {
	t.Helper()

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, clusterName, organization)
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	if mimir == nil {
		t.Fatalf("expected mimir service in default kind config, got services=%v", cfg.OpenCenter.Services)
	}
	mimir.Enabled = true
	mimir.StorageType = "swift"

	if h, ok := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig); ok && h != nil {
		h.Enabled = true
		h.S3Endpoint = "http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333"
	}

	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	return cfg, clusterPaths
}

func TestClusterSetOCTR812MimirS3SetterDoesNotBlockOnMimirSwiftReadiness(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveOCTR812ClusterConfigForCommandTest(t, dir, "octr812-mimir-set", "opencenter")
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.Enabled = true
	mimir.StorageType = "s3"

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"octr812-mimir-set", "opencenter.services.mimir.storage_type=s3"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set mimir s3 failed: %v\nstderr: %s", err, stderr.String())
	}

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	got := updated.OpenCenter.Services["mimir"].(*services.MimirConfig)
	if got.StorageType != "s3" {
		t.Fatalf("mimir storage_type = %q after set, want s3", got.StorageType)
	}
}

func TestClusterSetOCTR812UnrelatedServiceSetterDoesNotFailOnMimirSwiftReadiness(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveOCTR812ClusterConfigForCommandTest(t, dir, "octr812-unrelated-set", "opencenter")
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.Enabled = true
	mimir.StorageType = "swift"

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"octr812-unrelated-set",
		"opencenter.services.harbor.s3_endpoint=http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set harbor s3_endpoint failed: %v\nstderr: %s", err, stderr.String())
	}

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	harbor := updated.OpenCenter.Services["harbor"].(*services.HarborConfig)
	if harbor.S3Endpoint != "http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333" {
		t.Fatalf("harbor s3_endpoint = %q after set, want the provided endpoint", harbor.S3Endpoint)
	}
}

func TestClusterSetOCTR812BulkConfigurationFlow(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "octr812-flow", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))

	// Step 1: disable mimir and harbor manually in the config before running set.
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.Enabled = false
	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.Enabled = false
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	assignments := []string{
		"opencenter.services.loki.storage_type=s3",
		"opencenter.services.loki.s3_endpoint=http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333",
		"opencenter.services.loki.bucket_name=rackai-local-loki",
		"opencenter.services.loki.s3_force_path_style=true",
		"opencenter.services.loki.s3_insecure=true",
		"opencenter.services.mimir.storage_type=s3",
		"opencenter.services.mimir.s3_endpoint=http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333",
		"opencenter.services.mimir.bucket_name=rackai-local-mimir",
		"opencenter.services.mimir.s3_force_path_style=true",
		"opencenter.services.mimir.s3_insecure=true",
		"opencenter.services.harbor.storage_type=s3",
		"opencenter.services.harbor.s3_endpoint=http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333",
		"opencenter.services.harbor.s3_bucket=rackai-local-harbor",
		"opencenter.services.etcd-backup.storage_type=s3",
		"opencenter.services.etcd-backup.s3_endpoint=http://seaweedfs-filer-client.seaweed.svc.cluster.local:8333",
		"opencenter.services.etcd-backup.s3_bucket_name=rackai-local-etcd-backup",
		"opencenter.services.etcd-backup.s3_region=us-east-1",
	}

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	args := append([]string{"octr812-flow"}, assignments...)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("bulk cluster set flow step 2 failed: %v\nstderr: %s", err, stderr.String())
	}

	// Re-enable Mimir
	reEnable := newClusterSetCmd()
	reEnable.SetOut(&bytes.Buffer{})
	reEnable.SetErr(&bytes.Buffer{})
	reEnable.SetArgs([]string{"octr812-flow", "opencenter.services.mimir.enabled=true"})
	if err := reEnable.Execute(); err != nil {
		t.Fatalf("re-enabling mimir failed: %v", err)
	}

	// Re-enable Harbor
	reEnableHarbor := newClusterSetCmd()
	reEnableHarbor.SetOut(&bytes.Buffer{})
	reEnableHarbor.SetErr(&bytes.Buffer{})
	reEnableHarbor.SetArgs([]string{"octr812-flow", "opencenter.services.harbor.enabled=true"})
	if err := reEnableHarbor.Execute(); err != nil {
		t.Fatalf("re-enabling harbor failed: %v", err)
	}

	final := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	if final.OpenCenter.Services["mimir"].(*services.MimirConfig).StorageType != "s3" {
		t.Fatalf("final mimir storage_type = %q, want s3", final.OpenCenter.Services["mimir"].(*services.MimirConfig).StorageType)
	}
	if final.OpenCenter.Services["harbor"].(*services.HarborConfig).S3Bucket != "rackai-local-harbor" {
		t.Fatalf("final harbor s3_bucket = %q, want rackai-local-harbor", final.OpenCenter.Services["harbor"].(*services.HarborConfig).S3Bucket)
	}
	if !final.OpenCenter.Services["mimir"].(*services.MimirConfig).Enabled {
		t.Fatal("expected mimir to be re-enabled at end of flow")
	}
	if !final.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled {
		t.Fatal("expected harbor to be re-enabled at end of flow")
	}
}
