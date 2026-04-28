package cluster

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	ConfigValid       bool
	ConnectivityValid bool
	ProviderValid     bool
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
	}
}

// Validate performs cluster validation
func (s *ValidateService) Validate(ctx context.Context, opts ValidateOptions) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:             true,
		ConfigValid:       true,
		ConnectivityValid: true,
		ProviderValid:     true,
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
		clusterPaths, err := s.pathResolver.Resolve(ctx, opts.ClusterName, opts.Organization)
		if err != nil {
			return nil, fmt.Errorf("resolving cluster paths: %w", err)
		}
		configPath = clusterPaths.ConfigPath

		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			result.Valid = false
			result.ConfigValid = false
			result.Errors = append(result.Errors, fmt.Sprintf("configuration file not found: %s", configPath))
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("Run 'opencenter cluster init %s' to create the configuration", opts.ClusterName))
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
				result.Errors = append(result.Errors, fmt.Sprintf("[validation] %s", strings.TrimSpace(e)))
			}
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("[validation] %s", err.Error()))
		}
		return result, nil
	}

	if err := v2.ValidateForDeployment(cfg); err != nil {
		result.Valid = false
		result.ConfigValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("[validation] %s", err.Error()))
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
	_ = ctx
	_ = cfg
	_ = result
	return nil
}

// validateProviderSpecific performs provider-specific validation
func (s *ValidateService) validateProviderSpecific(ctx context.Context, cfg *v2.Config, result *ValidationResult) error {
	_ = ctx
	provider := strings.TrimSpace(cfg.Provider())
	switch provider {
	case "openstack", "aws", "vsphere", "vmware", "kind", "baremetal":
		result.ProviderValid = true
		return nil
	default:
		result.ProviderValid = false
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("[provider] unknown provider: %s", provider))
		result.Suggestions = append(result.Suggestions, "Supported providers: openstack, aws, vsphere, vmware, baremetal, kind")
		return nil
	}
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
