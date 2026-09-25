package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	magnumprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/magnum"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

const magnumReadyPollInterval = 5 * time.Second

const magnumIdentityStateVersion = 2

const (
	magnumCreateInFlight = "create-in-flight"
	magnumCreateAccepted = "create-accepted"
)

type magnumIdentityState struct {
	Version      int    `json:"version"`
	ClusterID    string `json:"cluster_id,omitempty"`
	ClusterName  string `json:"cluster_name,omitempty"`
	AttemptState string `json:"attempt_state"`
	AttemptAt    string `json:"attempt_at"`
	Observed     bool   `json:"observed"`
}

type magnumBootstrapProvider struct {
	runner          lifecycleCommandRunner
	providerFactory func(magnumprovider.Config) (*magnumprovider.Provider, error)
	pollInterval    time.Duration
}

func newMagnumBootstrapProvider(runner lifecycleCommandRunner) lifecycleBootstrapProvider {
	return &magnumBootstrapProvider{
		runner:          runner,
		providerFactory: magnumprovider.NewProvider,
		pollInterval:    magnumReadyPollInterval,
	}
}

// BuildSteps builds Magnum API lifecycle steps. Magnum owns the compute,
// networking, and Kubernetes provisioning, so this provider deliberately does
// not inspect or require an OpenTofu infrastructure directory.
func (p *magnumBootstrapProvider) BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}
	magnumCfg := cfg.OpenCenter.Infrastructure.Cloud.Magnum
	if magnumCfg == nil {
		return nil, fmt.Errorf("opencenter.infrastructure.cloud.magnum must be configured for the magnum provider")
	}

	localOpts := BootstrapOptions{}
	if opts != nil {
		localOpts = *opts
	}
	if strings.TrimSpace(localOpts.KubeconfigPath) == "" {
		return nil, fmt.Errorf("kubeconfig path must be set for the magnum provider")
	}
	clusterName := strings.TrimSpace(cfg.ClusterName())
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be set for the magnum provider")
	}

	clientConfig := magnumConfigFromV2(magnumCfg)
	factory := p.providerFactory
	if factory == nil {
		factory = magnumprovider.NewProvider
	}
	provider, err := factory(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create Magnum provider: %w", err)
	}
	identityPath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		return nil, err
	}

	masterCount := cfg.OpenCenter.Infrastructure.Compute.MasterCount
	workerCount := cfg.OpenCenter.Infrastructure.Compute.WorkerCount
	request := magnumprovider.Request{
		Name:        clusterName,
		ClusterName: clusterName,
		MasterCount: masterCount,
		WorkerCount: workerCount,
		NodeCount:   workerCount,
	}
	kubeconfigPath := localOpts.KubeconfigPath
	steps := []bootstrapStep{
		{
			ID:          "magnum-create",
			Description: "Create Magnum cluster",
			Plan: BootstrapPlanStep{
				ID:       "magnum-create",
				Action:   "Create or recover the Magnum cluster through the Magnum API",
				Commands: []BootstrapPlanCommand{commandPlan("Magnum API", "create cluster")},
				Reads:    []string{identityPath},
				Writes:   []string{fmt.Sprintf("remote Magnum cluster %q", clusterName), identityPath},
				Notes:    []string{"Plan only; the durable create-attempt record is written before any create request, and retries recover visibility instead of creating again."},
			},
			Run: func(ctx context.Context) error {
				state, err := loadMagnumIdentityState(identityPath)
				if err != nil {
					return err
				}
				if state.AttemptState == magnumCreateInFlight || (state.AttemptState == magnumCreateAccepted && !state.Observed) {
					return recoverMagnumCreate(ctx, provider, identityPath, &state, clusterName, localOpts.Timeout, p.pollInterval)
				}
				if state.ClusterID != "" {
					return nil
				}

				existing, lookupErr := provider.GetCluster(ctx, clusterName)
				if lookupErr == nil {
					if !validMagnumClusterID(existing.ID) {
						return fmt.Errorf("existing Magnum cluster %q returned an invalid UUID", clusterName)
					}
					state = newMagnumIdentityState(clusterName, magnumCreateAccepted)
					state.ClusterID = existing.ID
					state.Observed = true
					return persistMagnumIdentityState(identityPath, state)
				}
				if !isMagnumNotFound(lookupErr) {
					return fmt.Errorf("resolve existing Magnum cluster %q: %w", clusterName, lookupErr)
				}

				state = newMagnumIdentityState(clusterName, magnumCreateInFlight)
				if err := persistMagnumIdentityState(identityPath, state); err != nil {
					return fmt.Errorf("persist Magnum create attempt: %w", err)
				}
				created, createErr := provider.CreateCluster(ctx, request)
				if createErr != nil {
					return fmt.Errorf("create Magnum cluster: %w", createErr)
				}
				if !validMagnumClusterID(created.ID) {
					return fmt.Errorf("create Magnum cluster returned an invalid UUID")
				}
				state.ClusterID = created.ID
				state.AttemptState = magnumCreateAccepted
				state.Observed = false
				if err := persistMagnumIdentityState(identityPath, state); err != nil {
					return fmt.Errorf("persist Magnum cluster identity: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "magnum-wait-ready",
			Description: "Wait for Magnum cluster readiness",
			Plan: BootstrapPlanStep{
				ID:       "magnum-wait-ready",
				Action:   "Wait for the durable Magnum UUID to become ready",
				Commands: []BootstrapPlanCommand{commandPlan("Magnum API", "poll cluster status")},
				Reads:    []string{identityPath},
				Writes:   []string{"remote Magnum cluster readiness state"},
				Notes:    []string{"Plan only; readiness is polled by UUID with the caller's bounded bootstrap timeout."},
			},
			Run: func(ctx context.Context) error {
				state, err := loadMagnumIdentityState(identityPath)
				if err != nil {
					return err
				}
				interval := p.pollInterval
				if interval <= 0 {
					interval = magnumReadyPollInterval
				}
				timeout := localOpts.Timeout
				if timeout <= 0 {
					timeout = defaultReadyTimeout
				}
				waitCtx, cancel := context.WithTimeout(nonNilClusterContext(ctx), timeout)
				defer cancel()
				if state.AttemptState == magnumCreateInFlight {
					if err := recoverMagnumCreateVisible(waitCtx, provider, identityPath, &state, clusterName, interval, timeout); err != nil {
						return err
					}
				} else if state.AttemptState == magnumCreateAccepted && !state.Observed {
					visible, visibilityErr := provider.WaitVisible(waitCtx, state.ClusterID, interval)
					if visibilityErr != nil {
						return fmt.Errorf("magnum create recovery required before readiness: %w", visibilityErr)
					}
					if !validMagnumClusterID(visible.ID) {
						return fmt.Errorf("magnum create recovery required before readiness: observed an invalid cluster UUID")
					}
					state.ClusterID = visible.ID
					state.Observed = true
					if err := persistMagnumIdentityState(identityPath, state); err != nil {
						return fmt.Errorf("persist observed Magnum cluster identity: %w", err)
					}
				}
				if state.ClusterID == "" || state.AttemptState != magnumCreateAccepted {
					return fmt.Errorf("magnum cluster identity is missing; run magnum-create first")
				}
				clusterID := state.ClusterID
				if _, err := provider.WaitReady(waitCtx, clusterID, interval); err != nil {
					return fmt.Errorf("wait for Magnum cluster readiness within %s: %w", timeout, err)
				}
				state.Observed = true
				if err := persistMagnumIdentityState(identityPath, state); err != nil {
					return fmt.Errorf("persist observed Magnum cluster identity: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "magnum-export-kubeconfig",
			Description: "Export Magnum kubeconfig",
			Plan: BootstrapPlanStep{
				ID:       "magnum-export-kubeconfig",
				Action:   "Export kubeconfig for the durable Magnum UUID",
				Commands: []BootstrapPlanCommand{commandPlan("Magnum API", "get cluster kubeconfig")},
				Reads:    []string{identityPath},
				Writes:   []string{filepath.Dir(kubeconfigPath), kubeconfigPath},
				Notes:    []string{"Plan only; kubeconfig contents and remote certificate material were not retrieved."},
			},
			Run: func(ctx context.Context) error {
				state, err := loadMagnumIdentityState(identityPath)
				if err != nil {
					return err
				}
				if state.ClusterID == "" || state.AttemptState != magnumCreateAccepted {
					return fmt.Errorf("magnum cluster identity is missing; run magnum-create first")
				}
				if err := provider.ExportKubeconfig(ctx, state.ClusterID, kubeconfigPath); err != nil {
					return fmt.Errorf("export Magnum kubeconfig: %w", err)
				}
				return nil
			},
		},
	}

	// Keep the normal GitOps post-kubeconfig behavior. These helpers are only
	// enabled under the same condition used by the OpenStack provider.
	if cfg.OpenCenter.GitOps.Auth.Token != nil &&
		strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider) != "" &&
		cfg.ConfiguredGitURL() != "" {
		gitOpsDir := ""
		sopsKeyPath := ""
		if clusterPaths != nil {
			gitOpsDir = clusterPaths.GitOpsDir
			sopsKeyPath = clusterPaths.SOPSKeyPath
		}
		openstackProvider := &openstackBootstrapProvider{runner: p.runner}
		fluxPlanEnv := envPlanFromMap(buildBootstrapEnvironment(kubeconfigPath), nil)
		fluxStep, err := openstackProvider.buildFluxBootstrapStep(cfg, gitOpsDir, fluxPlanEnv, &localOpts)
		if err != nil {
			return nil, fmt.Errorf("building flux bootstrap step: %w", err)
		}
		steps = append(steps, fluxStep)
		steps = append(steps, newSopsAgeSecretStep(sopsKeyPath, kubeconfigPath, p.runner))
		steps = append(steps, newGrafanaAdminSecretStep(cfg, kubeconfigPath, p.runner))
	}

	return steps, nil
}

func magnumIdentityStatePath(cfg *v2.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is nil")
	}
	cluster := sanitizeRuntimeSegment(cfg.ClusterName())
	if cluster == "" {
		return "", fmt.Errorf("cluster name must be set for Magnum identity state")
	}
	organization := sanitizeRuntimeSegment(cfg.Organization())
	if organization == "" {
		organization = "opencenter"
	}
	return resolveBootstrapPath(filepath.Join(config.GetStateDir(), "magnum", organization, cluster, "identity.json"))
}

func newMagnumIdentityState(clusterName, attemptState string) magnumIdentityState {
	return magnumIdentityState{
		Version:      magnumIdentityStateVersion,
		ClusterName:  clusterName,
		AttemptState: attemptState,
		AttemptAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func loadMagnumIdentityState(path string) (magnumIdentityState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return magnumIdentityState{}, nil
		}
		return magnumIdentityState{}, fmt.Errorf("read Magnum identity state: %w", err)
	}
	var state magnumIdentityState
	if err := json.Unmarshal(data, &state); err != nil {
		return magnumIdentityState{}, fmt.Errorf("parse Magnum identity state: %w", err)
	}
	if state.Version != magnumIdentityStateVersion {
		return magnumIdentityState{}, fmt.Errorf("unsupported Magnum identity state version %d", state.Version)
	}
	if strings.TrimSpace(state.AttemptAt) == "" {
		return magnumIdentityState{}, fmt.Errorf("magnum identity state is missing create attempt time")
	}
	if _, err := time.Parse(time.RFC3339Nano, state.AttemptAt); err != nil {
		return magnumIdentityState{}, fmt.Errorf("magnum identity state has invalid create attempt time: %w", err)
	}
	switch state.AttemptState {
	case magnumCreateInFlight:
		if state.ClusterID != "" || state.Observed || strings.TrimSpace(state.ClusterName) == "" {
			return magnumIdentityState{}, fmt.Errorf("magnum identity state has invalid in-flight create attempt")
		}
	case magnumCreateAccepted:
		if strings.TrimSpace(state.ClusterName) == "" {
			return magnumIdentityState{}, fmt.Errorf("magnum identity state is missing cluster name")
		}
		if !validMagnumClusterID(state.ClusterID) {
			return magnumIdentityState{}, fmt.Errorf("magnum identity state contains an invalid cluster UUID")
		}
	default:
		return magnumIdentityState{}, fmt.Errorf("magnum identity state has unsupported attempt state %q", state.AttemptState)
	}
	return state, nil
}

func invalidateMagnumBootstrapState(cfg *v2.Config) error {
	runtimePaths, err := resolveBootstrapRuntimePaths(cfg, "", time.Now())
	if err != nil {
		return fmt.Errorf("resolve Magnum bootstrap state: %w", err)
	}
	for _, path := range []string{runtimePaths.StatePath, runtimePaths.LegacyStatePath} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("invalidate Magnum bootstrap state %s: %w", path, err)
		}
	}
	return nil
}

func persistMagnumIdentityState(path string, state magnumIdentityState) error {
	if state.Version == 0 {
		state.Version = magnumIdentityStateVersion
	}
	if state.AttemptAt == "" {
		state.AttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if _, err := validateMagnumIdentityState(state); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Magnum identity directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure Magnum identity directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".identity-*.tmp")
	if err != nil {
		return fmt.Errorf("create Magnum identity state: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure Magnum identity state: %w", err)
	}
	data, err := json.Marshal(state)
	if err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write Magnum identity state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit Magnum identity state: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure Magnum identity state: %w", err)
	}
	return nil
}

