package di

import (
	"fmt"
	"strings"

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

// NewAppContainer exposes the typed runtime graph behind the legacy Container interface.
func NewAppContainer(app *App) Container {
	return &appContainer{app: app}
}

type appContainer struct {
	app *App
}

func (c *appContainer) Register(name string, constructor interface{}) error {
	return fmt.Errorf("app container is read-only: cannot register %q", name)
}

func (c *appContainer) Resolve(name string) (interface{}, error) {
	if c.app == nil {
		return nil, fmt.Errorf("application graph is not initialized")
	}

	switch canonicalComponentName(name) {
	case "errorhandler":
		return c.app.ErrorHandler, nil
	case "filesystem":
		return c.app.FileSystem, nil
	case "pathresolver":
		return c.app.PathResolver, nil
	case "logger":
		return c.app.Logger, nil
	case "configmanager":
		return c.app.ConfigManager, nil
	case "validationengine":
		return c.app.ValidationEngine, nil
	case "errorformatter":
		return c.app.ErrorFormatter, nil
	case "auditlogger":
		return c.app.AuditLogger, nil
	case "inputvalidator":
		return c.app.InputValidator, nil
	case "credentialmasker":
		return c.app.CredentialMasker, nil
	case "commandsanitizer":
		return c.app.CommandSanitizer, nil
	case "commandrunner":
		return c.app.CommandRunner, nil
	case "initservice":
		return c.app.InitService, nil
	case "validateservice":
		return c.app.ValidateService, nil
	case "setupservice":
		return c.app.SetupService, nil
	case "bootstrapservice":
		return c.app.BootstrapService, nil
	default:
		return nil, fmt.Errorf("component %q is not registered in the typed app container", name)
	}
}

func (c *appContainer) ResolveAs(name string, target interface{}) error {
	if c.app == nil {
		return fmt.Errorf("application graph is not initialized")
	}

	switch t := target.(type) {
	case *errors.ErrorHandler:
		*t = c.app.ErrorHandler
	case *fs.FileSystem:
		*t = c.app.FileSystem
	case **paths.PathResolver:
		*t = c.app.PathResolver
	case **logrus.Logger:
		*t = c.app.Logger
	case **config.ConfigManager:
		*t = c.app.ConfigManager
	case **validation.ValidationEngine:
		*t = c.app.ValidationEngine
	case *ui.ErrorFormatter:
		*t = c.app.ErrorFormatter
	case **security.AuditLogger:
		*t = c.app.AuditLogger
	case *security.InputValidator:
		*t = c.app.InputValidator
	case *security.CredentialMasker:
		*t = c.app.CredentialMasker
	case *security.CommandSanitizer:
		*t = c.app.CommandSanitizer
	case *security.CommandRunner:
		*t = c.app.CommandRunner
	case **cluster.InitService:
		*t = c.app.InitService
	case **cluster.ValidateService:
		*t = c.app.ValidateService
	case **cluster.SetupService:
		*t = c.app.SetupService
	case **cluster.BootstrapService:
		*t = c.app.BootstrapService
	case *interface{}:
		resolved, err := c.Resolve(name)
		if err != nil {
			return err
		}
		*t = resolved
	default:
		return fmt.Errorf("unsupported target type %T for component %q", target, name)
	}

	return nil
}

func (c *appContainer) Singleton(name string, constructor interface{}) error {
	return fmt.Errorf("app container is read-only: cannot register singleton %q", name)
}

func (c *appContainer) Initialize() error {
	return nil
}

func (c *appContainer) Shutdown() error {
	return nil
}

func canonicalComponentName(name string) string {
	return strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(name)))
}
