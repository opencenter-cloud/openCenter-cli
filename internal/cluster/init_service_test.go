package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
)

// setupValidationEngine creates a validation engine with required validators
func setupValidationEngine(t *testing.T) *validation.ValidationEngine {
	t.Helper()
	engine := validation.NewValidationEngine()

	if err := engine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("Failed to register cluster validator: %v", err)
	}

	if err := engine.Register(validators.NewOrganizationNameValidator()); err != nil {
		t.Fatalf("Failed to register organization validator: %v", err)
	}

	return engine
}

func TestInitServiceInitializeUsesSecureLayoutAndHygiene(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	initService := NewInitService(pathResolver, validationEngine, configManager)

	result, err := initService.Initialize(context.Background(), InitOptions{
		ClusterName:  "secure-demo",
		Organization: "acme",
		Provider:     "kind",
		NoGitInit:    false,
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	clusterPaths := result.ClusterPaths
	wantConfigPath := filepath.Join(tmpDir, "blueprints", "acme", "secure-demo", "secure-demo-config.yaml")
	if result.ConfigPath != wantConfigPath {
		t.Fatalf("ConfigPath = %q, want %q", result.ConfigPath, wantConfigPath)
	}

	for _, path := range []string{clusterPaths.ConfigPath, clusterPaths.SOPSKeyPath, clusterPaths.SSHKeyPath} {
		rel, err := filepath.Rel(clusterPaths.GitOpsDir, path)
		if err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("%s must not be inside GitOps dir %s", path, clusterPaths.GitOpsDir)
		}
	}

	for _, path := range []string{
		filepath.Join(clusterPaths.GitOpsDir, ".gitignore"),
		filepath.Join(clusterPaths.GitOpsDir, ".sops.yaml"),
		filepath.Join(clusterPaths.GitOpsDir, ".opencenter", "hooks", "pre-commit"),
		filepath.Join(clusterPaths.GitOpsDir, ".opencenter", "scripts", "scan-secrets"),
		filepath.Join(clusterPaths.GitOpsDir, ".github", "workflows", "opencenter-secret-scan.yml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected GitOps hygiene file %s: %v", path, err)
		}
	}

	sopsConfig, err := os.ReadFile(filepath.Join(clusterPaths.GitOpsDir, ".sops.yaml"))
	if err != nil {
		t.Fatalf("read .sops.yaml: %v", err)
	}
	sopsText := string(sopsConfig)
	if strings.Contains(sopsText, `encrypted_regex: "^(secret)$"`) {
		t.Fatalf(".sops.yaml contains obsolete Secret regex:\n%s", sopsText)
	}
	if !strings.Contains(sopsText, `encrypted_regex: "^(data|stringData)$"`) {
		t.Fatalf(".sops.yaml is missing data|stringData regex:\n%s", sopsText)
	}
	if !strings.Contains(sopsText, `encrypted_regex: "^(data|stringData|secret)$"`) {
		t.Fatalf(".sops.yaml is missing infrastructure regex:\n%s", sopsText)
	}
	for _, rule := range strings.Split(sopsText, "  - path_regex: ")[1:] {
		if strings.Contains(rule, "secrets/age/") || strings.Contains(rule, "secrets/ssh/") {
			if strings.Contains(rule, "encrypted_regex:") {
				t.Fatalf("key rule must encrypt the whole file:\n%s", rule)
			}
		}
	}

	gitignoreData, err := os.ReadFile(filepath.Join(clusterPaths.GitOpsDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if strings.Contains(string(gitignoreData), "\n*-config.yaml") {
		t.Fatalf(".gitignore must not ignore nested rendered config manifests:\n%s", string(gitignoreData))
	}
	for _, want := range []string{"\n/*-config.yaml", "\n/.*-config.yaml"} {
		if !strings.Contains(string(gitignoreData), want) {
			t.Fatalf(".gitignore missing root-only config guard %q:\n%s", want, string(gitignoreData))
		}
	}

	assertMode := func(path string, want os.FileMode) {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("mode %s = %#o, want %#o", path, got, want)
		}
	}
	assertMode(clusterPaths.ClusterStateDir, 0o700)
	assertMode(clusterPaths.SecretsDir, 0o700)
	assertMode(clusterPaths.ConfigPath, 0o600)
	assertMode(clusterPaths.SOPSKeyPath, 0o600)
	assertMode(clusterPaths.SSHKeyPath, 0o600)
}

func TestInitService_Initialize(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver with test directory
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine with validators
	validationEngine := setupValidationEngine(t)

	// Create config manager
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create init service
	initService := NewInitService(pathResolver, validationEngine, configManager)

	tests := []struct {
		name    string
		opts    InitOptions
		wantErr bool
		setup   func() // Setup function to prepare test environment
	}{
		{
			name: "successful initialization",
			opts: InitOptions{
				ClusterName:  "test-cluster",
				Organization: "test-org",
				Provider:     "openstack",
				NoKeyGen:     true, // Skip key generation for faster test
				NoGitInit:    true, // Skip git init for faster test
			},
			wantErr: false,
			// No setup needed - Initialize should create directories
		},
		{
			name: "invalid cluster name",
			opts: InitOptions{
				ClusterName:  "INVALID_NAME",
				Organization: "test-org",
				Provider:     "openstack",
				NoKeyGen:     true,
				NoGitInit:    true,
			},
			wantErr: true,
		},
		{
			name: "empty cluster name",
			opts: InitOptions{
				ClusterName:  "",
				Organization: "test-org",
				Provider:     "openstack",
				NoKeyGen:     true,
				NoGitInit:    true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup if provided
			if tt.setup != nil {
				tt.setup()
			}

			ctx := context.Background()
			result, err := initService.Initialize(ctx, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("Initialize() returned nil result")
					return
				}

				if result.Config == nil {
					t.Error("Initialize() returned nil config")
				}

				if result.ClusterPaths == nil {
					t.Error("Initialize() returned nil cluster paths")
				}

				if result.ConfigPath == "" {
					t.Error("Initialize() returned empty config path")
				}
			}
		})
	}
}

