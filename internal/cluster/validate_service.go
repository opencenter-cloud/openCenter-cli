package cluster

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	openstackcloud "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// ValidateOptions contains options for cluster validation
type ValidateOptions struct {
	ClusterName         string
	Organization        string
	ConfigPath          string // Optional: direct path to config file
	CheckConnectivity   bool
	CheckProvider       bool
	GenerateDebugConfig bool
	OutputDir           string
	Verbose             bool
	OutputFormat        string // "text" (default) or "json"
	Provider            string // Active provider for filtering irrelevant errors
}

// ValidationResult contains the result of cluster validation
type ValidationResult struct {
	Valid             bool
	Errors            []string
	Warnings          []string
	Suggestions       []string
	Issues            []v2.ValidationIssue
	ConfigValid       bool
	ConnectivityValid bool
	ProviderValid     bool
	Provider          string
	SchemaVersion     string // normalized schema identifier
	DebugConfigPath   string // Path to generated debug config (if requested)
}

// ValidateService handles cluster validation business logic
type ValidateService struct {
	pathResolver          *paths.PathResolver
	validationEngine      *validation.ValidationEngine
	connectivityValidator *config.ConnectivityValidator
	configManager         *config.ConfigManager
	configurationMgr      *config.ConfigurationManager
	fileSystem            fs.FileSystem
	openStackDiscovery    openstackcloud.DiscoveryClient
}

// NewValidateService creates a new ValidateService
func NewValidateService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
) *ValidateService {
	return NewValidateServiceWithConfigMgr(pathResolver, validationEngine, configManager, nil, nil)
}

// NewValidateServiceWithConfigMgr creates a new ValidateService with optional ConfigurationManager
func NewValidateServiceWithConfigMgr(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
	configurationMgr *config.ConfigurationManager,
	fileSystem fs.FileSystem,
) *ValidateService {
	// Create ConfigurationManager if not provided
	if configurationMgr == nil {
		// Try to create one, but don't fail if it doesn't work
		configurationMgr, _ = config.NewConfigurationManager()
	}

	// Create FileSystem if not provided
	if fileSystem == nil {
		errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
		fileSystem = fs.NewDefaultFileSystem(errorHandler)
	}

	return &ValidateService{
		pathResolver:          pathResolver,
		validationEngine:      validationEngine,
		connectivityValidator: config.NewConnectivityValidator(10 * time.Second),
		configManager:         configManager,
		configurationMgr:      configurationMgr,
		fileSystem:            fileSystem,
		openStackDiscovery:    openstackcloud.NewDiscoveryClient(),
	}
}

// Validate performs cluster validation
func (s *ValidateService) Validate(ctx context.Context, opts ValidateOptions) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:             true,
		ConfigValid:       true,
		ConnectivityValid: true,
		ProviderValid:     true,
		Provider:          strings.TrimSpace(opts.Provider),
	}

	var configPath string

	// Determine config path
	if opts.ConfigPath != "" {
		// Direct config file path provided
		configPath = opts.ConfigPath
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			result.Valid = false
			result.ConfigValid = false
			result.Errors = append(result.Errors, fmt.Sprintf("configuration file not found: %s", configPath))
			return result, nil
		}
	} else {
		// Resolve paths from cluster name
		clusterName, organization := normalizeClusterIdentifier(opts.ClusterName, opts.Organization)
		var (
			clusterPaths *paths.ClusterPaths
			err          error
		)
		if organization == "" {
			clusterPaths, err = s.pathResolver.ResolveWithFallback(ctx, clusterName)
		} else {
			clusterPaths, err = s.pathResolver.Resolve(ctx, clusterName, organization)
		}
		if err != nil {
			return nil, fmt.Errorf("resolving cluster paths: %w", err)
		}
		configPath = clusterPaths.ConfigPath

		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			result.Valid = false
			result.ConfigValid = false
			result.Errors = append(result.Errors, fmt.Sprintf("configuration file not found: %s", configPath))
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("Run 'opencenter cluster init %s' to create the configuration", clusterName))
			return result, nil
		}
	}

	result.SchemaVersion = "v2"
	return s.validateV2Config(ctx, configPath, opts, result)
}

