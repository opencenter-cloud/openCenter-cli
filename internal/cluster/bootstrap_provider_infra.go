package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	openstackprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

type openstackBootstrapProvider struct {
	runner lifecycleCommandRunner
	// validateCredentials authenticates the cluster's OpenStack credentials at
	// preflight. It is a field so tests can substitute a stub; nil means the
	// real Keystone check.
	validateCredentials func(context.Context, *credentials.OpenStackCredentials) error
}

const calicoStateResource = "module.calico.local_file.calico_values"

// newOpenStackBootstrapProvider returns the concrete provider so callers can
// substitute validateCredentials (tests use placeholder keystone hosts that must
// never be contacted). It satisfies lifecycleBootstrapProvider.
func newOpenStackBootstrapProvider(runner lifecycleCommandRunner) *openstackBootstrapProvider {
	return &openstackBootstrapProvider{runner: runner, validateCredentials: ValidateOpenStackLive}
}

func (p *openstackBootstrapProvider) BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for bootstrap")
	}

	openTofuPath, err := resolveTofuBinary(cfg.OpenTofu.Path)
	if err != nil {
		return nil, err
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	planEnv := providerPlanEnv(provider, opts.KubeconfigPath)
	var kubeLifecycle *kubesprayLifecycle

	inventoryPath := ""
	if clusterPaths != nil {
		inventoryPath = clusterPaths.InventoryPath
	}
	if strings.TrimSpace(inventoryPath) == "" {
		stateDir := ""
		if clusterPaths != nil {
			stateDir = clusterPaths.ClusterStateDir
		}
		if strings.TrimSpace(stateDir) == "" {
			stateDir = filepath.Join(clusterDir, ".opencenter-state")
		}
		inventoryPath = filepath.Join(stateDir, "inventory")
	}

	isKubespray := strings.EqualFold(strings.TrimSpace(cfg.Deployment.Method), "kubespray")
	applyArgs := []string{"apply", "-auto-approve", "-var=opencenter_lifecycle_mode=cli"}
	if isKubespray && strings.TrimSpace(inventoryPath) != "" {
		applyArgs = append(applyArgs, "-var=inventory_output_dir="+inventoryPath)
	}

	steps := []bootstrapStep{
		p.buildPreflightStep(cfg, provider, clusterDir),
		{
			ID:          "opentofu-init",
			Description: "Initialize OpenTofu",
			NeverSkip:   true,
			Plan: BootstrapPlanStep{
				ID:          "opentofu-init",
				Action:      "Initialize OpenTofu",
				WorkingDir:  clusterDir,
				Commands:    []BootstrapPlanCommand{commandPlan(openTofuPath, "init")},
				Environment: planEnv,
				Reads:       []string{clusterDir},
				Writes:      []string{filepath.Join(clusterDir, ".terraform")},
				Notes:       []string{"Plan only; OpenTofu binary, backend access, and provider initialization were not checked."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildProviderBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				if kubeLifecycle != nil {
					kubeLifecycle.openTofuEnv = cloneStringMap(env)
				}
				_, runErr := p.runner.Run(ctx, clusterDir, env, openTofuPath, "init")
				return runErr
			},
		},
		{
			ID:          "opentofu-apply",
			Description: "Apply OpenTofu infrastructure",
			CanSkip: func() bool {
				if isKubespray && strings.TrimSpace(inventoryPath) != "" {
					if _, err := os.Stat(filepath.Join(inventoryPath, "inventory.yaml")); err != nil {
						return false
					}
				}
				return true
			},
			Plan: BootstrapPlanStep{
				ID:          "opentofu-apply",
				Action:      "Apply OpenTofu infrastructure",
				WorkingDir:  clusterDir,
				Commands:    []BootstrapPlanCommand{commandPlan(openTofuPath, applyArgs...)},
				Environment: planEnv,
				Reads:       []string{clusterDir},
				Writes:      []string{"infrastructure resources", filepath.Join(clusterDir, "terraform.tfstate")},
				Notes:       []string{"Plan only; API access and infrastructure changes were not simulated."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildProviderBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				if isKubespray && strings.TrimSpace(inventoryPath) != "" {
					env["TF_VAR_inventory_output_dir"] = inventoryPath
				}
				if kubeLifecycle != nil {
					kubeLifecycle.openTofuEnv = cloneStringMap(env)
				}
				return p.runOpenTofuApply(ctx, cfg, clusterDir, applyArgs, env, openTofuPath)
			},
		},
	}

	if strings.EqualFold(strings.TrimSpace(cfg.Deployment.Method), "kubespray") {
		// Seed the credential environment from the config so it is present no
		// matter which step sequence actually runs: a full deploy refreshes it
		// in opentofu-init/apply, but `--step kubespray-deploy` / `--from-step`
		// reach deploy() without ever running those. Planning must not fail
		// here: deploy() gates on the credentials themselves.
		var kubeEnv map[string]string
		if seeded, seedErr := buildProviderBootstrapEnvironment(cfg, opts.KubeconfigPath); seedErr == nil {
			kubeEnv = seeded
		}
		kubeLifecycle = newKubesprayLifecycle(p.runner, openTofuPath, clusterDir, clusterPaths, opts.KubeconfigPath, kubeEnv)
		cloudInitTimeout, err := kubesprayCloudInitTimeout(cfg)
		if err != nil {
			return nil, err
		}
		prepareCommands := []BootstrapPlanCommand{
			kubeLifecycle.outputPlan(),
			commandPlan("git", "clone", "--branch", kubesprayVersion(cfg), "--depth", "1", kubesprayRepositoryURL, kubeLifecycle.kubesprayPath),
			commandPlan("python3", "-m", "venv", kubeLifecycle.venvPath),
			commandPlan(filepath.Join(kubeLifecycle.venvPath, "bin", "pip"), "install", "-r", filepath.Join(kubeLifecycle.kubesprayPath, "requirements.txt")),
		}
		prepareWrites := []string{kubeLifecycle.stateDir, kubeLifecycle.inventoryPath, kubeLifecycle.outputsPath}
		if kubeLifecycle.osHardening {
			prepareCommands = append(prepareCommands,
				commandPlan("git", "clone", ansibleHardeningRepositoryURL, filepath.Join(kubeLifecycle.inventoryPath, "roles", "ansible-hardening")),
				commandPlan("git", "-C", filepath.Join(kubeLifecycle.inventoryPath, "roles", "ansible-hardening"), "checkout", "--detach", ansibleHardeningRef),
			)
			prepareWrites = append(prepareWrites,
				filepath.Join(kubeLifecycle.inventoryPath, "roles", "ansible-hardening"),
				filepath.Join(kubeLifecycle.inventoryPath, "os_hardening_playbook.yml"),
			)
		}
		steps = append(steps,
			bootstrapStep{
				ID:          "kubespray-prepare",
				Description: "Prepare Kubespray execution environment",
				CanSkip:     kubeLifecycle.prepareArtifactsPresent,
				Plan: BootstrapPlanStep{
					ID:          "kubespray-prepare",
					Action:      "Prepare Kubespray execution environment",
					WorkingDir:  kubeLifecycle.stateDir,
					Commands:    prepareCommands,
					Environment: planEnv,
					Reads:       []string{clusterDir},
					Writes:      prepareWrites,
					Notes:       []string{"Contract v2: OpenTofu renders the inventory directly into the state zone during opentofu-apply; this step prepares only the Kubespray checkout, venv, and hardening role. Nothing is written to the GitOps worktree."},
				},
				Run: func(ctx context.Context) error { return kubeLifecycle.prepare(ctx, cfg) },
			},
			bootstrapStep{
				ID:          "kubespray-wait-cloudinit",
				Description: "Wait for cloud-init on every Kubespray host",
				Plan: BootstrapPlanStep{
					ID:          "kubespray-wait-cloudinit",
					Action:      "Wait for cloud-init on every Kubespray host",
					WorkingDir:  kubeLifecycle.stateDir,
					Commands:    []BootstrapPlanCommand{commandPlan(filepath.Join(kubeLifecycle.venvPath, "bin", "ansible"), "k8s_cluster", "-i", filepath.Join(kubeLifecycle.inventoryPath, "inventory.yaml"), "-b", "-m", "shell", "-a", `cloud-init status --wait; rc=$?; case "$rc" in 0|2) exit 0;; *) exit "$rc";; esac`)},
					Environment: envPlanFromMap(kubeLifecycle.environment(), nil),
					Reads:       []string{kubeLifecycle.inventoryPath},
					Notes:       []string{fmt.Sprintf("Context timeout: %s. Failed waits collect per-host ping, hostname, cloud-init, blame, and journal diagnostics.", cloudInitTimeout)},
				},
				Run: func(ctx context.Context) error { return kubeLifecycle.waitCloudInit(ctx, cloudInitTimeout) },
			},
		)
		if cfg.OpenCenter.Infrastructure.Networking.Security.OSHardening {
			hardeningPlanEnv := kubeLifecycle.environment()
			hardeningPlanEnv["ANSIBLE_ROLES_PATH"] = filepath.Join(kubeLifecycle.inventoryPath, "roles")
			steps = append(steps, bootstrapStep{
				ID:          "kubespray-os-hardening",
				Description: "Apply CLI-owned Kubespray OS hardening",
				Plan: BootstrapPlanStep{
					ID:          "kubespray-os-hardening",
					Action:      "Apply CLI-owned Kubespray OS hardening",
					WorkingDir:  kubeLifecycle.stateDir,
					Commands:    []BootstrapPlanCommand{commandPlan(filepath.Join(kubeLifecycle.venvPath, "bin", "ansible-playbook"), "-i", filepath.Join(kubeLifecycle.inventoryPath, "inventory.yaml"), filepath.Join(kubeLifecycle.inventoryPath, "os_hardening_playbook.yml"), "-f", "10", "-b", "--become-user=root")},
					Environment: envPlanFromMap(hardeningPlanEnv, nil),
					Reads:       []string{kubeLifecycle.inventoryPath},
				},
				Run: func(ctx context.Context) error { return kubeLifecycle.harden(ctx) },
			})
		}
		deployCommands := []BootstrapPlanCommand{}
		deployPlanEnv := kubeLifecycle.environment()
		deployPlanEnv["ANSIBLE_ROLES_PATH"] = filepath.Join(kubeLifecycle.kubesprayPath, "roles")
		// The plan must show what the run actually sends: deploy() forwards the
		// OpenTofu credential environment to Ansible because the external CCM
		// reads OS_AUTH_URL / OS_APPLICATION_CREDENTIAL_* from it. Omitting them
		// here made a dry-run plan look credential-free for a step that cannot
		// produce a schedulable cluster without them.
		for _, entry := range providerPlanEnv(provider, opts.KubeconfigPath) {
			if strings.HasPrefix(entry.Name, "OS_") {
				deployPlanEnv[entry.Name] = entry.Value
			}
		}
		deployCommands = append(deployCommands, commandPlan(filepath.Join(kubeLifecycle.venvPath, "bin", "ansible-playbook"), "-i", filepath.Join(kubeLifecycle.inventoryPath, "inventory.yaml"), filepath.Join(kubeLifecycle.kubesprayPath, "cluster.yml"), "-f", "10", "-b", "--become-user=root"))
		steps = append(steps,
			bootstrapStep{
				ID:          "kubespray-deploy",
				Description: "Deploy Kubernetes with Kubespray",
				Plan: BootstrapPlanStep{
					ID:          "kubespray-deploy",
					Action:      "Deploy Kubernetes with Kubespray",
					WorkingDir:  kubeLifecycle.stateDir,
					Commands:    deployCommands,
					Environment: envPlanFromMap(deployPlanEnv, nil),
					Reads:       []string{kubeLifecycle.inventoryPath, kubeLifecycle.kubesprayPath},
				},
				Run: func(ctx context.Context) error { return kubeLifecycle.deploy(ctx, cfg) },
			},
			bootstrapStep{
				ID:          "kubespray-export-kubeconfig",
				Description: "Export the Kubernetes admin kubeconfig",
				CanSkip: func() bool {
					// The exported admin kubeconfig lives in the secrets zone, which
					// outlives the cluster directory; its presence is the evidence
					// this step succeeded. The next step re-verifies the file's
					// contents against the live API endpoint, so a stale kubeconfig
					// cannot slip through a resume.
					if kubeLifecycle == nil || strings.TrimSpace(kubeLifecycle.kubeconfigPath) == "" {
						return true
					}
					_, err := os.Stat(kubeLifecycle.kubeconfigPath)
					return err == nil
				},
				Plan: BootstrapPlanStep{
					ID:          "kubespray-export-kubeconfig",
					Action:      "Export the Kubernetes admin kubeconfig",
					WorkingDir:  kubeLifecycle.stateDir,
					Commands:    []BootstrapPlanCommand{commandPlan(filepath.Join(kubeLifecycle.venvPath, "bin", "ansible"), "kube_control_plane[0]", "-i", filepath.Join(kubeLifecycle.inventoryPath, "inventory.yaml"), "-b", "-m", "fetch", "-a", "src=/etc/kubernetes/admin.conf dest="+kubeLifecycle.kubeconfigTemp+" flat=true")},
					Environment: envPlanFromMap(kubeLifecycle.environment(), nil),
					Reads:       []string{kubeLifecycle.inventoryPath},
					Writes:      []string{opts.KubeconfigPath},
				},
				Run: func(ctx context.Context) error { return kubeLifecycle.exportKubeconfig(ctx) },
			},
		)
	}

	apiEndpointIP := resolveAPIEndpointIP(cfg)
	steps = append(steps, bootstrapStep{
		ID:          "openstack-normalize-kubeconfig",
		Description: "Normalize kubeconfig into the cluster-owned path and replace localhost with VIP",
		Plan: BootstrapPlanStep{
			ID:         "openstack-normalize-kubeconfig",
			Action:     "Normalize kubeconfig into the cluster-owned path and replace localhost with VIP",
			WorkingDir: clusterDir,
			Reads:      kubeconfigCandidatePaths(clusterDir, opts.KubeconfigPath),
			Writes:     []string{opts.KubeconfigPath},
			Notes:      []string{"Plan only; kubeconfig candidates were not checked."},
		},
		Run: func(ctx context.Context) error {
			if kubeLifecycle != nil {
				if err := kubeLifecycle.loadOutputs(); err != nil {
					return fmt.Errorf("validate Kubespray outputs before kubeconfig normalization: %w", err)
				}
				return kubeLifecycle.verifyExportedKubeconfig()
			}
			return normalizeOpenStackKubeconfig(clusterDir, opts.KubeconfigPath, apiEndpointIP)
		},
	})

	networkPluginStep, err := p.buildNetworkPluginInstallStep(cfg, clusterDir, planEnv, opts)
	if err != nil {
		return nil, err
	}
	steps = append(steps, networkPluginStep)

	// Patch CoreDNS to tolerate the cloud-provider-uninitialized taint so cluster
	// DNS is available before Flux bootstraps. Without this, CoreDNS cannot schedule
	// on tainted nodes, source-controller cannot resolve external endpoints, and the
	// GitRepository never becomes Ready — blocking CCM installation indefinitely.
	steps = append(steps, p.buildCoreDNSTolerationStep(opts.KubeconfigPath))

	// Flux bootstrap runs after the CNI is installed so the cluster is
	// network-ready when FluxCD source-controller starts reconciling.
	// Only add the step when token auth is configured and a real (non-placeholder)
	// repository URL is present.
	if cfg.OpenCenter.GitOps.Auth.Token != nil &&
		strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider) != "" &&
		cfg.ConfiguredGitURL() != "" {
		fluxStep, err := p.buildFluxBootstrapStep(cfg, clusterDir, planEnv, opts)
		if err != nil {
			return nil, fmt.Errorf("building flux bootstrap step: %w", err)
		}
		steps = append(steps, fluxStep)
		// The sops-age Secret must land before the CCM gate. Flux decrypts the
		// SOPS-encrypted overlay that renders the CCM with this key, and
		// openstack-ccm-override depends on it: without the Secret the override
		// fails with `secrets "sops-age" not found`, openstack-ccm-base never
		// runs, no CCM is ever installed, and the gate below polls a taint that
		// nothing can ever clear. Observed on kit-kat, where deploy blocked here
		// while cert-manager, CSI, headlamp, metrics-server and the postgres
		// stack sat unschedulable behind the same missing key.
		steps = append(steps, newSopsAgeSecretStep(clusterPaths.SOPSKeyPath, opts.KubeconfigPath, p.runner))
		// The CCM is installed by Flux, so its effect can only be observed
		// after bootstrap. This gate is what turns "cluster is up" into
		// "cluster can actually run workloads".
		steps = append(steps, p.buildWaitForCCMStep(opts.KubeconfigPath))
		// No opencenter-base credential Secret is created: the shared
		// openCenter-gitops-base repository is public, so its GitRepository
		// source is rendered anonymously (no secretRef).
		steps = append(steps, newGrafanaAdminSecretStep(cfg, opts.KubeconfigPath, p.runner))
	}

	return steps, nil
}

func (p *openstackBootstrapProvider) runOpenTofuApply(ctx context.Context, cfg *v2.Config, clusterDir string, applyArgs []string, env map[string]string, openTofuPath string) error {
	if len(applyArgs) == 0 {
		applyArgs = []string{"apply", "-auto-approve", "-var=opencenter_lifecycle_mode=cli"}
	}
	if !isHelmManagedCalico(cfg) {
		_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, applyArgs...)
		return err
	}

	state, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, "state", "list")
	if err != nil {
		if isOpenTofuNoStateFileResponse(state, err) {
			state = nil
		} else {
			return fmt.Errorf("Calico OpenTofu state safety check failed in %s: tofu state list: %w", clusterDir, err)
		}
	}
	if strings.Contains(string(state), calicoStateResource) {
		return fmt.Errorf("refusing to apply: OpenTofu state in %s contains %s; from the current infrastructure directory, run the non-destructive migration command: tofu state rm '%s'", clusterDir, calicoStateResource, calicoStateResource)
	}

	snapshot, err := snapshotCalicoGitOpsFiles(cfg)
	if err != nil {
		return fmt.Errorf("snapshot Calico GitOps files before OpenTofu apply: %w", err)
	}

	_, applyErr := p.runner.Run(ctx, clusterDir, env, openTofuPath, applyArgs...)
	verifyErr := verifyCalicoGitOpsFiles(cfg, snapshot)
	if verifyErr != nil {
		if applyErr != nil {
			return fmt.Errorf("OpenTofu apply failed: %w; Calico GitOps file safety check failed: %v", applyErr, verifyErr)
		}
		return verifyErr
	}
	return applyErr
}