func TestInitService_validateClusterName(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine with validators
	validationEngine := setupValidationEngine(t)

	// Create config manager
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create init service
	initService := NewInitService(pathResolver, validationEngine, configManager)

	tests := []struct {
		name        string
		clusterName string
		wantErr     bool
	}{
		{
			name:        "valid cluster name",
			clusterName: "test-cluster",
			wantErr:     false,
		},
		{
			name:        "valid cluster name with numbers",
			clusterName: "test-cluster-123",
			wantErr:     false,
		},
		{
			name:        "invalid cluster name with uppercase",
			clusterName: "Test-Cluster",
			wantErr:     true,
		},
		{
			name:        "invalid cluster name with underscore",
			clusterName: "test_cluster",
			wantErr:     true,
		},
		{
			name:        "invalid cluster name with slash",
			clusterName: "test/cluster",
			wantErr:     true,
		},
		{
			name:        "invalid cluster name with path traversal",
			clusterName: "../test-cluster",
			wantErr:     true,
		},
		{
			name:        "empty cluster name",
			clusterName: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := initService.validateClusterName(ctx, tt.clusterName)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateClusterName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitService_createDefaultConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := setupValidationEngine(t)

	// Create init service
	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	tests := []struct {
		name    string
		opts    InitOptions
		wantErr bool
	}{
		{
			name: "create default config",
			opts: InitOptions{
				ClusterName:  "test-cluster",
				Organization: "test-org",
				Provider:     "openstack",
			},
			wantErr: false,
		},
		{
			name: "create default config with empty organization",
			opts: InitOptions{
				ClusterName:  "test-cluster",
				Organization: "",
				Provider:     "openstack",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _, err := initService.createDefaultConfig(tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("createDefaultConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg.SchemaVersion == "" {
					t.Error("createDefaultConfig() returned empty config")
					return
				}

				if cfg.SchemaVersion != "2.0" {
					t.Errorf("createDefaultConfig() schema version = %v, want 2.0", cfg.SchemaVersion)
				}

				expectedOrg := tt.opts.Organization
				if expectedOrg == "" {
					expectedOrg = "opencenter"
				}
				if cfg.OpenCenter.Meta.Organization != expectedOrg {
					t.Errorf("createDefaultConfig() organization = %v, want %v", cfg.OpenCenter.Meta.Organization, expectedOrg)
				}

				if tt.opts.Provider != "" && cfg.OpenCenter.Infrastructure.Provider != tt.opts.Provider {
					t.Errorf("createDefaultConfig() provider = %v, want %v", cfg.OpenCenter.Infrastructure.Provider, tt.opts.Provider)
				}

				if cfg.OpenCenter.Meta.Stage != v2.StageInit {
					t.Errorf("createDefaultConfig() stage = %v, want %v", cfg.OpenCenter.Meta.Stage, v2.StageInit)
				}

				if cfg.OpenCenter.Meta.Status != v2.StatusSuccess {
					t.Errorf("createDefaultConfig() status = %v, want %v", cfg.OpenCenter.Meta.Status, v2.StatusSuccess)
				}
			}
		})
	}
}

// TestInitServiceApplyOverridesPreservesExplicitSJC3Region guards against a
// regression of a real bug: applyOverrides used to special-case the literal
// string "sjc3" (case-insensitively) as if it were an unset/legacy
// placeholder needing replacement by the CLI's configured default region.
// That treated a genuine, explicitly-configured SJC3 region identically to an
// actually-unset one, silently clobbering it (observed replacing "SJC3" with
// "DFW3" on a real cluster's --config-file init). Only a truly empty region
// may be filled from the default.
func TestInitServiceApplyOverridesPreservesExplicitSJC3Region(t *testing.T) {
	t.Setenv("OPENCENTER_CONFIG_DIR", t.TempDir())
	pathResolver := paths.NewPathResolver(t.TempDir())
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	initService := NewInitService(pathResolver, setupValidationEngine(t), configManager)
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Region = "SJC3"
	configMap := map[string]any{
		"opencenter": map[string]any{
			"meta": map[string]any{},
		},
	}

	if err := initService.applyOverrides(cfg, configMap, InitOptions{}); err != nil {
		t.Fatalf("applyOverrides() error = %v", err)
	}
	if cfg.OpenCenter.Meta.Region != "SJC3" {
		t.Fatalf("Meta.Region = %q, want unchanged SJC3", cfg.OpenCenter.Meta.Region)
	}
}

// TestInitServiceApplyOverridesFillsEmptyRegionFromDefault verifies a
// genuinely empty region still gets filled from the CLI's configured
// default — only the SJC3 special-case above was wrong, not the general
// empty-region fallback.
func TestInitServiceApplyOverridesFillsEmptyRegionFromDefault(t *testing.T) {
	t.Setenv("OPENCENTER_CONFIG_DIR", t.TempDir())
	pathResolver := paths.NewPathResolver(t.TempDir())
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	initService := NewInitService(pathResolver, setupValidationEngine(t), configManager)
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Region = ""
	configMap := map[string]any{
		"opencenter": map[string]any{
			"meta": map[string]any{},
		},
	}

	if err := initService.applyOverrides(cfg, configMap, InitOptions{}); err != nil {
		t.Fatalf("applyOverrides() error = %v", err)
	}
	if cfg.OpenCenter.Meta.Region != "DFW3" {
		t.Fatalf("Meta.Region = %q, want default DFW3", cfg.OpenCenter.Meta.Region)
	}
}

// TestInitServiceApplyOverridesRequiresExplicitOrganization guards against a
// real bug: applyOverrides used to overwrite Meta.Organization whenever
// opts.Organization was non-empty, but Initialize() force-defaults
// opts.Organization to "opencenter" before applyOverrides ever runs — so a
// --config-file's own meta.organization was always clobbered, even when the
// user never passed --org. Only an explicit organization (OrganizationExplicit)
// may override an already-parsed config value.
func TestInitServiceApplyOverridesRequiresExplicitOrganization(t *testing.T) {
	t.Setenv("OPENCENTER_CONFIG_DIR", t.TempDir())
	pathResolver := paths.NewPathResolver(t.TempDir())
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	initService := NewInitService(pathResolver, setupValidationEngine(t), configManager)
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Organization = "sjc3cli9-cluster-gitops"
	configMap := map[string]any{
		"opencenter": map[string]any{
			"meta": map[string]any{},
		},
	}

	// Organization is set (mirroring Initialize()'s forced default) but
	// OrganizationExplicit is false: the config file's value must survive.
	opts := InitOptions{Organization: "opencenter", OrganizationExplicit: false}
	if err := initService.applyOverrides(cfg, configMap, opts); err != nil {
		t.Fatalf("applyOverrides() error = %v", err)
	}
	if cfg.OpenCenter.Meta.Organization != "sjc3cli9-cluster-gitops" {
		t.Fatalf("Meta.Organization = %q, want unchanged sjc3cli9-cluster-gitops", cfg.OpenCenter.Meta.Organization)
	}
}

// TestInitServiceApplyOverridesHonorsExplicitOrganization verifies an
// explicitly-provided --org still correctly overrides whatever the config
// file parsed — only the non-explicit (defaulted) case must not clobber.
func TestInitServiceApplyOverridesHonorsExplicitOrganization(t *testing.T) {
	t.Setenv("OPENCENTER_CONFIG_DIR", t.TempDir())
	pathResolver := paths.NewPathResolver(t.TempDir())
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	initService := NewInitService(pathResolver, setupValidationEngine(t), configManager)
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Organization = "sjc3cli9-cluster-gitops"
	configMap := map[string]any{
		"opencenter": map[string]any{
			"meta": map[string]any{},
		},
	}

	opts := InitOptions{Organization: "explicit-org", OrganizationExplicit: true}
	if err := initService.applyOverrides(cfg, configMap, opts); err != nil {
		t.Fatalf("applyOverrides() error = %v", err)
	}
	if cfg.OpenCenter.Meta.Organization != "explicit-org" {
		t.Fatalf("Meta.Organization = %q, want explicit-org", cfg.OpenCenter.Meta.Organization)
	}
}

func TestInitService_generateKeys(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := setupValidationEngine(t)

	// Create init service
	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	// Create cluster paths
	ctx := context.Background()

	// Create cluster directories first
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Failed to resolve cluster paths: %v", err)
	}

	// Test key generation
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Region = "test-region"
	cfg.Secrets.SSHKey.Cypher = "ed25519"
	cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
	cfg.Secrets.SOPSConfig.Enabled = true
	cfg.Secrets.SOPSConfig.AgeKeyFile = clusterPaths.SOPSKeyPath

	opts := InitOptions{
		ClusterName:  "test-cluster",
		Organization: "test-org",
	}
	keysGenerated, err := initService.generateKeys(clusterPaths, cfg, opts)
	if err != nil {
		t.Errorf("generateKeys() error = %v", err)
		return
	}
	if !keysGenerated {
		t.Error("generateKeys() returned false, expected true")
	}

	// Verify SOPS key was created
	if _, err := os.Stat(clusterPaths.SOPSKeyPath); os.IsNotExist(err) {
		t.Errorf("SOPS key file was not created at %s", clusterPaths.SOPSKeyPath)
	}

	// Verify SSH key was created
	if _, err := os.Stat(clusterPaths.SSHKeyPath); os.IsNotExist(err) {
		t.Errorf("SSH private key file was not created at %s", clusterPaths.SSHKeyPath)
	}

	// Verify SSH public key was created
	sshPubKeyPath := clusterPaths.SSHKeyPath + ".pub"
	if _, err := os.Stat(sshPubKeyPath); os.IsNotExist(err) {
		t.Errorf("SSH public key file was not created at %s", sshPubKeyPath)
	}

	// Verify file permissions
	info, err := os.Stat(clusterPaths.SSHKeyPath)
	if err != nil {
		t.Errorf("Failed to stat SSH private key: %v", err)
	} else {
		mode := info.Mode()
		if mode.Perm() != 0o600 {
			t.Errorf("SSH private key has incorrect permissions: %v, want 0600", mode.Perm())
		}
	}
}

func TestInitService_initGitRepo(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := setupValidationEngine(t)

	// Create init service
	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	// Create cluster paths
	ctx := context.Background()

	// Create cluster directories first
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Failed to resolve cluster paths: %v", err)
	}

	// Test git initialization
	err = initService.initGitRepo(clusterPaths)
	if err != nil {
		t.Errorf("initGitRepo() error = %v", err)
		return
	}

	// Verify GitOps directory was created (initGitRepo creates the directory)
	if _, err := os.Stat(clusterPaths.GitOpsDir); os.IsNotExist(err) {
		t.Errorf("GitOps directory was not created at %s", clusterPaths.GitOpsDir)
	}

	// Note: The actual .git directory creation is handled by the command layer
	// which has access to the cobra command for output. The service just ensures
	// the GitOps directory exists.
}

