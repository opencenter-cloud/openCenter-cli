package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"gopkg.in/yaml.v3"
)

const (
	kubesprayLifecycleContractVersion = "2"
	kubesprayOutputsFile              = "kubespray-outputs.json"
	kubesprayRepositoryURL            = "https://github.com/kubernetes-sigs/kubespray.git"
	ansibleHardeningRepositoryURL     = "https://opendev.org/openstack/ansible-hardening"
	ansibleHardeningVersion           = "stable/2025.1"
	// The branch exists only as a remote-tracking ref after a plain clone. A bare
	// name makes `checkout --detach` try to create a local branch, which git
	// rejects as a conflict with -b.
	ansibleHardeningRef = "origin/" + ansibleHardeningVersion
)

type kubesprayLifecycleOutputs struct {
	InventoryPath   string `json:"inventory_path"`
	ContractVersion string `json:"lifecycle_contract_version"`
	APIAddress      string `json:"api_address"`
	APIPort         int    `json:"api_port"`
}

type kubesprayLifecycle struct {
	runner         lifecycleCommandRunner
	openTofuPath   string
	openTofuEnv    map[string]string
	clusterDir     string
	stateDir       string
	inventoryPath  string
	venvPath       string
	kubesprayPath  string
	kubeconfigPath string
	kubeconfigTemp string
	outputsPath    string
	sshControlPath string
	outputs        kubesprayLifecycleOutputs
	osHardening    bool
}

func newKubesprayLifecycle(runner lifecycleCommandRunner, openTofuPath, clusterDir string, clusterPaths *paths.ClusterPaths, kubeconfigPath string, openTofuEnv map[string]string) *kubesprayLifecycle {
	stateDir := ""
	inventoryPath := ""
	if clusterPaths != nil {
		stateDir = clusterPaths.ClusterStateDir
		inventoryPath = clusterPaths.InventoryPath
	}
	if strings.TrimSpace(stateDir) == "" {
		// BuildSteps is normally called with resolved paths. Keep the provider
		// unit-testable when a caller only supplies a rendered infrastructure dir.
		stateDir = filepath.Join(clusterDir, ".opencenter-state")
	}
	if strings.TrimSpace(inventoryPath) == "" {
		inventoryPath = filepath.Join(stateDir, "inventory")
	}
	clusterLabel := sanitizeRuntimeSegment(filepath.Base(clusterDir))
	if len(clusterLabel) > 24 {
		clusterLabel = clusterLabel[:24]
	}
	shortTempDir := "/tmp"
	if _, err := os.Stat(shortTempDir); err != nil {
		shortTempDir = os.TempDir()
	}
	kubeconfigTemp := filepath.Join(stateDir, ".kubeconfig.fetch.tmp")
	if strings.TrimSpace(kubeconfigPath) != "" {
		kubeconfigTemp = filepath.Join(filepath.Dir(kubeconfigPath), ".kubeconfig.fetch.tmp")
	}
	return &kubesprayLifecycle{
		runner:         runner,
		openTofuPath:   openTofuPath,
		openTofuEnv:    cloneStringMap(openTofuEnv),
		clusterDir:     clusterDir,
		stateDir:       stateDir,
		inventoryPath:  inventoryPath,
		venvPath:       filepath.Join(stateDir, "venv"),
		kubesprayPath:  filepath.Join(stateDir, "kubespray"),
		kubeconfigPath: kubeconfigPath,
		kubeconfigTemp: kubeconfigTemp,
		outputsPath:    filepath.Join(stateDir, kubesprayOutputsFile),
		sshControlPath: filepath.Join(shortTempDir, "oc-ssh-"+clusterLabel),
	}
}

func (l *kubesprayLifecycle) outputPlan() BootstrapPlanCommand {
	return commandPlan(l.openTofuPath, "output", "-json")
}

func (l *kubesprayLifecycle) environment() map[string]string {
	pathValue := filepath.Join(l.venvPath, "bin")
	if current := strings.TrimSpace(os.Getenv("PATH")); current != "" {
		pathValue += string(os.PathListSeparator) + current
	}
	return map[string]string{
		"ANSIBLE_INVENTORY":            filepath.Join(l.inventoryPath, "inventory.yaml"),
		"ANSIBLE_HOST_KEY_CHECKING":    "False",
		"ANSIBLE_LOCAL_TEMP":           filepath.Join(l.stateDir, "ansible-local-tmp"),
		"ANSIBLE_SSH_CONTROL_PATH_DIR": l.sshControlPath,
		"PATH":                         pathValue,
	}
}

