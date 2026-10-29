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
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
)

func writeTestConfig(t *testing.T, dir, name, provider, gitDir string) {
	t.Helper()

	baseDir := filepath.Join(dir, "clusters")
	gitopsRoot := filepath.Join(baseDir, "gitops")
	if gitDir != "" {
		gitopsRoot = filepath.Dir(gitDir)
		t.Setenv("OPENCENTER_GITOPS_DIR", gitopsRoot)
	}

	resolver := paths.NewPathResolverWithRoots(
		baseDir,
		filepath.Join(baseDir, "blueprints"),
		gitopsRoot,
		filepath.Join(baseDir, "state"),
		filepath.Join(baseDir, "secrets"),
		paths.DefaultResolutionOptions(),
	)
	if err := resolver.CreateClusterDirectories(context.Background(), name, "opencenter"); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	clusterPaths, err := resolver.Resolve(context.Background(), name, "opencenter")
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	cfgPtr, err := v2.NewV2Default(name, provider)
	if err != nil {
		t.Fatalf("create v2 config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	// These deploy-mechanics fixtures do not run secrets sync; clear the secret
	// payloads so the deploy secrets preflight (OCTR-787) has nothing to require.
	// The preflight itself is covered by TestCheckSecretsSyncedPreflight.
	cfg.Secrets = v2.SecretsConfig{}

	loader := v2.NewConfigLoader(configdefaults.NewRegistry())
	if err := loader.SaveToFile(&cfg, clusterPaths.ConfigPath); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Chmod(clusterPaths.ConfigPath, 0o600); err != nil {
		t.Fatalf("chmod config: %v", err)
	}
}

func TestClusterDeployDryRunMake(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "")

	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "opencenter")
	clusterDir := filepath.Join(gitDir, "infrastructure", "clusters", "demo")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	writeTestConfig(t, cfgDir, "demo", "openstack", gitDir)
	prepareCommandTestEnv(t, cfgDir)
	installFakeHelm4ForDeployTests(t)

	cmd := newClusterDeployCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deploy command failed: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"Deploy plan only (dry-run)",
		"No commands will be run, no files will be written, and prerequisites are not fully validated.",
		"Provider: openstack",
		"opentofu-init",
		"Working dir: " + clusterDir,
		"OS_APPLICATION_CREDENTIAL_SECRET=<redacted>",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected dry-run output to contain %q, got:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "Command: tofu init") && !strings.Contains(output, "Command: terraform init") {
		t.Fatalf("expected dry-run output to contain tofu or terraform init command, got:\n%s", output)
	}
	if strings.Contains(output, "Deploy complete") {
		t.Fatalf("dry-run should not report deploy completion, got:\n%s", output)
	}
}

func TestClusterDeployDryRunKind(t *testing.T) {
	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "opencenter")
	writeTestConfig(t, cfgDir, "demo", "kind", gitDir)
	prepareCommandTestEnv(t, cfgDir)
	t.Setenv("CONTAINER_RUNTIME", "docker")

	cmd := newClusterDeployCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deploy command failed: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"Deploy plan only (dry-run)",
		"Provider: kind",
		"Config:",
		"GitOps dir:",
		"Cluster dir:",
		"Kubeconfig:",
		"kind-create",
		"Command: kind get clusters",
		"Command: kind create cluster --name demo --config",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected dry-run output to contain %q, got:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Deploy complete") {
		t.Fatalf("dry-run should not report deploy completion, got:\n%s", output)
	}
}