func TestInitService_Initialize_WithKeyGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)

	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	ctx := context.Background()

	opts := InitOptions{
		ClusterName:  "test-cluster",
		Organization: "test-org",
		Provider:     "openstack",
		NoKeyGen:     false, // Enable key generation
		NoGitInit:    true,
	}

	result, err := initService.Initialize(ctx, opts)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if !result.KeysGenerated {
		t.Error("Initialize() did not generate keys")
	}

	// Verify keys were created
	if _, err := os.Stat(result.ClusterPaths.SOPSKeyPath); os.IsNotExist(err) {
		t.Error("SOPS key was not created")
	}
	if _, err := os.Stat(result.ClusterPaths.SSHKeyPath); os.IsNotExist(err) {
		t.Error("SSH key was not created")
	}
}

func TestInitService_Initialize_WithGitInit(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)

	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	ctx := context.Background()

	opts := InitOptions{
		ClusterName:  "test-cluster",
		Organization: "test-org",
		Provider:     "openstack",
		NoKeyGen:     true,
		NoGitInit:    false, // Enable git init
	}

	result, err := initService.Initialize(ctx, opts)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if !result.GitInitialized {
		t.Error("Initialize() did not initialize git")
	}

	// Verify GitOps directory was created
	if _, err := os.Stat(result.ClusterPaths.GitOpsDir); os.IsNotExist(err) {
		t.Error("GitOps directory was not created")
	}
}

