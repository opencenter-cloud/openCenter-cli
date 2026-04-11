package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func DefaultConfigDir() string {
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
			if base == "" {
				return filepath.Join(string(os.PathSeparator), "tmp", "opencenter")
			}
			dir = filepath.Join(base, "opencenter")
		default:
			home, err := os.UserHomeDir()
			if err != nil {
				return filepath.Join(string(os.PathSeparator), "tmp", "opencenter")
			}
			dir = filepath.Join(home, ".config", "opencenter")
		}
	}

	normalized, err := NormalizeDir(dir)
	if err != nil {
		return dir
	}
	return normalized
}

func DefaultStateDir() string {
	dir := os.Getenv("OPENCENTER_STATE_DIR")
	if dir == "" {
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("LOCALAPPDATA")
			if base == "" {
				base = os.Getenv("APPDATA")
			}
			if base == "" {
				userProfile := os.Getenv("USERPROFILE")
				if userProfile != "" {
					base = filepath.Join(userProfile, "AppData", "Local")
				}
			}
			if base == "" {
				return filepath.Join(string(os.PathSeparator), "tmp", "opencenter", "state")
			}
			dir = filepath.Join(base, "opencenter", "state")
		default:
			base := os.Getenv("XDG_STATE_HOME")
			if base == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return filepath.Join(string(os.PathSeparator), "tmp", "opencenter", "state")
				}
				base = filepath.Join(home, ".local", "state")
			}
			dir = filepath.Join(base, "opencenter")
		}
	}

	normalized, err := NormalizeDir(dir)
	if err != nil {
		return dir
	}
	return normalized
}

func ResolveConfigDir() (string, error) {
	return ResolveDir(DefaultConfigDir())
}

func ResolveStateDir() (string, error) {
	return ResolveDir(DefaultStateDir())
}

func NormalizeDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("directory path cannot be empty")
	}

	var err error
	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}

	return filepath.Clean(dir), nil
}

func ResolveDir(dir string) (string, error) {
	dir, err := NormalizeDir(dir)
	if err != nil {
		return "", err
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
