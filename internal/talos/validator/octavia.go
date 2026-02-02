package validator

import (
	"context"

	"github.com/rackerlabs/opencenter-cli/internal/talos"
)

// ValidateOctaviaImpl checks load balancer service availability and quota.
func (v *DefaultValidator) ValidateOctaviaImpl(ctx context.Context) error {
	v.logger.Debug("Validating Octavia load balancer service")

	// Check if Octavia service is available
	available, err := v.checkOctaviaAvailability(ctx)
	if err != nil {
		return talos.NewInfrastructureError(
			"OCTAVIA_UNAVAILABLE",
			"Failed to connect to Octavia service",
			true,
			err,
		)
	}

	if !available {
		// Octavia unavailability is not a hard failure - we can fall back to HAProxy
		v.logger.Warn("Octavia service is not available, HAProxy fallback will be used")
		remediation := &talos.RemediationAction{
			Check:       "Octavia",
			Description: "Octavia load balancer service is not available (HAProxy fallback will be used)",
			Steps: []string{
				"Note: This is not a critical failure - the system will deploy HAProxy instances as a fallback",
				"To use Octavia instead of HAProxy:",
				"  1. Verify that Octavia is installed in your OpenStack deployment",
				"  2. Check that the Octavia endpoint is registered in Keystone service catalog",
				"  3. Ensure the Octavia service is running",
				"  4. Verify firewall rules allow access to the Octavia API",
				"  5. Check Octavia service logs: journalctl -u openstack-octavia-api",
				"HAProxy fallback provides equivalent functionality with slightly reduced automation",
			},
		}
		// Return a warning-level error that won't fail validation
		return talos.NewValidationError(
			"OCTAVIA_NOT_AVAILABLE_FALLBACK",
			"Octavia service is not available, will use HAProxy fallback",
			remediation,
		)
	}

	// Check load balancer quota
	hasQuota, err := v.checkLoadBalancerQuota(ctx)
	if err != nil {
		return talos.NewInfrastructureError(
			"OCTAVIA_QUOTA_CHECK_FAILED",
			"Failed to check load balancer quota",
			true,
			err,
		)
	}

	if !hasQuota {
		remediation := &talos.RemediationAction{
			Check:       "Octavia",
			Description: "Insufficient load balancer quota",
			Steps: []string{
				"Request increased load balancer quota from your OpenStack administrator",
				"Required: At least 1 load balancer for Kubernetes API access",
				"Check current quota: openstack loadbalancer quota show",
				"Alternatively, the system can fall back to HAProxy if quota cannot be increased",
			},
		}
		return talos.NewValidationError(
			"OCTAVIA_INSUFFICIENT_QUOTA",
			"Insufficient load balancer quota available",
			remediation,
		)
	}

	v.logger.Info("Octavia validation passed", "available", available, "has_quota", hasQuota)
	return nil
}

// checkOctaviaAvailability verifies that Octavia service is reachable.
func (v *DefaultValidator) checkOctaviaAvailability(ctx context.Context) (bool, error) {
	v.logger.Debug("Checking Octavia service availability")
	// Placeholder implementation - returns true for now
	// Real implementation would use gophercloud octavia client to verify service availability
	return true, nil
}

// checkLoadBalancerQuota verifies sufficient load balancer quota is available.
func (v *DefaultValidator) checkLoadBalancerQuota(ctx context.Context) (bool, error) {
	v.logger.Debug("Checking load balancer quota")
	// Placeholder implementation - returns true for now
	// Real implementation would query Octavia quota and verify availability
	return true, nil
}