func TestInitService_Initialize_DifferentProviders(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)

	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	providers := []string{"openstack", "aws", "kind", "vsphere"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			clusterName := "test-cluster-" + provider
			ctx := context.Background()

			opts := InitOptions{
				ClusterName:  clusterName,
				Organization: "test-org",
				Provider:     provider,
				NoKeyGen:     true,
				NoGitInit:    true,
			}

			result, err := initService.Initialize(ctx, opts)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			if result.Config.OpenCenter.Infrastructure.Provider != provider {
				t.Errorf("Provider = %v, want %v", result.Config.OpenCenter.Infrastructure.Provider, provider)
			}
		})
	}
}

func TestInitService_Initialize_KindDisableDefaultCNI(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)

	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	enabled := true
	result, err := initService.Initialize(context.Background(), InitOptions{
		ClusterName:           "kind-cni",
		Organization:          "test-org",
		Provider:              "kind",
		NoKeyGen:              true,
		NoGitInit:             true,
		KindDisableDefaultCNI: &enabled,
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if result.Config.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected native v2 kind compatibility config to be present")
	}
	if !result.Config.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be true in native v2 config")
	}
}

func TestInitService_Initialize_RejectsKindDisableDefaultCNIForNonKind(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)

	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	enabled := true
	_, err := initService.Initialize(context.Background(), InitOptions{
		ClusterName:           "openstack-cni",
		Organization:          "test-org",
		Provider:              "openstack",
		NoKeyGen:              true,
		NoGitInit:             true,
		KindDisableDefaultCNI: &enabled,
	})
	if err == nil {
		t.Fatal("expected Initialize() to reject kind disable_default_cni for non-kind providers")
	}
}

