package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

// kubesprayPrepareFixture builds a provider plus lifecycle whose prepare step
// reports success, so a resume decision can be observed without running Ansible.
func kubesprayPrepareFixture(t *testing.T) (*openstackBootstrapProvider, *kubesprayLifecycle, *v2.Config) {
	t.Helper()

	root := t.TempDir()
	clusterDir := filepath.Join(root, "gitops", "infrastructure", "clusters", "kit")
	stateDir := filepath.Join(root, "state")
	inventoryPath := filepath.Join(stateDir, "inventory")
	for _, dir := range []string{clusterDir, inventoryPath} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	cfg := mustNewClusterTestConfig("kit", "openstack")
	cfg.Deployment.Method = "kubespray"
	cfg.OpenTofu.Path = "tofu"
	provider := &openstackBootstrapProvider{runner: &fakeLifecycleRunner{}, validateCredentials: stubCredentialValidation}
	lifecycle := newKubesprayLifecycle(provider.runner, "tofu", clusterDir,
		&paths.ClusterPaths{ClusterStateDir: stateDir, InventoryPath: inventoryPath},
		filepath.Join(stateDir, "kubeconfig.yaml"), nil)
	return provider, lifecycle, &cfg
}

func requireStep(t *testing.T, steps []bootstrapStep, id string) bootstrapStep {
	t.Helper()
	for _, step := range steps {
		if step.ID == id {
			return step
		}
	}
	t.Fatalf("step %q not built; got %s", id, stepIDs(steps))
	return bootstrapStep{}
}

func stepIDs(steps []bootstrapStep) string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ID)
	}
	return strings.Join(ids, ",")
}