func (l *kubesprayLifecycle) prepare(ctx context.Context, cfg *v2.Config) error {
	output, err := l.runner.Run(ctx, l.clusterDir, l.openTofuEnv, l.openTofuPath, "output", "-json")
	if err != nil {
		return fmt.Errorf("read Kubespray OpenTofu outputs: %w", err)
	}
	outputs, err := parseKubesprayLifecycleOutputs(output)
	if err != nil {
		return err
	}
	l.outputs = outputs

	if err := os.MkdirAll(l.stateDir, 0o700); err != nil {
		return fmt.Errorf("create Kubespray state directory: %w", err)
	}
	for _, dir := range []string{filepath.Join(l.stateDir, "ansible-local-tmp"), l.sshControlPath} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create Kubespray execution directory %s: %w", dir, err)
		}
	}
	encoded, err := json.MarshalIndent(outputs, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize Kubespray outputs: %w", err)
	}
	if err := os.WriteFile(l.outputsPath, encoded, 0o600); err != nil {
		return fmt.Errorf("save Kubespray outputs: %w", err)
	}

	if err := stageKubesprayInventory(outputs.InventoryPath, l.inventoryPath); err != nil {
		return err
	}
	version := "v2.31.0"
	if cfg != nil && cfg.Deployment.Kubespray != nil && strings.TrimSpace(cfg.Deployment.Kubespray.Version) != "" {
		version = strings.TrimSpace(cfg.Deployment.Kubespray.Version)
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
	}
	l.osHardening = cfg != nil && cfg.OpenCenter.Infrastructure.Networking.Security.OSHardening

	if _, err := os.Stat(l.kubesprayPath); os.IsNotExist(err) {
		if _, err := l.runner.Run(ctx, l.stateDir, nil, "git", "clone", "--branch", version, "--depth", "1", kubesprayRepositoryURL, l.kubesprayPath); err != nil {
			return fmt.Errorf("clone Kubespray %s: %w", version, err)
		}
	} else if err != nil {
		return fmt.Errorf("check Kubespray path %s: %w", l.kubesprayPath, err)
	} else {
		if _, err := l.runner.Run(ctx, l.stateDir, nil, "git", "-C", l.kubesprayPath, "fetch", "--tags", "--force", "origin"); err != nil {
			return fmt.Errorf("refresh Kubespray repository: %w", err)
		}
	}
	if _, err := l.runner.Run(ctx, l.stateDir, nil, "git", "-C", l.kubesprayPath, "checkout", "--detach", version); err != nil {
		return fmt.Errorf("checkout Kubespray %s: %w", version, err)
	}
	if _, err := l.runner.Run(ctx, l.stateDir, nil, "python3", "-m", "venv", l.venvPath); err != nil {
		return fmt.Errorf("create Kubespray virtual environment: %w", err)
	}
	pip := filepath.Join(l.venvPath, "bin", "pip")
	if _, err := l.runner.Run(ctx, l.stateDir, nil, pip, "install", "--upgrade", "pip"); err != nil {
		return fmt.Errorf("upgrade Kubespray pip: %w", err)
	}
	if _, err := l.runner.Run(ctx, l.stateDir, nil, pip, "install", "-r", filepath.Join(l.kubesprayPath, "requirements.txt")); err != nil {
		return fmt.Errorf("install Kubespray requirements: %w", err)
	}
	if l.osHardening {
		if err := l.prepareOSHardeningContext(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (l *kubesprayLifecycle) loadOutputs() error {
	if strings.TrimSpace(l.outputs.InventoryPath) != "" {
		return validateKubesprayLifecycleOutputs(l.outputs)
	}
	data, err := os.ReadFile(l.outputsPath)
	if err != nil {
		return fmt.Errorf("read saved Kubespray outputs %s: %w", l.outputsPath, err)
	}
	var outputs kubesprayLifecycleOutputs
	if err := json.Unmarshal(data, &outputs); err != nil {
		return fmt.Errorf("parse saved Kubespray outputs: %w", err)
	}
	l.outputs = outputs
	return validateKubesprayLifecycleOutputs(outputs)
}

func parseKubesprayLifecycleOutputs(data []byte) (kubesprayLifecycleOutputs, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return kubesprayLifecycleOutputs{}, fmt.Errorf("parse Kubespray OpenTofu outputs: %w", err)
	}
	var outputs kubesprayLifecycleOutputs
	outputValue := func(name string) (json.RawMessage, bool) {
		value, ok := raw[name]
		if !ok || string(value) == "null" {
			return nil, false
		}
		// `tofu output -json` returns {"value": ...} for named outputs.
		// Accept direct values too; this keeps parsing compatible with small
		// test runners and older Terraform-compatible implementations.
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(value, &envelope); err == nil {
			if nested, exists := envelope["value"]; exists {
				return nested, string(nested) != "null"
			}
		}
		return value, true
	}
	decodeString := func(name string, target *string) error {
		value, ok := outputValue(name)
		if !ok {
			return nil
		}
		return json.Unmarshal(value, target)
	}
	if err := decodeString("opencenter_kubespray_inventory_path", &outputs.InventoryPath); err != nil {
		return outputs, fmt.Errorf("decode Kubespray inventory output: %w", err)
	}
	if err := decodeString("opencenter_kubespray_lifecycle_contract_version", &outputs.ContractVersion); err != nil {
		var number json.Number
		if rawValue, ok := outputValue("opencenter_kubespray_lifecycle_contract_version"); ok {
			if err := json.Unmarshal(rawValue, &number); err == nil {
				outputs.ContractVersion = number.String()
			}
		}
	}
	if err := decodeString("opencenter_kubespray_api_address", &outputs.APIAddress); err != nil {
		return outputs, fmt.Errorf("decode Kubespray API address output: %w", err)
	}
	if rawValue, ok := outputValue("opencenter_kubespray_api_port"); ok {
		if err := json.Unmarshal(rawValue, &outputs.APIPort); err != nil {
			return outputs, fmt.Errorf("decode Kubespray API port output: %w", err)
		}
	}
	if err := validateKubesprayLifecycleOutputs(outputs); err != nil {
		return outputs, err
	}
	return outputs, nil
}

func validateKubesprayLifecycleOutputs(outputs kubesprayLifecycleOutputs) error {
	if strings.TrimSpace(outputs.ContractVersion) != kubesprayLifecycleContractVersion {
		return fmt.Errorf("unsupported Kubespray lifecycle contract %q; CLI requires contract %s", outputs.ContractVersion, kubesprayLifecycleContractVersion)
	}
	if strings.TrimSpace(outputs.InventoryPath) == "" {
		return fmt.Errorf("OpenTofu output opencenter_kubespray_inventory_path is empty")
	}
	if strings.TrimSpace(outputs.APIAddress) == "" {
		return fmt.Errorf("OpenTofu output opencenter_kubespray_api_address is empty")
	}
	if outputs.APIPort < 1 || outputs.APIPort > 65535 {
		return fmt.Errorf("OpenTofu output opencenter_kubespray_api_port is invalid: %d", outputs.APIPort)
	}
	return nil
}

// stageKubesprayInventory verifies that the OpenTofu-rendered inventory subtree
// is exactly the state-zone copy the CLI executes against. Contract v2 modules
// render directly into l.inventoryPath (inventory_output_dir), so source and
// destination coincide and no copy happens. A module that still renders inside
// the GitOps worktree (contract v1 layout, pre-regeneration tree) fails with an
// actionable regeneration hint instead of silently producing two divergent
// copies.
func stageKubesprayInventory(source, destination string) error {
	source = filepath.Clean(strings.TrimSpace(source))
	destination = filepath.Clean(strings.TrimSpace(destination))
	if info, err := os.Stat(source); err == nil && !info.IsDir() {
		source = filepath.Dir(source)
	}
	if source == destination {
		return nil
	}
	return fmt.Errorf("Kubespray inventory rendered at %s outside the cluster state zone %s; the infrastructure tree predates lifecycle contract 2, re-run 'opencenter cluster generate' with the updated kubespray module ref and apply again", source, destination)
}

func (l *kubesprayLifecycle) prepareOSHardening() error {
	return l.prepareOSHardeningContext(context.Background())
}

func (l *kubesprayLifecycle) prepareOSHardeningContext(ctx context.Context) error {
	playbookPath := filepath.Join(l.inventoryPath, "os_hardening_playbook.yml")
	playbook := []byte(`---

- name: Harden all OS Systems
  hosts: k8s_cluster
  become: yes
  roles:
    - ansible-hardening
`)
	if err := os.WriteFile(playbookPath, playbook, 0o600); err != nil {
		return fmt.Errorf("write OS hardening playbook in state zone: %w", err)
	}

	rolesPath := filepath.Join(l.inventoryPath, "roles")
	rolePath := filepath.Join(rolesPath, "ansible-hardening")
	if err := os.MkdirAll(rolesPath, 0o700); err != nil {
		return fmt.Errorf("create OS hardening roles directory: %w", err)
	}
	if info, err := os.Stat(rolePath); os.IsNotExist(err) {
		if _, err := l.runner.Run(ctx, l.stateDir, nil, "git", "clone", ansibleHardeningRepositoryURL, rolePath); err != nil {
			return fmt.Errorf("clone ansible-hardening role: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check ansible-hardening role path %s: %w", rolePath, err)
	} else if !info.IsDir() {
		return fmt.Errorf("ansible-hardening role path %s is not a directory", rolePath)
	}
	if _, err := l.runner.Run(ctx, l.stateDir, nil, "git", "-C", rolePath, "checkout", "--detach", ansibleHardeningRef); err != nil {
		return fmt.Errorf("checkout ansible-hardening %s: %w", ansibleHardeningVersion, err)
	}
	return nil
}

// prepareArtifactsPresent reports whether every artifact kubespray-prepare
// produces still exists on disk. Bootstrap resume state records step success
// by name only, so a cluster directory that was wiped and regenerated
// (destroy --remove-files, a manual rm -rf, git clean -fdx) would otherwise
// leave `kubespray-prepare (already completed)` standing for artifacts that
// are gone: deploy then dies in loadOutputs on a missing
// kubespray-outputs.json, or Ansible dies on a missing venv. This is the same
// probe opentofu-apply's CanSkip performs on the state-zone inventory:
// on-disk truth, not saved status.
func (l *kubesprayLifecycle) prepareArtifactsPresent() bool {
	probe := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
	required := []string{
		l.outputsPath,
		filepath.Join(l.venvPath, "bin", "ansible-playbook"),
		filepath.Join(l.kubesprayPath, "cluster.yml"),
		filepath.Join(l.inventoryPath, "inventory.yaml"),
	}
	// The hardening playbook and role come from prepare's hardening pass, so
	// they are required whenever hardening is on: harden() executes the playbook
	// directly, and prepare only writes it when the inventory directory already
	// exists - a wiped tree with hardening enabled must re-run prepare.
	if l.osHardening {
		required = append(required,
			filepath.Join(l.inventoryPath, "os_hardening_playbook.yml"),
			filepath.Join(l.inventoryPath, "roles", "ansible-hardening"),
		)
	}
	for _, path := range required {
		if !probe(path) {
			return false
		}
	}
	return true
}

const (
	cloudInitRetryInterval      = time.Second
	cloudInitDiagnosticsTimeout = 2 * time.Minute
)

func (l *kubesprayLifecycle) waitCloudInit(ctx context.Context, timeout time.Duration) error {
	if err := l.loadOutputs(); err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	env := l.environment()
	ansible := filepath.Join(l.venvPath, "bin", "ansible")
	waitCommand := `cloud-init status --wait; rc=$?; case "$rc" in 0|2) exit 0;; *) exit "$rc";; esac`
	var lastErr error
	for {
		if err := waitCtx.Err(); err != nil {
			lastErr = err
			break
		}
		_, err := l.runner.Run(waitCtx, l.stateDir, env, ansible, "k8s_cluster", "-i", filepath.Join(l.inventoryPath, "inventory.yaml"), "-b", "-m", "shell", "-a", waitCommand)
		if err == nil {
			return nil
		}
		lastErr = err
		if waitCtx.Err() != nil {
			break
		}
		if !isAnsibleUnreachable(err) {
			break
		}
		timer := time.NewTimer(cloudInitRetryInterval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
		case <-timer.C:
		}
		if waitCtx.Err() != nil {
			break
		}
	}

	diagnosticCtx, diagnosticCancel := context.WithTimeout(context.WithoutCancel(ctx), cloudInitDiagnosticsTimeout)
	defer diagnosticCancel()
	var diagnosticResults []string
	runDiagnostic := func(label string, args ...string) {
		output, err := l.runner.Run(diagnosticCtx, l.stateDir, env, ansible, args...)
		result := label
		if len(output) > 0 {
			result += "\noutput:\n" + string(output)
		}
		if err != nil {
			result += "\nerror: " + err.Error()
		}
		diagnosticResults = append(diagnosticResults, result)
	}
	runDiagnostic("ping", "k8s_cluster", "-i", filepath.Join(l.inventoryPath, "inventory.yaml"), "-T", "10", "-m", "ping", "-o")
	runDiagnostic("cloud-init details", "k8s_cluster", "-i", filepath.Join(l.inventoryPath, "inventory.yaml"), "-T", "10", "-b", "-m", "shell", "-a", "hostname; cloud-init status --long; cloud-init analyze blame; for unit in cloud-init-local cloud-init cloud-config cloud-final; do journalctl -u \"$unit\" --no-pager -n 200 || true; done")
	diagnostics := strings.Join(diagnosticResults, "\n")
	if errors.Is(waitCtx.Err(), context.DeadlineExceeded) || errors.Is(lastErr, context.DeadlineExceeded) {
		timeoutErr := waitCtx.Err()
		if timeoutErr == nil {
			timeoutErr = lastErr
		}
		return fmt.Errorf("timed out waiting for cloud-init after %s: %w; per-host diagnostics collected:\n%s\ninitial failure: %v", timeout, timeoutErr, diagnostics, lastErr)
	}
	if waitCtx.Err() != nil {
		return fmt.Errorf("waiting for cloud-init canceled: %w", waitCtx.Err())
	}
	return fmt.Errorf("cloud-init wait failed before timeout: %w; per-host diagnostics collected:\n%s", lastErr, diagnostics)
}

func isAnsibleUnreachable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unreachable") || strings.Contains(message, "exit status 4") || strings.Contains(message, "rc=4")
}

func (l *kubesprayLifecycle) harden(ctx context.Context) error {
	if err := l.loadOutputs(); err != nil {
		return err
	}
	ansiblePlaybook := filepath.Join(l.venvPath, "bin", "ansible-playbook")
	env := l.environment()
	env["ANSIBLE_ROLES_PATH"] = filepath.Join(l.inventoryPath, "roles")
	_, err := l.runner.Run(ctx, l.stateDir, env, ansiblePlaybook, "-i", filepath.Join(l.inventoryPath, "inventory.yaml"), filepath.Join(l.inventoryPath, "os_hardening_playbook.yml"), "-f", "10", "-b", "--become-user=root")
	if err != nil {
		return fmt.Errorf("CLI-owned OS hardening: %w", err)
	}
	return nil
}

func (l *kubesprayLifecycle) deploy(ctx context.Context, cfg *v2.Config) error {
	if err := l.loadOutputs(); err != nil {
		return err
	}
	env := l.environment()
	env["ANSIBLE_ROLES_PATH"] = filepath.Join(l.kubesprayPath, "roles")
	// OCTR-750: forward OS_* creds to the kubespray ansible run so the external
	// OpenStack cloud-controller-manager role (external_openstack_* default to
	// lookup('env','OS_AUTH_URL') etc.) can configure itself. These are already
	// built for OpenTofu (l.openTofuEnv) but were not passed to ansible, so the
	// CCM failed with "external_openstack_auth_url is missing".
	for key, value := range l.openTofuEnv {
		if strings.HasPrefix(key, "OS_") {
			env[key] = value
		}
	}
	// The forward is best-effort by construction — it copies whatever OpenTofu
	// was handed — so it needs an explicit gate. GitOps owns the CCM, and with
	// cloud_provider: external the CCM is the ONLY thing that clears the
	// node.cloudprovider.kubernetes.io/uninitialized taint kubelet applies at
	// registration. A cluster deployed without usable OS_AUTH_URL /
	// OS_APPLICATION_CREDENTIAL_* comes up Ready, stays tainted, and schedules
	// nothing; the deploy itself reports success. Failing here costs minutes
	// instead of the 24 hours that shape cost on kit-kat. Magnum clusters are
	// excluded: Magnum provisions its own CCM and needs no in-cluster
	// OpenStack credential.
	if missing := missingCCMCredentialKeys(env); len(missing) > 0 && !isMagnumProvider(cfg) {
		return fmt.Errorf("kubespray deploy aborted: missing in the deploy environment: %s. The external cloud controller manager cannot authenticate without them, so every node would keep the %s taint and nothing would schedule. Bind the cluster to a cloud with `opencenter cluster provider openstack`, then re-run deploy", strings.Join(missing, ", "), cloudProviderInitTaintKey)
	}
	ansiblePlaybook := filepath.Join(l.venvPath, "bin", "ansible-playbook")
	args := []string{"-i", filepath.Join(l.inventoryPath, "inventory.yaml"), filepath.Join(l.kubesprayPath, "cluster.yml"), "-f", "10", "-b", "--become-user=root"}
	if cfg != nil && cfg.OpenCenter.Cluster.Kubernetes.Security.K8sHardening {
		args = append(args, "-e", "@"+filepath.Join(l.inventoryPath, "k8s_hardening.yml"))
	}
	_, err := l.runner.Run(ctx, l.stateDir, env, ansiblePlaybook, args...)
	return err
}

func (l *kubesprayLifecycle) exportKubeconfig(ctx context.Context) error {
	if err := l.loadOutputs(); err != nil {
		return err
	}
	if strings.TrimSpace(l.kubeconfigPath) == "" {
		return fmt.Errorf("kubeconfig path must be set")
	}
	if err := os.MkdirAll(filepath.Dir(l.kubeconfigPath), 0o700); err != nil {
		return fmt.Errorf("create kubeconfig directory: %w", err)
	}
	env := l.environment()
	ansible := filepath.Join(l.venvPath, "bin", "ansible")
	if err := os.Remove(l.kubeconfigTemp); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear temporary kubeconfig: %w", err)
	}
	if _, err := l.runner.Run(ctx, l.stateDir, env, ansible, "kube_control_plane[0]", "-i", filepath.Join(l.inventoryPath, "inventory.yaml"), "-b", "-m", "fetch", "-a", "src=/etc/kubernetes/admin.conf dest="+l.kubeconfigTemp+" flat=true"); err != nil {
		return err
	}
	data, err := os.ReadFile(l.kubeconfigTemp)
	if err != nil {
		return fmt.Errorf("read fetched kubeconfig: %w", err)
	}
	defer os.Remove(l.kubeconfigTemp)
	data, err = normalizeKubeconfigAPIEndpoint(data, l.outputs.APIAddress, l.outputs.APIPort)
	if err != nil {
		return fmt.Errorf("normalize fetched kubeconfig: %w", err)
	}
	targetTemp := filepath.Join(filepath.Dir(l.kubeconfigPath), "."+filepath.Base(l.kubeconfigPath)+".normalized.tmp")
	defer os.Remove(targetTemp)
	if err := os.WriteFile(targetTemp, data, 0o600); err != nil {
		return fmt.Errorf("write normalized kubeconfig: %w", err)
	}
	if err := os.Chmod(targetTemp, 0o600); err != nil {
		return fmt.Errorf("set normalized kubeconfig permissions: %w", err)
	}
	if err := os.Rename(targetTemp, l.kubeconfigPath); err != nil {
		return fmt.Errorf("atomically install kubeconfig: %w", err)
	}
	return os.Chmod(l.kubeconfigPath, 0o600)
}

