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

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/ui"
)

// SetupContainer creates and configures a new DI container with all major components.
// Requirements: 19.2
// Note: This registers the core components that are currently implemented.
// Additional components will be registered as they are implemented in other phases.
func SetupContainer() (Container, error) {
	container := NewContainer()

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
	if err := container.Initialize(); err != nil {
		return nil, err
	}

	return container, nil
}
