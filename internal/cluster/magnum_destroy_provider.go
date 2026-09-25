package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	magnumprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/magnum"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type magnumDestroyProvider struct {
	provider     *magnumprovider.Provider
	identityPath string
	timeout      time.Duration
}

const magnumDestroyTimeout = 30 * time.Minute

func newMagnumDestroyProvider(cfg *v2.Config) (lifecycleDestroyProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}
	magnumCfg := cfg.OpenCenter.Infrastructure.Cloud.Magnum
	if magnumCfg == nil {
		return nil, fmt.Errorf("opencenter.infrastructure.cloud.magnum must be configured for the magnum provider")
	}
	identityPath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		return nil, err
	}
	provider, err := magnumprovider.NewProvider(magnumConfigFromV2(magnumCfg))
	if err != nil {
		return nil, fmt.Errorf("create Magnum provider: %w", err)
	}
	return &magnumDestroyProvider{provider: provider, identityPath: identityPath, timeout: magnumDestroyTimeout}, nil
}

// BuildSteps returns the sole Magnum destroy operation. Magnum owns all of the
// cluster infrastructure, so no OpenTofu command or infrastructure directory
// is involved.
func (p *magnumDestroyProvider) BuildSteps(cfg *v2.Config, _ *DestroyInfraOptions) ([]destroyStep, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}
	clusterName := strings.TrimSpace(cfg.ClusterName())
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be set for the magnum provider")
	}
	if p == nil || p.provider == nil {
		return nil, fmt.Errorf("magnum provider is not configured")
	}
	identityPath := p.identityPath
	if identityPath == "" {
		var err error
		identityPath, err = magnumIdentityStatePath(cfg)
		if err != nil {
			return nil, err
		}
	}
	return []destroyStep{
		{
			ID:          "magnum-delete",
			Description: "Delete Magnum cluster and confirm removal",
			Run: func(ctx context.Context) error {
				return p.deleteCluster(ctx, clusterName, identityPath, cfg)
			},
		},
	}, nil
}

func (p *magnumDestroyProvider) deleteCluster(ctx context.Context, clusterName, identityPath string, cfg *v2.Config) error {
	ctx = nonNilClusterContext(ctx)
	timeout := p.timeout
	if timeout <= 0 {
		timeout = magnumDestroyTimeout
	}
	deleteCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	state, err := loadMagnumIdentityState(identityPath)
	if err != nil {
		return err
	}

	if state.ClusterName != "" && state.ClusterName != clusterName {
		return fmt.Errorf("magnum identity state belongs to cluster %q, not %q", state.ClusterName, clusterName)
	}
	clusterID := state.ClusterID
	if state.AttemptState == magnumCreateInFlight || (state.AttemptState == magnumCreateAccepted && !state.Observed) {
		identifier := clusterName
		if clusterID != "" {
			identifier = clusterID
		}
		visible, visibilityErr := p.provider.WaitVisible(deleteCtx, identifier, magnumReadyPollInterval)
		if visibilityErr != nil {
			return fmt.Errorf("magnum cluster visibility required before destroy; identity state retained: %w", visibilityErr)
		}
		if !validMagnumClusterID(visible.ID) {
			return fmt.Errorf("magnum cluster visibility returned an invalid UUID; identity state retained")
		}
		state.ClusterID = visible.ID
		state.ClusterName = clusterName
		state.AttemptState = magnumCreateAccepted
		state.Observed = true
		if err := persistMagnumIdentityState(identityPath, state); err != nil {
			return fmt.Errorf("persist observed Magnum cluster identity before destroy: %w", err)
		}
		clusterID = visible.ID
	}

	if clusterID == "" {
		existing, lookupErr := p.provider.GetCluster(deleteCtx, clusterName)
		if lookupErr != nil {
			if isMagnumNotFound(lookupErr) {
				return p.clearDestroyedMagnumState(identityPath, cfg)
			}
			return fmt.Errorf("resolve existing Magnum cluster %q: %w", clusterName, lookupErr)
		}
		if !validMagnumClusterID(existing.ID) {
			return fmt.Errorf("existing Magnum cluster %q returned an invalid UUID", clusterName)
		}
		clusterID = existing.ID
	}

	deleteErr := p.provider.DeleteCluster(deleteCtx, clusterID)
	if deleteErr != nil && !isMagnumNotFound(deleteErr) {
		return fmt.Errorf("delete Magnum cluster: %w", deleteErr)
	}
	if err := p.provider.WaitDeleted(deleteCtx, clusterID); err != nil {
		return fmt.Errorf("wait for Magnum cluster deletion: %w", err)
	}
	return p.clearDestroyedMagnumState(identityPath, cfg)
}

func (p *magnumDestroyProvider) clearDestroyedMagnumState(identityPath string, cfg *v2.Config) error {
	if err := removeMagnumClusterID(identityPath); err != nil {
		return err
	}
	if err := invalidateMagnumBootstrapState(cfg); err != nil {
		return err
	}
	return nil
}
