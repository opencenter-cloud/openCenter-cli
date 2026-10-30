// Copyright 2026 OpenCenter contributors
// Licensed under the Apache License, Version 2.0 (the "License");

package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// ---------------------------------------------------------------------------
// Step wiring: the cleanup-openstack-networking step must reach the service
// with the resolved cluster name and network ID, and surface its errors.
// ---------------------------------------------------------------------------

func findDestroyStep(t *testing.T, steps []destroyStep, id string) destroyStep {
	t.Helper()
	for _, s := range steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("step %q not found", id)
	return destroyStep{}
}

func TestOpenStackDestroyProvider_NetworkingCleanupStep_Wiring(t *testing.T) {
	tests := []struct {
		name          string
		topNetworkID  string
		nestedNetwork string // networking.network_id fallback
		wantNetworkID string
		wantSkipped   bool
	}{
		{
			name:          "top-level network id",
			topNetworkID:  "net-top",
			wantNetworkID: "net-top",
		},
		{
			name:          "falls back to networking section",
			nestedNetwork: "net-nested",
			wantNetworkID: "net-nested",
		},
		{
			name:          "empty network id skips the sweep as a warning",
			wantNetworkID: "",
			wantSkipped:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := makeOpenStackCfg(t)
			cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID = tc.topNetworkID
			if tc.nestedNetwork != "" {
				cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking = &v2.OpenStackNetworkingConfig{
					NetworkID: tc.nestedNetwork,
				}
			}

			var out bytes.Buffer
			fake := &fakeNetworkCleanupForDestroy{}
			provider := newTestOpenStackDestroyProvider(&fakeDestroyCommandRunner{}, &out)
			result := &DestroyResult{}
			provider.result = result
			provider.networkCleanupFactory = func(_ *credentials.OpenStackCredentials) (openStackNetworkCleanupService, error) {
				return fake, nil
			}

			steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
			if err != nil {
				t.Fatalf("BuildSteps: %v", err)
			}
			step := findDestroyStep(t, steps, "cleanup-openstack-networking")
			if err := step.Run(context.Background()); err != nil {
				t.Fatalf("cleanup step: %v", err)
			}

			if tc.wantSkipped {
				// Un-scoped sweep: no service call, non-fatal, warning recorded.
				if fake.calls != 0 {
					t.Fatalf("CleanupClusterLoadBalancers must not be called without a network scope, calls=%d", fake.calls)
				}
				if step.Fatal {
					t.Fatal("un-scoped sweep must not be Fatal")
				}
				if len(result.CleanupWarnings) != 1 || !strings.Contains(result.CleanupWarnings[0].Error(), "network scope") {
					t.Fatalf("expected network-scope cleanup warning, got %v", result.CleanupWarnings)
				}
				return
			}
			if fake.calls != 1 {
				t.Fatalf("CleanupClusterLoadBalancers calls = %d, want 1", fake.calls)
			}
			if fake.clusterName != cfg.ClusterName() {
				t.Errorf("clusterName = %q, want %q", fake.clusterName, cfg.ClusterName())
			}
			if fake.networkID != tc.wantNetworkID {
				t.Errorf("networkID = %q, want %q", fake.networkID, tc.wantNetworkID)
			}
		})
	}
}

