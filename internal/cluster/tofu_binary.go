package cluster

import (
	"fmt"
	"os/exec"
	"strings"
)

// resolveTofuBinary determines the OpenTofu/Terraform binary path to use.
// It checks the configured path first. If no path is configured, it searches
// for "tofu" on $PATH, falling back to "terraform" if tofu is not found.
// Returns an error if no suitable binary is available.
func resolveTofuBinary(configuredPath string) (string, error) {
	if path := strings.TrimSpace(configuredPath); path != "" {
		return path, nil
	}
	if _, err := exec.LookPath("tofu"); err == nil {
		return "tofu", nil
	}
	if _, err := exec.LookPath("terraform"); err == nil {
		return "terraform", nil
	}
	return "", fmt.Errorf("neither tofu nor terraform found on PATH: install OpenTofu (tofu) or Terraform and ensure it is available in your PATH")
}
