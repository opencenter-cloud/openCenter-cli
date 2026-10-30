package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestClusterDestroyKindProvider(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-kind", "opencenter")
	cfg.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfg.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-kind\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"destroy-kind", "--force", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstderr: %s", err, stderr.String())
	}

	if _, err := os.Stat(clusterPaths.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be removed")
	}
	// The shared organization GitOps worktree must survive; only the
	// cluster's own trees are removed.
	if _, err := os.Stat(clusterPaths.GitOpsDir); err != nil {
		t.Fatalf("expected shared gitops worktree to remain: %v", err)
	}
	if _, err := os.Stat(clusterPaths.ClusterDir); !os.IsNotExist(err) {
		t.Fatalf("expected cluster directory to be removed")
	}
	if _, err := os.Stat(clusterPaths.ApplicationsDir); !os.IsNotExist(err) {
		t.Fatalf("expected applications overlay to be removed")
	}

	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil {
		t.Fatalf("read fake kind log: %v", err)
	}
	if !strings.Contains(string(kindLog), "kind delete cluster --name destroy-kind") {
		t.Fatalf("expected delete cluster invocation\nlog:\n%s", string(kindLog))
	}
}

func TestClusterDestroyKindProviderKeepsFilesByDefault(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-kind-keep", "opencenter")
	cfg.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfg.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-kind-keep\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// Note: no --remove-files flag
	cmd.SetArgs([]string{"destroy-kind-keep", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstderr: %s", err, stderr.String())
	}

	// Files should still exist
	if _, err := os.Stat(clusterPaths.ConfigPath); err != nil {
		t.Fatalf("expected config file to remain: %v", err)
	}
	if _, err := os.Stat(clusterPaths.GitOpsDir); err != nil {
		t.Fatalf("expected gitops directory to remain: %v", err)
	}

	// Kind cluster should still be destroyed
	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil {
		t.Fatalf("read fake kind log: %v", err)
	}
	if !strings.Contains(string(kindLog), "kind delete cluster --name destroy-kind-keep") {
		t.Fatalf("expected delete cluster invocation\nlog:\n%s", string(kindLog))
	}

	// Output should mention files are preserved
	if !strings.Contains(stdout.String(), "Local files preserved") {
		t.Fatalf("expected output to mention files are preserved\noutput:\n%s", stdout.String())
	}
}

func TestClusterDestroyKindProviderAlreadyMissing(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	_, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-missing-kind", "opencenter")
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	cmd := newClusterDestroyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"destroy-missing-kind", "--force", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v", err)
	}

	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil {
		t.Fatalf("read fake kind log: %v", err)
	}
	logText := string(kindLog)
	if !strings.Contains(logText, "kind get clusters") {
		t.Fatalf("expected destroy to check existing clusters\nlog:\n%s", logText)
	}
	if strings.Contains(logText, "kind delete cluster") {
		t.Fatalf("expected destroy to skip delete when cluster is absent\nlog:\n%s", logText)
	}
}

func TestClusterDestroyKindProviderDeleteFailure(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	_, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-kind-fail", "opencenter")
	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-kind-fail\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "delete_fail"), []byte("1\n"), 0o644); err != nil {
		t.Fatalf("write delete fail flag: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	cmd := newClusterDestroyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"destroy-kind-fail", "--force", "--remove-files"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cluster destroy to fail when kind delete fails")
	}
	if !strings.Contains(err.Error(), "failed to destroy kind cluster") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(clusterPaths.ConfigPath); statErr != nil {
		t.Fatalf("expected config file to remain after delete failure: %v", statErr)
	}
	if _, statErr := os.Stat(clusterPaths.GitOpsDir); statErr != nil {
		t.Fatalf("expected gitops directory to remain after delete failure: %v", statErr)
	}
}

func TestClusterDestroySkipInfrastructure(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-skip-infra", "opencenter")
	cfg.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfg.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-skip-infra\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	cmd := newClusterDestroyCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"destroy-skip-infra", "--force", "--skip-infrastructure", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v", err)
	}

	// The cluster's files should be removed; the shared worktree survives.
	if _, err := os.Stat(clusterPaths.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be removed")
	}
	if _, err := os.Stat(clusterPaths.ClusterDir); !os.IsNotExist(err) {
		t.Fatalf("expected cluster directory to be removed")
	}
	if _, err := os.Stat(clusterPaths.GitOpsDir); err != nil {
		t.Fatalf("expected shared gitops worktree to remain: %v", err)
	}

	// Kind should NOT have been called for delete
	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake kind log: %v", err)
	}
	if strings.Contains(string(kindLog), "kind delete cluster") {
		t.Fatalf("expected skip-infrastructure to skip kind delete\nlog:\n%s", string(kindLog))
	}

	// Output should mention skipping
	if !strings.Contains(stdout.String(), "Skipping infrastructure destruction") {
		t.Fatalf("expected output to mention skipping infrastructure\noutput:\n%s", stdout.String())
	}
}

