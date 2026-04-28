package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"gopkg.in/yaml.v3"
)

func TestValidateService_Validate(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) (string, string) // Returns clusterName and configDir
		opts           ValidateOptions
		wantErr        bool
		wantConfigFile bool
	}{
		{
			name: "valid configuration file exists",
			setupFunc: func(t *testing.T) (string, string) {
				return setupTestCluster(t, "valid-cluster", validTestConfig())
			},
			opts: ValidateOptions{
				ClusterName:       "valid-cluster",
				Organization:      "opencenter",
				CheckConnectivity: false,
				CheckProvider:     false,
			},
			wantErr:        false,
			wantConfigFile: true,
		},
		{
			name: "missing cluster directory",
			setupFunc: func(t *testing.T) (string, string) {
				configDir := t.TempDir()
				clustersBaseDir := filepath.Join(configDir, "clusters")
				// Create organization directory but not cluster directory
				orgDir := filepath.Join(clustersBaseDir, "opencenter", "infrastructure", "clusters")
				if err := os.MkdirAll(orgDir, 0755); err != nil {
					t.Fatalf("failed to create org directory: %v", err)
				}
				return "missing-cluster", clustersBaseDir
			},
			opts: ValidateOptions{
				ClusterName:  "missing-cluster",
				Organization: "opencenter",
			},
			wantErr:        true,
			wantConfigFile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterName, configDir := tt.setupFunc(t)
			defer os.RemoveAll(configDir)

			// Set up test environment
			oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
			os.Setenv("OPENCENTER_CONFIG_DIR", configDir)
			defer os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)

			// Create service
			pathResolver := paths.NewPathResolver(configDir)
			validationEngine := validation.NewValidationEngine()
			configManager, _ := config.NewConfigManager("")

			service := NewValidateService(pathResolver, validationEngine, configManager)

			// Update opts with cluster name
			tt.opts.ClusterName = clusterName

			// Execute validation
			result, err := service.Validate(context.Background(), tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Validate() unexpected error = %v", err)
			}

			// For successful path resolution, check that we got a result
			if result == nil {
				t.Fatal("Validate() returned nil result")
			}

			// Check that config file check worked as expected
			if tt.wantConfigFile && len(result.Errors) > 0 {
				// Check if error is about missing config file
				hasConfigFileError := false
				for _, err := range result.Errors {
					if contains(err, "configuration file not found") {
						hasConfigFileError = true
						break
					}
				}
				if !hasConfigFileError {
					t.Logf("Validation errors: %v", result.Errors)
				}
			}
		})
	}
}

