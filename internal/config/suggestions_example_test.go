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
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// ExampleSuggestionEngine demonstrates how to use the suggestion engine.
func ExampleSuggestionEngine() {
	engine := config.NewSuggestionEngine()

	// Get suggestions for a cluster name field
	suggestions := engine.GetSuggestionsForField("cluster_name", "invalid@name")
	fmt.Println("Cluster name suggestions:")
	for _, s := range suggestions {
		fmt.Printf("  - %s\n", s)
	}

	// Output:
	// Cluster name suggestions:
	//   - Use alphanumeric characters, hyphens, and underscores only
	//   - Start with an alphanumeric character
	//   - Keep length under 255 characters
	//   - Example: 'my-cluster-prod'
}

// ExampleSuggestionEngine_contextAware demonstrates context-aware suggestions.
func ExampleSuggestionEngine_contextAware() {
	engine := config.NewSuggestionEngine()

	// Get suggestions for a password field (context-aware)
	suggestions := engine.GetSuggestionsForField("admin_password", "secret123")
	fmt.Println("Password field suggestions:")
	for i, s := range suggestions {
		if i < 3 { // Show first 3 suggestions
			fmt.Printf("  - %s\n", s)
		}
	}

	// Output:
	// Password field suggestions:
	//   - Use SOPS to encrypt sensitive credentials
	//   - Generate strong passwords with sufficient entropy
	//   - Never commit plaintext passwords to version control
}

// ExampleSuggestionEngine_formatting demonstrates suggestion formatting.
func ExampleSuggestionEngine_formatting() {
	engine := config.NewSuggestionEngine()

	suggestions := []string{
		"Set cluster_name to a valid value",
		"Use alphanumeric characters only",
		"Keep length under 255 characters",
	}

	formatted := engine.FormatSuggestions(suggestions)
	fmt.Print(formatted)

	// Output:
	// Suggestions:
	//   1. Set cluster_name to a valid value
	//   2. Use alphanumeric characters only
	//   3. Keep length under 255 characters
}

// ExampleClusterConfigValidator_withSuggestions demonstrates validation with suggestions.
func ExampleClusterConfigValidator_withSuggestions() {
	validator := config.NewConfigValidator(false)

	// Create a config with an invalid cluster name
	cfg := &config.Config{
		SchemaVersion: "v2.0.0",
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "", // Invalid: empty cluster name
				Organization: "test-org",
			},
			GitOps: config.GitOpsConfig{
				GitDir: "", // Invalid: empty git dir
			},
		},
	}

	ctx := context.Background()
	result := validator.Validate(ctx, cfg)

	if !result.Valid {
		fmt.Println("Validation failed with errors:")
		// Show only first 2 errors for brevity
		for i, err := range result.Errors {
			if i >= 2 {
				break
			}
			fmt.Printf("\nField: %s\n", err.Field)
			fmt.Printf("Message: %s\n", err.Message)
			if len(err.Suggestions) > 0 {
				fmt.Println("Suggestions:")
				for j, suggestion := range err.Suggestions {
					if j < 2 { // Show first 2 suggestions
						fmt.Printf("  %d. %s\n", j+1, suggestion)
					}
				}
			}
		}
	}

	// Output:
	// Validation failed with errors:
	//
	// Field: opencenter.cluster.cluster_name
	// Message: cluster name must be set
	// Suggestions:
	//   1. Set opencenter.cluster.cluster_name to a valid cluster name
	//   2. Cluster name should be alphanumeric with hyphens and underscores
	//
	// Field: opencenter.gitops.git_dir
	// Message: GitOps directory must be set
	// Suggestions:
	//   1. Set opencenter.gitops.git_dir to a valid directory path
	//   2. Use a path where GitOps repository will be created
}
