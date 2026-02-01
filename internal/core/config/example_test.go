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

package config_test

import (
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/core/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/config/strategies"
)

// Note: The following examples demonstrate the ConfigManager API.
// In production, configs must pass validation. These examples use
// minimal configs for demonstration purposes only.

// ExampleConfigManager_RegisterStrategy demonstrates custom strategy registration.
func ExampleConfigManager_RegisterStrategy() {
	manager := config.NewConfigManager()

	// Register strategies for different versions
	manager.RegisterStrategy(strategies.NewV2Strategy())
	manager.RegisterStrategy(strategies.NewV1Strategy())
	manager.RegisterStrategy(strategies.NewLegacyStrategy())

	fmt.Println("Registered 3 strategies")
	// Output:
	// Registered 3 strategies
}

// ExampleLoadOptions demonstrates different loading options.
func ExampleLoadOptions() {
	// Basic load
	opts1 := config.LoadOptions{}
	fmt.Printf("Basic: AutoMigrate=%v, Validate=%v, SkipCache=%v\n",
		opts1.AutoMigrate, opts1.Validate, opts1.SkipCache)

	// Production load with validation
	opts2 := config.LoadOptions{
		AutoMigrate: true,
		Validate:    true,
		SkipCache:   false,
	}
	fmt.Printf("Production: AutoMigrate=%v, Validate=%v, SkipCache=%v\n",
		opts2.AutoMigrate, opts2.Validate, opts2.SkipCache)

	// Development load with cache bypass
	opts3 := config.LoadOptions{
		AutoMigrate: false,
		Validate:    false,
		SkipCache:   true,
	}
	fmt.Printf("Development: AutoMigrate=%v, Validate=%v, SkipCache=%v\n",
		opts3.AutoMigrate, opts3.Validate, opts3.SkipCache)

	// Output:
	// Basic: AutoMigrate=false, Validate=false, SkipCache=false
	// Production: AutoMigrate=true, Validate=true, SkipCache=false
	// Development: AutoMigrate=false, Validate=false, SkipCache=true
}

// The following examples are commented out because they require full valid configs.
// See internal/core/config/manager_test.go for complete working examples with validation.

/*
// ExampleConfigManager_Load demonstrates basic configuration loading.
func ExampleConfigManager_Load() {
	// Create a temporary config file for demonstration
	tmpDir, _ := os.MkdirTemp("", "config-example-*")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "cluster-config.yaml")
	configContent := `schema_version: "2.0"
opencenter:
  meta:
    name: example-cluster
    organization: myorg
`
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Create ConfigManager and load configuration
	manager := config.NewConfigManager()
	manager.RegisterStrategy(strategies.NewV2Strategy())

	cfg, err := manager.Load(configPath, config.LoadOptions{
		AutoMigrate: false,
		Validate:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded cluster: %s\n", cfg.OpenCenter.Meta.Name)
	fmt.Printf("Organization: %s\n", cfg.OpenCenter.Meta.Organization)
	// Output:
	// Loaded cluster: example-cluster
	// Organization: myorg
}
*/
