package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
	"github.com/spf13/cobra"
)

func saveOpenStackStatusConfig(t *testing.T, dir, clusterName, organization string) (v2.Config, string) {
	t.Helper()

	resolver, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, organization)

	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(dir, "gitops", clusterName)
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	return cfg, clusterPaths.KubeconfigPath
}

func TestClusterStatusHonorsExplicitClusterArgument(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	if _, err := os.Stat(filepath.Join(dir, "clusters")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
	}

	saveOpenStackStatusConfig(t, dir, "active-cluster", "opencenter")
	saveOpenStackStatusConfig(t, dir, "requested-cluster", "opencenter")

	manager, err := config.NewConfigurationManager()
	if err != nil {
		t.Fatalf("NewConfigurationManager() error = %v", err)
	}
	if err := manager.SetActive("active-cluster"); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"requested-cluster"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "Cluster: requested-cluster") {
		t.Fatalf("expected requested cluster in output, got:\n%s", output)
	}
	if strings.Contains(output, "Cluster: active-cluster") {
		t.Fatalf("expected explicit cluster argument to take precedence over active cluster, got:\n%s", output)
	}
}

func TestClusterStatusShowsOpenStackInfrastructureDetails(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, kubeconfigPath := saveOpenStackStatusConfig(t, dir, "status-cluster", "opencenter")

	infraDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", cfg.ClusterName())
	if err := os.MkdirAll(infraDir, 0o755); err != nil {
		t.Fatalf("mkdir infra dir: %v", err)
	}

	statePath := cfg.OpenTofu.Backend.Local.Path
	if !filepath.IsAbs(statePath) {
		statePath = filepath.Join(infraDir, statePath)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("terraform-state"), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o755); err != nil {
		t.Fatalf("mkdir kubeconfig dir: %v", err)
	}
	if err := os.WriteFile(kubeconfigPath, []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKubectlBinary(t, binDir)
	prependTestPath(t, binDir)

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-cluster"})
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	expectedSnippets := []string{
		"OpenStack Status:",
		"GitOps Repo:       ✓ Ready",
		"Infrastructure:    ✓ Rendered",
		"OpenTofu State:    ✓ Present",
		"Kubeconfig:        ✓ Present",
		"API Ready:         ✓ Ready",
		"API Endpoint:      https://127.0.0.1:6443",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
}

func TestClusterStatusHonorsJSONOutput(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveOpenStackStatusConfig(t, dir, "status-json", "opencenter")

	root := &cobra.Command{
		Use:           "opencenter",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)
	root.AddCommand(NewClusterCmd())

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"cluster", "status", "status-json", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster status --output json failed: %v\nstderr: %s", err, errOut.String())
	}

	if strings.Contains(out.String(), "Cluster: status-json") {
		t.Fatalf("expected structured JSON output, got text:\n%s", out.String())
	}

	var payload struct {
		Cluster      string   `json:"cluster"`
		Name         string   `json:"name"`
		Environment  string   `json:"environment"`
		Organization string   `json:"organization"`
		Provider     string   `json:"provider"`
		NextSteps    []string `json:"next_steps"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got error %v and output:\n%s", err, out.String())
	}
	if payload.Cluster != "status-json" {
		t.Fatalf("expected cluster status-json, got %#v", payload)
	}
	if payload.Provider != "openstack" {
		t.Fatalf("expected provider openstack, got %#v", payload)
	}
	if len(payload.NextSteps) == 0 {
		t.Fatalf("expected next steps in json payload, got %#v", payload)
	}
}
