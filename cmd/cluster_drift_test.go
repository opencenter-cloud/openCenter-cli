package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opencenter-cloud/opencenter-cli/internal/cloud"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestClusterDriftDetectUsesGlobalOutputFlag(t *testing.T) {
	cmd := newClusterDriftDetectCmd()

	if cmd.Flags().Lookup("output") != nil {
		t.Fatal("cluster drift detect must use global --output instead of local --output")
	}
}

type driftCallbackDoerFunc func(*http.Request) (*http.Response, error)

func (fn driftCallbackDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestBuildDesiredStateOpenStackCoversManagedResources(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("prod-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 40
	cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize = 40
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID = "image-123"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking = &v2.OpenStackNetworkingConfig{
		K8sAPIPortACL:  []string{"10.0.0.0/8"},
		FloatingIPPool: "public",
	}
	cfg.OpenCenter.Infrastructure.Networking.LoadbalancerProvider = "octavia"

	state := buildDesiredState(cfg)

	if len(state.Servers) != 5 {
		t.Fatalf("expected 5 servers, got %d", len(state.Servers))
	}
	if len(state.Volumes) != 5 {
		t.Fatalf("expected 5 boot volumes, got %d", len(state.Volumes))
	}
	if len(state.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(state.Networks))
	}
	if len(state.SecurityGroups) != 2 {
		t.Fatalf("expected 2 security groups, got %d", len(state.SecurityGroups))
	}
	if len(state.LoadBalancers) != 1 {
		t.Fatalf("expected 1 load balancer, got %d", len(state.LoadBalancers))
	}
	if len(state.FloatingIPs) != 0 {
		t.Fatalf("expected Octavia-managed config to skip floating IP desired state, got %d entries", len(state.FloatingIPs))
	}

	if state.Servers[0].Name != "prod-cluster-master-1" {
		t.Fatalf("unexpected first control plane name: %s", state.Servers[0].Name)
	}
	if state.Servers[0].Image != "image-123" {
		t.Fatalf("unexpected desired image: %s", state.Servers[0].Image)
	}
	if state.SecurityGroups[0].Name != "prod-cluster-control-plane-sg" {
		t.Fatalf("unexpected control plane security group name: %s", state.SecurityGroups[0].Name)
	}
	if len(state.SecurityGroups[0].Rules) != 1 || state.SecurityGroups[0].Rules[0].RemoteIP != "10.0.0.0/8" {
		t.Fatalf("unexpected API ACL rules: %#v", state.SecurityGroups[0].Rules)
	}
}

func TestCreateCloudProviderFactoryRegistersCloudDriftProvidersOnly(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("prod-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "sjc3"
	cfg.OpenCenter.Infrastructure.Cloud.VMware = &v2.VMwareCloudConfig{VCenterServer: "vc.example.com"}
	cfg.Secrets.VSphereCsi.Username = "administrator@vsphere.local"
	cfg.Secrets.VSphereCsi.Password = "super-secret"

	factory := createCloudProviderFactory(cfg)

	for _, providerName := range []string{"openstack", "vmware"} {
		if _, err := factory.GetProvider(providerName); err != nil {
			t.Fatalf("expected provider %s to be registered: %v", providerName, err)
		}
	}

	for _, providerName := range []string{"aws", "baremetal", "talos", "kind"} {
		if _, err := factory.GetProvider(providerName); err == nil {
			t.Fatalf("expected provider %s to be unavailable for drift detection", providerName)
		}
	}
}

func TestBuildDesiredStateVMwareUsesConfiguredNodes(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("prod-cluster", "vmware")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Provider = "vmware"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Network = "dvpg-prod"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Datastore = "vsanDatastore"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 1

	state := buildDesiredState(cfg)

	require.Len(t, state.Servers, 2)
	require.Len(t, state.Networks, 1)
	require.Len(t, state.Volumes, 2)
	require.Empty(t, state.SecurityGroups)
	require.Empty(t, state.LoadBalancers)
	require.Empty(t, state.FloatingIPs)

	if state.Servers[0].Name != "prod-cluster-master-1" {
		t.Fatalf("unexpected first vmware node name: %s", state.Servers[0].Name)
	}
	if state.Servers[0].Tags["role"] != "control-plane" {
		t.Fatalf("unexpected first vmware node role: %s", state.Servers[0].Tags["role"])
	}
	if state.Servers[0].Networks[0] != "dvpg-prod" {
		t.Fatalf("unexpected vmware network: %v", state.Servers[0].Networks)
	}
	if state.Networks[0].Name != "dvpg-prod" {
		t.Fatalf("unexpected vmware desired network name: %s", state.Networks[0].Name)
	}
	if state.Volumes[0].Name != "prod-cluster-master-1@vsanDatastore" {
		t.Fatalf("unexpected vmware desired datastore volume: %s", state.Volumes[0].Name)
	}
}

func TestSendDriftCallbackPostsJSON(t *testing.T) {
	t.Helper()

	report := &cloud.DriftReport{
		ClusterName: "prod-cluster",
		DetectedAt:  "2026-03-21T12:00:00Z",
		Drifts: []cloud.DriftItem{
			{
				ResourceType: "security_group",
				ResourceName: "prod-cluster-control-plane-sg",
				Field:        "rules",
			},
		},
	}

	var received cloud.DriftReport
	doer := driftCallbackDoerFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.String() != "https://callback.example.com/drift" {
			t.Fatalf("expected callback URL to be preserved, got %s", req.URL.String())
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json content type, got %s", got)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read callback body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("unmarshal callback body: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})

	if err := sendDriftCallbackWithDoer(context.Background(), "https://callback.example.com/drift", report, doer); err != nil {
		t.Fatalf("sendDriftCallback returned error: %v", err)
	}

	if received.ClusterName != report.ClusterName {
		t.Fatalf("expected callback cluster %s, got %s", report.ClusterName, received.ClusterName)
	}
	if len(received.Drifts) != 1 {
		t.Fatalf("expected one drift item, got %d", len(received.Drifts))
	}
}

func TestBuildDesiredState_MasterVolumeZero_SkipsMasterBootVolumes(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("vol-test", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize = 0
	cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 40

	state := buildDesiredState(cfg)

	require.Len(t, state.Servers, 5, "should have 3 masters + 2 workers")
	require.Len(t, state.Volumes, 2, "master volumes should be skipped when MasterVolumeSize=0")

	for _, vol := range state.Volumes {
		if !contains(vol.Name, "worker") {
			t.Errorf("expected only worker boot volumes, got %s", vol.Name)
		}
	}
}

func TestBuildDesiredState_WorkerVolumeZero_SkipsWorkerBootVolumes(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("vol-test", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize = 40
	cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 0

	state := buildDesiredState(cfg)

	require.Len(t, state.Servers, 5, "should have 3 masters + 2 workers")
	require.Len(t, state.Volumes, 3, "worker volumes should be skipped when WorkerVolumeSize=0")

	for _, vol := range state.Volumes {
		if !contains(vol.Name, "master") {
			t.Errorf("expected only master boot volumes, got %s", vol.Name)
		}
	}
}

func TestBuildDesiredState_BothVolumesZero_NoBootVolumes(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("vol-test", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize = 0
	cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 0

	state := buildDesiredState(cfg)

	require.Len(t, state.Servers, 5)
	require.Empty(t, state.Volumes, "no boot volumes when both sizes are 0")
}

// contains checks if substr is in s (simple helper to avoid importing strings in test).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildDesiredState_WindowsPoolsIncluded(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 0
	cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize = 40
	cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 0
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID = "img-123"
	cfg.OpenCenter.Infrastructure.Networking.LoadbalancerProvider = "octavia"
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows = []v2.WindowsWorkerPoolConfig{
		{Name: "win-pool", Count: 2, Flavor: "gp.0.8.32", BootVolume: v2.VolumeConfig{Size: 100}},
	}

	state := buildDesiredState(cfg)

	// Expect 1 master + 2 windows workers = 3 servers
	var winServers []cloud.Server
	for _, s := range state.Servers {
		if s.Tags["pool"] == "win-pool" {
			winServers = append(winServers, s)
		}
	}
	if len(winServers) != 2 {
		t.Fatalf("expected 2 Windows pool servers, got %d", len(winServers))
	}
	if winServers[0].Flavor != "gp.0.8.32" {
		t.Errorf("expected flavor gp.0.8.32, got %s", winServers[0].Flavor)
	}
	if winServers[0].Tags["os"] != "windows" {
		t.Error("expected os=windows tag")
	}

	// Check boot volumes for Windows pool
	var winVolumes []cloud.Volume
	for _, vol := range state.Volumes {
		if strings.Contains(vol.Name, "win-pool") {
			winVolumes = append(winVolumes, vol)
		}
	}
	if len(winVolumes) != 2 {
		t.Fatalf("expected 2 Windows pool boot volumes, got %d", len(winVolumes))
	}
	if winVolumes[0].Size != 100 {
		t.Errorf("expected boot volume size 100, got %d", winVolumes[0].Size)
	}
}

func TestBuildDesiredState_WindowsPoolCountZero_NoServers(t *testing.T) {
	cfgPtr, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 0
	cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize = 40
	cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 0
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID = "img-123"
	cfg.OpenCenter.Infrastructure.Networking.LoadbalancerProvider = "octavia"
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows = []v2.WindowsWorkerPoolConfig{
		{Name: "drained-pool", Count: 0, Flavor: "gp.0.4.16", BootVolume: v2.VolumeConfig{Size: 80}},
	}

	state := buildDesiredState(cfg)

	for _, s := range state.Servers {
		if s.Tags["pool"] == "drained-pool" {
			t.Fatal("expected no servers for pool with count=0")
		}
	}
}