// validateV2Config validates a v2 configuration
func (s *ValidateService) validateV2Config(ctx context.Context, configPath string, opts ValidateOptions, result *ValidationResult) (*ValidationResult, error) {
	// Create v2 loader with default registry
	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	// Load and validate v2 configuration
	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		result.Valid = false
		result.ConfigValid = false

		// Split YAML type errors into individual entries for readable output
		var yamlTypeErrs *v2.YAMLTypeErrors
		if stderrors.As(err, &yamlTypeErrs) {
			for _, e := range yamlTypeErrs.Errors {
				result.addIssue(v2.ValidationIssue{
					Severity: v2.SeverityError,
					Category: v2.CategorySchema,
					Message:  strings.TrimSpace(e),
				})
			}
		} else {
			result.addIssue(v2.ValidationIssue{
				Severity: v2.SeverityError,
				Category: v2.CategorySchema,
				Message:  err.Error(),
			})
		}
		return result, nil
	}

	result.Provider = strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)

	report := v2.ValidateReadiness(cfg)
	for _, issue := range report.Issues {
		result.addIssue(issue)
	}

	if opts.CheckProvider {
		s.validateProviderSpecific(ctx, cfg, result)
	} else if opts.CheckConnectivity {
		s.validateConnectivity(ctx, cfg, result)
	}

	// Generate debug config if requested
	if opts.GenerateDebugConfig || os.Getenv("OPENCENTER_DEBUG") != "" {
		outputDir := opts.OutputDir
		if outputDir == "" {
			outputDir = "."
		}

		// Export effective configuration with applied defaults
		effectiveConfig, err := loader.ExportEffectiveConfig(cfg)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to export effective config: %v", err))
		} else {
			debugPath := filepath.Join(outputDir, ".opencenter-v2.yaml")
			if err := s.fileSystem.WriteFile(debugPath, effectiveConfig, 0600); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to save debug config: %v", err))
			} else {
				result.DebugConfigPath = debugPath
			}
		}
	}

	return result, nil
}

// validateConnectivity checks connectivity to required services
func (s *ValidateService) validateConnectivity(ctx context.Context, cfg *v2.Config, result *ValidationResult) error {
	if !strings.EqualFold(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider), "openstack") {
		return nil
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		return nil
	}
	authURL := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
	if authURL == "" {
		result.addIssue(v2.ValidationIssue{
			Severity:   v2.SeverityError,
			Category:   v2.CategoryConnectivity,
			Path:       "opencenter.infrastructure.cloud.openstack.auth_url",
			Message:    "OpenStack auth URL is required for connectivity checks.",
			Suggestion: "Set opencenter.infrastructure.cloud.openstack.auth_url.",
		})
		return nil
	}
	parsed, err := url.Parse(authURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		result.addIssue(v2.ValidationIssue{
			Severity: v2.SeverityError,
			Category: v2.CategoryConnectivity,
			Path:     "opencenter.infrastructure.cloud.openstack.auth_url",
			Message:  "OpenStack auth URL is not a valid absolute URL.",
		})
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, authURL, nil)
	if err != nil {
		result.addIssue(v2.ValidationIssue{
			Severity: v2.SeverityError,
			Category: v2.CategoryConnectivity,
			Path:     "opencenter.infrastructure.cloud.openstack.auth_url",
			Message:  fmt.Sprintf("failed to create OpenStack connectivity request: %v", err),
		})
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.addIssue(v2.ValidationIssue{
			Severity:   v2.SeverityError,
			Category:   v2.CategoryConnectivity,
			Path:       "opencenter.infrastructure.cloud.openstack.auth_url",
			Message:    fmt.Sprintf("failed to reach OpenStack auth URL: %v", err),
			Suggestion: "Verify network access and the Keystone endpoint URL.",
		})
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		result.addIssue(v2.ValidationIssue{
			Severity: v2.SeverityError,
			Category: v2.CategoryConnectivity,
			Path:     "opencenter.infrastructure.cloud.openstack.auth_url",
			Message:  fmt.Sprintf("OpenStack auth URL returned server status %d.", resp.StatusCode),
		})
	}
	return nil
}

