package cluster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

func TestParseKubesprayLifecycleOutputs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPath string
		wantAPI  string
		wantPort int
		wantErr  string
	}{
		{
			name:     "terraform envelope",
			input:    `{"opencenter_kubespray_inventory_path":{"value":"/state/inventory/inventory.yaml"},"opencenter_kubespray_lifecycle_contract_version":{"value":2},"opencenter_kubespray_api_address":{"value":"10.0.0.5"},"opencenter_kubespray_api_port":{"value":6443}}`,
			wantPath: "/state/inventory/inventory.yaml",
			wantAPI:  "10.0.0.5",
			wantPort: 6443,
		},
		{
			name:     "direct values",
			input:    `{"opencenter_kubespray_inventory_path":"/state/inventory/inventory.yaml","opencenter_kubespray_lifecycle_contract_version":"2","opencenter_kubespray_api_address":"api.example.test","opencenter_kubespray_api_port":6443}`,
			wantPath: "/state/inventory/inventory.yaml",
			wantAPI:  "api.example.test",
			wantPort: 6443,
		},
		{
			name:    "unsupported contract",
			input:   `{"opencenter_kubespray_inventory_path":{"value":"/state/inventory/inventory.yaml"},"opencenter_kubespray_lifecycle_contract_version":{"value":1},"opencenter_kubespray_api_address":{"value":"10.0.0.5"},"opencenter_kubespray_api_port":{"value":6443}}`,
			wantErr: "requires contract 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKubesprayLifecycleOutputs([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKubesprayLifecycleOutputs() error = %v", err)
			}
			if got.InventoryPath != tt.wantPath || got.APIAddress != tt.wantAPI || got.APIPort != tt.wantPort {
				t.Fatalf("outputs = %#v, want path=%q api=%q port=%d", got, tt.wantPath, tt.wantAPI, tt.wantPort)
			}
		})
	}
}

// TestStageKubesprayInventoryContract2 covers the contract v2 staging
// semantics: the module renders directly into the state-zone destination (no-op
// success), and a module that still renders elsewhere (v1 layout, e.g. inside
// the GitOps worktree) fails with an actionable regeneration hint instead of
// producing a second divergent copy.
func TestStageKubesprayInventoryContract2(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "state", "inventory")
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	const stateContents = "all:\n"
	if err := os.WriteFile(filepath.Join(destination, "inventory.yaml"), []byte(stateContents), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stageKubesprayInventory(filepath.Join(destination, "inventory.yaml"), destination); err != nil {
		t.Fatalf("state-zone source must be a no-op, got %v", err)
	}

	worktree := filepath.Join(root, "repo", "infrastructure", "clusters", "demo", "inventory")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "inventory.yaml"), []byte("stale-v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := stageKubesprayInventory(filepath.Join(worktree, "inventory.yaml"), destination)
	if err == nil || !strings.Contains(err.Error(), "cluster generate") {
		t.Fatalf("worktree source must fail with regeneration hint, got %v", err)
	}

	// A rejected v1 layout must never be copied over the state-zone copy.
	data, readErr := os.ReadFile(filepath.Join(destination, "inventory.yaml"))
	if readErr != nil || string(data) != stateContents {
		t.Fatalf("state-zone inventory was modified by staging: data=%q err=%v", data, readErr)
	}
}

func TestMigrateBootstrapState(t *testing.T) {
	steps := []bootstrapStep{
		{ID: "kubespray-prepare"},
		{ID: "kubespray-wait-cloudinit"},
		{ID: "kubespray-os-hardening"},
		{ID: "kubespray-deploy"},
		{ID: "kubespray-export-kubeconfig"},
	}

	tests := []struct {
		name          string
		applyStatus   string
		wantSynthetic bool
	}{
		{name: "successful legacy apply", applyStatus: bootstrapStatusSuccess, wantSynthetic: true},
		{name: "failed legacy apply", applyStatus: bootstrapStatusFailed, wantSynthetic: false},
		{name: "running legacy apply", applyStatus: bootstrapStatusRunning, wantSynthetic: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &bootstrapState{Version: 1, Steps: map[string]bootstrapStepState{
				"opentofu-apply": {Status: tt.applyStatus, UpdatedAt: "2026-01-01T00:00:00Z"},
			}}
			if !migrateBootstrapState(state, steps) {
				t.Fatal("migrateBootstrapState() reported no migration")
			}
			if state.Version != bootstrapStateVersion {
				t.Fatalf("version = %d, want %d", state.Version, bootstrapStateVersion)
			}
			for _, step := range steps {
				_, ok := state.Steps[step.ID]
				if ok != tt.wantSynthetic {
					t.Errorf("step %q synthetic = %v, want %v", step.ID, ok, tt.wantSynthetic)
				}
			}
		})
	}
}

