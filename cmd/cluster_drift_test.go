package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opencenter-cloud/opencenter-cli/internal/cloud"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

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
