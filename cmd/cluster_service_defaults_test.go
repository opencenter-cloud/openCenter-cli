package cmd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

func TestClusterServiceEnableAbsentBuiltInRestoresPublicDefaults(t *testing.T) {
	clusterName := "absent-built-in-defaults"
	_, cleanup := setupServiceTestEnv(t, clusterName)
	defer cleanup()

	cfg, err := loadConfig(context.Background(), clusterName)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	delete(cfg.OpenCenter.Services, "keycloak")
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config without keycloak: %v", err)
	}

	cmd := newClusterServiceEnableCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"keycloak"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("enable absent keycloak: %v", err)
	}

	cfg, err = loadConfig(context.Background(), clusterName)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	keycloak, ok := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig)
	if !ok {
		t.Fatalf("keycloak type = %T, want *services.KeycloakConfig", cfg.OpenCenter.Services["keycloak"])
	}
	if !keycloak.Enabled || keycloak.Namespace != "keycloak" || keycloak.Hostname != "auth.absent-built-in-defaults.dfw3.k8s.opencenter.cloud" {
		t.Fatalf("restored keycloak defaults = %#v", keycloak)
	}
	if keycloak.Source != (services.ServiceSource{}) || keycloak.Image != (services.ServiceImage{}) {
		t.Fatalf("restored renderer-owned metadata: source=%#v image=%#v", keycloak.Source, keycloak.Image)
	}
	if cfg.OpenCenter.Services["keycloak"] == nil {
		t.Fatal("keycloak was not persisted")
	}
}

func TestClusterServiceEnableExistingPartialBuiltInHydratesMissingDefaultsAndPreservesExplicitValues(t *testing.T) {
	clusterName := "partial-built-in-defaults"
	_, cleanup := setupServiceTestEnv(t, clusterName)
	defer cleanup()

	configPath, err := getConfigPath(context.Background(), clusterName, "opencenter")
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	opencenter := document["opencenter"].(map[string]any)
	serviceMap := opencenter["services"].(map[string]any)
	serviceMap["keycloak"] = map[string]any{
		"enabled":              false,
		"instances":            0,
		"resource_limits_cpu":  "",
		"realm_import_enabled": false,
	}
	data, err = yaml.Marshal(document)
	if err != nil {
		t.Fatalf("encode partial config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write partial config: %v", err)
	}
	resetCommandStateForTests()

	cmd := newClusterServiceEnableCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"keycloak"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("enable partial keycloak: %v", err)
	}

	cfg, err := loadConfig(context.Background(), clusterName)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	keycloak := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig)
	if !keycloak.Enabled || keycloak.Namespace != "keycloak" || keycloak.Hostname == "" || keycloak.ResourceRequestsCPU != "500m" {
		t.Fatalf("hydrated keycloak = %#v", keycloak)
	}
	if keycloak.Instances != 0 || keycloak.ResourceLimitsCPU != "" || keycloak.RealmImportEnabled {
		t.Fatalf("explicit zero/empty/false values were overwritten: %#v", keycloak)
	}
}

func TestHydrateBuiltInServiceConfigCoversStorageAndRepresentativeBuiltIns(t *testing.T) {
	tests := []struct {
		name       string
		existing   any
		explicit   map[string]any
		assertions func(*testing.T, any)
	}{
		{
			name:     "loki",
			existing: &services.LokiConfig{BaseConfig: services.BaseConfig{Namespace: "custom-loki"}, StorageType: "s3"},
			explicit: map[string]any{"namespace": "custom-loki", "storage_type": "s3"},
			assertions: func(t *testing.T, value any) {
				got := value.(*services.LokiConfig)
				if got.Namespace != "custom-loki" || got.StorageType != "s3" {
					t.Fatalf("loki = %#v", got)
				}
			},
		},
		{
			name:     "tempo",
			existing: &services.TempoConfig{},
			explicit: map[string]any{},
			assertions: func(t *testing.T, value any) {
				if got := value.(*services.TempoConfig).Namespace; got != "observability" {
					t.Fatalf("tempo namespace = %q", got)
				}
			},
		},
		{
			name:     "mimir",
			existing: &services.MimirConfig{},
			explicit: map[string]any{},
			assertions: func(t *testing.T, value any) {
				if got := value.(*services.MimirConfig).Namespace; got != "observability" {
					t.Fatalf("mimir namespace = %q", got)
				}
			},
		},
		{
			name:     "velero",
			existing: &services.VeleroConfig{StorageType: "swift"},
			explicit: map[string]any{"storage_type": "swift"},
			assertions: func(t *testing.T, value any) {
				got := value.(*services.VeleroConfig)
				if got.Namespace != "velero" || got.StorageType != "swift" {
					t.Fatalf("velero = %#v", got)
				}
			},
		},
		{
			name:     "headlamp",
			existing: &services.HeadlampConfig{},
			explicit: map[string]any{},
			assertions: func(t *testing.T, value any) {
				got := value.(*services.HeadlampConfig)
				if got.Namespace != "headlamp" || got.Hostname != "dashboard.example.com" {
					t.Fatalf("headlamp = %#v", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hydrateBuiltInServiceConfig(tt.name, tt.existing, "example.com", tt.explicit)
			if err != nil {
				t.Fatalf("hydrate: %v", err)
			}
			tt.assertions(t, got)
		})
	}
}