func TestValidateService_ValidateFailsForEnabledServicePlaceholders(t *testing.T) {
	cfg, err := v2.NewV2Default("placeholder-cluster", "kind")
	if err != nil {
		t.Fatalf("create default config: %v", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "placeholder-cluster.yaml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	service := NewValidateService(
		paths.NewPathResolver(t.TempDir()),
		validation.NewValidationEngine(),
		nil,
	)

	result, err := service.Validate(context.Background(), ValidateOptions{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected validation to fail for enabled services with CHANGEME placeholders")
	}

	errMsg := strings.Join(result.Errors, "\n")
	for _, want := range []string{
		`placeholder value "CHANGEME"`,
		"secrets.keycloak.client_secret",
		"secrets.keycloak.admin_password",
		"secrets.headlamp.oidc_client_secret",
	} {
		if !strings.Contains(errMsg, want) {
			t.Fatalf("expected validation errors to contain %q, got:\n%s", want, errMsg)
		}
	}
}

func TestValidateService_FormatResult(t *testing.T) {
	tests := []struct {
		name       string
		result     *ValidationResult
		wantSubstr []string
	}{
		{
			name: "successful validation",
			result: &ValidationResult{
				Valid:             true,
				ConfigValid:       true,
				ConnectivityValid: true,
				ProviderValid:     true,
			},
			wantSubstr: []string{
				"✓ Validation successful",
				"Configuration: ✓ Valid",
				"Connectivity:  ✓ Valid",
				"Provider:      ✓ Valid",
			},
		},
		{
			name: "failed validation with errors",
			result: &ValidationResult{
				Valid:       false,
				ConfigValid: false,
				Errors: []string{
					"cluster name is required",
					"invalid provider",
				},
			},
			wantSubstr: []string{
				"✗ Validation failed",
				"Configuration: ✗ Invalid",
				"cluster name is required",
				"invalid provider",
			},
		},
		{
			name: "validation with warnings",
			result: &ValidationResult{
				Valid:       true,
				ConfigValid: true,
				Warnings: []string{
					"using default value for field X",
				},
			},
			wantSubstr: []string{
				"✓ Validation successful",
				"Warnings:",
				"using default value for field X",
			},
		},
		{
			name: "validation with suggestions",
			result: &ValidationResult{
				Valid:       false,
				ConfigValid: false,
				Errors: []string{
					"invalid cluster name",
				},
				Suggestions: []string{
					"cluster name must be lowercase",
					"cluster name must start with a letter",
				},
			},
			wantSubstr: []string{
				"✗ Validation failed",
				"Suggestions:",
				"cluster name must be lowercase",
				"cluster name must start with a letter",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &ValidateService{}
			output := service.FormatResult(tt.result)

			for _, substr := range tt.wantSubstr {
				if !contains(output, substr) {
					t.Errorf("FormatResult() output missing substring %q\nGot:\n%s", substr, output)
				}
			}
		})
	}
}

// Helper functions

func setupTestCluster(t *testing.T, clusterName string, configContent string) (string, string) {
	t.Helper()

	configDir := t.TempDir()
	// Use organization-based structure - need to create the clusters base directory
	clustersBaseDir := filepath.Join(configDir, "clusters")
	clusterDir := filepath.Join(clustersBaseDir, "opencenter", "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatalf("failed to create cluster directory: %v", err)
	}

	configFile := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Return cluster name and the clusters base directory (not configDir)
	return clusterName, clustersBaseDir
}

func validTestConfig() string {
	return `opencenter:
  cluster:
    cluster_name: test-cluster
    cluster_fqdn: test.example.com
    kubernetes_version: "1.28.0"
  infrastructure:
    provider: kind
  deployment:
    method: kind
  gitops:
    enabled: true
    git_dir: /tmp/gitops
`
}

func invalidTestConfig() string {
	return `opencenter:
  cluster:
    cluster_name: ""
    cluster_fqdn: ""
  infrastructure:
    provider: invalid-provider
`
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewValidateService(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := validation.NewValidationEngine()
	configManager, _ := config.NewConfigManager("")

	service := NewValidateService(pathResolver, validationEngine, configManager)

	if service == nil {
		t.Fatal("NewValidateService returned nil")
	}

	if service.pathResolver == nil {
		t.Error("pathResolver is nil")
	}

	if service.validationEngine == nil {
		t.Error("validationEngine is nil")
	}

	if service.connectivityValidator == nil {
		t.Error("connectivityValidator is nil")
	}
}

func TestValidateService_validateConnectivity(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := validation.NewValidationEngine()
	configManager, _ := config.NewConfigManager("")
	service := NewValidateService(pathResolver, validationEngine, configManager)

	cfg := v2.Config{}
	cfg.OpenCenter.Infrastructure.Provider = "kind"

	result := &ValidationResult{
		Valid:             true,
		ConfigValid:       true,
		ConnectivityValid: true,
		ProviderValid:     true,
	}

	ctx := context.Background()
	err := service.validateConnectivity(ctx, &cfg, result)

	if err != nil {
		t.Errorf("validateConnectivity() error = %v", err)
	}
}

func TestValidateService_validateProviderSpecific(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := validation.NewValidationEngine()
	configManager, _ := config.NewConfigManager("")
	service := NewValidateService(pathResolver, validationEngine, configManager)

	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "kind provider",
			provider: "kind",
			wantErr:  false,
		},
		{
			name:     "openstack provider",
			provider: "openstack",
			wantErr:  false,
		},
		{
			name:     "aws provider",
			provider: "aws",
			wantErr:  false,
		},
		{
			name:     "vsphere provider",
			provider: "vsphere",
			wantErr:  false,
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			wantErr:  false, // Should not error, but mark as invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := v2.Config{}
			cfg.OpenCenter.Infrastructure.Provider = tt.provider

			result := &ValidationResult{
				Valid:             true,
				ConfigValid:       true,
				ConnectivityValid: true,
				ProviderValid:     true,
			}

			ctx := context.Background()
			err := service.validateProviderSpecific(ctx, &cfg, result)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateProviderSpecific() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.provider == "unknown" && result.ProviderValid {
				t.Error("validateProviderSpecific() should mark unknown provider as invalid")
			}
		})
	}
}