func (l *kubesprayLifecycle) verifyExportedKubeconfig() error {
	data, err := os.ReadFile(l.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("read exported kubeconfig: %w", err)
	}
	info, err := os.Stat(l.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("stat exported kubeconfig: %w", err)
	}
	if info.Mode().Perm() != 0o600 {
		return fmt.Errorf("exported kubeconfig permissions are %o, want 600", info.Mode().Perm())
	}
	endpoint := "https://" + net.JoinHostPort(strings.Trim(strings.TrimSpace(l.outputs.APIAddress), "[]"), strconv.Itoa(l.outputs.APIPort))
	if !strings.Contains(string(data), endpoint) {
		return fmt.Errorf("exported kubeconfig does not contain API endpoint %s", endpoint)
	}
	return nil
}

func normalizeKubeconfigAPIEndpoint(data []byte, address string, port int) ([]byte, error) {
	if err := validateKubeconfigYAML(data); err != nil {
		return nil, err
	}
	address = strings.Trim(strings.TrimSpace(address), "[]")
	if address == "" || port <= 0 {
		return data, nil
	}
	host := net.JoinHostPort(address, strconv.Itoa(port))
	lines := strings.SplitAfter(string(data), "\n")
	for i, line := range lines {
		marker := strings.Index(line, "server:")
		if marker < 0 {
			continue
		}
		lineBody, lineEnding := line, ""
		if strings.HasSuffix(lineBody, "\n") {
			lineBody = strings.TrimSuffix(lineBody, "\n")
			lineEnding = "\n"
			if strings.HasSuffix(lineBody, "\r") {
				lineBody = strings.TrimSuffix(lineBody, "\r")
				lineEnding = "\r\n"
			}
		}
		valueStart := marker + len("server:")
		for valueStart < len(lineBody) && (lineBody[valueStart] == ' ' || lineBody[valueStart] == '\t') {
			valueStart++
		}
		valueEnd := len(lineBody)
		for valueEnd > valueStart && (lineBody[valueEnd-1] == ' ' || lineBody[valueEnd-1] == '\t') {
			valueEnd--
		}
		value := lineBody[valueStart:valueEnd]
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		u.Host = host
		lines[i] = lineBody[:valueStart] + u.String() + lineBody[valueEnd:] + lineEnding
	}
	result := []byte(strings.Join(lines, ""))
	if err := validateKubeconfigYAML(result); err != nil {
		return nil, fmt.Errorf("normalized kubeconfig is invalid YAML: %w", err)
	}
	return result, nil
}

