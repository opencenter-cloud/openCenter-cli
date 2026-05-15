package config

import (
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// ConfigurationManager re-exported from v2.
type ConfigurationManager = v2.ConfigurationManager

// NewConfigurationManagerWithDeps re-exported from v2.
var NewConfigurationManagerWithDeps = v2.NewConfigurationManagerWithDeps

// NewConfigurationManager creates a ConfigurationManager with default dependencies
// resolved from CLI configuration.
func NewConfigurationManager() (*ConfigurationManager, error) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := NewPathResolverFromConfig()
	validator := validation.NewValidationEngine()

	return v2.NewConfigurationManagerWithDeps(
		v2.NewConfigIOHandler(fileSystem),
		validator,
		v2.NewConfigCache(),
		pathResolver,
		fileSystem,
	), nil
}
