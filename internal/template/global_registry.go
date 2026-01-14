/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"fmt"
	"sync"

	"github.com/rackerlabs/openCenter-cli/internal/gitops"
)

var (
	// globalRegistry is the singleton template registry instance
	globalRegistry TemplateRegistry
	// registryOnce ensures the registry is initialized only once
	registryOnce sync.Once
	// registryInitErr stores any error that occurred during initialization
	registryInitErr error
)

// GetGlobalRegistry returns the global template registry, initializing it if necessary.
// This function is thread-safe and will only initialize the registry once.
func GetGlobalRegistry() (TemplateRegistry, error) {
	registryOnce.Do(func() {
		globalRegistry = NewInMemoryTemplateRegistry()
		registryInitErr = initializeGlobalRegistry(globalRegistry)
	})

	if registryInitErr != nil {
		return nil, registryInitErr
	}

	return globalRegistry, nil
}

// initializeGlobalRegistry registers all embedded templates into the provided registry
func initializeGlobalRegistry(registry TemplateRegistry) error {
	// Register GitOps templates
	if err := RegisterGitOpsTemplates(registry, gitops.Files); err != nil {
		return fmt.Errorf("failed to register gitops templates: %w", err)
	}

	// Note: provision templates use an unexported embed.FS variable (templatesFS)
	// so they cannot be registered here. They are already parsed into provision.Templates
	// and can be accessed directly through that package.

	return nil
}

// ResetGlobalRegistry resets the global registry for testing purposes.
// This should only be used in tests.
func ResetGlobalRegistry() {
	globalRegistry = nil
	registryInitErr = nil
	registryOnce = sync.Once{}
}