func TestValidateService_formatStatus(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
		want  string
	}{
		{
			name:  "valid status",
			valid: true,
			want:  "✓ Valid",
		},
		{
			name:  "invalid status",
			valid: false,
			want:  "✗ Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatus(tt.valid)
			if got != tt.want {
				t.Errorf("formatStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateService_Validate_WithConnectivity(t *testing.T) {
	clusterName, configDir := setupTestCluster(t, "connectivity-cluster", validTestConfig())
	defer os.RemoveAll(configDir)

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)

	pathResolver := paths.NewPathResolver(configDir)
	validationEngine := validation.NewValidationEngine()
	configManager, _ := config.NewConfigManager("")
	service := NewValidateService(pathResolver, validationEngine, configManager)

	opts := ValidateOptions{
		ClusterName:       clusterName,
		Organization:      "opencenter",
		CheckConnectivity: true,
		CheckProvider:     false,
	}

	result, err := service.Validate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result == nil {
		t.Fatal("Validate() returned nil result")
	}
}

func TestValidateService_Validate_WithProvider(t *testing.T) {
	clusterName, configDir := setupTestCluster(t, "provider-cluster", validTestConfig())
	defer os.RemoveAll(configDir)

	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)

	pathResolver := paths.NewPathResolver(configDir)
	validationEngine := validation.NewValidationEngine()
	configManager, _ := config.NewConfigManager("")
	service := NewValidateService(pathResolver, validationEngine, configManager)

	opts := ValidateOptions{
		ClusterName:       clusterName,
		Organization:      "opencenter",
		CheckConnectivity: false,
		CheckProvider:     true,
	}

	result, err := service.Validate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result == nil {
		t.Fatal("Validate() returned nil result")
	}
}

func TestValidateService_FormatResult_WithMultipleErrors(t *testing.T) {
	service := &ValidateService{}
	result := &ValidationResult{
		Valid:       false,
		ConfigValid: false,
		Errors: []string{
			"error 1",
			"error 2",
			"error 3",
		},
		Warnings: []string{
			"warning 1",
		},
		Suggestions: []string{
			"suggestion 1",
			"suggestion 2",
		},
	}

	output := service.FormatResult(result)

	if !contains(output, "error 1") {
		t.Error("FormatResult() missing error 1")
	}
	if !contains(output, "error 2") {
		t.Error("FormatResult() missing error 2")
	}
	if !contains(output, "error 3") {
		t.Error("FormatResult() missing error 3")
	}
	if !contains(output, "warning 1") {
		t.Error("FormatResult() missing warning 1")
	}
	if !contains(output, "suggestion 1") {
		t.Error("FormatResult() missing suggestion 1")
	}
	if !contains(output, "suggestion 2") {
		t.Error("FormatResult() missing suggestion 2")
	}
}

func TestValidateService_FormatResult_DuplicateSuggestions(t *testing.T) {
	service := &ValidateService{}
	result := &ValidationResult{
		Valid:       false,
		ConfigValid: false,
		Errors: []string{
			"error 1",
		},
		Suggestions: []string{
			"suggestion 1",
			"suggestion 1", // Duplicate
			"suggestion 2",
		},
	}

	output := service.FormatResult(result)

	// Count occurrences of "suggestion 1"
	count := 0
	for i := 0; i < len(output); i++ {
		if i+len("suggestion 1") <= len(output) && output[i:i+len("suggestion 1")] == "suggestion 1" {
			count++
		}
	}

	if count > 1 {
		t.Errorf("FormatResult() has duplicate suggestions, count = %d", count)
	}
}
