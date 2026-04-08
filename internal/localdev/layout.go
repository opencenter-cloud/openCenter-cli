package localdev

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const defaultLocalSubdir = "local"

// Layout describes the local-dev plugin state directory.
type Layout struct {
	Root           string
	GiteaDataDir   string
	GiteaConfDir   string
	GiteaCertDir   string
	TokensDir      string
	MetadataPath   string
	AppIniPath     string
	CACertPath     string
	ServerCertPath string
	ServerKeyPath  string
	AdminTokenPath string
	UserTokenPath  string
}

// ResolveLayout returns the absolute state layout for the plugin.
// When root is empty the layout defaults to <OPENCENTER_CONFIG_DIR>/local,
// keeping all state inside the standard configuration tree instead of
// creating a dot-directory in the working directory.
func ResolveLayout(root string) (Layout, error) {
	if root == "" {
		configDir, err := resolveConfigDir()
		if err != nil {
			return Layout{}, fmt.Errorf("resolve config directory: %w", err)
		}
		root = filepath.Join(configDir, defaultLocalSubdir)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve state dir: %w", err)
	}

	giteaRoot := filepath.Join(absRoot, "gitea")
	return Layout{
		Root:           absRoot,
		GiteaDataDir:   giteaRoot,
		GiteaConfDir:   filepath.Join(giteaRoot, "gitea", "conf"),
		GiteaCertDir:   filepath.Join(giteaRoot, "gitea", "certs"),
		TokensDir:      filepath.Join(absRoot, "tokens"),
		MetadataPath:   filepath.Join(absRoot, "gitea.json"),
		AppIniPath:     filepath.Join(giteaRoot, "gitea", "conf", "app.ini"),
		CACertPath:     filepath.Join(giteaRoot, "gitea", "certs", "ca.pem"),
		ServerCertPath: filepath.Join(giteaRoot, "gitea", "certs", "cert.pem"),
		ServerKeyPath:  filepath.Join(giteaRoot, "gitea", "certs", "key.pem"),
		AdminTokenPath: filepath.Join(absRoot, "tokens", "gitea-admin.token"),
		UserTokenPath:  filepath.Join(absRoot, "tokens", "gitea-user.token"),
	}, nil
}

// Ensure creates the state directory structure.
func (l Layout) Ensure() error {
	for _, dir := range []string{l.Root, l.GiteaConfDir, l.GiteaCertDir, l.TokensDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// resolveConfigDir returns the openCenter configuration directory.
// It mirrors the logic in internal/config/persistence without importing
// that package, avoiding a circular dependency.
func resolveConfigDir() (string, error) {
	if dir := os.Getenv("OPENCENTER_CONFIG_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("resolve OPENCENTER_CONFIG_DIR: %w", err)
		}
		return abs, nil
	}

	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			base = os.Getenv("LOCALAPPDATA")
		}
		if base == "" {
			base = os.Getenv("USERPROFILE")
		}
		return filepath.Join(base, "opencenter"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "opencenter"), nil
	}
}
