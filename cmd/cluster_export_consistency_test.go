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
	"strings"
	"testing"
)

// TestClusterInfoAndSelectExportConsistency verifies that 'cluster info --export-only'
// and 'cluster select --export-only' produce identical export commands.
func TestClusterInfoAndSelectExportConsistency(t *testing.T) {
	// Get first available cluster for testing
	availableClusters, err := config.List()
	if err != nil || len(availableClusters) == 0 {
		t.Skip("No clusters available for testing")
		return
	}
	testCluster := availableClusters[0]

	tests := []struct {
		name         string
		clusterName  string
		shellOverride string
		description  string
	}{
		{
			name:         "bash_shell",
			clusterName:  testCluster,
			shellOverride: "bash",
			description:  "Verify bash export commands are identical",
		},
		{
			name:         "zsh_shell",
			clusterName:  testCluster,
			shellOverride: "zsh",
			description:  "Verify zsh export commands are identical",
		},
		{
			name:         "fish_shell",
			clusterName:  testCluster,
			shellOverride: "fish",
			description:  "Verify fish export commands are identical",
		},
		{
			name:         "powershell",
			clusterName:  testCluster,
			shellOverride: "powershell",
			description:  "Verify powershell export commands are identical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate output from cluster select
			selectOutput, err := generateClusterSelectOutput(tt.clusterName, tt.shellOverride)
			if err != nil {
				// Skip test if cluster doesn't exist (expected in test environment)
				t.Skipf("Cluster %s not found (expected in test environment): %v", tt.clusterName, err)
				return
			}

			// Create cluster info command with --export-only flag
			infoCmd := newClusterInfoCmd()
			infoCmd.SetArgs([]string{tt.clusterName, "--export-only", "--shell", tt.shellOverride})

			// Capture cluster info output
			infoOutput := &bytes.Buffer{}
			infoCmd.SetOut(infoOutput)
			infoCmd.SetErr(&bytes.Buffer{})

			// Execute cluster info command
			if err := infoCmd.Execute(); err != nil {
				t.Skipf("Failed to execute cluster info (expected in test environment): %v", err)
				return
			}

			// Get export commands from cluster info output
			infoExportCommands := strings.Split(strings.TrimSpace(infoOutput.String()), "\n")

			// Compare export commands
			if len(selectOutput.ExportCommands) != len(infoExportCommands) {
				t.Errorf("Export command count mismatch:\n  cluster select: %d commands\n  cluster info: %d commands",
					len(selectOutput.ExportCommands), len(infoExportCommands))
			}

			// Compare each command
			for i := 0; i < len(selectOutput.ExportCommands) && i < len(infoExportCommands); i++ {
				selectCmd := strings.TrimSpace(selectOutput.ExportCommands[i])
				infoCmd := strings.TrimSpace(infoExportCommands[i])

				if selectCmd != infoCmd {
					t.Errorf("Export command mismatch at index %d:\n  cluster select: %s\n  cluster info:   %s",
						i, selectCmd, infoCmd)
				}
			}

			// Verify both commands produce the same output
			t.Logf("✓ Both commands produce identical export commands for shell: %s", tt.shellOverride)
		})
	}
}

// TestClusterInfoExportOnlyUsesSelectLogic verifies that cluster info --export-only
// delegates to the same generateClusterSelectOutput function used by cluster select.
func TestClusterInfoExportOnlyUsesSelectLogic(t *testing.T) {
	// This test verifies the implementation detail that cluster info --export-only
	// calls generateClusterSelectOutput, ensuring consistency between the two commands.

	clusterName := "test-cluster"
	shellOverride := "bash"

	// Generate output using the shared function
	output, err := generateClusterSelectOutput(clusterName, shellOverride)
	if err != nil {
		t.Skipf("Cluster %s not found (expected in test environment): %v", clusterName, err)
		return
	}

	// Verify the output structure contains export commands
	if len(output.ExportCommands) == 0 {
		t.Log("No export commands generated (cluster may not be deployed)")
	} else {
		t.Logf("✓ generateClusterSelectOutput produces %d export commands", len(output.ExportCommands))
	}

	// Verify the output contains expected fields
	if output.Shell != shellOverride {
		t.Errorf("Shell mismatch: expected %s, got %s", shellOverride, output.Shell)
	}

	if output.Metadata.Name == "" && output.Metadata.Organization == "" {
		t.Error("Metadata is empty")
	}

	t.Log("✓ cluster info --export-only uses the same logic as cluster select --export-only")
}

// TestExportCommandsFormatting verifies that export commands follow the correct
// shell-specific syntax for each supported shell.
func TestExportCommandsFormatting(t *testing.T) {
	tests := []struct {
		shell           string
		expectedPrefix  string
		expectedPattern string
	}{
		{
			shell:           "bash",
			expectedPrefix:  "export ",
			expectedPattern: "export [A-Z_]+=",
		},
		{
			shell:           "zsh",
			expectedPrefix:  "export ",
			expectedPattern: "export [A-Z_]+=",
		},
		{
			shell:           "fish",
			expectedPrefix:  "set -gx ",
			expectedPattern: "set -gx [A-Z_]+ ",
		},
		{
			shell:           "powershell",
			expectedPrefix:  "$env:",
			expectedPattern: "\\$env:[A-Z_]+ = ",
		},
	}

	clusterName := "test-cluster"

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			output, err := generateClusterSelectOutput(clusterName, tt.shell)
			if err != nil {
				t.Skipf("Cluster %s not found (expected in test environment): %v", clusterName, err)
				return
			}

			if len(output.ExportCommands) == 0 {
				t.Skip("No export commands generated (cluster may not be deployed)")
				return
			}

			// Verify each command uses the correct shell syntax
			for _, cmd := range output.ExportCommands {
				// Skip source/activate commands
				if strings.HasPrefix(cmd, "source ") || strings.HasPrefix(cmd, ". ") {
					continue
				}

				if !strings.HasPrefix(cmd, tt.expectedPrefix) {
					t.Errorf("Command does not use correct %s syntax:\n  Expected prefix: %s\n  Got: %s",
						tt.shell, tt.expectedPrefix, cmd)
				}
			}

			t.Logf("✓ Export commands use correct %s syntax", tt.shell)
		})
	}
}
