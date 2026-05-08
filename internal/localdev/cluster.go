package localdev

import (
	"context"
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

// ClusterContext contains the resolved config and filesystem paths for a cluster.
type ClusterContext struct {
	Identifier   string
	ClusterName  string
	Organization string
	Config       *v2.Config
	Paths        *paths.ClusterPaths
}

// ClusterResolver resolves cluster identifiers to canonical config and path state.
type ClusterResolver struct {
	configManager *config.ConfigurationManager
	pathResolver  *paths.PathResolver
}

// NewClusterResolver returns a resolver backed by the shared config directories.
func NewClusterResolver() (*ClusterResolver, error) {
	configManager, err := config.NewConfigurationManager()
	if err != nil {
		return nil, fmt.Errorf("create configuration manager: %w", err)
	}

	return &ClusterResolver{
		configManager: configManager,
		pathResolver:  config.NewPathResolverFromConfig(),
	}, nil
}

// Resolve loads a cluster config and resolves its organization-aware paths.
func (r *ClusterResolver) Resolve(ctx context.Context, identifier string) (*ClusterContext, error) {
	if strings.TrimSpace(identifier) == "" {
		return nil, fmt.Errorf("cluster name must be set")
	}

	cfg, err := r.configManager.Load(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("load cluster config %q: %w", identifier, err)
	}

	clusterName := cfg.ClusterName()
	organization := strings.TrimSpace(cfg.OpenCenter.Meta.Organization)

	var clusterPaths *paths.ClusterPaths
	if organization != "" {
		clusterPaths, err = r.pathResolver.Resolve(ctx, clusterName, organization)
	} else {
		clusterPaths, err = r.pathResolver.ResolveWithFallback(ctx, clusterName)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve cluster paths for %q: %w", identifier, err)
	}

	return &ClusterContext{
		Identifier:   identifier,
		ClusterName:  clusterName,
		Organization: organization,
		Config:       cfg,
		Paths:        clusterPaths,
	}, nil
}
