package cluster

import (
	"os/exec"
	"strings"
)

// resolveTofuBinary determines the OpenTofu/Terraform binary path to use.
// It checks the configured path first, then looks for "tofu" on PATH,
// and falls back to "terraform" if tofu is not installed.
func resolveTofuBinary(configuredPath string) (string, error) {
	if path := strings.TrimSpace(configuredPath); path != "" {
		return path, nil
	}
	if _, err := exec.LookPath("tofu"); err == nil {
		return "tofu", nil
	}
	return "terraform", nil
}
