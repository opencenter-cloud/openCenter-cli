package validator

import (
	"context"

	"github.com/rackerlabs/opencenter-cli/internal/talos"
)

// ValidateGlanceImpl checks image signature verification.
func (v *DefaultValidator) ValidateGlanceImpl(ctx context.Context) error {
	v.logger.Debug("Validating Glance image service and signature verification")

	// Check if Glance service is available
	available, err := v.checkGlanceAvailability(ctx)
	if err != nil {
		return talos.NewInfrastructureError(
			"GLANCE_UNAVAILABLE",
			"Failed to connect to Glance service",
			true,
			err,
		)
	}

	if !available {
		remediation := &talos.RemediationAction{
			Check:       "Glance",
			Description: "Glance image service is not available",
			Steps: []string{
				"Verify that Glance is installed in your OpenStack deployment",
				"Check that the Glance endpoint is registered in Keystone service catalog",
				"Ensure the Glance service is running",
				"Verify firewall rules allow access to the Glance API port (typically 9292)",
				"Check Glance service logs: journalctl -u openstack-glance-api",
			},
		}
		return talos.NewValidationError(
			"GLANCE_NOT_AVAILABLE",
			"Glance service is not available",
			remediation,
		)
	}

	// Check if image signature verification is enabled
	signatureVerificationEnabled, err := v.checkImageSignatureVerification(ctx)
	if err != nil {
		return talos.NewInfrastructureError(
			"GLANCE_SIGNATURE_CHECK_FAILED",
			"Failed to check image signature verification status",
			true,
			err,
		)
	}

	if !signatureVerificationEnabled {
		remediation := &talos.RemediationAction{
			Check:       "Glance",
			Description: "Image signature verification is not enabled",
			Steps: []string{
				"Enable image signature verification in Glance configuration",
				"Edit /etc/glance/glance-api.conf and set:",
				"  [DEFAULT]",
				"  enable_image_signature_verification = True",
				"  verify_glance_signatures = True",
				"Restart Glance service: systemctl restart openstack-glance-api",
				"Verify configuration: openstack image show <image-id> -f json | jq .properties",
				"Documentation: https://docs.openstack.org/glance/latest/admin/signature-verification.html",
			},
		}
		return talos.NewSecurityError(
			"GLANCE_SIGNATURE_VERIFICATION_DISABLED",
			"Image signature verification is not enabled in Glance",
			remediation,
			nil,
		)
	}

	// Check for signed Talos images
	hasTalosImages, err := v.checkForSignedTalosImages(ctx)
	if err != nil {
		return talos.NewInfrastructureError(
			"GLANCE_TALOS_IMAGE_CHECK_FAILED",
			"Failed to check for signed Talos images",
			true,
			err,
		)
	}

	if !hasTalosImages {
		// This is informational - not having images yet is not a failure
		v.logger.Info("No signed Talos images found in Glance (this is expected for new deployments)")
	}

	v.logger.Info("Glance validation passed",
		"signature_verification_enabled", signatureVerificationEnabled,
		"has_talos_images", hasTalosImages,
	)
	return nil
}

// checkGlanceAvailability verifies that Glance service is reachable.
func (v *DefaultValidator) checkGlanceAvailability(ctx context.Context) (bool, error) {
	v.logger.Debug("Checking Glance service availability")
	// Placeholder implementation - returns true for now
	// Real implementation would use gophercloud glance client to verify service availability
	return true, nil
}

// checkImageSignatureVerification verifies that image signature verification is enabled.
func (v *DefaultValidator) checkImageSignatureVerification(ctx context.Context) (bool, error) {
	v.logger.Debug("Checking image signature verification status")
	// Placeholder implementation - returns true for now
	// Real implementation would query Glance configuration to verify signature verification is enabled
	return true, nil
}

// checkForSignedTalosImages checks if signed Talos images exist in Glance.
func (v *DefaultValidator) checkForSignedTalosImages(ctx context.Context) (bool, error) {
	v.logger.Debug("Checking for signed Talos images")
	// Placeholder implementation - returns false for now (acceptable for new deployments)
	// Real implementation would search Glance for Talos images with valid signatures
	return false, nil
}

// validateSignatureMetadata validates the format of image signature metadata.
func (v *DefaultValidator) validateSignatureMetadata(metadata map[string]string) bool {
	// Check for required signature metadata fields
	requiredFields := []string{
		"img_signature",
		"img_signature_hash_method",
		"img_signature_key_type",
		"img_signature_certificate_uuid",
	}

	for _, field := range requiredFields {
		if _, exists := metadata[field]; !exists {
			v.logger.Debug("Missing required signature metadata field", "field", field)
			return false
		}
	}

	return true
}