func validateKubeconfigYAML(data []byte) error {
	var document interface{}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("kubeconfig is invalid YAML: %w", err)
	}
	return nil
}

func kubesprayCloudInitTimeout(cfg *v2.Config) (time.Duration, error) {
	if cfg == nil || cfg.Deployment.Kubespray == nil {
		return 10 * time.Minute, nil
	}
	timeout := cfg.Deployment.Kubespray.EffectiveCloudInitTimeout()
	if timeout <= 0 {
		return 0, fmt.Errorf("deployment.kubespray.cloud_init_timeout is invalid")
	}
	return timeout, nil
}

func kubesprayVersion(cfg *v2.Config) string {
	version := "v2.31.0"
	if cfg != nil && cfg.Deployment.Kubespray != nil && strings.TrimSpace(cfg.Deployment.Kubespray.Version) != "" {
		version = strings.TrimSpace(cfg.Deployment.Kubespray.Version)
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

// ccmCredentialEnvKeys are the environment variables the external OpenStack
// cloud controller manager needs to authenticate. Kubespray's own CCM role is
// disabled (external_cloud_provider: manual), so GitOps renders the CCM
// cloudConfig from the cluster config; these keys are what the CLI validates
// and what the deploy environment must carry for the kubespray-side lookups.
var ccmCredentialEnvKeys = []string{
	"OS_AUTH_URL",
	"OS_APPLICATION_CREDENTIAL_ID",
	"OS_APPLICATION_CREDENTIAL_SECRET",
}

// missingCCMCredentialKeys returns the CCM credential variables that are absent
// from env or still hold a `cluster init` placeholder. Username/password auth
// satisfies the check the same way it satisfies Keystone: an application
// credential is preferred, not required.
func missingCCMCredentialKeys(env map[string]string) []string {
	hasUserPass := !isEmptyOrPlaceholder(env["OS_USERNAME"]) && !isEmptyOrPlaceholder(env["OS_PASSWORD"])
	missing := make([]string, 0, len(ccmCredentialEnvKeys))
	for _, key := range ccmCredentialEnvKeys {
		if !isEmptyOrPlaceholder(env[key]) {
			continue
		}
		if hasUserPass && key != "OS_AUTH_URL" {
			continue
		}
		missing = append(missing, key)
	}
	return missing
}

// isMagnumProvider reports whether the cluster is provisioned by Magnum, which
// runs its own cloud controller manager and therefore needs no in-cluster
// OpenStack credential.
func isMagnumProvider(cfg *v2.Config) bool {
	if cfg == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Provider()), "magnum")
}
