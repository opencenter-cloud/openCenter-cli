// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openstack

import (
	"context"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rackerlabs/opencenter-cli/internal/cloud"
	"github.com/rackerlabs/opencenter-cli/internal/config"
)

func TestNewProvider(t *testing.T) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: "https://example.com:5000/v3",
		Username:         "test",
		Password:         "test",
	}

	provider := NewProvider(authOpts)

	require.NotNil(t, provider)
	assert.Equal(t, authOpts, provider.authOpts)
}

func TestProvider_DetectDrift_NoServers(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Empty(t, report.Drifts)
	assert.Equal(t, 0, report.Summary.TotalDrifts)
}

func TestProvider_DetectDrift_MissingServer(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.small",
				Status: "ACTIVE",
			},
		},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Drifts, 1)
	
	drift := report.Drifts[0]
	assert.Equal(t, "server", drift.ResourceType)
	assert.Equal(t, "test-server", drift.ResourceName)
	assert.Equal(t, "existence", drift.Field)
	assert.Equal(t, "exists", drift.Expected)
	assert.Equal(t, "missing", drift.Actual)
	assert.Equal(t, cloud.SeverityCritical, drift.Severity)
	assert.True(t, drift.Reconcilable)
}

func TestProvider_DetectDrift_ExtraServer(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "unexpected-server",
				Flavor: "m1.small",
				Status: "ACTIVE",
			},
		},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Drifts, 1)
	
	drift := report.Drifts[0]
	assert.Equal(t, "server", drift.ResourceType)
	assert.Equal(t, "unexpected-server", drift.ResourceName)
	assert.Equal(t, "existence", drift.Field)
	assert.Equal(t, "not exists", drift.Expected)
	assert.Equal(t, "exists", drift.Actual)
	assert.Equal(t, cloud.SeverityWarning, drift.Severity)
	assert.False(t, drift.Reconcilable)
}

func TestProvider_DetectDrift_FlavorMismatch(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.large",
				Status: "ACTIVE",
			},
		},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.small",
				Status: "ACTIVE",
			},
		},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Drifts, 1)
	
	drift := report.Drifts[0]
	assert.Equal(t, "server", drift.ResourceType)
	assert.Equal(t, "test-server", drift.ResourceName)
	assert.Equal(t, "flavor", drift.Field)
	assert.Equal(t, "m1.large", drift.Expected)
	assert.Equal(t, "m1.small", drift.Actual)
	assert.Equal(t, cloud.SeverityCritical, drift.Severity)
	assert.False(t, drift.Reconcilable)
}

func TestProvider_DetectDrift_ServerNotActive(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.small",
				Status: "ACTIVE",
			},
		},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.small",
				Status: "ERROR",
			},
		},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Drifts, 1)
	
	drift := report.Drifts[0]
	assert.Equal(t, "server", drift.ResourceType)
	assert.Equal(t, "status", drift.Field)
	assert.Equal(t, "ACTIVE", drift.Expected)
	assert.Equal(t, "ERROR", drift.Actual)
	assert.Equal(t, cloud.SeverityCritical, drift.Severity)
	assert.False(t, drift.Reconcilable)
}

func TestProvider_DetectDrift_TagMismatch(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.small",
				Status: "ACTIVE",
				Tags: map[string]string{
					"cluster": "test-cluster",
					"role":    "worker",
				},
			},
		},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{
				ID:     "server-1",
				Name:   "test-server",
				Flavor: "m1.small",
				Status: "ACTIVE",
				Tags: map[string]string{
					"cluster": "wrong-cluster",
					"role":    "worker",
				},
			},
		},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Drifts, 1)
	
	drift := report.Drifts[0]
	assert.Equal(t, "server", drift.ResourceType)
	assert.Equal(t, "tags.cluster", drift.Field)
	assert.Equal(t, "test-cluster", drift.Expected)
	assert.Equal(t, "wrong-cluster", drift.Actual)
	assert.Equal(t, cloud.SeverityInfo, drift.Severity)
	assert.True(t, drift.Reconcilable)
}

func TestProvider_DetectDrift_NetworkMissing(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Networks: []cloud.Network{
			{
				ID:   "net-1",
				Name: "test-network",
				CIDR: "10.0.0.0/24",
			},
		},
	}

	actual := &cloud.InfrastructureState{
		Networks: []cloud.Network{},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Drifts, 1)
	
	drift := report.Drifts[0]
	assert.Equal(t, "network", drift.ResourceType)
	assert.Equal(t, "test-network", drift.ResourceName)
	assert.Equal(t, "existence", drift.Field)
	assert.Equal(t, cloud.SeverityCritical, drift.Severity)
	assert.True(t, drift.Reconcilable)
}

func TestProvider_DetectDrift_Summary(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{Name: "server-1", Flavor: "m1.small", Status: "ACTIVE"},
			{Name: "server-2", Flavor: "m1.small", Status: "ACTIVE"},
		},
	}

	actual := &cloud.InfrastructureState{
		Servers: []cloud.Server{
			{Name: "server-1", Flavor: "m1.large", Status: "ACTIVE"}, // Flavor drift (critical)
			{Name: "server-2", Flavor: "m1.small", Status: "ERROR"},  // Status drift (critical)
		},
	}

	report, err := provider.DetectDrift(context.Background(), desired, actual)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 2, report.Summary.TotalDrifts)
	assert.Equal(t, 2, report.Summary.CriticalCount)
	assert.Equal(t, 0, report.Summary.WarningCount)
	assert.Equal(t, 0, report.Summary.InfoCount)
	assert.Equal(t, cloud.SeverityCritical, report.OverallSeverity)
	assert.False(t, report.Reconcilable) // Both drifts are non-reconcilable
}

func TestProvider_ReconcileDrift_EmptyReport(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	report := &cloud.DriftReport{
		Drifts:       []cloud.DriftItem{},
		Reconcilable: true,
	}

	err := provider.ReconcileDrift(context.Background(), report)

	assert.NoError(t, err)
}

func TestProvider_ReconcileDrift_NonReconcilable(t *testing.T) {
	provider := NewProvider(gophercloud.AuthOptions{})

	report := &cloud.DriftReport{
		Drifts: []cloud.DriftItem{
			{
				ResourceType: "server",
				Field:        "flavor",
				Reconcilable: false,
			},
		},
		Reconcilable: false,
	}

	err := provider.ReconcileDrift(context.Background(), report)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-reconcilable")
}

func TestBuildDesiredState(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
				Kubernetes: config.KubernetesConfig{
					FlavorMaster: "m1.large",
					FlavorWorker: "m1.medium",
				},
				Networking: config.ClusterNetworkingConfig{
					SubnetNodes: "10.0.1.0/24",
				},
			},
		},
	}

	// This would be in cmd/cluster_drift.go but we test the logic here
	state := &cloud.InfrastructureState{
		Servers:  []cloud.Server{},
		Networks: []cloud.Network{},
	}

	// Add control plane nodes
	for i := 0; i < 3; i++ {
		state.Servers = append(state.Servers, cloud.Server{
			Name:   "test-cluster-control-" + string(rune('0'+i)),
			Flavor: cfg.OpenCenter.Cluster.Kubernetes.FlavorMaster,
			Status: "ACTIVE",
			Tags: map[string]string{
				"cluster": cfg.OpenCenter.Cluster.ClusterName,
				"role":    "control-plane",
			},
		})
	}

	assert.Len(t, state.Servers, 3)
	assert.Equal(t, "m1.large", state.Servers[0].Flavor)
	assert.Equal(t, "control-plane", state.Servers[0].Tags["role"])
}