func TestSuccessfulLegacyMigrationDoesNotReplayKubesprayLifecycle(t *testing.T) {
	state := &bootstrapState{Version: 1, Steps: map[string]bootstrapStepState{
		"opentofu-apply": {Status: bootstrapStatusSuccess, UpdatedAt: "2026-01-01T00:00:00Z"},
	}}
	var executed int
	steps := []bootstrapStep{
		{ID: "kubespray-prepare", Run: func(context.Context) error { executed++; return nil }},
		{ID: "kubespray-wait-cloudinit", Run: func(context.Context) error { executed++; return nil }},
		{ID: "kubespray-deploy", Run: func(context.Context) error { executed++; return nil }},
		{ID: "kubespray-export-kubeconfig", Run: func(context.Context) error { executed++; return nil }},
	}
	migrateBootstrapState(state, steps)
	if err := (&BootstrapService{}).executeBootstrapSteps(context.Background(), steps, false, true, "", state, &BootstrapResult{}, &BootstrapOptions{}); err != nil {
		t.Fatalf("executeBootstrapSteps() error = %v", err)
	}
	if executed != 0 {
		t.Fatalf("legacy lifecycle steps executed %d times after successful migration", executed)
	}
}

func TestKubesprayWaitCloudInitCollectsDiagnostics(t *testing.T) {
	runner := &waitCloudInitRunner{err: errors.New("cloud-init failed")}
	lifecycle := &kubesprayLifecycle{
		runner:        runner,
		stateDir:      t.TempDir(),
		inventoryPath: t.TempDir(),
		outputs: kubesprayLifecycleOutputs{
			InventoryPath:   "/state/inventory/inventory.yaml",
			ContractVersion: kubesprayLifecycleContractVersion,
			APIAddress:      "10.0.0.5",
			APIPort:         6443,
		},
	}

	err := lifecycle.waitCloudInit(context.Background(), time.Second)
	if err == nil || !strings.Contains(err.Error(), "cloud-init wait failed before timeout") || !strings.Contains(err.Error(), "per-host diagnostics collected") {
		t.Fatalf("waitCloudInit() error = %v, want initial failure diagnostics", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("diagnostic command count = %d, want 3", len(runner.calls))
	}
	if len(runner.deadlines) != 3 || runner.deadlines[0].IsZero() || runner.deadlines[1].IsZero() || !runner.deadlines[1].After(runner.deadlines[0]) {
		t.Fatalf("diagnostics must use a separate timeout: deadlines=%v", runner.deadlines)
	}
}

func TestKubesprayPrepareReusesConfiguredVersionAndUsesVenvBinaries(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	clusterDir := filepath.Join(root, "gitops")
	// Contract v2: the module renders the inventory directly into the state zone.
	inventoryPath := filepath.Join(stateDir, "inventory")
	if err := os.MkdirAll(inventoryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inventoryPath, "inventory.yaml"), []byte("all:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	kubesprayPath := filepath.Join(stateDir, "kubespray")
	if err := os.MkdirAll(kubesprayPath, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeLifecycleRunner{onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
		if name == "tofu" && len(args) > 0 && args[0] == "output" {
			return []byte(`{"opencenter_kubespray_inventory_path":{"value":"` + filepath.Join(inventoryPath, "inventory.yaml") + `"},"opencenter_kubespray_lifecycle_contract_version":{"value":2},"opencenter_kubespray_api_address":{"value":"10.0.0.5"},"opencenter_kubespray_api_port":{"value":6443}}`), nil
		}
		return nil, nil
	}}
	lifecycle := newKubesprayLifecycle(runner, "tofu", clusterDir, &paths.ClusterPaths{ClusterStateDir: stateDir, InventoryPath: inventoryPath}, filepath.Join(stateDir, "kubeconfig"), nil)
	cfg := mustNewClusterTestConfig("prepare", "openstack")
	cfg.Deployment.Kubespray.Version = "2.30.0"
	if err := lifecycle.prepare(context.Background(), &cfg); err != nil {
		t.Fatalf("prepare() error = %v", err)
	}

	var commands []string
	for _, call := range runner.calls {
		commands = append(commands, call.name+" "+strings.Join(call.args, " "))
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"git -C " + kubesprayPath + " fetch --tags --force origin",
		"git -C " + kubesprayPath + " checkout --detach v2.30.0",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("commands = %s, want %q", joined, want)
		}
	}

	lifecycle.outputs = kubesprayLifecycleOutputs{InventoryPath: "/state/inventory/inventory.yaml", ContractVersion: "2", APIAddress: "10.0.0.5", APIPort: 6443}
	waitRunner := &fakeLifecycleRunner{}
	lifecycle.runner = waitRunner
	if err := lifecycle.waitCloudInit(context.Background(), time.Second); err != nil {
		t.Fatalf("waitCloudInit() error = %v", err)
	}
	if len(waitRunner.calls) == 0 || !strings.HasSuffix(waitRunner.calls[0].name, filepath.Join("venv", "bin", "ansible")) {
		t.Fatalf("wait command binary = %#v, want prepared venv ansible", waitRunner.calls)
	}
	if !strings.Contains(waitRunner.calls[0].env["PATH"], filepath.Join("venv", "bin")) {
		t.Fatalf("PATH = %q, want prepared venv prefix", waitRunner.calls[0].env["PATH"])
	}
}

func TestKubesprayWaitCloudInitRetriesUnreachableAndAcceptsRemoteStatusTwo(t *testing.T) {
	runner := &fakeLifecycleRunner{}
	attempts := 0
	runner.onRun = func(_ string, _ map[string]string, name string, args ...string) ([]byte, error) {
		if name == "ansible" || strings.HasSuffix(name, filepath.Join("venv", "bin", "ansible")) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("ansible: UNREACHABLE! exit status 4")
			}
			if attempts == 2 {
				// The shell module maps the remote cloud-init status 2 to a
				// successful Ansible process. The Go lifecycle must not infer
				// that meaning from an Ansible process error.
				return []byte("cloud-init status --wait: exit status 2"), nil
			}
		}
		return nil, nil
	}
	lifecycle := &kubesprayLifecycle{
		runner:        runner,
		stateDir:      t.TempDir(),
		inventoryPath: t.TempDir(),
		venvPath:      filepath.Join(t.TempDir(), "venv"),
		outputs:       kubesprayLifecycleOutputs{InventoryPath: "/state/inventory/inventory.yaml", ContractVersion: "2", APIAddress: "10.0.0.5", APIPort: 6443},
	}
	if err := lifecycle.waitCloudInit(context.Background(), 3*time.Second); err != nil {
		t.Fatalf("waitCloudInit() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("wait attempts = %d, want 2", attempts)
	}
}