func isOpenTofuNoStateFileResponse(output []byte, err error) bool {
	const noStateFileMessage = "no state file was found"

	if strings.Contains(strings.ToLower(string(output)), noStateFileMessage) {
		return true
	}
	return err != nil && strings.Contains(strings.ToLower(err.Error()), noStateFileMessage)
}

func isHelmManagedCalico(cfg *v2.Config) bool {
	if cfg == nil || cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico == nil {
		return false
	}
	calico := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
	if !calico.Enabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(calico.InstallMethod)) {
	case "", openStackNetworkPluginMethodHelm, openStackNetworkPluginMethodKustomizeHelm:
		return true
	default:
		return false
	}
}

func calicoGitOpsFilesRoot(cfg *v2.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is nil")
	}
	gitDir := strings.TrimSpace(cfg.GitDir())
	if gitDir == "" {
		return "", fmt.Errorf("gitops.git_dir must be configured for Calico GitOps safety checks")
	}
	clusterName := strings.TrimSpace(cfg.ClusterName())
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must be set for Calico GitOps safety checks")
	}
	return filepath.Join(gitDir, "applications", "overlays", clusterName, "services", "calico"), nil
}

type calicoGitOpsSnapshot map[string][]byte

func snapshotCalicoGitOpsFiles(cfg *v2.Config) (calicoGitOpsSnapshot, error) {
	root, err := calicoGitOpsFilesRoot(cfg)
	if err != nil {
		return nil, err
	}

	snapshot := make(calicoGitOpsSnapshot)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && path == root {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve relative path for %s: %w", path, err)
		}
		snapshot[relative] = append([]byte(nil), data...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func verifyCalicoGitOpsFiles(cfg *v2.Config, before calicoGitOpsSnapshot) error {
	after, err := snapshotCalicoGitOpsFiles(cfg)
	if err != nil {
		return fmt.Errorf("snapshot Calico GitOps files after OpenTofu apply: %w", err)
	}

	changed := make([]string, 0)
	for path, beforeData := range before {
		afterData, ok := after[path]
		if !ok {
			changed = append(changed, "deleted "+path)
			continue
		}
		if !bytes.Equal(beforeData, afterData) {
			changed = append(changed, "changed "+path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			changed = append(changed, "added "+path)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	sort.Strings(changed)
	return fmt.Errorf("Calico GitOps files changed during OpenTofu apply; no files were restored: %s", strings.Join(changed, ", "))
}

func extractOpenStackBootstrapCredentials(cfg *v2.Config) (*credentials.OpenStackCredentials, error) {
	extractor := credentials.NewExtractor(*cfg)
	creds, err := extractor.ExtractOpenStack()
	if err != nil {
		return nil, fmt.Errorf("extract openstack credentials: %w", err)
	}
	return creds, nil
}

// resolveOpenStackBootstrapCredentials extracts the cluster's OpenStack
// credentials and rejects any that still hold an unreplaced placeholder.
//
// `cluster init` writes CHANGEME into application_credential_id/secret, so a
// config that was never bound to a cloud carries syntactically valid but
// useless values. The extractor's IsEmpty check passes them, and
// mergeBootstrapEnvironment then publishes OS_APPLICATION_CREDENTIAL_ID=CHANGEME
// into the OpenTofu environment and, via the kubespray deploy step, into the
// Ansible run. The result is the kit-kat failure: OpenTofu applies against the
// operator's own OS_* ambient cloud, the cluster comes up, and the GitOps CCM
// renders a cloudConfig whose credential is the literal string CHANGEME. It
// can never authenticate, so it never sets spec.providerID or clears the
// node.cloudprovider.kubernetes.io/uninitialized taint, and the cluster stays
// unschedulable behind a deploy that reported success.
//
// Failing here — before any command runs — is the only place that catches it:
// OpenTofu authenticates with whatever OS_* it is handed, so a placeholder
// never surfaces as a provisioning error.
func resolveOpenStackBootstrapCredentials(cfg *v2.Config) (*credentials.OpenStackCredentials, error) {
	creds, err := extractOpenStackBootstrapCredentials(cfg)
	if err != nil {
		return nil, err
	}
	identity := cfg.ClusterName()
	if organization := strings.TrimSpace(cfg.Organization()); organization != "" {
		identity = organization + "/" + identity
	}
	for _, field := range []struct {
		path  string
		value string
	}{
		{"opencenter.infrastructure.cloud.openstack.auth_url", creds.AuthURL},
		{"opencenter.infrastructure.cloud.openstack.application_credential_id", creds.ApplicationCredentialID},
		{"opencenter.infrastructure.cloud.openstack.application_credential_secret", creds.ApplicationCredentialSecret},
	} {
		if isEmptyOrPlaceholder(field.value) {
			return nil, fmt.Errorf("%s for cluster %s is %s; bind the cluster to a cloud with `opencenter cluster provider openstack` (or set the value) before deploying", field.path, identity, strings.TrimSpace(field.value))
		}
	}
	return creds, nil
}

// validateOpenStackBootstrap authenticates the cluster's OpenStack credentials
// against Keystone for real. Placeholder rejection lives in
// resolveOpenStackBootstrapCredentials, which every caller goes through first.
//
// The live check is what makes this worth the network call. auth_url is
// rendered verbatim into the in-cluster OpenStack CCM cloudConfig and the
// cinder CSI cloud.conf, so a credential that belongs to a different region
// than auth_url serves leaves the cluster permanently broken even though
// OpenTofu — which authenticates through OS_* environment variables —
// provisions it successfully. The observed failure was 24 hours of a cluster
// whose CCM could never authenticate, so the cloud-provider-uninitialized
// taint was never removed and nothing could schedule.
func (p *openstackBootstrapProvider) validateOpenStackBootstrap(creds *credentials.OpenStackCredentials) error {
	if creds == nil || creds.IsEmpty() {
		return fmt.Errorf("openstack credentials are incomplete; set auth_url and application credentials or username/password before bootstrap")
	}

	for _, warning := range openstackprovider.PreflightOpenStack(creds.AuthURL) {
		if strings.Contains(warning, "auth_url is empty") {
			return fmt.Errorf("%s", warning)
		}
	}

	validate := p.validateCredentials
	if validate == nil {
		validate = ValidateOpenStackLive
	}
	return validate(context.Background(), creds)
}

// ValidateOpenStackLive authenticates the cluster's OpenStack credentials and
// asserts that the configured region is served by the configured auth_url. It
// is exported so validate and deploy share one implementation.
func ValidateOpenStackLive(ctx context.Context, creds *credentials.OpenStackCredentials) error {
	if creds == nil {
		return nil
	}
	_, err := openstackprovider.ValidateOpenStackCredentials(ctx, openstackprovider.CredentialCheckOptions{
		AuthURL:                     creds.AuthURL,
		Region:                      creds.Region,
		ApplicationCredentialID:     creds.ApplicationCredentialID,
		ApplicationCredentialSecret: creds.ApplicationCredentialSecret,
		Domain:                      creds.Domain,
		TenantName:                  creds.TenantName,
		ProjectName:                 creds.ProjectName,
		ProjectDomainName:           creds.ProjectDomainName,
		UserDomainName:              creds.UserDomainName,
		Insecure:                    creds.Insecure,
		Username:                    creds.Username,
		Password:                    creds.Password,
	})
	if err != nil {
		return fmt.Errorf("openstack credential preflight: %w", err)
	}
	return nil
}

func buildBootstrapEnvironment(kubeconfigPath string) map[string]string {
	env := make(map[string]string)

	if strings.TrimSpace(kubeconfigPath) != "" {
		env["KUBECONFIG"] = kubeconfigPath
	}
	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}

func mergeBootstrapEnvironment(target, extra map[string]string) {
	for key, value := range extra {
		if strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
	}
}

// resolveAPIEndpointIP returns the IP address that should replace localhost
// in the kubeconfig server URL. It prefers the explicit k8s_api_ip override
// and falls back to the VRRP VIP when VRRP is enabled.
func resolveAPIEndpointIP(cfg *v2.Config) string {
	if ip := strings.TrimSpace(cfg.OpenCenter.Infrastructure.K8sAPIIP); ip != "" {
		return ip
	}
	net := cfg.OpenCenter.Infrastructure.Networking
	if net.VRRPEnabled {
		return strings.TrimSpace(net.VRRPIP)
	}
	return ""
}

func infrastructureClusterDir(cfg *v2.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is nil")
	}

	gitDir := strings.TrimSpace(cfg.GitDir())
	if gitDir == "" {
		return "", fmt.Errorf("gitops.git_dir must be configured for provider %q", cfg.Provider())
	}

	clusterName := strings.TrimSpace(cfg.ClusterName())
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must be set")
	}

	return filepath.Join(gitDir, "infrastructure", "clusters", clusterName), nil
}

func normalizeOpenStackKubeconfig(clusterDir, targetPath, apiEndpointIP string) error {
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("kubeconfig path must be set")
	}

	candidates := []string{
		targetPath,
		filepath.Join(clusterDir, "kubeconfig.yaml"),
		filepath.Join(clusterDir, "kubeconfig"),
		filepath.Join(clusterDir, "kube_config_cluster.yml"),
	}

	var sourcePath string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			sourcePath = candidate
			break
		}
	}

	if sourcePath == "" {
		return fmt.Errorf("kubeconfig not found after bootstrap in %s", clusterDir)
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read kubeconfig %s: %w", sourcePath, err)
	}

	data = replaceLocalhostInKubeconfig(data, apiEndpointIP)
	if err := validateKubeconfigYAML(data); err != nil {
		return fmt.Errorf("validate normalized kubeconfig: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return fmt.Errorf("create kubeconfig directory: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(targetPath), ".kubeconfig.normalized-*")
	if err != nil {
		return fmt.Errorf("create temporary kubeconfig: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("set temporary kubeconfig permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary kubeconfig: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary kubeconfig: %w", err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("atomically install kubeconfig %s: %w", targetPath, err)
	}
	if err := os.Chmod(targetPath, 0o600); err != nil {
		return fmt.Errorf("set kubeconfig permissions: %w", err)
	}

	return nil
}

// replaceLocalhostInKubeconfig rewrites cluster server URLs that point to
// localhost (127.0.0.1 or ::1) so they use the cluster's VIP instead.
// This is necessary for OpenStack deployments where the bootstrap tooling
// (e.g. Kubespray) writes a kubeconfig with a localhost endpoint that is
// only reachable from the control-plane node itself.
//
// When apiEndpointIP is empty the data is returned unchanged.
func replaceLocalhostInKubeconfig(data []byte, apiEndpointIP string) []byte {
	if strings.TrimSpace(apiEndpointIP) == "" {
		return data
	}

	// Match server lines whose host portion is a localhost address.
	// Kubeconfig server values follow the pattern:
	//   server: https://<host>:<port>
	// We replace only the host, preserving scheme and port.
	localhostHosts := []string{"127.0.0.1", "localhost", "[::1]"}
	result := string(data)
	for _, host := range localhostHosts {
		// Replace https://<localhost>: with https://<vip>:
		result = strings.ReplaceAll(result,
			"https://"+host+":",
			"https://"+apiEndpointIP+":")
		// Also handle the less common http:// variant
		result = strings.ReplaceAll(result,
			"http://"+host+":",
			"http://"+apiEndpointIP+":")
	}
	return []byte(result)
}

// buildCoreDNSTolerationStep returns a bootstrap step that patches the CoreDNS
// Deployment to tolerate the cloud-provider-uninitialized taint. Kubespray
// leaves nodes tainted until the external CCM initialises them, but CCM is
// installed by Flux/GitOps, which requires cluster DNS. Without this toleration
// CoreDNS cannot schedule, DNS is unavailable, and Flux source-controller
// cannot resolve external git endpoints — creating a deadlock. The step is
// non-fatal (the patch is idempotent on repeat runs) and logs a warning rather
// than failing the bootstrap if the patch is not needed or already applied.
func (p *openstackBootstrapProvider) buildCoreDNSTolerationStep(kubeconfigPath string) bootstrapStep {
	return bootstrapStep{
		ID:          "patch-coredns-toleration",
		Description: "Patch CoreDNS to tolerate the cloud-provider-uninitialized taint",
		Plan: BootstrapPlanStep{
			ID:     "patch-coredns-toleration",
			Action: "kubectl patch deployment coredns -n kube-system (add cloud-provider-uninitialized toleration)",
		},
		Run: func(ctx context.Context) error {
			// Strategic merge patch: Kubernetes merges tolerations by key, so
			// this is idempotent and safe to apply on every bootstrap run.
			patch := `{"spec":{"template":{"spec":{"tolerations":[{"key":"node.cloudprovider.kubernetes.io/uninitialized","operator":"Exists","effect":"NoSchedule"}]}}}}`
			_, err := p.runner.Run(ctx, "", nil, "kubectl",
				"--kubeconfig", kubeconfigPath,
				"-n", "kube-system",
				"patch", "deployment", "coredns",
				"--type=strategic",
				"-p", patch,
			)
			return err
		},
	}
}

// buildPreflightStep returns a provider-aware preflight validation step.
func (p *openstackBootstrapProvider) buildPreflightStep(cfg *v2.Config, provider, clusterDir string) bootstrapStep {
	return bootstrapStep{
		ID:          "preflight",
		Description: "Validate credentials and bootstrap prerequisites",
		Plan: BootstrapPlanStep{
			ID:         "preflight",
			Action:     "Validate credentials and bootstrap prerequisites",
			WorkingDir: clusterDir,
			Reads:      []string{clusterDir},
			Notes:      []string{"Plan only; credentials, infrastructure directory, and OpenTofu availability were not checked."},
		},
		Run: func(ctx context.Context) error {
			if _, err := os.Stat(clusterDir); err != nil {
				return fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
			}
			if err := p.validateProviderBootstrap(cfg, provider); err != nil {
				return err
			}
			return p.checkBastionSSH(ctx, cfg)
		},
	}
}

// validateProviderBootstrap performs provider-specific preflight validation.
func (p *openstackBootstrapProvider) validateProviderBootstrap(cfg *v2.Config, provider string) error {
	switch provider {
	case "openstack":
		creds, err := resolveOpenStackBootstrapCredentials(cfg)
		if err != nil {
			return err
		}
		return p.validateOpenStackBootstrap(creds)
	case "vmware", "vsphere":
		secret := extractVSphereBootstrapCredentials(cfg)
		if isEmptyOrPlaceholder(secret.VCenterHost) || isEmptyOrPlaceholder(secret.Username) || isEmptyOrPlaceholder(secret.Password) {
			return fmt.Errorf("vmware credentials incomplete; set secrets.vsphere_csi (vcenter_host, username, password)")
		}
		return validateStaticNodes(cfg)
	case "baremetal":
		return validateStaticNodes(cfg)
	default:
		return fmt.Errorf("unsupported provider %q for bootstrap", provider)
	}
}

// validateStaticNodes checks that pre-provisioned nodes are defined in the config.
func validateStaticNodes(cfg *v2.Config) error {
	compute := cfg.OpenCenter.Infrastructure.Compute
	hasNodes := len(compute.MasterNodes) > 0

	// Also accept vmware cloud nodes as a source.
	if !hasNodes && cfg.OpenCenter.Infrastructure.Cloud.VMware != nil {
		// VMware nodes are defined in cloud.vmware.nodes via the schema;
		// the Terraform module reads them directly from the config.
		hasNodes = true
	}

	if !hasNodes {
		return fmt.Errorf("no master nodes defined; set infrastructure.compute.master_nodes for static node deployment")
	}

	if strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.User) == "" &&
		strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.Username) == "" {
		return fmt.Errorf("ssh user must be set for static node deployment; set infrastructure.ssh.user")
	}
	return nil
}

