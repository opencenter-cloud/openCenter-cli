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

package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// TestClusterValidateIntegration tests the cluster validate command end-to-end
func TestClusterValidateIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name          string
		clusterName   string
		setupConfig   bool
		expectError   bool
		errorContains string
	}{
		{
			name:          "validate non-existent cluster",
			clusterName:   "missing-cluster",
			setupConfig:   false,
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := newClusterValidateCmd()
			cmd.SetContext(context.Background())

			// Capture output
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			// Set args
			cmd.SetArgs([]string{tt.clusterName})

			// Execute command
			err := cmd.Execute()

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v\nstderr: %s", err, stderr.String())
			}

			// Check error message
			if tt.expectError && tt.errorContains != "" {
				errMsg := err.Error() + stderr.String()
				if !strings.Contains(errMsg, tt.errorContains) {
					t.Errorf("expected error to contain %q, got: %s", tt.errorContains, errMsg)
				}
			}
		})
	}
}

// TestClusterValidateWithFlags tests the cluster validate command with various flags
func TestClusterValidateWithFlags(t *testing.T) {
	// This test verifies that flags are properly parsed and passed to the service
	// We don't need actual cluster configs for this test

	t.Run("validate help flag", func(t *testing.T) {
		// Create command
		cmd := newClusterValidateCmd()
		cmd.SetContext(context.Background())

		// Capture output
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		// Set args with help flag
		cmd.SetArgs([]string{"--help"})

		// Execute command
		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error with help flag: %v", err)
		}

		// Check that help output contains expected sections
		output := stdout.String()
		if !strings.Contains(output, "Validate cluster configuration") {
			t.Error("expected help output to contain description")
		}
		if !strings.Contains(output, "--check-connectivity") {
			t.Error("expected help output to contain --check-connectivity flag")
		}
		if !strings.Contains(output, "--generate-debug-config") {
			t.Error("expected help output to contain --generate-debug-config flag")
		}
	})
}

// TestClusterValidateCommandStructure tests the command structure and flags
func TestClusterValidateCommandStructure(t *testing.T) {
	cmd := newClusterValidateCmd()

	// Verify command properties
	if cmd.Use != "validate [name]" {
		t.Errorf("expected Use to be 'validate [name]', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}

	// Verify flags exist
	flags := []string{
		"check-connectivity",
		"check-provider",
		"generate-debug-config",
		"output-dir",
		"verbose",
	}

	for _, flagName := range flags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}
}

// TestClusterValidateDIContainer tests that the DI container is properly initialized
func TestClusterValidateDIContainer(t *testing.T) {
	// Get container
	container := getContainer()
	if container == nil {
		t.Fatal("expected container to be initialized")
	}

	// Verify ValidateService can be resolved
	var validateService interface{}
	err := container.ResolveAs("ValidateService", &validateService)
	if err != nil {
		t.Errorf("failed to resolve ValidateService: %v", err)
	}
	if validateService == nil {
		t.Error("expected ValidateService to be non-nil")
	}

	// Verify PathResolver can be resolved
	var pathResolver interface{}
	err = container.ResolveAs("PathResolver", &pathResolver)
	if err != nil {
		t.Errorf("failed to resolve PathResolver: %v", err)
	}
	if pathResolver == nil {
		t.Error("expected PathResolver to be non-nil")
	}

	// Verify ValidationEngine can be resolved
	var validationEngine interface{}
	err = container.ResolveAs("ValidationEngine", &validationEngine)
	if err != nil {
		t.Errorf("failed to resolve ValidationEngine: %v", err)
	}
	if validationEngine == nil {
		t.Error("expected ValidationEngine to be non-nil")
	}
}
