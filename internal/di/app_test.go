package di

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
)

func TestNewApp(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}
	if app == nil {
		t.Fatal("NewApp() returned nil")
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

func TestNewAppContainerResolveAs(t *testing.T) {
	app, err := NewApp(t.TempDir())
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