func TestOpenStackDestroyProvider_NetworkingCleanupStep_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("boom")

	t.Run("factory error", func(t *testing.T) {
		cfg := makeOpenStackCfg(t)
		// The sweep only reaches the service when network-scoped.
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID = "net-scope"
		provider := newTestOpenStackDestroyProvider(&fakeDestroyCommandRunner{}, &bytes.Buffer{})
		provider.networkCleanupFactory = func(_ *credentials.OpenStackCredentials) (openStackNetworkCleanupService, error) {
			return nil, sentinel
		}
		steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
		if err != nil {
			t.Fatalf("BuildSteps: %v", err)
		}
		step := findDestroyStep(t, steps, "cleanup-openstack-networking")
		err = step.Run(context.Background())
		if !errors.Is(err, sentinel) {
			t.Fatalf("Run error = %v, want wrapped %v", err, sentinel)
		}
		if !strings.Contains(err.Error(), "create OpenStack networking cleanup client") {
			t.Errorf("error message = %q, want context about the cleanup client", err.Error())
		}
	})

	t.Run("cleanup error", func(t *testing.T) {
		cfg := makeOpenStackCfg(t)
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID = "net-scope"
		provider := newTestOpenStackDestroyProvider(&fakeDestroyCommandRunner{}, &bytes.Buffer{})
		provider.networkCleanupFactory = func(_ *credentials.OpenStackCredentials) (openStackNetworkCleanupService, error) {
			return &fakeNetworkCleanupForDestroy{err: sentinel}, nil
		}
		steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
		if err != nil {
			t.Fatalf("BuildSteps: %v", err)
		}
		step := findDestroyStep(t, steps, "cleanup-openstack-networking")
		err = step.Run(context.Background())
		if !errors.Is(err, sentinel) {
			t.Fatalf("Run error = %v, want wrapped %v", err, sentinel)
		}
		if !strings.Contains(err.Error(), "clean up cluster networking") {
			t.Errorf("error message = %q, want context about cluster networking", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// Ownership predicate
// ---------------------------------------------------------------------------

func TestOwnsLoadBalancer(t *testing.T) {
	const cluster, network = "prod", "net-1"
	tests := []struct {
		name string
		lb   loadbalancers.LoadBalancer
		want bool
	}{
		{
			name: "CCM-style name on the cluster network",
			lb:   loadbalancers.LoadBalancer{Name: "k8s-prod-default-nginx-abc123", VipNetworkID: network},
			want: true,
		},
		{
			name: "substring-only match for another cluster is rejected",
			lb:   loadbalancers.LoadBalancer{Name: "k8s-staging-prod-default-web-fff", VipNetworkID: network},
			want: false,
		},
		{
			name: "bare substring in an unrelated name is rejected",
			lb:   loadbalancers.LoadBalancer{Name: "my-prod-lb", VipNetworkID: network},
			want: false,
		},
		{
			name: "load balancer on another network is rejected",
			lb:   loadbalancers.LoadBalancer{Name: "k8s-prod-default-nginx-abc123", VipNetworkID: "net-other"},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownsLoadBalancer(tc.lb, cluster, network); got != tc.want {
				t.Errorf("ownsLoadBalancer(%q, network=%q) = %v, want %v", tc.lb.Name, tc.lb.VipNetworkID, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Wait loop: 404 means gone. Regression guard for the gophercloud v1.14.1
// behavior that Result.StatusCode is never populated on error responses; the
// only reliable 404 signal is the deferred ErrDefault404.
// ---------------------------------------------------------------------------

func newCleanupServiceForEndpoint(url string) *gophercloudOpenStackNetworkCleanupService {
	pc := &gophercloud.ProviderClient{}
	return &gophercloudOpenStackNetworkCleanupService{
		loadBalancerClient: &gophercloud.ServiceClient{ProviderClient: pc, Endpoint: url + "/"},
		deletionTimeout:    2 * time.Second,
		pollInterval:       50 * time.Millisecond,
	}
}

func TestWaitForLoadBalancerDeletion_GoneAfterPending(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		if hits == 1 {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"loadbalancer":{"id":"lb1","provisioning_status":"PENDING_DELETE"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"itemNotFound":{"message":"Loadbalancer not found"}}`)
	}))
	defer srv.Close()

	svc := newCleanupServiceForEndpoint(srv.URL)
	if err := svc.waitForLoadBalancerDeletion(context.Background(), "lb1"); err != nil {
		t.Fatalf("wait returned %v, want nil once the LB 404s", err)
	}
	if hits < 2 {
		t.Errorf("expected at least 2 GETs (pending then 404), got %d", hits)
	}
}

func TestWaitForLoadBalancerDeletion_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"loadbalancer":{"id":"lb1","provisioning_status":"PENDING_DELETE"}}`)
	}))
	defer srv.Close()

	svc := newCleanupServiceForEndpoint(srv.URL)
	svc.deletionTimeout = 300 * time.Millisecond
	err := svc.waitForLoadBalancerDeletion(context.Background(), "lb1")
	if err == nil {
		t.Fatal("expected timeout error while the LB never disappears")
	}
	if !strings.Contains(err.Error(), "timed out after 300ms") {
		t.Errorf("error = %v, want it to mention the configured timeout", err)
	}
}

func TestWaitForLoadBalancerDeletion_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"faultCode":"internal","message":"octavia is sad"}`)
	}))
	defer srv.Close()

	svc := newCleanupServiceForEndpoint(srv.URL)
	err := svc.waitForLoadBalancerDeletion(context.Background(), "lb1")
	var internal gophercloud.ErrDefault500
	if err == nil || !errors.As(err, &internal) {
		t.Fatalf("error = %v, want ErrDefault500 surfaced immediately", err)
	}
}

func TestWaitForLoadBalancerDeletion_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"loadbalancer":{"id":"lb1","provisioning_status":"PENDING_DELETE"}}`)
	}))
	defer srv.Close()

	svc := newCleanupServiceForEndpoint(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	if err := svc.waitForLoadBalancerDeletion(ctx, "lb1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// ---------------------------------------------------------------------------
// Scope guards: the service must refuse to run without a network scope and
// must not issue any HTTP request when it does.
// ---------------------------------------------------------------------------

func TestCleanupClusterLoadBalancers_RefusesWithoutNetworkScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no HTTP request expected when networkID is empty, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	svc := newCleanupServiceForEndpoint(srv.URL)
	for _, networkID := range []string{"", "   "} {
		err := svc.CleanupClusterLoadBalancers(context.Background(), "prod", networkID)
		if err == nil || !strings.Contains(err.Error(), "refusing to clean up load balancers without a network scope") {
			t.Errorf("networkID=%q: error = %v, want refusal", networkID, err)
		}
	}

	// Empty cluster name is rejected too, before any HTTP call.
	if err := svc.CleanupClusterLoadBalancers(context.Background(), "  ", "net-1"); err == nil {
		t.Error("expected error for empty cluster name")
	}
}

// TestNewGophercloudOpenStackNetworkCleanupService_NilCredentials guards the
// destroy path for configurations without OpenStack credentials: the factory
// must return an error, not panic dereferencing creds.
func TestNewGophercloudOpenStackNetworkCleanupService_NilCredentials(t *testing.T) {
	if _, err := newGophercloudOpenStackNetworkCleanupService(nil); err == nil {
		t.Fatal("expected error for nil credentials")
	}
}
