package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	configdefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func writeTestConfig(t *testing.T, dir, name, provider, gitDir string) {
	t.Helper()
	if gitDir != "" {
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatalf("mkdir git dir: %v", err)
		}
	}
	orgDir := filepath.Join(dir, "clusters", "opencenter")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", name)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("create cluster directory: %v", err)
	}

	cfgPtr, err := v2.NewV2Default(name, provider)
	if err != nil {
		t.Fatalf("create v2 config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Organization = "opencenter"
	if gitDir != "" {
		cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	}

	loader := v2.NewConfigLoader(configdefaults.NewRegistry())
	configPath := filepath.Join(orgDir, "."+name+"-config.yaml")
	if err := loader.SaveToFile(&cfg, configPath); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestClusterBootstrapDryRunMake(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "")

	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "repo")
	clusterDir := filepath.Join(gitDir, "infrastructure", "clusters", "demo")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	writeTestConfig(t, cfgDir, "demo", "openstack", gitDir)
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterBootstrapCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("bootstrap command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Bootstrap complete in") {
		t.Fatalf("expected completion message in output, got: %s", output)
	}
}

func TestClusterBootstrapDryRunKind(t *testing.T) {
	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "repo")
	writeTestConfig(t, cfgDir, "demo", "kind", gitDir)
	prepareCommandTestEnv(t, cfgDir)
	t.Setenv("CONTAINER_RUNTIME", "docker")

	cmd := newClusterBootstrapCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("bootstrap command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Bootstrap complete in") {
		t.Fatalf("expected completion message in output, got: %s", output)
	}
}

func TestClusterBootstrapFailurePrintsLogAndResumeState(t *testing.T) {
	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "repo")
	clusterDir := filepath.Join(gitDir, "infrastructure", "clusters", "demo")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	writeTestConfig(t, cfgDir, "demo", "openstack", gitDir)
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterBootstrapCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected bootstrap command to fail")
	}
	if !strings.Contains(err.Error(), "openstack credentials are incomplete") {
		t.Fatalf("unexpected bootstrap error: %v", err)
	}

	stateRoot := filepath.Join(cfgDir, ".local", "state", "opencenter")
	expectedLogDir := filepath.Join(stateRoot, "logs", "bootstrap", "opencenter", "demo")
	if _, statErr := os.Stat(expectedLogDir); statErr != nil {
		t.Fatalf("expected bootstrap log directory at %s: %v", expectedLogDir, statErr)
	}

	expectedStatePath := filepath.Join(stateRoot, "bootstrap", "opencenter", "demo", "state.json")
	if _, statErr := os.Stat(expectedStatePath); statErr != nil {
		t.Fatalf("expected bootstrap state at %s: %v", expectedStatePath, statErr)
	}

	stderr := errOut.String()
	if !strings.Contains(stderr, "Bootstrap log:") {
		t.Fatalf("expected bootstrap log path in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "Resume state:") {
		t.Fatalf("expected resume state path in stderr, got: %s", stderr)
	}
}

// initGitRepo initializes a bare-minimum git repo in dir with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
}

func TestVerifyOriginMatchesGitURL_Match(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	expectedURL := "https://gitea.local:3001/user/repo.git"
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", expectedURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add remote: %s: %v", out, err)
	}

	if err := verifyOriginMatchesGitURL(context.Background(), dir, expectedURL); err != nil {
		t.Fatalf("expected no error for matching origin, got: %v", err)
	}
}

func TestVerifyOriginMatchesGitURL_Mismatch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://wrong.example.com/repo.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add remote: %s: %v", out, err)
	}

	err := verifyOriginMatchesGitURL(context.Background(), dir, "https://gitea.local:3001/user/repo.git")
	if err == nil {
		t.Fatal("expected error for mismatched origin, got nil")
	}
	if !strings.Contains(err.Error(), "git remote origin") {
		t.Fatalf("expected descriptive mismatch error, got: %v", err)
	}
}

func TestVerifyOriginMatchesGitURL_NoRemote(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// No origin remote — should pass silently so bootstrap can add it later.
	if err := verifyOriginMatchesGitURL(context.Background(), dir, "https://gitea.local:3001/user/repo.git"); err != nil {
		t.Fatalf("expected no error when origin is absent, got: %v", err)
	}
}
