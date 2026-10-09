package sops

import (
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// overlayFilesToEncrypt returns the ordered overlay files that should be encrypted.
func overlayFilesToEncrypt(cfg *v2.Config) []string {
	files := []string{
		"flux-system/gotk-sync.yaml",
		"managed-services/sources/base-repo.yaml",
	}

	switch cfg.OpenCenter.Infrastructure.Provider {
	case "openstack":
		files = append(files, "secrets/openstack-credentials.yaml")
	case "vsphere":
		files = append(files,
			"secrets/vsphere-credentials.yaml",
			"customer-managed/services/cloud-provider-vsphere/secret.yaml",
		)
	}

	// Service override-values files that contain credentials must be
	// encrypted before commit. The encryption is full-file (not partial via
	// encrypted_regex) so that no plaintext credential leaks into git.
	// FluxCD decrypts these at reconciliation time via its SOPS provider.
	files = append(files, serviceOverrideValuesFilesToEncrypt(cfg)...)

	return files
}

// serviceOverrideValuesFilesToEncrypt returns override-values.yaml paths for
// services whose Helm values contain embedded credentials.
// Non-existent files are silently skipped by callers (EncryptOverlayFiles,
// encryptFilesForCommit) so it is safe to include paths for services that
// may not be enabled or may not have generated an override-values file yet.
func serviceOverrideValuesFilesToEncrypt(cfg *v2.Config) []string {
	// Services with credentials in their override-values.yaml templates:
	//   - openstack-ccm: cloud credentials (openstack and Magnum)
	//   - openstack-csi: cloud credentials (openstack and Magnum)
	//   - loki: swift application_credential_secret or S3 secretAccessKey
	//   - tempo: swift application_credential_secret or S3 secret_key
	//   - mimir: Swift application_credential_secret
	//   - headlamp: OIDC client secret
	//   - harbor: object-storage and registry/admin credentials
	var files []string

	serviceNames := []string{"loki", "tempo", "mimir", "headlamp", "harbor"}
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	if provider == "openstack" || provider == "magnum" {
		serviceNames = append([]string{"openstack-ccm", "openstack-csi"}, serviceNames...)
	}

	for _, serviceName := range serviceNames {
		files = append(files, "services/"+serviceName+"/helm-values/override-values.yaml")
	}

	return files
}
