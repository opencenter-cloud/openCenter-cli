package di

import (
	"fmt"
	"path/filepath"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/sirupsen/logrus"
)

// App is the typed runtime graph used by the CLI critical path.
type App struct {
	BaseDir string

	ErrorHandler errors.ErrorHandler
	FileSystem   fs.FileSystem
	PathResolver *paths.PathResolver
	Logger       *logrus.Logger

	ConfigManager    *config.ConfigManager
	ValidationEngine *validation.ValidationEngine
	ErrorFormatter   ui.ErrorFormatter

	AuditLogger      *security.AuditLogger
	InputValidator   security.InputValidator
	CredentialMasker security.CredentialMasker
	CommandSanitizer security.CommandSanitizer
	CommandRunner    security.CommandRunner

	InitService      *cluster.InitService
	ConfigureService *cluster.ConfigureService
	ValidateService  *cluster.ValidateService
	SetupService     *cluster.SetupService
	BootstrapService *cluster.BootstrapService
}

// NewApp builds the core application graph using explicit constructor chaining.
func NewApp(baseDir string) (*App, error) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	pathResolver, err := ProvidePathResolver(baseDir)
	if err != nil {
		return nil, fmt.Errorf("provide PathResolver: %w", err)
	}

	logger, err := ProvideLogger()
	if err != nil {
		return nil, fmt.Errorf("provide logger: %w", err)
	}

	configManager, err := config.NewConfigManager(deriveCLIConfigPath(baseDir))
	if err != nil {
		return nil, fmt.Errorf("provide ConfigManager: %w", err)
	}

	validationEngine, err := ProvideValidationEngine()
	if err != nil {
		return nil, fmt.Errorf("provide ValidationEngine: %w", err)
	}

	errorFormatter, err := ProvideErrorFormatter()
	if err != nil {
		return nil, fmt.Errorf("provide ErrorFormatter: %w", err)
	}

	auditLogger, err := ProvideAuditLogger()
	if err != nil {
		return nil, fmt.Errorf("provide AuditLogger: %w", err)
	}

	inputValidator, err := ProvideInputValidator(auditLogger)
	if err != nil {
		return nil, fmt.Errorf("provide InputValidator: %w", err)
	}

	credentialMasker, err := ProvideCredentialMasker()
	if err != nil {
		return nil, fmt.Errorf("provide CredentialMasker: %w", err)
	}

	commandSanitizer, err := ProvideCommandSanitizer()
	if err != nil {
		return nil, fmt.Errorf("provide CommandSanitizer: %w", err)
	}

	commandRunner, err := ProvideCommandRunner(commandSanitizer)
	if err != nil {
		return nil, fmt.Errorf("provide CommandRunner: %w", err)
	}

	initService, err := ProvideInitService(pathResolver, validationEngine, configManager)
	if err != nil {
		return nil, fmt.Errorf("provide InitService: %w", err)
	}

	validateService, err := ProvideValidateService(pathResolver, validationEngine, configManager)
	if err != nil {
		return nil, fmt.Errorf("provide ValidateService: %w", err)
	}

	configureService, err := ProvideConfigureService(pathResolver, validationEngine, configManager)
	if err != nil {
		return nil, fmt.Errorf("provide ConfigureService: %w", err)
	}

	setupService, err := ProvideSetupService(pathResolver, validationEngine)
	if err != nil {
		return nil, fmt.Errorf("provide SetupService: %w", err)
	}

	bootstrapService, err := ProvideBootstrapService(pathResolver, validationEngine)
	if err != nil {
		return nil, fmt.Errorf("provide BootstrapService: %w", err)
	}

	return &App{
		BaseDir:          baseDir,
		ErrorHandler:     errorHandler,
		FileSystem:       fileSystem,
		PathResolver:     pathResolver,
		Logger:           logger,
		ConfigManager:    configManager,
		ValidationEngine: validationEngine,
		ErrorFormatter:   errorFormatter,
		AuditLogger:      auditLogger,
		InputValidator:   inputValidator,
		CredentialMasker: credentialMasker,
		CommandSanitizer: commandSanitizer,
		CommandRunner:    commandRunner,
		InitService:      initService,
		ConfigureService: configureService,
		ValidateService:  validateService,
		SetupService:     setupService,
		BootstrapService: bootstrapService,
	}, nil
}

func deriveCLIConfigPath(baseDir string) string {
	cleanBaseDir := filepath.Clean(baseDir)
	if filepath.Base(cleanBaseDir) == "clusters" {
		return filepath.Join(filepath.Dir(cleanBaseDir), "config.yaml")
	}
	return filepath.Join(cleanBaseDir, "config.yaml")
}