func validateMagnumIdentityState(state magnumIdentityState) (magnumIdentityState, error) {
	if state.Version != magnumIdentityStateVersion {
		return magnumIdentityState{}, fmt.Errorf("unsupported Magnum identity state version %d", state.Version)
	}
	if strings.TrimSpace(state.AttemptAt) == "" {
		return magnumIdentityState{}, fmt.Errorf("magnum identity state is missing create attempt time")
	}
	switch state.AttemptState {
	case magnumCreateInFlight:
		if strings.TrimSpace(state.ClusterName) == "" || state.ClusterID != "" || state.Observed {
			return magnumIdentityState{}, fmt.Errorf("magnum identity state has invalid in-flight create attempt")
		}
	case magnumCreateAccepted:
		if strings.TrimSpace(state.ClusterName) == "" {
			return magnumIdentityState{}, fmt.Errorf("magnum identity state is missing cluster name")
		}
		if !validMagnumClusterID(state.ClusterID) {
			return magnumIdentityState{}, fmt.Errorf("magnum identity state contains an invalid cluster UUID")
		}
	default:
		return magnumIdentityState{}, fmt.Errorf("magnum identity state has unsupported attempt state %q", state.AttemptState)
	}
	return state, nil
}

func recoverMagnumCreate(ctx context.Context, provider *magnumprovider.Provider, path string, state *magnumIdentityState, clusterName string, timeout, pollInterval time.Duration) error {
	if state == nil {
		return fmt.Errorf("magnum create recovery requires identity state")
	}
	if state.ClusterName != "" && state.ClusterName != clusterName {
		return fmt.Errorf("magnum create recovery requires manual review: state cluster name %q does not match %q", state.ClusterName, clusterName)
	}
	if timeout <= 0 {
		timeout = defaultReadyTimeout
	}
	if pollInterval <= 0 {
		pollInterval = magnumReadyPollInterval
	}
	recoveryCtx, cancel := context.WithTimeout(nonNilClusterContext(ctx), timeout)
	defer cancel()
	return recoverMagnumCreateVisible(recoveryCtx, provider, path, state, clusterName, pollInterval, timeout)
}

