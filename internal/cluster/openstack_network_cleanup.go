// Copyright 2026 OpenCenter contributors
// Licensed under the Apache License, Version 2.0 (the "License");

package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"

	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// Default per-load-balancer wait budget and poll interval for deletion. Tests
// override them on the service struct to keep runs sub-second.
const (
	defaultLBDeletionTimeout = 2 * time.Minute
	defaultLBPollInterval    = 5 * time.Second
)

// openStackNetworkCleanupService removes only cluster-owned Octavia resources
// before OpenTofu attempts to remove the cluster network and router interface.
// CCM-created load balancers otherwise leave Neutron ports attached and can
// cause the OpenTofu destroy to hang on router-interface deletion.
type openStackNetworkCleanupService interface {
	CleanupClusterLoadBalancers(ctx context.Context, clusterName, networkID string) error
}

type gophercloudOpenStackNetworkCleanupService struct {
	loadBalancerClient *gophercloud.ServiceClient
	// deletionTimeout bounds how long a single load balancer may take to
	// disappear after the cascade delete is accepted.
	deletionTimeout time.Duration
	// pollInterval is how often deletion is re-checked while waiting.
	pollInterval time.Duration
}

func newGophercloudOpenStackNetworkCleanupService(creds *credentials.OpenStackCredentials) (openStackNetworkCleanupService, error) {
	provider, err := authenticateOpenStack(creds)
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewLoadBalancerV2(provider, gophercloud.EndpointOpts{Region: creds.Region})
	if err != nil {
		return nil, fmt.Errorf("create load balancer client: %w", err)
	}
	return &gophercloudOpenStackNetworkCleanupService{
		loadBalancerClient: client,
		deletionTimeout:    defaultLBDeletionTimeout,
		pollInterval:       defaultLBPollInterval,
	}, nil
}

func (s *gophercloudOpenStackNetworkCleanupService) CleanupClusterLoadBalancers(ctx context.Context, clusterName, networkID string) error {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	networkID = strings.TrimSpace(networkID)
	if networkID == "" {
		// Without the owning network the delete scope cannot be bounded to
		// this cluster: name matching alone would cascade-delete any
		// name-matching load balancer in the project. Refuse instead; the
		// operator must configure infrastructure.cloud.openstack.network_id.
		return fmt.Errorf("network ID is empty: refusing to clean up load balancers without a network scope")
	}

	// gophercloud v1.14.1 threads ProviderClient.Context into every request,
	// so a canceled destroy context aborts in-flight HTTP calls instead of
	// blocking until the 30s transport timeout expires.
	if s.loadBalancerClient != nil && s.loadBalancerClient.ProviderClient != nil {
		s.loadBalancerClient.ProviderClient.Context = ctx
	}

	page, err := loadbalancers.List(s.loadBalancerClient, loadbalancers.ListOpts{VipNetworkID: networkID}).AllPages()
	if err != nil {
		return fmt.Errorf("list Octavia load balancers: %w", err)
	}
	items, err := loadbalancers.ExtractLoadBalancers(page)
	if err != nil {
		return fmt.Errorf("extract Octavia load balancers: %w", err)
	}

	for _, lb := range items {
		// Keep both predicates: the exact network prevents touching another
		// cluster in the project, and the CCM name anchor prevents touching
		// unrelated LBs on this network. The client-side network check is
		// defensive in case a deployment's server-side filter is lax.
		if !ownsLoadBalancer(lb, clusterName, networkID) {
			continue
		}

		if err := loadbalancers.Delete(s.loadBalancerClient, lb.ID, loadbalancers.DeleteOpts{Cascade: true}).ExtractErr(); err != nil {
			return fmt.Errorf("delete Octavia load balancer %s (%s): %w", lb.ID, lb.Name, err)
		}
		if err := s.waitForLoadBalancerDeletion(ctx, lb.ID); err != nil {
			return fmt.Errorf("wait for Octavia load balancer %s deletion: %w", lb.ID, err)
		}
	}
	return nil
}

// ownsLoadBalancer reports whether lb belongs to clusterName on networkID.
// The CCM names its load balancers "k8s-<cluster>-<namespace>-<service>-<hash>";
// anchoring on that prefix prevents cluster "prod" from matching a
// "k8s-staging-prod-..." load balancer, and the exact-network check keeps the
// scope inside this cluster's network.
func ownsLoadBalancer(lb loadbalancers.LoadBalancer, clusterName, networkID string) bool {
	if lb.VipNetworkID != networkID {
		return false
	}
	prefix := "k8s-" + clusterName
	return lb.Name == prefix || strings.HasPrefix(lb.Name, prefix+"-")
}

// waitForLoadBalancerDeletion polls GET until Octavia reports 404 (gone).
// gophercloud v1.14.1 never populates Result.StatusCode on error responses,
// so the 404 must be detected via the deferred ErrDefault404 error.
func (s *gophercloudOpenStackNetworkCleanupService) waitForLoadBalancerDeletion(ctx context.Context, id string) error {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	deadline := time.NewTimer(s.deletionTimeout)
	defer deadline.Stop()

	for {
		result := loadbalancers.Get(s.loadBalancerClient, id)
		var notFound gophercloud.ErrDefault404
		if errors.As(result.Err, &notFound) {
			return nil
		}
		if result.Err != nil {
			return result.Err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out after %s", s.deletionTimeout)
		case <-ticker.C:
		}
	}
}