// writePrepareArtifacts materializes exactly what kubespray-prepare produces.
func writePrepareArtifacts(t *testing.T, lifecycle *kubesprayLifecycle, hardening bool) {
	t.Helper()
	write := func(path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(lifecycle.outputsPath, `{"opencenter_kubespray_lifecycle_contract_version":"2"}`)
	write(filepath.Join(lifecycle.venvPath, "bin", "ansible-playbook"), "#!/bin/sh\n")
	write(filepath.Join(lifecycle.kubesprayPath, "cluster.yml"), "---\n")
	write(filepath.Join(lifecycle.inventoryPath, "inventory.yaml"), "all:\n")
	if hardening {
		write(filepath.Join(lifecycle.inventoryPath, "os_hardening_playbook.yml"), "---\n")
		if err := os.MkdirAll(filepath.Join(lifecycle.inventoryPath, "roles", "ansible-hardening"), 0o700); err != nil {
			t.Fatalf("mkdir hardening role: %v", err)
		}
	}
}

// TestKubesprayPrepareRerunsAfterClusterArtifactsAreDeleted reproduces the
// reported failure: the resume state records kubespray-prepare as successful,
// the cluster directory is wiped and regenerated underneath it, and the next
// deploy must re-run prepare instead of skipping it and dying in deploy's
// loadOutputs on a missing kubespray-outputs.json.
func TestKubesprayPrepareRerunsAfterClusterArtifactsAreDeleted(t *testing.T) {
	provider, lifecycle, cfg := kubesprayPrepareFixture(t)

	steps, err := provider.BuildSteps(cfg, &paths.ClusterPaths{
		ClusterStateDir: lifecycle.stateDir,
		InventoryPath:   lifecycle.inventoryPath,
	}, &BootstrapOptions{KubeconfigPath: lifecycle.kubeconfigPath})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	prepare := requireStep(t, steps, "kubespray-prepare")
	if prepare.CanSkip == nil {
		t.Fatal("kubespray-prepare must declare CanSkip: saved success is not evidence its artifacts survive")
	}
	if prepare.CanSkip() {
		t.Fatal("kubespray-prepare reported skippable with no artifacts on disk")
	}

	writePrepareArtifacts(t, lifecycle, false)
	if !prepare.CanSkip() {
		t.Fatal("kubespray-prepare refused to skip with every artifact present")
	}

	// A manual rm -rf of the venv: same generation, artifacts gone.
	if err := os.RemoveAll(lifecycle.venvPath); err != nil {
		t.Fatalf("remove venv: %v", err)
	}
	if prepare.CanSkip() {
		t.Fatal("kubespray-prepare reported skippable after its venv was deleted")
	}
}

// TestKubesprayPrepareRerunsWhenHardeningArtifactsMissing pins the
// hardening-scoped probes: with OS hardening enabled, a missing role or playbook
// must force prepare to re-run even when every other artifact exists.
func TestKubesprayPrepareRerunsWhenHardeningArtifactsMissing(t *testing.T) {
	_, lifecycle, cfg := kubesprayPrepareFixture(t)
	cfg.OpenCenter.Infrastructure.Networking.Security.OSHardening = true
	lifecycle.osHardening = true

	// Hardening on with only the non-hardening artifacts present is exactly the
	// wiped-tree case: prepare must re-run to write the playbook and role.
	if lifecycle.prepareArtifactsPresent() {
		t.Fatal("prepare reported complete with the hardening playbook and role missing")
	}

	writePrepareArtifacts(t, lifecycle, true)
	if !lifecycle.prepareArtifactsPresent() {
		t.Fatal("prepare artifacts reported missing with hardening artifacts present")
	}

	if err := os.RemoveAll(filepath.Join(lifecycle.inventoryPath, "roles", "ansible-hardening")); err != nil {
		t.Fatalf("remove hardening role: %v", err)
	}
	if lifecycle.prepareArtifactsPresent() {
		t.Fatal("prepare reported complete with the ansible-hardening role deleted")
	}
}

// TestExportKubeconfigSkipsOnlyWhenKubeconfigExists pins the resume evidence for
// the kubeconfig export: the secrets-zone file's presence.
func TestExportKubeconfigSkipsOnlyWhenKubeconfigExists(t *testing.T) {
	provider, lifecycle, cfg := kubesprayPrepareFixture(t)

	steps, err := provider.BuildSteps(cfg, &paths.ClusterPaths{
		ClusterStateDir: lifecycle.stateDir,
		InventoryPath:   lifecycle.inventoryPath,
	}, &BootstrapOptions{KubeconfigPath: lifecycle.kubeconfigPath})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	export := requireStep(t, steps, "kubespray-export-kubeconfig")
	if export.CanSkip == nil {
		t.Fatal("kubespray-export-kubeconfig must declare CanSkip")
	}
	if export.CanSkip() {
		t.Fatal("export reported skippable with no kubeconfig on disk")
	}
	if err := os.MkdirAll(filepath.Dir(lifecycle.kubeconfigPath), 0o700); err != nil {
		t.Fatalf("mkdir kubeconfig dir: %v", err)
	}
	if err := os.WriteFile(lifecycle.kubeconfigPath, []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	if !export.CanSkip() {
		t.Fatal("export refused to skip with the kubeconfig present")
	}
}

// TestResolveBootstrapRuntimePathsScopesStateToGeneration pins the core of the
// fix: two generations of the same cluster resolve to different resume files, so
// a recreated cluster cannot read the dead generation's completed steps.
func TestResolveBootstrapRuntimePathsScopesStateToGeneration(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())

	cfg := mustNewClusterTestConfig("kit-kat", "openstack")
	cfg.OpenCenter.Meta.Organization = "opensnack"

	unscoped, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolveBootstrapRuntimePaths() error = %v", err)
	}
	if !strings.HasSuffix(unscoped.StatePath, filepath.Join("bootstrap", "opensnack", "kit-kat", "state.json")) {
		t.Fatalf("pre-generation config must keep the unscoped state path, got %s", unscoped.StatePath)
	}

	first := v2.NewGenerationID()
	cfg.Metadata.GenerationID = first
	genA, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolveBootstrapRuntimePaths(generation A) error = %v", err)
	}
	if want := filepath.Join("bootstrap", "opensnack", "kit-kat", first, "state.json"); !strings.HasSuffix(genA.StatePath, want) {
		t.Fatalf("generation A state path %s must end in %s", genA.StatePath, want)
	}
	if genA.GenerationDir != filepath.Dir(genA.StatePath) {
		t.Fatalf("GenerationDir %s must be the parent of StatePath %s", genA.GenerationDir, genA.StatePath)
	}

	second := v2.NewGenerationID()
	cfg.Metadata.GenerationID = second
	genB, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolveBootstrapRuntimePaths(generation B) error = %v", err)
	}
	if genB.StatePath == genA.StatePath {
		t.Fatalf("recreated cluster resolved to the previous generation's state path %s", genB.StatePath)
	}
	if _, err := os.Stat(genA.StatePath); !os.IsNotExist(err) {
		t.Fatalf("generation B must not resolve onto an existing generation A file: %v", err)
	}
}

