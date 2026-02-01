package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
	v2 "github.com/rackerlabs/opencenter-cli/internal/config/v2"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// ValidateOptions contains options for cluster validation
type ValidateOptions struct {
	ClusterName        string
	Organization       string
	ConfigPath         string // Optional: direct path to config file
	CheckConnectivity  bool
	CheckProvider      bool
	GenerateDebugConfig bool
	OutputDir          string
	Verbose            bool
}

// ValidationResult contains the result of cluster validation
type ValidationResult struct {
	Valid              bool
	Errors             []string
	Warnings           []string
	Suggestions        []string
	ConfigValid        bool
	ConnectivityValid  bool
	ProviderValid      bool
	SchemaVersion      string // v1 or v2
	DebugConfigPath    string // Path to generated debug config (if requested)
}

// ValidateService handles cluster validation business logic
type ValidateService struct {
	pathResolver          *paths.PathResolver
	validationEngine      *validation.ValidationEngine
	connectivityValidator *config.ConnectivityValidator
	configManager         *config.ConfigManager
}

// NewValidateService creates a new ValidateService
func NewValidateService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
) *ValidateService {
	return &ValidateService{
		pathResolver:          pathResolver,
		validationEngine:      validationEngine,
		connectivityValidator: config.NewConnectivityValidator(10 * time.Second),
		configManager:         configManager,
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
	var err error

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

	// Detect schema version
	versionInfo, err := config.DetectSchemaVersionFromFile(configPath)
	if err != nil {
		result.Valid = false
		result.ConfigValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to detect schema version: %v", err))
		result.Suggestions = append(result.Suggestions, "Check that the configuration file is valid YAML")
		return result, nil
	}

	// Set schema version in result
	if versionInfo.IsV2 {
		result.SchemaVersion = "v2"
	} else {
		result.SchemaVersion = "v1"
	}

	// Route to appropriate validator based on version
	if versionInfo.IsV2 {
		return s.validateV2Config(ctx, configPath, opts, result)
	}

	return s.validateV1Config(ctx, opts, result)
}

// validateV1Config validates a v1 configuration
func (s *ValidateService) validateV1Config(ctx context.Context, opts ValidateOptions, result *ValidationResult) (*ValidationResult, error) {
	// Load configuration
	cfg, err := config.Load(opts.ClusterName)
	if err != nil {
		result.Valid = false
		result.ConfigValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to load configuration: %v", err))
		result.Suggestions = append(result.Suggestions, "Check that the configuration file is valid YAML")
		return result, nil
	}

	// Validate configuration using the config validator
	configValidator := config.NewConfigValidator(false)
	configResult := configValidator.Validate(ctx, &cfg)

	result.ConfigValid = configResult.Valid
	if !configResult.Valid {
		result.Valid = false
		// Convert validation errors to string messages
		for _, err := range configResult.Errors {
			if err.Field != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s: %s", err.Type, err.Field, err.Message))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s", err.Type, err.Message))
			}
			// Add suggestions from validation errors
			result.Suggestions = append(result.Suggestions, err.Suggestions...)
		}
		// Add warnings
		for _, warn := range configResult.Warnings {
			if warn.Field != "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("[%s] %s: %s", warn.Type, warn.Field, warn.Message))
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("[%s] %s", warn.Type, warn.Message))
			}
		}
	}

	// Check connectivity if requested
	if opts.CheckConnectivity {
		if err := s.validateConnectivity(ctx, &cfg, result); err != nil {
			return nil, fmt.Errorf("checking connectivity: %w", err)
		}
	}

	// Check provider-specific validation if requested
	if opts.CheckProvider {
		if err := s.validateProviderSpecific(ctx, &cfg, result); err != nil {
			return nil, fmt.Errorf("checking provider: %w", err)
		}
	}

	// Generate debug config if requested
	if opts.GenerateDebugConfig || os.Getenv("OPENCENTER_DEBUG") != "" {
		outputDir := opts.OutputDir
		if outputDir == "" {
			// Use GitOps directory if available, otherwise current directory
			if cfg.GitOps().GitDir != "" {
				outputDir = cfg.GitOps().GitDir
			} else {
				outputDir = "."
			}
		}

		if err := config.SaveDebugConfig(cfg.ClusterName(), outputDir); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to save debug config: %v", err))
		} else {
			result.DebugConfigPath = filepath.Join(outputDir, ".opencenter.yaml")
		}
	}

	// Update status if validation successful
	if result.Valid {
		if err := config.UpdateStatus(opts.ClusterName, config.StageValidate, config.StatusSuccess); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to update cluster status: %v", err))
		}
	}

	return result, nil
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
		result.Errors = append(result.Errors, fmt.Sprintf("[validation] %s", err.Error()))
		return result, nil
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
			if err := os.WriteFile(debugPath, effectiveConfig, 0600); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to save debug config: %v", err))
			} else {
				result.DebugConfigPath = debugPath
			}
		}
	}

	return result, nil
}

