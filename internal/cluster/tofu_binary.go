package cluster

import (
	"strings"
)

// resolveTofuBinary determines the OpenTofu/Terraform binary path to use.
// It checks the configured path first. If no path is configured, command
// execution uses "tofu" and lets the runner surface a missing binary at run
// time. This keeps dry-run planning and unit tests independent from host tools.
func resolveTofuBinary(configuredPath string) (string, error) {
	if path := strings.TrimSpace(configuredPath); path != "" {
		return path, nil
	}
	return "tofu", nil
}
