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

	"github.com/opencenter-cloud/opencenter-cli/internal/di"
)

func TestGetApp(t *testing.T) {
	tempDir := t.TempDir()
	prepareCommandTestEnv(t, tempDir)

	app, err := di.NewApp(tempDir)
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}

	ctx := context.WithValue(context.Background(), AppKey, app)
	retrieved, err := GetApp(ctx)
	if err != nil {
		t.Fatalf("GetApp() failed: %v", err)
	}
	if retrieved != app {
		t.Fatal("GetApp() returned a different app instance")
	}
}

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
	// Create a test container with temp directory
	tempDir := t.TempDir()
	prepareCommandTestEnv(t, tempDir)
	container, err := di.SetupContainer(tempDir)
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

// TestParseGlobalOptionsFromFlags tests parsing of global flags.
func TestParseGlobalOptionsFromFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    map[string]interface{}
		expected GlobalOptions
	}{
		{
			name: "default flags",
			flags: map[string]interface{}{
				"config-dir": "",
				"dry-run":    false,
				"log-level":  "warn",
				"output":     "text",
				"quiet":      false,
				"yes":        false,
			},
			expected: GlobalOptions{
				ConfigDir: "",
				DryRun:    false,
				LogLevel:  "warn",
				Output:    OutputText,
				Quiet:     false,
				Yes:       false,
			},
		},
		{
			name: "debug log level",
			flags: map[string]interface{}{
				"config-dir": "/tmp/opencenter",
				"dry-run":    true,
				"log-level":  "debug",
				"output":     "json",
				"quiet":      true,
				"yes":        true,
			},
			expected: GlobalOptions{
				ConfigDir: "/tmp/opencenter",
				DryRun:    true,
				LogLevel:  "debug",
				Output:    OutputJSON,
				Quiet:     true,
				Yes:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test command
			cmd := &cobra.Command{}
			addGlobalFlags(cmd)

			// Set flag values
			for name, value := range tt.flags {
				switch v := value.(type) {
				case string:
					if err := cmd.PersistentFlags().Set(name, v); err != nil {
						t.Fatalf("Set(%q) failed: %v", name, err)
					}
				case bool:
					if v {
						if err := cmd.PersistentFlags().Set(name, "true"); err != nil {
							t.Fatalf("Set(%q) failed: %v", name, err)
						}
					}
				case []string:
					for _, s := range v {
						if err := cmd.PersistentFlags().Set(name, s); err != nil {
							t.Fatalf("Set(%q) failed: %v", name, err)
						}
					}
				}
			}

			// Parse flags
			result, err := parseGlobalOptions(cmd)
			if err != nil {
				t.Errorf("parseGlobalOptions() failed: %v", err)
			}

			if result.ConfigDir != tt.expected.ConfigDir {
				t.Errorf("ConfigDir = %v, want %v", result.ConfigDir, tt.expected.ConfigDir)
			}
			if result.DryRun != tt.expected.DryRun {
				t.Errorf("DryRun = %v, want %v", result.DryRun, tt.expected.DryRun)
			}
			if result.LogLevel != tt.expected.LogLevel {
				t.Errorf("LogLevel = %v, want %v", result.LogLevel, tt.expected.LogLevel)
			}
			if result.Output != tt.expected.Output {
				t.Errorf("Output = %v, want %v", result.Output, tt.expected.Output)
			}
			if result.Quiet != tt.expected.Quiet {
				t.Errorf("Quiet = %v, want %v", result.Quiet, tt.expected.Quiet)
			}
			if result.Yes != tt.expected.Yes {
				t.Errorf("Yes = %v, want %v", result.Yes, tt.expected.Yes)
			}
		})
	}
}