// checkBastionSSH verifies SSH connectivity through the bastion host.
// It SSHs to the bastion and confirms it can reach localhost via SSH.
// Skipped when bastion is not enabled or has no address.
func (p *openstackBootstrapProvider) checkBastionSSH(ctx context.Context, cfg *v2.Config) error {
	bastion := cfg.OpenCenter.Infrastructure.Bastion
	if !bastion.Enabled {
		return nil
	}
	bastionAddr := strings.TrimSpace(bastion.Address)
	if bastionAddr == "" {
		return nil
	}

	sshUser := strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.Username)
	if sshUser == "" {
		sshUser = strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.User)
	}
	if sshUser == "" {
		sshUser = "ubuntu"
	}

	keyPath := strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.KeyPath)

	// Build SSH args for bastion connectivity check
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
	}
	if keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	args = append(args, fmt.Sprintf("%s@%s", sshUser, bastionAddr), "ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "localhost", "true")

	_, err := p.runner.Run(ctx, "", nil, "ssh", args...)
	if err != nil {
		return fmt.Errorf("bastion SSH check failed: cannot SSH to localhost from bastion %s: %w", bastionAddr, err)
	}
	return nil
}

// buildProviderBootstrapEnvironment returns environment variables for the
// OpenTofu run, selecting credentials based on the infrastructure provider.
//
// The OpenStack branch resolves credentials through
// resolveOpenStackBootstrapCredentials, so a placeholder never reaches the
// environment: this map is what OpenTofu authenticates with AND what the
// kubespray deploy step forwards to Ansible for the external CCM.
func buildProviderBootstrapEnvironment(cfg *v2.Config, kubeconfigPath string) (map[string]string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	env := buildBootstrapEnvironment(kubeconfigPath)

	switch provider {
	case "openstack":
		creds, err := resolveOpenStackBootstrapCredentials(cfg)
		if err != nil {
			return nil, err
		}
		mergeBootstrapEnvironment(env, creds.ToEnvMap())
	case "vmware", "vsphere":
		secret := extractVSphereBootstrapCredentials(cfg)
		if strings.TrimSpace(secret.VCenterHost) != "" {
			env["VSPHERE_SERVER"] = secret.VCenterHost
		}
		if strings.TrimSpace(secret.Username) != "" {
			env["VSPHERE_USER"] = secret.Username
		}
		if strings.TrimSpace(secret.Password) != "" {
			env["VSPHERE_PASSWORD"] = secret.Password
		}
		if strings.TrimSpace(secret.InsecureFlag) != "" {
			env["VSPHERE_ALLOW_UNVERIFIED_SSL"] = secret.InsecureFlag
		}
	case "baremetal":
		// No extra credentials needed.
	}

	return env, nil
}