// TestClusterDestroyRemoveFilesOnlyRemovesDesignatedCluster covers the
// two-cluster scenario: both clusters share one organization GitOps worktree
// (GitOpsDir is the org repo root, e.g. clusters/gitops/<org>). Destroying
// cluster-a with --remove-files must remove only cluster-a's own trees
// (infrastructure/clusters/a, applications/overlays/a, clusters/a bridge,
// blueprint dir, config file) and must leave cluster-b's files, the shared
// repository content, and the GitOps worktree itself intact.
func TestClusterDestroyRemoveFilesOnlyRemovesDesignatedCluster(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	cfgA, pathsA := saveKindConfigForCommandTest(t, dir, "destroy-two-a", "opencenter")
	cfgA.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfgA.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfgA); err != nil {
		t.Fatalf("save config a: %v", err)
	}
	cfgB, pathsB := saveKindConfigForCommandTest(t, dir, "destroy-two-b", "opencenter")
	cfgB.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfgB.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfgB); err != nil {
		t.Fatalf("save config b: %v", err)
	}

	// Both clusters resolve to the same organization GitOps worktree.
	if pathsA.GitOpsDir != pathsB.GitOpsDir {
		t.Fatalf("expected shared gitops worktree, got %q and %q", pathsA.GitOpsDir, pathsB.GitOpsDir)
	}

	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// Shared repository content.
	write(filepath.Join(pathsA.GitOpsDir, "README.md"), "shared gitops repo")
	// Cluster A files (to be removed).
	write(filepath.Join(pathsA.ClusterDir, "main.tf"), "a infra")
	write(filepath.Join(pathsA.ApplicationsDir, "kustomization.yaml"), "a overlay")
	write(filepath.Join(pathsA.GitOpsDir, "clusters", "destroy-two-a", "services.yaml"), "a bridge")
	// Cluster B files (must survive).
	write(filepath.Join(pathsB.ClusterDir, "main.tf"), "b infra")
	write(filepath.Join(pathsB.ApplicationsDir, "kustomization.yaml"), "b overlay")
	write(filepath.Join(pathsB.GitOpsDir, "clusters", "destroy-two-b", "services.yaml"), "b bridge")

	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-two-a\ndestroy-two-b\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}

	cmd := newClusterDestroyCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"destroy-two-a", "--force", "--skip-infrastructure", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstdout: %s", err, stdout.String())
	}

	// Cluster A is gone.
	if _, err := os.Stat(pathsA.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected cluster-a config file to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(pathsA.ClusterDir); !os.IsNotExist(err) {
		t.Fatalf("expected cluster-a infrastructure dir to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(pathsA.ApplicationsDir); !os.IsNotExist(err) {
		t.Fatalf("expected cluster-a applications overlay to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pathsA.GitOpsDir, "clusters", "destroy-two-a")); !os.IsNotExist(err) {
		t.Fatalf("expected cluster-a bridge dir to be removed, stat err: %v", err)
	}

	// The shared GitOps worktree and cluster B must survive.
	if _, err := os.Stat(pathsB.GitOpsDir); err != nil {
		t.Fatalf("expected shared gitops worktree to remain: %v", err)
	}
	for _, path := range []string{
		filepath.Join(pathsB.GitOpsDir, "README.md"),
		pathsB.ConfigPath,
		pathsB.ClusterDir,
		pathsB.ApplicationsDir,
		filepath.Join(pathsB.GitOpsDir, "clusters", "destroy-two-b", "services.yaml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected cluster-b/shared path to survive destroy of cluster-a: %s: %v", path, err)
		}
	}

	// Destroy must not have touched cluster B's kind cluster.
	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read fake kind log: %v", err)
	}
	if strings.Contains(string(kindLog), "destroy-two-b") {
		t.Fatalf("expected destroy to never touch cluster-b\nlog:\n%s", string(kindLog))
	}
}

