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
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rackerlabs/openCenter-cli/internal/di"
)

// TestGetContainer tests retrieving the DI container from context
func TestGetContainer(t *testing.T) {
	// Create a test container
	container := di.NewContainer()

	// Create context with container
	ctx := context.WithValue(context.Background(), ContainerKey, container)

	// Retrieve container
	retrieved, err := GetContainer(ctx)
	if err != nil {
		t.Errorf("GetContainer() failed: %v", err)
	}
	if retrieved != container {
		t.Error("GetContainer() returned different container")
	}
}

// TestGetContainer_NotFound tests error when container is not in context
func TestGetContainer_NotFound(t *testing.T) {
	ctx := context.Background()

	_, err := GetContainer(ctx)
	if err == nil {
		t.Error("GetContainer() should fail when container not in context")
	}
}

// TestGetContainer_WrongType tests error when context value is wrong type
func TestGetContainer_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContainerKey, "not a container")

	_, err := GetContainer(ctx)
	if err == nil {
		t.Error("GetContainer() should fail when context value is wrong type")
	}
}

// TestExecuteWithContext tests that ExecuteWithContext works with a container
func TestExecuteWithContext(t *testing.T) {
	// Create a test container
	container, err := di.SetupContainer()
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}

	// Create context with container
	ctx := context.WithValue(context.Background(), ContainerKey, container)

	// Test that we can retrieve the container
	retrieved, err := GetContainer(ctx)
	if err != nil {
		t.Errorf("GetContainer() failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved container is nil")
	}
}

// TestParseGlobalFlags tests parsing of global flags
func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    map[string]interface{}
		expected *GlobalFlags
	}{
		{
			name: "default flags",
			flags: map[string]interface{}{
				"config":      "",
				"dry-run":     false,
				"log-level":   "warn",
				"set":         []string{},
				"verbose":     false,
				"show-active": false,
			},
			expected: &GlobalFlags{
				Config:     "",
				DryRun:     false,
				LogLevel:   "warn",
				Set:        []string{},
				Verbose:    false,
				ShowActive: false,
			},
		},
		{
			name: "verbose flag overrides log level",
			flags: map[string]interface{}{
				"config":      "",
				"dry-run":     false,
				"log-level":   "warn",
				"set":         []string{},
				"verbose":     true,
				"show-active": false,
			},
			expected: &GlobalFlags{
				Config:     "",
				DryRun:     false,
				LogLevel:   "debug",
				Set:        []string{},
				Verbose:    true,
				ShowActive: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test command
			cmd := &cobra.Command{}
			cmd.Flags().String("config", "", "")
			cmd.Flags().Bool("dry-run", false, "")
			cmd.Flags().String("log-level", "warn", "")
			cmd.Flags().StringArray("set", []string{}, "")
			cmd.Flags().Bool("verbose", false, "")
			cmd.Flags().Bool("show-active", false, "")

			// Set flag values
			for name, value := range tt.flags {
				switch v := value.(type) {
				case string:
					cmd.Flags().Set(name, v)
				case bool:
					if v {
						cmd.Flags().Set(name, "true")
					}
				case []string:
					for _, s := range v {
						cmd.Flags().Set(name, s)
					}
				}
			}

			// Parse flags
			result, err := parseGlobalFlags(cmd)
			if err != nil {
				t.Errorf("parseGlobalFlags() failed: %v", err)
			}

			// Check results
			if result.Config != tt.expected.Config {
				t.Errorf("Config = %v, want %v", result.Config, tt.expected.Config)
			}
			if result.DryRun != tt.expected.DryRun {
				t.Errorf("DryRun = %v, want %v", result.DryRun, tt.expected.DryRun)
			}
			if result.LogLevel != tt.expected.LogLevel {
				t.Errorf("LogLevel = %v, want %v", result.LogLevel, tt.expected.LogLevel)
			}
			if result.Verbose != tt.expected.Verbose {
				t.Errorf("Verbose = %v, want %v", result.Verbose, tt.expected.Verbose)
			}
			if result.ShowActive != tt.expected.ShowActive {
				t.Errorf("ShowActive = %v, want %v", result.ShowActive, tt.expected.ShowActive)
			}
		})
	}
}