// validateProviderSpecific performs provider-specific validation
func (s *ValidateService) validateProviderSpecific(ctx context.Context, cfg *v2.Config, result *ValidationResult) error {
	provider := strings.TrimSpace(cfg.Provider())
	switch provider {
	case "openstack":
		s.validateOpenStackCatalog(ctx, cfg, result)
		return nil
	case "aws", "vsphere", "vmware", "kind", "baremetal", "gcp", "azure":
		return nil
	default:
		result.addIssue(v2.ValidationIssue{
			Severity:   v2.SeverityError,
			Category:   v2.CategoryProvider,
			Path:       "opencenter.infrastructure.provider",
			Message:    fmt.Sprintf("unknown provider: %s", provider),
			Suggestion: "Supported providers: openstack, aws, gcp, azure, vsphere, vmware, baremetal, kind",
		})
		return nil
	}
}

func (s *ValidateService) validateOpenStackCatalog(ctx context.Context, cfg *v2.Config, result *ValidationResult) {
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		result.addIssue(v2.ValidationIssue{
			Severity:   v2.SeverityError,
			Category:   v2.CategoryProvider,
			Path:       "opencenter.infrastructure.cloud.openstack",
			Message:    "openstack provider requires openstack cloud configuration.",
			Suggestion: "Add opencenter.infrastructure.cloud.openstack.",
		})
		return
	}
	if s.openStackDiscovery == nil {
		s.openStackDiscovery = openstackcloud.NewDiscoveryClient()
	}
	catalog, err := s.openStackDiscovery.Discover(ctx, cfg)
	if err != nil {
		result.addIssue(v2.ValidationIssue{
			Severity:   v2.SeverityError,
			Category:   v2.CategoryProvider,
			Path:       "opencenter.infrastructure.cloud.openstack",
			Message:    fmt.Sprintf("failed to discover OpenStack catalog: %v", err),
			Suggestion: "Verify OpenStack credentials, auth URL, project, and region.",
		})
		return
	}
	if catalog == nil {
		result.addIssue(v2.ValidationIssue{
			Severity: v2.SeverityError,
			Category: v2.CategoryProvider,
			Path:     "opencenter.infrastructure.cloud.openstack",
			Message:  "OpenStack discovery returned an empty catalog.",
		})
		return
	}

	osCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	compute := cfg.OpenCenter.Infrastructure.Compute

	checkCatalogItem(result, catalog.Images, osCfg.ImageID, "opencenter.infrastructure.cloud.openstack.image_id", "OpenStack image")
	if strings.TrimSpace(osCfg.ImageName) != "" {
		checkCatalogItem(result, catalog.Images, osCfg.ImageName, "opencenter.infrastructure.cloud.openstack.image_name", "OpenStack image")
	}
	if compute.WorkerCountWindows > 0 && strings.TrimSpace(osCfg.ImageIDWindows) != "" {
		checkCatalogItem(result, catalog.Images, osCfg.ImageIDWindows, "opencenter.infrastructure.cloud.openstack.image_id_windows", "OpenStack Windows image")
	}

	if compute.MasterCount > 0 {
		checkCatalogItem(result, catalog.Flavors, compute.FlavorMaster, "opencenter.infrastructure.compute.flavor_master", "OpenStack master flavor")
	}
	if compute.WorkerCount > 0 {
		checkCatalogItem(result, catalog.Flavors, compute.FlavorWorker, "opencenter.infrastructure.compute.flavor_worker", "OpenStack worker flavor")
	}
	if compute.WorkerCountWindows > 0 {
		checkCatalogItem(result, catalog.Flavors, compute.FlavorWorkerWindows, "opencenter.infrastructure.compute.flavor_worker_windows", "OpenStack Windows worker flavor")
	}
	if cfg.OpenCenter.Infrastructure.Bastion.Enabled {
		checkCatalogItem(result, catalog.Flavors, compute.FlavorBastion, "opencenter.infrastructure.compute.flavor_bastion", "OpenStack bastion flavor")
	}
	for i, pool := range compute.AdditionalServerPoolsWorker {
		if pool.Count > 0 {
			checkCatalogItem(result, catalog.Flavors, pool.Flavor, fmt.Sprintf("opencenter.infrastructure.compute.additional_server_pools_worker[%d].flavor", i), "OpenStack worker pool flavor")
		}
		if strings.TrimSpace(pool.Image) != "" {
			checkCatalogItem(result, catalog.Images, pool.Image, fmt.Sprintf("opencenter.infrastructure.compute.additional_server_pools_worker[%d].image", i), "OpenStack worker pool image")
		}
	}

	checkCatalogItem(result, catalog.Networks, osCfg.NetworkID, "opencenter.infrastructure.cloud.openstack.network_id", "OpenStack network")
	if strings.TrimSpace(osCfg.NetworkName) != "" {
		checkCatalogItem(result, catalog.Networks, osCfg.NetworkName, "opencenter.infrastructure.cloud.openstack.network_name", "OpenStack network")
	}
	checkCatalogItem(result, catalog.Subnets, osCfg.SubnetID, "opencenter.infrastructure.cloud.openstack.subnet_id", "OpenStack subnet")
	checkCatalogItem(result, catalog.ExternalNetworks, osCfg.FloatingNetworkID, "opencenter.infrastructure.cloud.openstack.floating_network_id", "OpenStack external network")
	checkCatalogItem(result, catalog.ExternalNetworks, osCfg.RouterExternalNetworkID, "opencenter.infrastructure.cloud.openstack.router_external_network_id", "OpenStack router external network")
	if strings.TrimSpace(osCfg.ExternalNetworkName) != "" {
		checkCatalogItem(result, catalog.ExternalNetworks, osCfg.ExternalNetworkName, "opencenter.infrastructure.cloud.openstack.external_network_name", "OpenStack external network")
	}
	if strings.TrimSpace(osCfg.FloatingIPPool) != "" {
		checkCatalogItem(result, catalog.ExternalNetworks, osCfg.FloatingIPPool, "opencenter.infrastructure.cloud.openstack.floating_ip_pool", "OpenStack floating IP pool")
	}
	if strings.TrimSpace(osCfg.AvailabilityZone) != "" {
		checkCatalogItem(result, catalog.AvailabilityZones, osCfg.AvailabilityZone, "opencenter.infrastructure.cloud.openstack.availability_zone", "OpenStack availability zone")
	}
	for i, az := range osCfg.AvailabilityZones {
		checkCatalogItem(result, catalog.AvailabilityZones, az, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.availability_zones[%d]", i), "OpenStack availability zone")
	}
	if (osCfg.UseDesignate || cfg.OpenCenter.Infrastructure.Networking.UseDesignate) && !catalog.DesignateAvailable {
		result.addIssue(v2.ValidationIssue{
			Severity:   v2.SeverityError,
			Category:   v2.CategoryProvider,
			Path:       "opencenter.infrastructure.cloud.openstack.use_designate",
			Message:    "OpenStack Designate is enabled in config but was not discovered in the service catalog.",
			Suggestion: "Disable Designate usage or enable the DNS service in OpenStack.",
		})
	}
}

