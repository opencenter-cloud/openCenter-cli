package di

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
)

func TestNewApp(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)

	app, err := NewApp(dirs.ClustersDir)
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}
	if app == nil {
		t.Fatal("NewApp() returned nil")
		return
	}
	if app.PathResolver == nil || app.ValidationEngine == nil || app.ConfigManager == nil {
		t.Fatal("NewApp() did not initialize core dependencies")
	}
	if app.InitService == nil || app.ValidateService == nil || app.SetupService == nil || app.BootstrapService == nil {
		t.Fatal("NewApp() did not initialize core services")
	}
	if app.CommandRunner == nil {
		t.Fatal("NewApp() did not initialize security services")
	}
}

func TestNewAppDoesNotCreateCLIConfigInClustersDir(t *testing.T) {
	root := t.TempDir()
	homeDir := filepath.Join(root, "home")
	configDir := filepath.Join(homeDir, ".config", "opencenter")
	clustersDir := filepath.Join(root, "cluster-store")

	t.Setenv("HOME", homeDir)
	t.Setenv("OPENCENTER_CLUSTERS_DIR", clustersDir)
	t.Setenv("OPENCENTER_CONFIG_DIR", "")
	t.Setenv("OPENCENTER_PLUGINS_DIR", "")
	t.Setenv("OPENCENTER_STATE_DIR", "")

	if _, err := NewApp(clustersDir); err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(configDir, "settings.yaml")); err != nil {
		t.Fatalf("tool config stat error = %v, want settings.yaml under config dir", err)
	}

	if _, err := os.Stat(filepath.Join(clustersDir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("clusters dir config.yaml stat error = %v, want not exist", err)
	}
}

func TestNewAppContainerResolveAs(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)

	app, err := NewApp(dirs.ClustersDir)
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}

	container := NewAppContainer(app)
	var setupService *cluster.SetupService
	if err := container.ResolveAs("SetupService", &setupService); err != nil {
		t.Fatalf("ResolveAs() failed: %v", err)
	}
	if setupService == nil {
		t.Fatal("ResolveAs() returned nil service")
	}
}