// vSphereBootstrapSecret holds credentials extracted from the cluster config.
type vSphereBootstrapSecret struct {
	VCenterHost  string `json:"vcenter_host" yaml:"vcenter_host"`
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password" yaml:"password"`
	InsecureFlag string `json:"insecure_flag" yaml:"insecure_flag"`
}

// extractVSphereBootstrapCredentials extracts vSphere credentials from the
// secrets.vsphere_csi or secrets.vsphere-csi config block, falling back to
// the typed secrets.VSphereCsi struct and infrastructure.cloud.vmware for the host.
func extractVSphereBootstrapCredentials(cfg *v2.Config) vSphereBootstrapSecret {
	var secret vSphereBootstrapSecret
	for _, key := range []string{"vsphere_csi", "vsphere-csi"} {
		raw, ok := cfg.Secrets.ServiceSecrets[key]
		if !ok || raw == nil {
			continue
		}
		data, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &secret); err == nil {
			break
		}
	}

	// Fall back to the typed secrets.vsphere_csi struct fields.
	if strings.TrimSpace(secret.VCenterHost) == "" {
		secret.VCenterHost = strings.TrimSpace(cfg.Secrets.VSphereCsi.VCenterHost)
	}
	if strings.TrimSpace(secret.Username) == "" {
		secret.Username = strings.TrimSpace(cfg.Secrets.VSphereCsi.Username)
	}
	if strings.TrimSpace(secret.Password) == "" {
		secret.Password = cfg.Secrets.VSphereCsi.Password
	}
	if strings.TrimSpace(secret.InsecureFlag) == "" {
		secret.InsecureFlag = strings.TrimSpace(cfg.Secrets.VSphereCsi.InsecureFlag)
	}

	// Fall back to cloud.vmware.vcenter_server if still no host.
	if strings.TrimSpace(secret.VCenterHost) == "" {
		if vmwareCfg := cfg.OpenCenter.Infrastructure.Cloud.VMware; vmwareCfg != nil {
			secret.VCenterHost = strings.TrimSpace(vmwareCfg.VCenterServer)
		}
	}

	// Normalize: strip https:// scheme from vcenter_host so VSPHERE_SERVER
	// receives a bare hostname. Users may paste a full URL.
	secret.VCenterHost = normalizeVCenterHost(secret.VCenterHost)

	return secret
}

