package tofu

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/provision"
)

// Provision generates OpenTofu backend/provider configuration for the cluster.
// It writes a provider.tf containing the terraform backend block to
// <git_dir>/infrastructure/clusters/<cluster>/provider.tf.
// The template is selected based on the backend type:
// - "local" -> provider.local.tf.tpl
// - "s3" or "aws" -> provider.s3.tf.tpl
// Note: main.tf is rendered by RenderInfrastructureCluster from the static template

// ProvisionAt materializes OpenTofu configuration under gitDir without
// changing the caller's configured live repository path.
func ProvisionAt(cfg v2.Config, gitDir string) error {
	if strings.TrimSpace(gitDir) == "" {
		return fmt.Errorf("OpenTofu output directory is empty")
	}
	staged := cfg
	staged.OpenCenter.GitOps.Repository.LocalDir = gitDir
	return Provision(staged)
}

// to preserve human-readable ordering of locals and modules.
func Provision(cfg v2.Config) error {
	if cfg.OpenCenter.Infrastructure.Provider == "kind" {
		return nil
	}

	if !cfg.OpenTofu.Enabled {
		return nil
	}

	clusterDir := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName())
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cluster iac directory: %w", err)
	}

	if err := writeOpenStackCredentialTfvars(clusterDir, cfg); err != nil {
		return fmt.Errorf("failed to write OpenStack credential variables: %w", err)
	}

	// Validate template data before rendering
	if err := provision.ValidateTemplateData(cfg); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Determine which template to use based on backend type
	backendType := cfg.OpenTofu.Backend.Type
	if backendType == "" {
		backendType = "local"
	}

	var templateName string
	switch backendType {
	case "local":
		templateName = "provider.local.tf.tmpl"
	case "s3", "aws":
		templateName = "provider.s3.tf.tmpl"
	default:
		return fmt.Errorf("unsupported backend type: %s", backendType)
	}

	// Render provider.tf for OpenTofu backend
	providerPath := filepath.Join(clusterDir, "provider.tf")
	pf, err := os.Create(providerPath)
	if err != nil {
		return fmt.Errorf("failed to create provider.tf: %w", err)
	}
	if err := provision.Templates.ExecuteTemplate(pf, templateName, cfg); err != nil {
		_ = pf.Close()
		return fmt.Errorf("failed to execute %s template: %w", templateName, err)
	}
	if err := pf.Close(); err != nil {
		return err
	}
	return os.Chmod(providerPath, 0o644)
}

func writeOpenStackCredentialTfvars(clusterDir string, cfg v2.Config) error {
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		return nil
	}

	var content strings.Builder
	if strings.TrimSpace(openstack.ApplicationCredentialID) != "" {
		fmt.Fprintf(&content, "os_application_credential_id = %s\n", strconv.Quote(openstack.ApplicationCredentialID))
	}
	if strings.TrimSpace(openstack.ApplicationCredentialSecret) != "" {
		fmt.Fprintf(&content, "os_application_credential_secret = %s\n", strconv.Quote(openstack.ApplicationCredentialSecret))
	}
	if content.Len() == 0 {
		return nil
	}

	path := filepath.Join(clusterDir, "terraform.tfvars")
	return os.WriteFile(path, []byte(content.String()), 0o600)
}
