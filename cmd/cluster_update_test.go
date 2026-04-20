package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClusterUpdateKindDisableDefaultCNIFlagSetsTrue(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	runClusterInitForUpdateTest(t, "update-kind-cni", "kind")

	cmd := newClusterUpdateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"update-kind-cni", "--kind-disable-default-cni"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster update failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated, err := loadCanonicalConfig("update-kind-cni")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	if updated.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if !updated.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be true after cluster update")
	}
}

func TestClusterUpdateKindDisableDefaultCNIFlagSetsFalse(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	runClusterInitForUpdateTest(t, "update-kind-cni-false", "kind")

	enableCmd := newClusterUpdateCmd()
	var enableStdout, enableStderr bytes.Buffer
	enableCmd.SetOut(&enableStdout)
	enableCmd.SetErr(&enableStderr)
	enableCmd.SetArgs([]string{"update-kind-cni-false", "--kind-disable-default-cni"})

	if err := enableCmd.Execute(); err != nil {
		t.Fatalf("cluster update enable failed: %v\nstderr: %s", err, enableStderr.String())
	}

	cmd := newClusterUpdateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"update-kind-cni-false", "--kind-disable-default-cni=false"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster update failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated, err := loadCanonicalConfig("update-kind-cni-false")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	if updated.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if updated.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be false after cluster update")
	}
}

func TestClusterUpdateRejectsKindDisableDefaultCNIForNonKind(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	runClusterInitForUpdateTest(t, "update-openstack-cni", "openstack")

	cmd := newClusterUpdateCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"update-openstack-cni", "--kind-disable-default-cni"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cluster update to reject --kind-disable-default-cni for non-kind clusters")
	}
	if !strings.Contains(err.Error(), "--kind-disable-default-cni is only valid for kind clusters") {
		t.Fatalf("expected kind-only error, got: %v", err)
	}
}

func TestClusterUpdateKindDisableDefaultCNIFromNativeV2Init(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	runClusterInitForUpdateTest(t, "update-kind-native-v2", "kind")

	resetCommandStateForTests()

	updateCmd := newClusterUpdateCmd()
	var updateStdout, updateStderr bytes.Buffer
	updateCmd.SetOut(&updateStdout)
	updateCmd.SetErr(&updateStderr)
	updateCmd.SetArgs([]string{"update-kind-native-v2", "--kind-disable-default-cni"})

	if err := updateCmd.Execute(); err != nil {
		t.Fatalf("cluster update failed: %v\nstderr: %s", err, updateStderr.String())
	}

	resetCommandStateForTests()

	updated, err := loadCanonicalConfig("update-kind-native-v2")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	if updated.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if !updated.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be true after updating a native v2 kind cluster")
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".update-kind-native-v2-config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if strings.Contains(string(data), "ssh_user:") {
		t.Fatalf("expected cluster update to preserve native v2 structure, got:\n%s", string(data))
	}

	v2Cfg := loadV2ConfigForTest(t, configPath)
	if v2Cfg.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected native v2 kind compatibility config to remain present")
	}
	if !v2Cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected native v2 disable_default_cni to be true after update")
	}
}

func TestClusterUpdateNativeV2GitOpsFieldsValidate(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	runClusterInitForUpdateTest(t, "update-kind-gitops", "kind")

	updateCmd := newClusterUpdateCmd()
	var updateStdout, updateStderr bytes.Buffer
	updateCmd.SetOut(&updateStdout)
	updateCmd.SetErr(&updateStderr)
	updateCmd.SetArgs([]string{
		"update-kind-gitops",
		"--opencenter.gitops.repository.url=https://172.16.0.146:3001/newuser/test-repo.git",
		"--opencenter.gitops.auth.token.token_file=/tmp/gitea-user.token",
		"--opencenter.gitops.auth.token.provider=gitea",
		"--strict",
	})

	if err := updateCmd.Execute(); err != nil {
		t.Fatalf("cluster update failed: %v\nstderr: %s", err, updateStderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".update-kind-gitops-config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if strings.Contains(string(data), "ssh_user:") {
		t.Fatalf("expected cluster update to keep native v2 YAML, got:\n%s", string(data))
	}

	v2Cfg := loadV2ConfigForTest(t, configPath)
	if got := v2Cfg.OpenCenter.GitOps.Repository.URL; got != "https://172.16.0.146:3001/newuser/test-repo.git" {
		t.Fatalf("git_url = %q", got)
	}
	if v2Cfg.OpenCenter.GitOps.Auth.Token == nil {
		t.Fatal("expected token auth to be set")
	}
	if got := v2Cfg.OpenCenter.GitOps.Auth.Token.TokenFile; got != "/tmp/gitea-user.token" {
		t.Fatalf("git_token = %q", got)
	}
	if got := v2Cfg.OpenCenter.GitOps.Auth.Token.Provider; got != "gitea" {
		t.Fatalf("git_token_provider = %q", got)
	}
}

func runClusterInitForUpdateTest(t *testing.T, clusterName, provider string) {
	t.Helper()

	initCmd := newClusterInitCmd()
	var initStdout, initStderr bytes.Buffer
	initCmd.SetOut(&initStdout)
	initCmd.SetErr(&initStderr)
	initCmd.SetArgs([]string{clusterName, "--type", provider, "--org", "opencenter", "--no-keygen"})

	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, initStderr.String())
	}
}
