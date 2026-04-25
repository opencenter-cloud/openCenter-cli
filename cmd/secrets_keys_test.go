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
	"testing"
)

// TestSecretsKeysCmd_Structure verifies the keys subcommand structure
func TestSecretsKeysCmd_Structure(t *testing.T) {
	cmd := NewSecretsKeysCmd()

	// Verify command exists
	if cmd == nil {
		t.Fatal("NewSecretsKeysCmd() returned nil")
	}

	// Verify command name
	if cmd.Use != "keys" {
		t.Errorf("Expected Use='keys', got %q", cmd.Use)
	}

	// Verify subcommands exist
	expectedSubcommands := []string{"generate", "rotate", "backup", "validate", "check", "revoke"}
	subcommands := cmd.Commands()

	if len(subcommands) != len(expectedSubcommands) {
		t.Errorf("Expected %d subcommands, got %d", len(expectedSubcommands), len(subcommands))
	}

	// Build map of actual subcommands
	actualSubcommands := make(map[string]bool)
	for _, subcmd := range subcommands {
		actualSubcommands[subcmd.Use] = true
	}

	// Verify each expected subcommand exists
	for _, expected := range expectedSubcommands {
		if !actualSubcommands[expected] {
			t.Errorf("Missing expected subcommand: %s", expected)
		}
	}
}

// TestSecretsKeysSubcommands_RequiredFlags verifies SOPS lifecycle keys subcommands have required flags
// This is a unit test complement to the property test TestProperty_KeysSubcommandsFlagsPresent
func TestSecretsKeysSubcommands_RequiredFlags(t *testing.T) {
	keysCmd := NewSecretsKeysCmd()
	subcommands := []string{"generate", "rotate", "backup", "validate"}

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			subcmd := findSubcommand(keysCmd, name)
			if subcmd == nil {
				t.Fatalf("missing subcommand %q", name)
			}

			// Check for --key-file flag
			keyFileFlag := subcmd.Flags().Lookup("key-file")
			if keyFileFlag == nil {
				t.Errorf("Subcommand %q missing --key-file flag", subcmd.Use)
			} else {
				// Verify it's a string flag
				if keyFileFlag.Value.Type() != "string" {
					t.Errorf("--key-file should be string, got %s", keyFileFlag.Value.Type())
				}
			}

			// Check for --dry-run flag
			dryRunFlag := subcmd.Flags().Lookup("dry-run")
			if dryRunFlag == nil {
				t.Errorf("Subcommand %q missing --dry-run flag", subcmd.Use)
			} else {
				// Verify it's a bool flag
				if dryRunFlag.Value.Type() != "bool" {
					t.Errorf("--dry-run should be bool, got %s", dryRunFlag.Value.Type())
				}
			}
		})
	}
}

// TestSecretsKeysGenerateCmd_Flags verifies generate command flags
func TestSecretsKeysGenerateCmd_Flags(t *testing.T) {
	cmd := newSecretsKeysGenerateCmd()

	// Verify --key-file flag
	keyFileFlag := cmd.Flags().Lookup("key-file")
	if keyFileFlag == nil {
		t.Error("Missing --key-file flag")
	}

	// Verify --update-sops-config flag
	updateSOPSFlag := cmd.Flags().Lookup("update-sops-config")
	if updateSOPSFlag == nil {
		t.Error("Missing --update-sops-config flag")
	}

	// Verify --dry-run flag
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Missing --dry-run flag")
	}
}

// TestSecretsKeysRotateCmd_Flags verifies rotate command flags
func TestSecretsKeysRotateCmd_Flags(t *testing.T) {
	cmd := newSecretsKeysRotateCmd()

	// Verify --key-file flag
	keyFileFlag := cmd.Flags().Lookup("key-file")
	if keyFileFlag == nil {
		t.Error("Missing --key-file flag")
	}

	// Verify --path flag (renamed from --search-path)
	pathFlag := cmd.Flags().Lookup("path")
	if pathFlag == nil {
		t.Error("Missing --path flag")
	}

	// Verify --dry-run flag
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Missing --dry-run flag")
	}
}

// TestSecretsKeysBackupCmd_Flags verifies backup command flags
func TestSecretsKeysBackupCmd_Flags(t *testing.T) {
	cmd := newSecretsKeysBackupCmd()

	// Verify --key-file flag
	keyFileFlag := cmd.Flags().Lookup("key-file")
	if keyFileFlag == nil {
		t.Error("Missing --key-file flag")
	}

	// Verify --backup-dir flag
	backupDirFlag := cmd.Flags().Lookup("backup-dir")
	if backupDirFlag == nil {
		t.Error("Missing --backup-dir flag")
	}

	// Verify --dry-run flag
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Missing --dry-run flag")
	}
}

// TestSecretsKeysValidateCmd_Flags verifies validate command flags
func TestSecretsKeysValidateCmd_Flags(t *testing.T) {
	cmd := newSecretsKeysValidateCmd()

	// Verify --key-file flag
	keyFileFlag := cmd.Flags().Lookup("key-file")
	if keyFileFlag == nil {
		t.Error("Missing --key-file flag")
	}

	// Verify --config-file flag
	configFileFlag := cmd.Flags().Lookup("config-file")
	if configFileFlag == nil {
		t.Error("Missing --config-file flag")
	}

	// Verify --dry-run flag
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Missing --dry-run flag")
	}
}
