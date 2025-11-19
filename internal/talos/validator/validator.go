package validator

import (
	"context"
	"fmt"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// DefaultValidator implements the Validator interface.
type DefaultValidator struct {
	// OpenStack clients will be added here as we implement specific validators
	logger Logger
}

// Logger defines logging interface for the validator.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NewValidator creates a new validator instance.
func NewValidator(logger Logger) talos.Validator {
	return &DefaultValidator{
		logger: logger,
	}
}

// ValidateEnvironment checks all OpenStack prerequisites.
func (v *DefaultValidator) ValidateEnvironment(ctx context.Context, cfg *config.Config) (*talos.ValidationReport, error) {
	v.logger.Info("Starting OpenStack environment validation")

	report := &talos.ValidationReport{
		Passed:       true,
		Checks:       []talos.ValidationCheck{},
		Remediations: []talos.RemediationAction{},
		Timestamp:    time.Now(),
	}

	// Run all validation checks
	checks := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Keystone", v.ValidateKeystone},
		{"Barbican", v.ValidateBarbican},
		{"Octavia", v.ValidateOctavia},
		{"Glance", v.ValidateGlance},
	}

	for _, check := range checks {
		v.logger.Debug("Running validation check", "check", check.name)
		err := check.fn(ctx)

		validationCheck := talos.ValidationCheck{
			Name:     check.name,
			Passed:   err == nil,
			Severity: "error",
		}

		if err != nil {
			report.Passed = false
			validationCheck.Message = err.Error()

			// Extract remediation if available
			if talosErr, ok := err.(*talos.TalosError); ok && talosErr.Remediation != nil {
				report.Remediations = append(report.Remediations, *talosErr.Remediation)
			}
		} else {
			validationCheck.Message = fmt.Sprintf("%s validation passed", check.name)
		}

		report.Checks = append(report.Checks, validationCheck)
	}

	// Validate quotas
	// Always validate quotas for Talos deployments
	required := calculateRequiredResources(cfg)
	v.logger.Debug("Validating resource quotas", "required", required)
	err := v.ValidateQuotas(ctx, required)

	quotaCheck := talos.ValidationCheck{
		Name:     "Resource Quotas",
		Passed:   err == nil,
		Severity: "error",
	}

	if err != nil {
		report.Passed = false
		quotaCheck.Message = err.Error()

		if talosErr, ok := err.(*talos.TalosError); ok && talosErr.Remediation != nil {
			report.Remediations = append(report.Remediations, *talosErr.Remediation)
		}
	} else {
		quotaCheck.Message = "Resource quota validation passed"
	}

	report.Checks = append(report.Checks, quotaCheck)

	if report.Passed {
		v.logger.Info("All validation checks passed")
	} else {
		v.logger.Warn("Validation failed", "failed_checks", countFailedChecks(report.Checks))
	}

	return report, nil
}

// calculateRequiredResources determines resource requirements based on cluster configuration.
func calculateRequiredResources(cfg *config.Config) talos.ResourceRequirements {
	// Default requirements for a minimal cluster
	// These will be refined based on actual cluster configuration
	return talos.ResourceRequirements{
		Instances:      5,     // 3 control plane + 2 workers
		Cores:          20,    // 4 cores per node
		RAM:            40960, // 8GB per node
		Networks:       3,     // management, control, data
		Routers:        1,
		SecurityGroups: 5,
		Volumes:        5,
		VolumeStorage:  250, // 50GB per node
		Snapshots:      5,
		LoadBalancers:  1,
	}
}

// countFailedChecks counts the number of failed validation checks.
func countFailedChecks(checks []talos.ValidationCheck) int {
	count := 0
	for _, check := range checks {
		if !check.Passed {
			count++
		}
	}
	return count
}

// ValidateKeystone checks Keystone availability and MFA.
// Implementation is in keystone.go
func (v *DefaultValidator) ValidateKeystone(ctx context.Context) error {
	return v.ValidateKeystoneImpl(ctx)
}

// ValidateBarbican tests secret creation/retrieval.
// Implementation is in barbican.go
func (v *DefaultValidator) ValidateBarbican(ctx context.Context) error {
	return v.ValidateBarbicanImpl(ctx)
}

// ValidateOctavia checks load balancer service.
// Implementation is in octavia.go
func (v *DefaultValidator) ValidateOctavia(ctx context.Context) error {
	return v.ValidateOctaviaImpl(ctx)
}

// ValidateQuotas verifies tenant resource quotas.
// Implementation is in quota.go
func (v *DefaultValidator) ValidateQuotas(ctx context.Context, required talos.ResourceRequirements) error {
	return v.ValidateQuotasImpl(ctx, required)
}

// ValidateGlance checks image signature verification.
// Implementation is in glance.go
func (v *DefaultValidator) ValidateGlance(ctx context.Context) error {
	return v.ValidateGlanceImpl(ctx)
}