// TestBootstrapStateGenerationIsPersisted proves the identifier survives the
// state file round-trip, which is what lets a foreign file be recognized.
func TestBootstrapStateGenerationIsPersisted(t *testing.T) {
	service := createTestBootstrapService(paths.NewPathResolver(t.TempDir()))
	path := filepath.Join(t.TempDir(), "state.json")
	generation := v2.NewGenerationID()

	state := service.newBootstrapState()
	state.GenerationID = generation
	service.setStepStatus(state, "kubespray-prepare", bootstrapStatusSuccess, "")
	if err := service.saveBootstrapState(path, state); err != nil {
		t.Fatalf("saveBootstrapState() error = %v", err)
	}

	loaded, enabled, err := service.loadBootstrapState(path)
	if err != nil {
		t.Fatalf("loadBootstrapState() error = %v", err)
	}
	if !enabled {
		t.Fatal("loadBootstrapState() reported state disabled")
	}
	if loaded.GenerationID != generation {
		t.Fatalf("generation id lost through state round-trip: %q -> %q", generation, loaded.GenerationID)
	}
}

// TestResolveBootstrapRuntimePathsGenerationScoping pins the resume trust model:
// the generation identifier scopes the state path, and the pre-generation
// location is resolved separately so it can be retired without ever being a
// resume source.
func TestResolveBootstrapRuntimePathsGenerationScoping(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("OPENCENTER_STATE_DIR", stateRoot)

	cfg := mustNewClusterTestConfig("kit-kat", "openstack")
	cfg.OpenCenter.Meta.Organization = "opensnack"
	generation := v2.NewGenerationID()
	cfg.Metadata.GenerationID = generation

	runtimePaths, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolveBootstrapRuntimePaths() error = %v", err)
	}
	clusterRoot := filepath.Join(stateRoot, "bootstrap", "opensnack", "kit-kat")
	if want := filepath.Join(clusterRoot, generation, "state.json"); runtimePaths.StatePath != want {
		t.Fatalf("StatePath = %s, want %s", runtimePaths.StatePath, want)
	}
	if want := filepath.Join(clusterRoot, "state.json"); runtimePaths.UnscopedStatePath != want {
		t.Fatalf("UnscopedStatePath = %s, want %s", runtimePaths.UnscopedStatePath, want)
	}
	if runtimePaths.StatePath == runtimePaths.UnscopedStatePath {
		t.Fatal("scoped and unscoped state paths must differ")
	}

	// A configuration that predates generation identifiers keeps the cluster
	// directory as its state path, so an in-flight resume is not orphaned.
	unscopedCfg := mustNewClusterTestConfig("kit-kat", "openstack")
	unscopedCfg.OpenCenter.Meta.Organization = "opensnack"
	unscopedRuntimePaths, err := resolveBootstrapRuntimePaths(&unscopedCfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolveBootstrapRuntimePaths() without identifier error = %v", err)
	}
	if want := filepath.Join(clusterRoot, "state.json"); unscopedRuntimePaths.StatePath != want {
		t.Fatalf("StatePath without identifier = %s, want %s", unscopedRuntimePaths.StatePath, want)
	}
}

// TestBootstrapStateClusterDir pins the directory destroy removes.
func TestBootstrapStateClusterDir(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("OPENCENTER_STATE_DIR", stateRoot)

	got := BootstrapStateClusterDir("kit-kat", "opensnack")
	if want := filepath.Join(stateRoot, "bootstrap", "opensnack", "kit-kat"); got != want {
		t.Fatalf("BootstrapStateClusterDir() = %s, want %s", got, want)
	}
	if got := BootstrapStateClusterDir("", "opensnack"); got != "" {
		t.Fatalf("BootstrapStateClusterDir() with empty cluster name = %q, want empty", got)
	}
	if got := BootstrapStateClusterDir("kit-kat", ""); got != filepath.Join(stateRoot, "bootstrap", "opencenter", "kit-kat") {
		t.Fatalf("BootstrapStateClusterDir() without organization = %s, want the opencenter default", got)
	}
}

