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
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// SetupContainer creates and configures a new DI container with all major components.
// Requirements: 5.2, 5.3, 5.4, 5.5, 19.2
// Note: This registers the core components that are currently implemented.
// Additional components will be registered as they are implemented in other phases.
func SetupContainer(baseDir string) (Container, error) {
	container := NewContainer()

	// Register ErrorHandler as singleton (no dependencies)
	// Requirements: 5.3
	if err := container.Singleton("ErrorHandler", func() (errors.ErrorHandler, error) {
		return errors.NewDefaultErrorHandlerWithoutMasking(), nil
	}); err != nil {
		return nil, fmt.Errorf("registering ErrorHandler: %w", err)
	}

	// Register FileSystem as singleton (depends on ErrorHandler)
	// Requirements: 5.4
	if err := container.Singleton("FileSystem", func(errorHandler errors.ErrorHandler) (fs.FileSystem, error) {
		return fs.NewDefaultFileSystem(errorHandler), nil
	}); err != nil {
		return nil, fmt.Errorf("registering FileSystem: %w", err)
	}

	// Register PathResolver as singleton (no dependencies for now)
	// Requirements: 5.5
	if err := container.Singleton("PathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(baseDir)
	}); err != nil {
		return nil, fmt.Errorf("registering PathResolver: %w", err)
	}

	// Register Logger as singleton
	if err := container.Singleton("logger", func() (*logrus.Logger, error) {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
		return logger, nil
	}); err != nil {
		return nil, err
	}

	// Register ConfigManager as singleton
	if err := container.Singleton("configManager", func() (*config.ConfigManager, error) {
		return config.NewConfigManager("")
	}); err != nil {
		return nil, err
	}

	// Register ErrorFormatter as singleton
	if err := container.Singleton("errorFormatter", func() (ui.ErrorFormatter, error) {
		return ui.NewDefaultErrorFormatter(), nil
	}); err != nil {
		return nil, err
	}

	if err := container.Singleton("AuditLogger", ProvideAuditLogger); err != nil {
		return nil, err
	}

	if err := container.Singleton("InputValidator", ProvideInputValidator); err != nil {
		return nil, err
	}

	if err := container.Singleton("CredentialMasker", ProvideCredentialMasker); err != nil {
		return nil, err
	}

	if err := container.Singleton("CommandSanitizer", ProvideCommandSanitizer); err != nil {
		return nil, err
	}

	if err := container.Singleton("CommandRunner", func(sanitizer security.CommandSanitizer) (security.CommandRunner, error) {
		return ProvideCommandRunner(sanitizer)
	}); err != nil {
		return nil, err
	}

	// Additional components will be registered here as they are implemented:
	// - GitOpsGenerator (from gitops package)
	// - SOPSManager (from sops package)
	// - MetricsExporter (from observability package - Phase 2)
	// - RetryHandler (from resilience package - Phase 2)
	// - CircuitBreaker (from resilience package - Phase 2)
	// - LockManager (from resilience package - Phase 2)
	// - DriftDetector (from operations package - Phase 2)
	// - BackupManager (from operations package - Phase 2)
	// - InputValidator (from security package - Phase 1)
	// - CredentialMasker (from security package - Phase 1)
	// - CommandSanitizer (from security package - Phase 1)
	// - AuditLogger (from security package - Phase 1)

	// Initialize all singletons
	// Requirements: 5.2
	if err := container.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing container: %w", err)
	}

	return container, nil
}