// normalizeVCenterHost strips an https:// or http:// prefix from a vCenter
// host value, returning just the hostname (with port if present). This allows
// users to set secrets.vsphere_csi.vcenter_host to either
// "vcenter.example.com" or "https://vcenter.example.com".
func normalizeVCenterHost(host string) string {
	host = strings.TrimSpace(host)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(host), prefix) {
			host = host[len(prefix):]
			// Strip trailing path (e.g. "/sdk")
			if idx := strings.Index(host, "/"); idx != -1 {
				host = host[:idx]
			}
			break
		}
	}
	return host
}

// isEmptyOrPlaceholder returns true if the value is empty or still set to the
// default placeholder ("CHANGEME") that must be replaced before deployment.
func isEmptyOrPlaceholder(value string) bool {
	v := strings.TrimSpace(value)
	return v == "" || strings.EqualFold(v, "CHANGEME")
}

// providerPlanEnv returns the dry-run plan environment for the given provider.
func providerPlanEnv(provider, kubeconfigPath string) []BootstrapPlanEnv {
	switch provider {
	case "vmware", "vsphere":
		env := map[string]string{
			"VSPHERE_SERVER":               "",
			"VSPHERE_USER":                 "",
			"VSPHERE_PASSWORD":             "",
			"VSPHERE_ALLOW_UNVERIFIED_SSL": "",
			"PATH":                         "<current PATH>",
		}
		if strings.TrimSpace(kubeconfigPath) != "" {
			env["KUBECONFIG"] = kubeconfigPath
		}
		redacted := map[string]bool{
			"VSPHERE_SERVER":   true,
			"VSPHERE_USER":     true,
			"VSPHERE_PASSWORD": true,
		}
		return envPlanFromMap(env, redacted)
	case "baremetal":
		env := map[string]string{
			"PATH": "<current PATH>",
		}
		if strings.TrimSpace(kubeconfigPath) != "" {
			env["KUBECONFIG"] = kubeconfigPath
		}
		return envPlanFromMap(env, nil)
	default:
		return openStackPlanEnv(kubeconfigPath)
	}
}
