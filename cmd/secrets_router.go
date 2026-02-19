package cmd

import (
	"context"
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// resolveBackend reads the secrets.backend field from the active cluster config
// and returns the backend name. If the field is empty, it defaults to "barbican"
// for backward compatibility. Returns an error if the backend value is not supported.
//
// Supported backends: barbican, sops, file
func resolveBackend(ctx context.Context, clusterName string) (string, *config.Config, error) {
	cfg, err := loadConfig(ctx, clusterName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load cluster config: %w", err)
	}

	backend := cfg.OpenCenter.Secrets.Backend
	if backend == "" {
		// Default to barbican for backward compatibility
		backend = "barbican"
	}

	// Validate backend value
	switch backend {
	case "barbican", "sops", "file":
		return backend, &cfg, nil
	default:
		return "", nil, fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
	}
}
