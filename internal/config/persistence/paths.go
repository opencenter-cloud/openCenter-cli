package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func ResolveConfigDir() (string, error) {
	var err error
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if dir == "" {
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("APPDATA")
			if base == "" {
				base = os.Getenv("LOCALAPPDATA")
			}
			if base == "" {
				base = os.Getenv("USERPROFILE")
			}
			dir = filepath.Join(base, "opencenter")
		default:
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			dir = filepath.Join(home, ".config", "opencenter")
		}
	}

	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return dir, nil
}

func ParseClusterIdentifier(identifier string, validateClusterName func(string) error) (organization string, clusterName string, err error) {
	if identifier == "" {
		return "", "", fmt.Errorf("cluster identifier cannot be empty")
	}

	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid cluster identifier format: expected 'organization/cluster'")
		}
		organization = parts[0]
		clusterName = parts[1]
		if organization == "" {
			return "", "", fmt.Errorf("organization name cannot be empty")
		}
		if err := validateClusterName(clusterName); err != nil {
			return "", "", err
		}
		return organization, clusterName, nil
	}

	if err := validateClusterName(identifier); err != nil {
		return "", "", err
	}
	return "opencenter", identifier, nil
}
