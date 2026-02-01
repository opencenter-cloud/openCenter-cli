// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package di

import (
	"github.com/sirupsen/logrus"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/rackerlabs/opencenter-cli/internal/ui"
)

// ProvideLogger creates a new logger instance.
// Requirements: 19.2
func ProvideLogger() (*logrus.Logger, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return logger, nil
}

// ProvidePathResolver creates a new PathResolver instance.
// Requirements: 19.2, 1.1
func ProvidePathResolver(baseDir string) (*paths.PathResolver, error) {
	return paths.NewPathResolver(baseDir), nil
}

// ProvideConfigManager creates a new ConfigManager instance.
// Requirements: 19.2, 1.3
func ProvideConfigManager() (*config.ConfigManager, error) {
	return config.NewConfigManager("")
}

// ProvideValidationEngine creates a new ValidationEngine with registered validators.
// Requirements: 19.2, 1.2
func ProvideValidationEngine() (*validation.ValidationEngine, error) {
	engine := validation.NewValidationEngine()

	// Register core validators
	if err := engine.Register(validators.NewClusterNameValidator()); err != nil {
		return nil, err
	}
	if err := engine.Register(validators.NewConfigValidator()); err != nil {
		return nil, err
	}
	if err := engine.Register(validators.NewFileValidator()); err != nil {
		return nil, err
	}
	if err := engine.Register(validators.NewSecurityValidator()); err != nil {
		return nil, err
	}

	return engine, nil
}

// ProvideErrorFormatter creates a new ErrorFormatter instance.
// Requirements: 19.2
func ProvideErrorFormatter() (ui.ErrorFormatter, error) {
	return ui.NewDefaultErrorFormatter(), nil
}

// ProvideInitService creates a new InitService with dependencies.
// Requirements: 19.2, 2.1.2
func ProvideInitService(
	pathResolver *paths.PathResolver,
	validator *validation.ValidationEngine,
	configManager *config.ConfigManager,
) (*cluster.InitService, error) {
	return cluster.NewInitService(pathResolver, validator, configManager), nil
}

// ProvideValidateService creates a new ValidateService with dependencies.
// Requirements: 19.2, 2.1.3
func ProvideValidateService(
	pathResolver *paths.PathResolver,
	validator *validation.ValidationEngine,
	configManager *config.ConfigManager,
) (*cluster.ValidateService, error) {
	return cluster.NewValidateService(pathResolver, validator, configManager), nil
}

// ProvideSetupService creates a new SetupService with dependencies.
// Requirements: 19.2, 2.1.4
func ProvideSetupService(
	pathResolver *paths.PathResolver,
	validator *validation.ValidationEngine,
) (*cluster.SetupService, error) {
	return cluster.NewSetupService(pathResolver, validator), nil
}

// ProvideBootstrapService creates a new BootstrapService with dependencies.
// Requirements: 19.2, 2.1.5
func ProvideBootstrapService(
	pathResolver *paths.PathResolver,
	validator *validation.ValidationEngine,
) (*cluster.BootstrapService, error) {
	return cluster.NewBootstrapService(pathResolver, validator), nil
}