func TestKubesprayWaitCloudInitDoesNotAcceptAnsibleProcessStatusTwo(t *testing.T) {
	runner := &fakeLifecycleRunner{}
	runner.onRun = func(_ string, _ map[string]string, name string, _ ...string) ([]byte, error) {
		if strings.HasSuffix(name, filepath.Join("venv", "bin", "ansible")) {
			return []byte("remote host failure"), errors.New("ansible process failed: exit status 2")
		}
		return []byte("diagnostic output"), nil
	}
	lifecycle := &kubesprayLifecycle{
		runner:        runner,
		stateDir:      t.TempDir(),
		inventoryPath: t.TempDir(),
		venvPath:      filepath.Join(t.TempDir(), "venv"),
		outputs:       kubesprayLifecycleOutputs{InventoryPath: "/state/inventory/inventory.yaml", ContractVersion: "2", APIAddress: "10.0.0.5", APIPort: 6443},
	}
	err := lifecycle.waitCloudInit(context.Background(), time.Second)
	if err == nil || !strings.Contains(err.Error(), "exit status 2") {
		t.Fatalf("waitCloudInit() error = %v, want Ansible status-two failure", err)
	}
}

func TestKubesprayExportKubeconfigNormalizesAtomicallyToStateTarget(t *testing.T) {
	stateDir := t.TempDir()
	target := filepath.Join(stateDir, "owned", "kubeconfig.yaml")
	lifecycle := &kubesprayLifecycle{
		runner:         &fakeLifecycleRunner{},
		stateDir:       stateDir,
		inventoryPath:  filepath.Join(stateDir, "inventory"),
		venvPath:       filepath.Join(stateDir, "venv"),
		kubeconfigPath: target,
		kubeconfigTemp: filepath.Join(stateDir, ".kubeconfig.fetch.tmp"),
		outputs:        kubesprayLifecycleOutputs{InventoryPath: "/state/inventory/inventory.yaml", ContractVersion: "2", APIAddress: "api.example.test", APIPort: 7443},
	}
	lifecycle.runner.(*fakeLifecycleRunner).onRun = func(_ string, _ map[string]string, _ string, args ...string) ([]byte, error) {
		for i, arg := range args {
			if arg == "-a" && i+1 < len(args) {
				for _, part := range strings.Fields(args[i+1]) {
					if strings.HasPrefix(part, "dest=") {
						return nil, os.WriteFile(strings.TrimPrefix(part, "dest="), []byte("server: https://127.0.0.1:6443\n"), 0o600)
					}
				}
			}
		}
		return nil, nil
	}
	if err := lifecycle.exportKubeconfig(context.Background()); err != nil {
		t.Fatalf("exportKubeconfig() error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, "https://api.example.test:7443") {
		t.Fatalf("kubeconfig = %q, want normalized address and port", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("kubeconfig mode = %o, want 600", info.Mode().Perm())
	}
	if _, err := os.Stat(lifecycle.kubeconfigTemp); !os.IsNotExist(err) {
		t.Fatalf("temporary kubeconfig still exists, err=%v", err)
	}
}

func TestNormalizeKubeconfigAPIEndpointPreservesCRLFAndUpdatesHostAndPort(t *testing.T) {
	input := []byte("apiVersion: v1\r\nclusters:\r\n- cluster:\r\n    server: https://127.0.0.1:6443\r\n  name: demo\r\n")
	got, err := normalizeKubeconfigAPIEndpoint(input, "10.0.0.8", 7443)
	if err != nil {
		t.Fatalf("normalizeKubeconfigAPIEndpoint() error = %v", err)
	}
	if !strings.Contains(string(got), "server: https://10.0.0.8:7443\r\n") {
		t.Fatalf("normalized endpoint = %q", got)
	}
	if strings.Contains(string(got), "\n") && strings.Contains(string(got), "\r\n") {
		for _, line := range strings.Split(string(got), "\n") {
			if line != "" && !strings.HasSuffix(line, "\r") {
				t.Fatalf("line ending was not preserved in %q", got)
			}
		}
	}
}

func TestNormalizeKubeconfigAPIEndpointRejectsInvalidYAML(t *testing.T) {
	if _, err := normalizeKubeconfigAPIEndpoint([]byte("clusters: [\n"), "10.0.0.8", 7443); err == nil {
		t.Fatal("normalizeKubeconfigAPIEndpoint() accepted invalid YAML")
	}
}

func TestKubesprayHardeningUsesCLIOwnedLifecycleStep(t *testing.T) {
	cfg := mustNewClusterTestConfig("hardening", "openstack")
	cfg.OpenCenter.Infrastructure.Networking.Security.OSHardening = true
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", cfg.ClusterName())
	if err := os.MkdirAll(clusterDir, 0o700); err != nil {
		t.Fatal(err)
	}
	provider := &openstackBootstrapProvider{runner: &fakeLifecycleRunner{}}
	steps, err := provider.BuildSteps(&cfg, &paths.ClusterPaths{ClusterStateDir: filepath.Join(t.TempDir(), "state"), InventoryPath: filepath.Join(t.TempDir(), "state", "inventory")}, &BootstrapOptions{KubeconfigPath: filepath.Join(t.TempDir(), "kubeconfig")})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	var hardening, deploy bootstrapStep
	for _, step := range steps {
		switch step.ID {
		case "kubespray-os-hardening":
			hardening = step
		case "kubespray-deploy":
			deploy = step
		}
	}
	if len(hardening.Plan.Commands) != 1 || !strings.Contains(strings.Join(hardening.Plan.Commands[0].Args, " "), "os_hardening_playbook.yml") {
		t.Fatalf("hardening commands = %#v, want CLI-owned playbook command", hardening.Plan.Commands)
	}
	if len(deploy.Plan.Commands) != 1 || strings.Contains(strings.Join(deploy.Plan.Commands[0].Args, " "), "os_hardening_playbook.yml") {
		t.Fatalf("deploy commands = %#v, want only Kubespray deployment", deploy.Plan.Commands)
	}
}

func TestKubesprayOSHardeningStagesPinnedRoleAndExistingPlaybookSemantics(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	// Contract v2: the module renders the inventory directly into the state zone.
	sourceInventory := filepath.Join(stateDir, "inventory")
	if err := os.MkdirAll(sourceInventory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceInventory, "inventory.yaml"), []byte("all:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "kubespray"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeLifecycleRunner{onRun: func(_ string, _ map[string]string, name string, args ...string) ([]byte, error) {
		if name == "tofu" && len(args) > 0 && args[0] == "output" {
			return []byte(`{"opencenter_kubespray_inventory_path":{"value":"` + filepath.Join(sourceInventory, "inventory.yaml") + `"},"opencenter_kubespray_lifecycle_contract_version":{"value":2},"opencenter_kubespray_api_address":{"value":"10.0.0.5"},"opencenter_kubespray_api_port":{"value":6443}}`), nil
		}
		return nil, nil
	}}
	lifecycle := newKubesprayLifecycle(runner, "tofu", root, &paths.ClusterPaths{ClusterStateDir: stateDir, InventoryPath: filepath.Join(stateDir, "inventory")}, filepath.Join(stateDir, "kubeconfig"), nil)
	lifecycle.osHardening = true
	cfg := mustNewClusterTestConfig("hardening-owned", "openstack")
	cfg.OpenCenter.Infrastructure.Networking.Security.OSHardening = true
	if err := lifecycle.prepare(context.Background(), &cfg); err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	playbook, err := os.ReadFile(filepath.Join(stateDir, "inventory", "os_hardening_playbook.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(playbook)
	if !strings.Contains(content, "roles:\n    - ansible-hardening") || !strings.Contains(content, "Harden all OS Systems") {
		t.Fatalf("playbook does not preserve gitops-base semantics: %s", content)
	}
	joined := ""
	for _, call := range runner.calls {
		joined += call.name + " " + strings.Join(call.args, " ") + "\n"
	}
	if !strings.Contains(joined, "git clone "+ansibleHardeningRepositoryURL) || !strings.Contains(joined, "git -C "+filepath.Join(stateDir, "inventory", "roles", "ansible-hardening")+" checkout --detach "+ansibleHardeningRef) {
		t.Fatalf("missing pinned ansible-hardening preparation commands:\n%s", joined)
	}
	if got := runner.calls[len(runner.calls)-1].name; got != "git" {
		t.Fatalf("last preparation command = %q, want git checkout", got)
	}
	for _, call := range runner.calls {
		if strings.HasSuffix(call.name, filepath.Join("venv", "bin", "ansible-playbook")) {
			if call.env["ANSIBLE_ROLES_PATH"] != filepath.Join(stateDir, "inventory", "roles") {
				t.Fatalf("hardening ANSIBLE_ROLES_PATH = %q", call.env["ANSIBLE_ROLES_PATH"])
			}
		}
	}
}

// newDeployTestLifecycle builds a lifecycle whose outputs are already loaded,
// so deploy() reaches the credential gate without touching OpenTofu.
func newDeployTestLifecycle(runner lifecycleCommandRunner, stateDir string, openTofuEnv map[string]string) *kubesprayLifecycle {
	return &kubesprayLifecycle{
		runner:        runner,
		openTofuEnv:   openTofuEnv,
		stateDir:      stateDir,
		inventoryPath: filepath.Join(stateDir, "inventory"),
		venvPath:      filepath.Join(stateDir, "venv"),
		kubesprayPath: filepath.Join(stateDir, "kubespray"),
		outputs: kubesprayLifecycleOutputs{
			InventoryPath:   filepath.Join(stateDir, "inventory", "inventory.yaml"),
			ContractVersion: kubesprayLifecycleContractVersion,
			APIAddress:      "10.0.0.5",
			APIPort:         6443,
		},
	}
}

// TestKubesprayDeployForwardsCCMCredentials covers the positive path of
// OCTR-750: the OpenTofu credential environment must reach the Ansible run, or
// the external CCM fails with "external_openstack_auth_url is missing" and
// every node keeps the cloud-provider-uninitialized taint.
func TestKubesprayDeployForwardsCCMCredentials(t *testing.T) {
	runner := &fakeLifecycleRunner{}
	lifecycle := newDeployTestLifecycle(runner, t.TempDir(), map[string]string{
		"OS_AUTH_URL":                      "https://keystone.api.iad3.rackspacecloud.com/v3/",
		"OS_APPLICATION_CREDENTIAL_ID":     "cred-id",
		"OS_APPLICATION_CREDENTIAL_SECRET": "cred-secret",
		"OS_REGION_NAME":                   "IAD3",
		"TF_VAR_unrelated":                 "must-not-leak",
	})
	cfg := mustNewClusterTestConfig("fwd", "openstack")

	if err := lifecycle.deploy(context.Background(), &cfg); err != nil {
		t.Fatalf("deploy() error = %v", err)
	}
	var env map[string]string
	for _, call := range runner.calls {
		if strings.HasSuffix(call.name, filepath.Join("venv", "bin", "ansible-playbook")) {
			env = call.env
		}
	}
	if env == nil {
		t.Fatal("no ansible-playbook invocation recorded")
	}
	if env["OS_AUTH_URL"] != "https://keystone.api.iad3.rackspacecloud.com/v3/" || env["OS_APPLICATION_CREDENTIAL_SECRET"] != "cred-secret" {
		t.Fatalf("CCM credentials were not forwarded to ansible: %#v", env)
	}
	if _, ok := env["TF_VAR_unrelated"]; ok {
		t.Fatalf("non-OS_ variable leaked into the ansible environment: %#v", env)
	}
}

// TestKubesprayDeployAbortsWithoutCCMCredentials pins the gate: deploying
// without credible OpenStack credentials produces a cluster that is Ready,
// permanently tainted, and unable to schedule anything, while deploy reports
// success. The gate must fail before Ansible starts.
func TestKubesprayDeployAbortsWithoutCCMCredentials(t *testing.T) {
	tests := []struct {
		name        string
		openTofuEnv map[string]string
		wantMissing string
	}{
		{
			name:        "no credentials at all",
			openTofuEnv: nil,
			wantMissing: "OS_AUTH_URL",
		},
		{
			name: "init placeholders",
			openTofuEnv: map[string]string{
				"OS_AUTH_URL":                      "https://keystone.api.iad3.rackspacecloud.com/v3/",
				"OS_APPLICATION_CREDENTIAL_ID":     v2.PlaceholderSecret,
				"OS_APPLICATION_CREDENTIAL_SECRET": v2.PlaceholderSecret,
			},
			wantMissing: "OS_APPLICATION_CREDENTIAL_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeLifecycleRunner{}
			lifecycle := newDeployTestLifecycle(runner, t.TempDir(), tt.openTofuEnv)
			cfg := mustNewClusterTestConfig("gate", "openstack")

			err := lifecycle.deploy(context.Background(), &cfg)
			if err == nil {
				t.Fatal("deploy() accepted an environment the CCM cannot authenticate with")
			}
			for _, want := range []string{tt.wantMissing, cloudProviderInitTaintKey, "cluster provider openstack"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q missing %q", err.Error(), want)
				}
			}
			for _, call := range runner.calls {
				if strings.HasSuffix(call.name, "ansible-playbook") {
					t.Fatalf("ansible-playbook ran despite the credential gate: %#v", call.args)
				}
			}
		})
	}
}

// TestKubesprayDeployAcceptsPasswordAuthAndSkipsMagnum covers the two shapes
// that must NOT be gated: username/password credentials (valid Keystone auth
// without an application credential) and Magnum, which runs its own CCM.
func TestKubesprayDeployAcceptsPasswordAuthAndSkipsMagnum(t *testing.T) {
	t.Run("username and password satisfy the gate", func(t *testing.T) {
		runner := &fakeLifecycleRunner{}
		lifecycle := newDeployTestLifecycle(runner, t.TempDir(), map[string]string{
			"OS_AUTH_URL": "https://keystone.api.iad3.rackspacecloud.com/v3/",
			"OS_USERNAME": "user",
			"OS_PASSWORD": "pass",
		})
		cfg := mustNewClusterTestConfig("password-auth", "openstack")
		if err := lifecycle.deploy(context.Background(), &cfg); err != nil {
			t.Fatalf("deploy() error = %v", err)
		}
	})

	t.Run("magnum is exempt", func(t *testing.T) {
		runner := &fakeLifecycleRunner{}
		lifecycle := newDeployTestLifecycle(runner, t.TempDir(), nil)
		cfg := mustNewClusterTestConfig("magnum-cluster", "magnum")
		if err := lifecycle.deploy(context.Background(), &cfg); err != nil {
			t.Fatalf("deploy() error = %v, want the Magnum CCM exemption to apply", err)
		}
	})
}

type waitCloudInitRunner struct {
	calls     []string
	deadlines []time.Time
	err       error
}

func (r *waitCloudInitRunner) Run(ctx context.Context, _ string, _ map[string]string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	deadline, _ := ctx.Deadline()
	r.deadlines = append(r.deadlines, deadline)
	return nil, r.err
}
