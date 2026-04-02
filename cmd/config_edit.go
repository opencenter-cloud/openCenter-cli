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
	"fmt"
	"os"
	"os/exec"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/spf13/cobra"
)

// newConfigEditCmd creates the "config edit" command to open the CLI config file in an editor.
func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the CLI configuration file in your default editor",
		Long: `Edit the CLI configuration file in your default editor.

This command opens the configuration file in the editor specified by the EDITOR
environment variable. If EDITOR is not set, it falls back to common editors
in the following order: vim, vi, nano.

After editing, the configuration will be validated. If validation fails, you'll
be notified of the errors but the file will remain saved.

Examples:
  opencenter config edit
  EDITOR=nano opencenter config edit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the configuration file path
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			configPath := cm.GetConfigPath()

			// Ensure the config file exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				// Create default config if it doesn't exist
				if err := cm.Save(); err != nil {
					return fmt.Errorf("failed to create configuration file: %w", err)
				}
				fmt.Printf("Created new configuration file at: %s\n", configPath)
			}

			// Determine the editor to use
			editor := getEditor()
			if editor == "" {
				return fmt.Errorf("no editor found: set EDITOR environment variable or install vim, vi, or nano")
			}
			sanitizer := security.NewDefaultCommandSanitizer()
			if err := sanitizer.ValidateEditor(editor); err != nil {
				return fmt.Errorf("invalid EDITOR environment variable: %w", err)
			}

			fmt.Printf("Opening %s in %s...\n", configPath, editor)

			// Open the editor
			editorCmd, err := security.GetDefaultCommandRunner().PrepareCommand(editor, configPath)
			if err != nil {
				return fmt.Errorf("failed to prepare editor command: %w", err)
			}
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}

			// Reload and validate the configuration after editing
			fmt.Println("\nValidating configuration...")
			newCM, err := config.NewConfigManager(configPath)
			if err != nil {
				fmt.Printf("⚠️  Configuration validation failed: %v\n", err)
				fmt.Printf("The file has been saved but may contain errors.\n")
				fmt.Printf("Run 'opencenter config view' to see the current configuration.\n")
				return nil // Don't return error to avoid confusion
			}

			// Show validation summary
			result := newCM.ValidateConfig()
			if result.Valid {
				fmt.Println("✓ Configuration is valid")
			} else {
				fmt.Println("⚠️  Configuration has issues:")
				for _, err := range result.Errors {
					fmt.Printf("  - %s\n", err.Error())
				}
				if len(result.Warnings) > 0 {
					fmt.Println("\nWarnings:")
					for _, warning := range result.Warnings {
						fmt.Printf("  - %s\n", warning.Error())
					}
				}
			}

			return nil
		},
	}
}

// getEditor determines which editor to use based on environment and availability.
func getEditor() string {
	// Check EDITOR environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Fall back to common editors
	editors := []string{"vim", "vi", "nano"}
	for _, editor := range editors {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	return ""
}
