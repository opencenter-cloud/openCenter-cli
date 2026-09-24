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
		"opencenter.gitops.auth.token=not-null",
	})

	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unsupported field type") {
		t.Fatalf("expected non-null pointer assignment error, got: %v", err)
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

func TestSetReflectValueRejectsNonNullPointer(t *testing.T) {
	token := &v2.GitOpsTokenAuth{Provider: "github", Token: "original"}
	before := *token

	err := setReflectValue(reflect.ValueOf(&token).Elem(), "replacement")
	if err == nil || !strings.Contains(err.Error(), "unsupported field type") {
		t.Fatalf("expected unsupported field type error, got: %v", err)
	}
	if *token != before {
		t.Fatalf("non-null pointer assignment changed value: before=%#v after=%#v", before, *token)
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