func checkCatalogItem(result *ValidationResult, items []openstackcloud.CatalogItem, value, path, label string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.ID), value) || strings.EqualFold(strings.TrimSpace(item.Name), value) {
			return
		}
	}
	result.addIssue(v2.ValidationIssue{
		Severity:   v2.SeverityError,
		Category:   v2.CategoryProvider,
		Path:       path,
		Message:    fmt.Sprintf("%s %q was not found in the OpenStack catalog.", label, value),
		Suggestion: "Run OpenStack discovery or update the config with an existing resource.",
	})
}

func (result *ValidationResult) addIssue(issue v2.ValidationIssue) {
	result.Issues = append(result.Issues, issue)
	if issue.Suggestion != "" {
		result.Suggestions = append(result.Suggestions, issue.Suggestion)
	}

	text := issue.Message
	if issue.Path != "" {
		text = fmt.Sprintf("%s — %s", issue.Path, issue.Message)
	}
	if issue.Severity == v2.SeverityWarning {
		result.Warnings = append(result.Warnings, text)
		return
	}

	result.Valid = false
	switch issue.Category {
	case v2.CategoryConnectivity:
		result.ConnectivityValid = false
	case v2.CategoryProvider:
		result.ProviderValid = false
	default:
		result.ConfigValid = false
	}
	result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s", issue.Category, text))
}