// validateConnectivity checks connectivity to required services
func (s *ValidateService) validateConnectivity(ctx context.Context, cfg *config.Config, result *ValidationResult) error {
	// Validate cloud provider connectivity
	connectivityErrors := s.connectivityValidator.ValidateCloudProviderConnectivity(ctx, cfg)
	
	if len(connectivityErrors) > 0 {
		result.ConnectivityValid = false
		result.Valid = false
		
		for _, err := range connectivityErrors {
			if err.Field != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("[connectivity] %s: %s", err.Field, err.Message))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("[connectivity] %s", err.Message))
			}
			
			// Add suggestions from connectivity errors
			result.Suggestions = append(result.Suggestions, err.Suggestions...)
		}
	}
	
	// Validate credential format
	credentialErrors := s.connectivityValidator.ValidateCredentialFormat(ctx, cfg)
	if len(credentialErrors) > 0 {
		result.ConnectivityValid = false
		result.Valid = false
		
		for _, err := range credentialErrors {
			if err.Field != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("[credentials] %s: %s", err.Field, err.Message))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("[credentials] %s", err.Message))
			}
			result.Suggestions = append(result.Suggestions, err.Suggestions...)
		}
	}
	
	// Validate credential security (warnings only)
	securityErrors := s.connectivityValidator.ValidateCredentialSecurity(ctx, cfg)
	if len(securityErrors) > 0 {
		for _, err := range securityErrors {
			if err.Field != "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("[security] %s: %s", err.Field, err.Message))
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("[security] %s", err.Message))
			}
			result.Suggestions = append(result.Suggestions, "Use SOPS to encrypt sensitive credentials")
		}
	}
	
	return nil
}

// validateProviderSpecific performs provider-specific validation
func (s *ValidateService) validateProviderSpecific(ctx context.Context, cfg *config.Config, result *ValidationResult) error {
	provider := cfg.OpenCenter.Infrastructure.Provider
	
	// Get the appropriate provider validator
	var providerErrors []*errors.StructuredError
	
	switch provider {
	case "openstack":
		validator := config.NewOpenStackValidator()
		providerErrors = validator.ValidateConnectivity(ctx, cfg)
		
	case "aws":
		validator := config.NewAWSValidator()
		providerErrors = validator.ValidateConnectivity(ctx, cfg)
		
	case "vsphere":
		validator := config.NewVSphereValidator()
		providerErrors = validator.ValidateConnectivity(ctx, cfg)
		
	case "kind":
		// Kind runs locally, no provider-specific validation needed
		result.ProviderValid = true
		return nil
		
	default:
		result.ProviderValid = false
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("[provider] unknown provider: %s", provider))
		result.Suggestions = append(result.Suggestions, "Supported providers: openstack, aws, vsphere, kind")
		return nil
	}
	
	// Process provider validation errors
	if len(providerErrors) > 0 {
		result.ProviderValid = false
		result.Valid = false
		
		for _, err := range providerErrors {
			if err.Field != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s: %s", provider, err.Field, err.Message))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s", provider, err.Message))
			}
			
			// Add suggestions from provider errors
			result.Suggestions = append(result.Suggestions, err.Suggestions...)
		}
	}
	
	return nil
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