func TestClusterDeployDryRunDoesNotAcquireDeployLock(t *testing.T) {
	cfgDir := t.TempDir()
	lockDir := filepath.Join(t.TempDir(), "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}

	origConfig := resilience.DefaultLockConfig
	resilience.DefaultLockConfig.LockDir = lockDir
	defer func() { resilience.DefaultLockConfig = origConfig }()

	lockPath := filepath.Join(lockDir, "demo.lock")
	lockContent := `owner=other-host:99999
acquired=2026-04-14T01:00:00Z
expires=2099-04-14T02:00:00Z
ttl=1h0m0s
operation=deploy
command=cluster deploy
`
	if err := os.WriteFile(lockPath, []byte(lockContent), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	writeTestConfig(t, cfgDir, "demo", "kind", filepath.Join(t.TempDir(), "opencenter"))
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterDeployCmd()
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run deploy should ignore existing locks, got: %v", err)
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("dry-run should leave existing lock in place: %v", err)
	}
	if string(data) != lockContent {
		t.Fatalf("dry-run mutated lock file:\n%s", string(data))
	}
	if strings.Contains(out.String(), "Broke existing lock") {
		t.Fatalf("dry-run should not break locks, got:\n%s", out.String())
	}
}

func TestClusterDeployFailurePrintsLogAndResumeState(t *testing.T) {
	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "opencenter")
	clusterDir := filepath.Join(gitDir, "infrastructure", "clusters", "demo")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	writeTestConfig(t, cfgDir, "demo", "openstack", gitDir)
	prepareCommandTestEnv(t, cfgDir)

	cmd := newClusterDeployCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected deploy command to fail")
	}
	// The credential resolver rejects the placeholder before any command runs.
	if !strings.Contains(err.Error(), "application_credential_id") || !strings.Contains(err.Error(), "cluster provider openstack") {
		t.Fatalf("unexpected deploy error: %v", err)
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

func TestPrintPostDeployNextSteps(t *testing.T) {
	cmd := newClusterDeployCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{})

	printPostDeployNextSteps(cmd, "my-cluster")

	output := out.String()

	// Secrets are synced BEFORE deploy (generate -> secrets sync -> deploy, now
	// enforced by the deploy secrets preflight), so the post-deploy next-steps
	// must NOT tell the user to sync secrets afterwards (OCTR-787: the old
	// post-deploy ordering was misleading and caused "secret not found").
	if strings.Contains(output, "secrets sync") {
		t.Fatalf("post-deploy next-steps must not suggest secrets sync (it precedes deploy), got:\n%s", output)
	}
	if !strings.Contains(output, "git add -A") {
		t.Fatalf("expected next-steps to contain git commit instruction, got:\n%s", output)
	}
	if !strings.Contains(output, "git push") {
		t.Fatalf("expected next-steps to contain git push instruction, got:\n%s", output)
	}
}