func normalizeClusterIdentifier(clusterName, organization string) (string, string) {
	if strings.Contains(clusterName, "/") {
		parts := strings.SplitN(clusterName, "/", 2)
		if organization == "" {
			organization = parts[0]
		}
		clusterName = parts[1]
	}
	return clusterName, organization
}

// FormatResult formats the validation result for display
func (s *ValidateService) FormatResult(result *ValidationResult) string {
	var output strings.Builder

	if result.Valid {
		output.WriteString("✓ Validation successful\n")

		// Show validation details
		output.WriteString("\nValidation Details:\n")
		output.WriteString(fmt.Sprintf("  Configuration: %s\n", formatStatus(result.ConfigValid)))
		output.WriteString(fmt.Sprintf("  Connectivity:  %s\n", formatStatus(result.ConnectivityValid)))
		output.WriteString(fmt.Sprintf("  Provider:      %s\n", formatStatus(result.ProviderValid)))

		// Show warnings if any
		if len(result.Warnings) > 0 {
			output.WriteString("\nWarnings:\n")
			for _, warning := range result.Warnings {
				output.WriteString(fmt.Sprintf("  ⚠ %s\n", warning))
			}
		}

		return output.String()
	}

	// Validation failed
	output.WriteString("✗ Validation failed\n")

	// Show validation details
	output.WriteString("\nValidation Details:\n")
	output.WriteString(fmt.Sprintf("  Configuration: %s\n", formatStatus(result.ConfigValid)))
	output.WriteString(fmt.Sprintf("  Connectivity:  %s\n", formatStatus(result.ConnectivityValid)))
	output.WriteString(fmt.Sprintf("  Provider:      %s\n", formatStatus(result.ProviderValid)))

	// Show errors
	if len(result.Errors) > 0 {
		output.WriteString("\nErrors:\n")
		for _, err := range result.Errors {
			output.WriteString(fmt.Sprintf("  ✗ %s\n", err))
		}
	}

	// Show warnings
	if len(result.Warnings) > 0 {
		output.WriteString("\nWarnings:\n")
		for _, warning := range result.Warnings {
			output.WriteString(fmt.Sprintf("  ⚠ %s\n", warning))
		}
	}

	// Show suggestions
	if len(result.Suggestions) > 0 {
		output.WriteString("\nSuggestions:\n")
		// Deduplicate suggestions
		seen := make(map[string]bool)
		for _, suggestion := range result.Suggestions {
			if suggestion != "" && !seen[suggestion] {
				seen[suggestion] = true
				output.WriteString(fmt.Sprintf("  → %s\n", suggestion))
			}
		}
	}

	return output.String()
}

// formatStatus formats a boolean status as a colored string
func formatStatus(valid bool) string {
	if valid {
		return "✓ Valid"
	}
	return "✗ Invalid"
}