func TestInitService_generateSOPSKey(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)
	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Failed to resolve cluster paths: %v", err)
	}

	err = initService.generateSOPSKey(clusterPaths)
	if err != nil {
		t.Fatalf("generateSOPSKey() error = %v", err)
	}

	// Verify key file exists
	if _, err := os.Stat(clusterPaths.SOPSKeyPath); os.IsNotExist(err) {
		t.Error("SOPS key file was not created")
	}

	// Verify key file has content
	content, err := os.ReadFile(clusterPaths.SOPSKeyPath)
	if err != nil {
		t.Fatalf("Failed to read SOPS key: %v", err)
	}
	if len(content) == 0 {
		t.Error("SOPS key file is empty")
	}
}

func TestInitService_generateSSHKey(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)
	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Failed to resolve cluster paths: %v", err)
	}

	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Region = "test-region"
	cfg.Secrets.SSHKey.Cypher = "ed25519"

	err = initService.generateSSHKey(clusterPaths, cfg)
	if err != nil {
		t.Fatalf("generateSSHKey() error = %v", err)
	}

	// Verify private key exists
	if _, err := os.Stat(clusterPaths.SSHKeyPath); os.IsNotExist(err) {
		t.Error("SSH private key was not created")
	}

	// Verify public key exists
	pubKeyPath := clusterPaths.SSHKeyPath + ".pub"
	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		t.Error("SSH public key was not created")
	}

	// Verify private key permissions
	info, err := os.Stat(clusterPaths.SSHKeyPath)
	if err != nil {
		t.Fatalf("Failed to stat private key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("Private key permissions = %v, want 0600", info.Mode().Perm())
	}

	// Verify public key permissions
	pubInfo, err := os.Stat(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to stat public key: %v", err)
	}
	if pubInfo.Mode().Perm() != 0o644 {
		t.Errorf("Public key permissions = %v, want 0644", pubInfo.Mode().Perm())
	}
}

