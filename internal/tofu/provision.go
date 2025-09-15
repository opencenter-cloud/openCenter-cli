package tofu

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/rackerlabs/openCenter/internal/config"
    "github.com/rackerlabs/openCenter/internal/provision"
)

// Provision generates OpenTofu backend/provider configuration for the cluster.
// It writes a provider.tf containing the terraform backend block to
// <git_dir>/infrastructure/clusters/<cluster>/provider.tf.
func Provision(cfg config.Config) error {
    if !cfg.OpenTofu.Enabled {
        return nil
    }

    clusterDir := filepath.Join(cfg.GitOps.GitDir, "infrastructure", "clusters", cfg.ClusterName)
    if err := os.MkdirAll(clusterDir, 0o755); err != nil {
        return fmt.Errorf("failed to create cluster iac directory: %w", err)
    }

    // Render main.tf from structured IaC (locals + modules)
    mainPath := filepath.Join(clusterDir, "main.tf")
    mf, err := os.Create(mainPath)
    if err != nil {
        return fmt.Errorf("failed to create main.tf: %w", err)
    }
    if err := provision.Templates.ExecuteTemplate(mf, "main.tf.tmpl", cfg); err != nil {
        mf.Close()
        return fmt.Errorf("failed to execute main.tf template: %w", err)
    }
    if err := mf.Close(); err != nil {
        return err
    }

    // Render provider.tf for OpenTofu backend
    providerPath := filepath.Join(clusterDir, "provider.tf")
    pf, err := os.Create(providerPath)
    if err != nil {
        return fmt.Errorf("failed to create provider.tf: %w", err)
    }
    if err := provision.Templates.ExecuteTemplate(pf, "provider.tf.tmpl", cfg); err != nil {
        pf.Close()
        return fmt.Errorf("failed to execute provider.tf template: %w", err)
    }
    return pf.Close()
}