func TestClusterDestroyCreatesLog(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-log-test", "opencenter")
	cfg.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfg.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-log-test\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"destroy-log-test", "--force", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify "Destroy log: " is in stdout
	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "Destroy log: ") {
		t.Fatalf("expected 'Destroy log: ' in stdout, got:\n%s", stdoutStr)
	}

	// Find the log file in the state directory
	stateRoot := filepath.Join(dir, ".local", "state", "opencenter")
	logDir := filepath.Join(stateRoot, "logs", "destroy", "opencenter", "destroy-log-test")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("expected destroy log directory at %s: %v", logDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one destroy log in %s", logDir)
	}

	logContent, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read destroy log: %v", err)
	}
	logStr := string(logContent)

	expectedPhrases := []string{
		"destroy started for opencenter/destroy-log-test",
		"destroying Kind cluster destroy-log-test",
		"executing command: kind delete cluster",
		"removed cluster directory:",
		"removed config file:",
		"destroy complete for opencenter/destroy-log-test",
	}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(logStr, phrase) {
			t.Errorf("expected phrase %q in destroy log, but got:\n%s", phrase, logStr)
		}
	}
}

func TestClusterDestroyCustomLogFlag(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-custom-log", "opencenter")
	cfg.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfg.OpenCenter.Meta.Status = v2.StatusSuccess
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-custom-log\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	customLog := filepath.Join(dir, "custom-destroy.log")

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"destroy-custom-log", "--force", "--remove-files", "--log", customLog})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Destroy log: "+customLog) {
		t.Fatalf("expected custom destroy log path in stdout, got:\n%s", stdout.String())
	}

	logContent, err := os.ReadFile(customLog)
	if err != nil {
		t.Fatalf("expected custom log file to exist: %v", err)
	}
	if !strings.Contains(string(logContent), "destroy complete for opencenter/destroy-custom-log") {
		t.Fatalf("expected completion in custom log, got:\n%s", string(logContent))
	}
}

func TestClusterDestroyFailurePrintsLogPath(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKindBinary(t, binDir)
	prependTestPath(t, binDir)

	_, clusterPaths := saveKindConfigForCommandTest(t, dir, "destroy-fail-log", "opencenter")
	if err := os.WriteFile(filepath.Join(stateDir, "clusters"), []byte("destroy-fail-log\n"), 0o644); err != nil {
		t.Fatalf("write cluster state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "delete_fail"), []byte("1\n"), 0o644); err != nil {
		t.Fatalf("write delete fail flag: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.GitOpsDir, "README.md"), []byte("gitops"), 0o644); err != nil {
		t.Fatalf("write gitops marker: %v", err)
	}

	customLog := filepath.Join(dir, "fail.log")

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"destroy-fail-log", "--force", "--remove-files", "--log", customLog})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected destroy to fail")
	}

	if !strings.Contains(stderr.String(), "Destroy log: "+customLog) {
		t.Fatalf("expected destroy log path in stderr, got:\n%s", stderr.String())
	}

	logContent, readErr := os.ReadFile(customLog)
	if readErr != nil {
		t.Fatalf("read fail log: %v", readErr)
	}
	if !strings.Contains(string(logContent), "destroy failed") {
		t.Fatalf("expected failure message in log, got:\n%s", string(logContent))
	}
}

// TestReportDestroyCleanupWarnings covers the CLI surfacing contract: every
// cleanup warning is printed and logged, and --strict converts the set into
// a command error so automation gets a non-zero exit.
func TestReportDestroyCleanupWarnings(t *testing.T) {
	t.Run("no warnings", func(t *testing.T) {
		cmd := newClusterDestroyCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := reportDestroyCleanupWarnings(cmd, context.Background(), nil, true); err != nil {
			t.Fatalf("no warnings must not error even in strict mode: %v", err)
		}
		if out.Len() != 0 {
			t.Fatalf("no warnings must print nothing, got: %s", out.String())
		}
	})

	t.Run("warnings reported", func(t *testing.T) {
		cmd := newClusterDestroyCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		warnings := []error{errors.New("leaked load balancer"), errors.New("orphaned volume pvc-1")}
		if err := reportDestroyCleanupWarnings(cmd, context.Background(), warnings, false); err != nil {
			t.Fatalf("non-strict warnings must not fail the command: %v", err)
		}
		text := out.String()
		if !strings.Contains(text, "Warnings (2)") || !strings.Contains(text, "leaked load balancer") || !strings.Contains(text, "orphaned volume pvc-1") {
			t.Fatalf("expected both warnings listed, got: %s", text)
		}
	})

	t.Run("strict fails", func(t *testing.T) {
		cmd := newClusterDestroyCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		err := reportDestroyCleanupWarnings(cmd, context.Background(), []error{errors.New("orphaned volume pvc-1")}, true)
		if err == nil {
			t.Fatal("strict mode must return an error when warnings exist")
		}
		if !strings.Contains(err.Error(), "1 cleanup warning") {
			t.Fatalf("unexpected strict error: %v", err)
		}
	})
}