// TestCheckSecretsSyncedPreflight covers OCTR-787: cluster deploy must fail
// fast when a required secret manifest is missing or created-but-unwired, and
// pass once it is created and wired into the service kustomization.
func TestCheckSecretsSyncedPreflight(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("pf", "kind")
	if err != nil {
		t.Fatalf("v2 default: %v", err)
	}
	cfg := *cfgPtr
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	if !ok {
		t.Fatalf("mimir service type unexpected")
	}
	mimir.Enabled = true
	mimir.StorageType = "s3"
	mimir.S3Endpoint = "http://s3.example:8333"
	mimir.BucketName = "pf-mimir"
	cfg.Secrets.Mimir.S3AccessKeyID = "k"
	cfg.Secrets.Mimir.S3SecretAccessKey = "v"

	// Isolate the preflight to the mimir secret: clear the other secret payloads
	// the default config would otherwise require, so this test asserts only the
	// mimir create/wire states. (The preflight covering all services is exercised
	// by the live reproduction; here we keep the fixture minimal.)
	cfg.Secrets.Loki = v2.LokiSecrets{}
	// Clear the global AWS fallback as well; Loki resolves typed credentials
	// from it when its service-specific credentials are empty.
	cfg.Secrets.Global.AWS = v2.AWSGlobalSecrets{}
	cfg.Secrets.Tempo = v2.TempoSecrets{}
	cfg.Secrets.Headlamp = v2.HeadlampSecrets{}
	cfg.Secrets.Grafana = v2.GrafanaSecrets{}

	gitDir := filepath.Join(t.TempDir(), "opencenter")
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	mimirDir := filepath.Join(gitDir, "applications", "overlays", "pf", "services", "mimir")
	if err := os.MkdirAll(mimirDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// (1) No secret.yaml at all -> must fail with actionable message.
	if err := checkSecretsSynced(&cfg, "pf"); err == nil ||
		!strings.Contains(err.Error(), "not created") ||
		!strings.Contains(err.Error(), "opencenter secrets sync pf") {
		t.Fatalf("expected 'not created' preflight failure, got %v", err)
	}

	// (2) secret.yaml present but NOT wired into kustomization -> must fail.
	if err := os.WriteFile(filepath.Join(mimirDir, "secret.yaml"), []byte("kind: Secret\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mimirDir, "kustomization.yaml"),
		[]byte("resources:\n  - custom/\n"), 0o644); err != nil {
		t.Fatalf("write kustomization: %v", err)
	}
	if err := checkSecretsSynced(&cfg, "pf"); err == nil ||
		!strings.Contains(err.Error(), "not wired") {
		t.Fatalf("expected 'not wired' preflight failure, got %v", err)
	}

	// (3) secret.yaml present AND wired -> must pass.
	if err := os.WriteFile(filepath.Join(mimirDir, "kustomization.yaml"),
		[]byte("resources:\n  - custom/\n  - secret.yaml\n"), 0o644); err != nil {
		t.Fatalf("rewrite kustomization: %v", err)
	}
	if err := checkSecretsSynced(&cfg, "pf"); err != nil {
		t.Fatalf("preflight should pass when secret is created and wired, got %v", err)
	}
}

// TestCheckSecretsSyncedPreflightGrafanaDedicatedSecret locks the PR #55
// exemption: when a Grafana admin password is set, kube-prometheus-stack's
// credential is rendered as the dedicated grafana-admin-password.yaml (the
// generic secret.yaml shares the observability/grafana-admin-password identity
// and is intentionally omitted). The preflight must accept the dedicated
// manifest in place of secret.yaml, and still block when it is absent or
// unwired.
func TestCheckSecretsSyncedPreflightGrafanaDedicatedSecret(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("gp", "kind")
	if err != nil {
		t.Fatalf("v2 default: %v", err)
	}
	cfg := *cfgPtr
	// Isolate the preflight to the Grafana artifact.
	cfg.Secrets.Loki = v2.LokiSecrets{}
	cfg.Secrets.Global.AWS = v2.AWSGlobalSecrets{}
	cfg.Secrets.Tempo = v2.TempoSecrets{}
	cfg.Secrets.Headlamp = v2.HeadlampSecrets{}
	cfg.Secrets.Mimir = v2.MimirSecrets{}
	cfg.OpenCenter.Services["kube-prometheus-stack"] = &services.PrometheusStackConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "observability"},
	}
	cfg.Secrets.Grafana = v2.GrafanaSecrets{AdminPassword: "s3cret-password"}

	gitDir := filepath.Join(t.TempDir(), "opencenter")
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	svcDir := filepath.Join(gitDir, "applications", "overlays", "gp", "services", "kube-prometheus-stack")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// (1) Dedicated manifest rendered and wired, generic secret.yaml absent
	// (the post-generate state) -> must pass.
	if err := os.WriteFile(filepath.Join(svcDir, "grafana-admin-password.yaml"), []byte("kind: Secret\n"), 0o600); err != nil {
		t.Fatalf("write dedicated secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svcDir, "kustomization.yaml"),
		[]byte("resources:\n  - custom/\n  - grafana-admin-password.yaml\n"), 0o644); err != nil {
		t.Fatalf("write kustomization: %v", err)
	}
	if err := checkSecretsSynced(&cfg, "gp"); err != nil {
		t.Fatalf("preflight should pass with the dedicated Grafana secret wired, got %v", err)
	}

	// (2) Dedicated manifest present but NOT wired -> must fail.
	if err := os.WriteFile(filepath.Join(svcDir, "kustomization.yaml"),
		[]byte("resources:\n  - custom/\n"), 0o644); err != nil {
		t.Fatalf("rewrite kustomization: %v", err)
	}
	if err := checkSecretsSynced(&cfg, "gp"); err == nil ||
		!strings.Contains(err.Error(), "kube-prometheus-stack") {
		t.Fatalf("expected preflight failure for unwired dedicated secret, got %v", err)
	}

	// (3) The dedicated manifest removed while a stale generic secret.yaml (e.g.
	// materialized by an older release) is left unwired -> must still fail. The
	// planned artifact is now the dedicated file itself, so the preflight reports
	// it as missing rather than reporting the generic path.
	if err := os.WriteFile(filepath.Join(svcDir, "secret.yaml"), []byte("kind: Secret\n"), 0o600); err != nil {
		t.Fatalf("write stale secret: %v", err)
	}
	if err := os.Remove(filepath.Join(svcDir, "grafana-admin-password.yaml")); err != nil {
		t.Fatalf("remove dedicated secret: %v", err)
	}
	if err := checkSecretsSynced(&cfg, "gp"); err == nil ||
		!strings.Contains(err.Error(), "grafana-admin-password.yaml (not created)") {
		t.Fatalf("expected preflight failure when the dedicated manifest is absent, got %v", err)
	}

	// (4) Dedicated manifest present but the generic secret.yaml is the only one
	// wired -> must fail: the credential Flux applies would be the stale one.
	if err := os.WriteFile(filepath.Join(svcDir, "grafana-admin-password.yaml"), []byte("kind: Secret\n"), 0o600); err != nil {
		t.Fatalf("rewrite dedicated secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svcDir, "kustomization.yaml"),
		[]byte("resources:\n  - custom/\n  - secret.yaml\n"), 0o644); err != nil {
		t.Fatalf("rewrite kustomization: %v", err)
	}
	if err := checkSecretsSynced(&cfg, "gp"); err == nil ||
		!strings.Contains(err.Error(), "not wired") {
		t.Fatalf("expected 'not wired' failure when only the stale generic secret is wired, got %v", err)
	}
}