func recoverMagnumCreateVisible(ctx context.Context, provider *magnumprovider.Provider, path string, state *magnumIdentityState, clusterName string, pollInterval, timeout time.Duration) error {
	identifier := clusterName
	if state.ClusterID != "" {
		identifier = state.ClusterID
	}
	visible, err := provider.WaitVisible(ctx, identifier, pollInterval)
	if err != nil {
		return fmt.Errorf("magnum create recovery required: cluster %q was not observed within %s: %w", clusterName, timeout, err)
	}
	if !validMagnumClusterID(visible.ID) {
		return fmt.Errorf("magnum create recovery required: observed cluster %q returned an invalid UUID", clusterName)
	}
	state.ClusterID = visible.ID
	state.ClusterName = clusterName
	state.AttemptState = magnumCreateAccepted
	state.Observed = true
	if err := persistMagnumIdentityState(path, *state); err != nil {
		return fmt.Errorf("persist recovered Magnum cluster identity: %w", err)
	}
	return nil
}

func removeMagnumClusterID(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Magnum identity state: %w", err)
	}
	return nil
}

func validMagnumClusterID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func isMagnumNotFound(err error) bool {
	var statusErr gophercloud.StatusCodeError
	return errors.As(err, &statusErr) && statusErr.GetStatusCode() == http.StatusNotFound
}

func nonNilClusterContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// magnumConfigFromV2 intentionally reads only the selected Magnum cloud
// configuration. In particular, it does not fall back to the OpenStack cloud
// block or to global identity settings.
func magnumConfigFromV2(cfg *v2.MagnumCloudConfig) magnumprovider.Config {
	if cfg == nil {
		return magnumprovider.Config{}
	}
	var labels map[string]string
	if cfg.Labels != nil {
		labels = make(map[string]string, len(cfg.Labels))
		for key, value := range cfg.Labels {
			labels[key] = value
		}
	}
	return magnumprovider.Config{
		IdentityEndpoint:            cfg.AuthURL,
		CA:                          cfg.CA,
		Region:                      cfg.Region,
		TenantID:                    cfg.ProjectID,
		ApplicationCredentialID:     cfg.ApplicationCredentialID,
		ApplicationCredentialSecret: cfg.ApplicationCredentialSecret,
		Insecure:                    cfg.Insecure,
		ClusterTemplate:             cfg.ClusterTemplate,
		Labels:                      labels,
		Keypair:                     cfg.Keypair,
		MasterFlavorID:              cfg.MasterFlavorID,
		NodeFlavorID:                cfg.NodeFlavorID,
		CreateTimeout:               cfg.CreateTimeout,
		MasterLBEnabled:             cfg.MasterLBEnabled,
	}
}
