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

package vmware

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"

	"github.com/opencenter-cloud/opencenter-cli/internal/cloud"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
)

func TestProvider_GetCurrentState(t *testing.T) {
	testenv.RequireLoopbackBind(t)

	ctx := context.Background()
	model := simulator.VPX()
	model.Machine = 2
	defer model.Remove()

	require.NoError(t, model.Create())

	server := model.Service.NewServer()
	defer server.Close()

	client, err := govmomi.NewClient(ctx, server.URL, true)
	require.NoError(t, err)
	defer client.Logout(ctx)

	finder := find.NewFinder(client.Client, false)
	datacenter, err := finder.DefaultDatacenter(ctx)
	require.NoError(t, err)
	finder.SetDatacenter(datacenter)

	datastore, err := finder.DefaultDatastore(ctx)
	require.NoError(t, err)

	virtualMachines, err := finder.VirtualMachineList(ctx, "*")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(virtualMachines), 2)
	vm1Name, err := virtualMachines[0].ObjectName(ctx)
	require.NoError(t, err)
	vm2Name, err := virtualMachines[1].ObjectName(ctx)
	require.NoError(t, err)

	virtualMachineState, err := listVirtualMachines(ctx, client, datacenter)
	require.NoError(t, err)
	require.NotEmpty(t, virtualMachineState)

	datacenterName, err := datacenter.ObjectName(ctx)
	require.NoError(t, err)
	datastoreName, err := datastore.ObjectName(ctx)
	require.NoError(t, err)
	networkNames, err := resolveReferenceNames(ctx, client, virtualMachineState[0].Network)
	require.NoError(t, err)
	require.NotEmpty(t, networkNames)
	networkName := networkNames[0]
	password, _ := server.URL.User.Password()

	cfgPtr, err := v2.NewV2Default("prod-cluster", "vmware")
	require.NoError(t, err)
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Provider = "vmware"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.VCenterServer = server.URL.Hostname()
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Datacenter = datacenterName
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Datastore = datastoreName
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Network = networkName
	cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []v2.StaticNode{
		{Name: vm1Name, AccessIPv4: "10.0.0.1"},
	}
	cfg.OpenCenter.Infrastructure.Compute.WorkerNodes = []v2.StaticNode{
		{Name: vm2Name, AccessIPv4: "10.0.0.2"},
	}
	if cfg.Secrets.ServiceSecrets == nil {
		cfg.Secrets.ServiceSecrets = make(map[string]any)
	}
	cfg.Secrets.ServiceSecrets["vsphere_csi"] = map[string]any{
		"vcenter_host":  server.URL.Hostname(),
		"username":      server.URL.User.Username(),
		"password":      password,
		"port":          server.URL.Port(),
		"insecure_flag": "true",
	}

	provider := NewProvider()
	provider.newClient = func(context.Context, *url.URL, bool) (*govmomi.Client, error) {
		return client, nil
	}
	provider.logoutClient = false

	state, err := provider.GetCurrentState(ctx, cfg)
	require.NoError(t, err)

	require.Len(t, state.Servers, 2)
	serverByName := make(map[string]cloud.Server, len(state.Servers))
	for _, server := range state.Servers {
		serverByName[server.Name] = server
	}

	assert.Equal(t, "ACTIVE", serverByName[vm1Name].Status)
	assert.Contains(t, serverByName[vm1Name].Networks, networkName)
	assert.Equal(t, "control-plane", serverByName[vm1Name].Tags["role"])
	assert.Equal(t, "worker", serverByName[vm2Name].Tags["role"])

	require.Len(t, state.Networks, 1)
	assert.Equal(t, networkName, state.Networks[0].Name)

	require.Len(t, state.Volumes, 2)
	assert.ElementsMatch(t, []string{vm1Name + "@" + datastoreName, vm2Name + "@" + datastoreName}, []string{
		state.Volumes[0].Name,
		state.Volumes[1].Name,
	})
	assert.Equal(t, "in-use", state.Volumes[0].Status)
}

func TestProvider_DetectDriftUsesSharedComparer(t *testing.T) {
	provider := NewProvider()

	desired := &cloud.InfrastructureState{
		Servers: []cloud.Server{{
			Name:   "prod-master-1",
			Status: "ACTIVE",
			Tags: map[string]string{
				"role": "control-plane",
			},
		}},
	}
	actual := &cloud.InfrastructureState{}

	report, err := provider.DetectDrift(context.Background(), desired, actual)
	require.NoError(t, err)
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, cloud.SeverityCritical, report.Drifts[0].Severity)
}