func TestInitService_createDefaultConfig_EmptyOrganization(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)
	configManager, _ := config.NewConfigManager("")
	initService := NewInitService(pathResolver, validationEngine, configManager)

	opts := InitOptions{
		ClusterName:  "test-cluster",
		Organization: "", // Empty organization
		Provider:     "openstack",
	}

	cfg, _, err := initService.createDefaultConfig(opts)
	if err != nil {
		t.Fatalf("createDefaultConfig() error = %v", err)
	}

	// Should default to "opencenter"
	if cfg.OpenCenter.Meta.Organization != "opencenter" {
		t.Errorf("Organization = %v, want opencenter", cfg.OpenCenter.Meta.Organization)
	}
}

func TestInitService_NewInitService(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)

	configManager, _ := config.NewConfigManager("")
	service := NewInitService(pathResolver, validationEngine, configManager)

	if service == nil {
		t.Fatal("NewInitService returned nil")
	}

	if service.pathResolver == nil {
		t.Error("pathResolver is nil")
	}

	if service.validationEngine == nil {
		t.Error("validationEngine is nil")
	}
}

func TestInitServiceLoadOrCreateConfigUsesStrictPublicDecode(t *testing.T) {
	initService := newInitServiceForPublicDecodeTest(t)
	configPath := filepath.Join(t.TempDir(), "input.yaml")
	configData := []byte(`schema_version: "2.0"
opencenter:
  meta:
    name: init-config
    organization: acme
    env: dev
    region: dfw3
  services:
    olm:
      enabled: true
      override_values_renderer: legacy
`)
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = writer
	cfg, configMap, decodeErr := initService.loadOrCreateConfig(InitOptions{ConfigFile: configPath})
	_ = writer.Close()
	os.Stderr = oldStderr
	warningOutput, _ := io.ReadAll(reader)
	_ = reader.Close()

	if decodeErr != nil {
		t.Fatalf("loadOrCreateConfig() error = %v", decodeErr)
	}
	if cfg.OpenCenter.Meta.Name != "init-config" {
		t.Fatalf("decoded name = %q, want init-config", cfg.OpenCenter.Meta.Name)
	}
	if strings.Contains(string(warningOutput), "opencenter.services.olm.override_values_renderer is deprecated") == false {
		t.Fatalf("warning output = %q, want legacy metadata warning", warningOutput)
	}
	if strings.Contains(fmt.Sprintf("%v", configMap), "override_values_renderer") {
		t.Fatalf("config map contains legacy metadata: %#v", configMap)
	}
}

func TestInitServiceLoadOrCreateConfigRejectsUnknownServiceKey(t *testing.T) {
	initService := newInitServiceForPublicDecodeTest(t)
	configPath := filepath.Join(t.TempDir(), "input.yaml")
	configData := []byte(`schema_version: "2.0"
opencenter:
  services:
    olm:
      enabled: true
      unknown_service_key: must-fail
`)
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, _, err := initService.loadOrCreateConfig(InitOptions{ConfigFile: configPath}); err == nil || !strings.Contains(err.Error(), "unknown_service_key") {
		t.Fatalf("loadOrCreateConfig() error = %v, want unknown service key rejection", err)
	}
}

func newInitServiceForPublicDecodeTest(t *testing.T) *InitService {
	t.Helper()
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("create config manager: %v", err)
	}
	return NewInitService(paths.NewPathResolver(t.TempDir()), setupValidationEngine(t), configManager)
}