// TestProvisionInfrastructureDiscardsForeignGenerationState proves a resume
// never trusts another generation's completed steps: the pre-generation state
// file is retired on sight, nothing is reported as "already completed", and
// every step re-runs.
func TestProvisionInfrastructureDiscardsForeignGenerationState(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "recreated"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	service := createTestBootstrapService(pathResolver)
	service.openStackValidateCredentials = stubCredentialValidation

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := pathResolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenTofu.Path = "tofu"
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	// Placeholder ("CHANGEME") credentials are rejected at preflight before any
	// command runs, so the fixture carries syntactically usable ones; the live
	// Keystone call is stubbed above.
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"
	if err := os.MkdirAll(clusterPaths.InventoryPath, 0o700); err != nil {
		t.Fatalf("mkdir inventory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterPaths.InventoryPath, "inventory.yaml"), []byte("all:\n"), 0o600); err != nil {
		t.Fatalf("write inventory: %v", err)
	}

	// The calico install step requires the rendered Helm override values file.
	calicoValuesDir := filepath.Join(clusterPaths.GitOpsDir, "applications", "overlays", clusterName, "services", "calico", "helm-values")
	if err := os.MkdirAll(calicoValuesDir, 0o700); err != nil {
		t.Fatalf("mkdir calico values dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(calicoValuesDir, "override_values.yaml"), []byte("installation: {}\n"), 0o600); err != nil {
		t.Fatalf("write calico override values: %v", err)
	}

	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
	cfg.Metadata.GenerationID = v2.NewGenerationID()

	runtimePaths, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolve bootstrap runtime paths: %v", err)
	}
	// The dead generation's file, left at the pre-generation path where an older
	// CLI wrote it: name-only, indistinguishable from this cluster's own state.
	deadPath := runtimePaths.UnscopedStatePath
	if err := os.MkdirAll(filepath.Dir(deadPath), 0o700); err != nil {
		t.Fatalf("mkdir dead state dir: %v", err)
	}
	deadState := service.newBootstrapState()
	for _, id := range []string{"preflight", "opentofu-init", "opentofu-apply", "kubespray-prepare", "kubespray-wait-cloudinit"} {
		service.setStepStatus(deadState, id, bootstrapStatusSuccess, "")
	}
	if err := service.saveBootstrapState(deadPath, deadState); err != nil {
		t.Fatalf("save dead generation state: %v", err)
	}

	// The fake runner answers every command with success, so the lifecycle's
	// `tofu output -json` needs the contract v2 payload the real apply produces;
	// without it prepare fails parsing empty output.
	inventoryFile := filepath.Join(clusterPaths.InventoryPath, "inventory.yaml")
	runner := &fakeLifecycleRunner{onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "output" {
			return []byte(`{"opencenter_kubespray_inventory_path":{"value":"` + inventoryFile + `"},"opencenter_kubespray_lifecycle_contract_version":{"value":2},"opencenter_kubespray_api_address":{"value":"10.2.128.5"},"opencenter_kubespray_api_port":{"value":6443}}`), nil
		}
		// The kubeconfig export fetches admin.conf via ansible; emulate the
		// dest= write so the step's follow-up read has a file.
		if strings.HasSuffix(name, string(filepath.Separator)+"ansible") {
			for i, arg := range args {
				if arg != "-a" || i+1 >= len(args) {
					continue
				}
				for _, part := range strings.Fields(args[i+1]) {
					if dest := strings.TrimPrefix(part, "dest="); dest != part {
						if err := os.WriteFile(dest, []byte("apiVersion: v1\nserver: https://10.2.128.5:6443\n"), 0o600); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		return nil, nil
	}}
	service.runner = runner
	var progress strings.Builder
	service.SetOutput(&progress)

	result := &BootstrapResult{}
	if err := service.provisionInfrastructure(ctx, &cfg, clusterPaths, &BootstrapOptions{
		KubeconfigPath: clusterPaths.KubeconfigPath,
	}, runtimePaths, result); err != nil {
		t.Fatalf("provisionInfrastructure() error = %v", err)
	}

	if strings.Contains(progress.String(), "already completed") {
		t.Fatalf("resume trusted the foreign generation's steps:\n%s", progress.String())
	}
	// The discarded file must be gone: leaving it would let a later run re-read
	// the foreign generation's claim.
	if _, statErr := os.Stat(deadPath); !os.IsNotExist(statErr) {
		t.Fatalf("unattributable state %s survived the load (stat err = %v)", deadPath, statErr)
	}
	if len(result.StepsCompleted) == 0 {
		t.Fatal("no steps completed")
	}
	for _, id := range []string{"preflight", "kubespray-prepare"} {
		if !containsString(result.StepsCompleted, id) {
			t.Fatalf("step %q must re-run after discarding unattributable state; completed: %v", id, result.StepsCompleted)
		}
	}
}
