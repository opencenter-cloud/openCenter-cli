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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newConfigIDECmd creates the IDE integration setup command
func newConfigIDECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ide",
		Short: "Set up IDE integration for cluster configuration files",
		Long: `Set up IDE integration with JSON schema and YAML language server support.

This command configures your IDE to provide autocomplete, validation, and
documentation for openCenter cluster configuration files. It generates the
JSON schema and creates IDE-specific configuration files.

Supported IDEs:
  • Visual Studio Code (via YAML extension)
  • JetBrains IDEs (IntelliJ IDEA, PyCharm, WebStorm, etc.)
  • Vim/Neovim (via coc.nvim or nvim-lspconfig)
  • Emacs (via lsp-mode)

The setup process:
  1. Generates the latest JSON schema
  2. Creates IDE configuration files
  3. Sets up schema associations
  4. Configures YAML validation and formatting

After running this command, restart your IDE to activate the integration.`,
		Example: `  # Set up IDE integration with default settings
  openCenter config ide

  # Set up for specific IDE
  openCenter config ide --ide=vscode

  # Generate schema only
  openCenter config ide --schema-only

  # Show setup instructions
  openCenter config ide --show-instructions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ide, _ := cmd.Flags().GetString("ide")
			schemaOnly, _ := cmd.Flags().GetBool("schema-only")
			showInstructions, _ := cmd.Flags().GetBool("show-instructions")

			if showInstructions {
				return showIDEInstructions(cmd, ide)
			}

			return setupIDEIntegration(cmd, ide, schemaOnly)
		},
	}

	cmd.Flags().String("ide", "auto", "Target IDE (auto, vscode, jetbrains, vim, emacs)")
	cmd.Flags().Bool("schema-only", false, "Only generate JSON schema without IDE configuration")
	cmd.Flags().Bool("show-instructions", false, "Show setup instructions for the specified IDE")

	return cmd
}

// setupIDEIntegration sets up IDE integration
func setupIDEIntegration(cmd *cobra.Command, ide string, schemaOnly bool) error {
	fmt.Println("🔧 Setting up IDE integration for openCenter...")

	// Generate JSON schema
	fmt.Println("📄 Generating JSON schema...")
	schemaPath := "schema/cluster.schema.json"
	
	// Ensure schema directory exists
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o755); err != nil {
		return fmt.Errorf("failed to create schema directory: %w", err)
	}

	// Generate schema
	schemaData, err := config.GenerateSchema(true)
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Write schema file
	if err := os.WriteFile(schemaPath, schemaData, 0o644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	fmt.Printf("✅ Schema generated: %s (version %s)\n", schemaPath, config.GetSchemaVersion())

	if schemaOnly {
		fmt.Println("ℹ️  Schema-only mode: Skipping IDE configuration")
		return nil
	}

	// Detect or use specified IDE
	if ide == "auto" {
		ide = detectIDE()
		fmt.Printf("🔍 Detected IDE: %s\n", ide)
	}

	// Set up IDE-specific configuration
	switch ide {
	case "vscode":
		if err := setupVSCode(); err != nil {
			return fmt.Errorf("failed to setup VS Code integration: %w", err)
		}
		fmt.Println("✅ VS Code integration configured")
		fmt.Println("💡 Restart VS Code to activate the integration")

	case "jetbrains":
		fmt.Println("ℹ️  JetBrains IDE integration requires manual setup")
		fmt.Println("💡 Run 'openCenter config ide --show-instructions --ide=jetbrains' for setup steps")

	case "vim":
		fmt.Println("ℹ️  Vim/Neovim integration requires manual setup")
		fmt.Println("💡 Run 'openCenter config ide --show-instructions --ide=vim' for setup steps")

	case "emacs":
		fmt.Println("ℹ️  Emacs integration requires manual setup")
		fmt.Println("💡 Run 'openCenter config ide --show-instructions --ide=emacs' for setup steps")

	default:
		fmt.Printf("⚠️  Unknown IDE: %s\n", ide)
		fmt.Println("💡 Supported IDEs: vscode, jetbrains, vim, emacs")
		return nil
	}

	fmt.Println("\n🎉 IDE integration setup complete!")
	fmt.Println("📚 For more information, see: docs/ide-integration.md")

	return nil
}

// detectIDE attempts to detect the user's IDE
func detectIDE() string {
	// Check for VS Code
	if _, err := os.Stat(".vscode"); err == nil {
		return "vscode"
	}

	// Check for JetBrains IDEs
	if _, err := os.Stat(".idea"); err == nil {
		return "jetbrains"
	}

	// Default to vscode as it's most common
	return "vscode"
}

// setupVSCode creates VS Code configuration
func setupVSCode() error {
	vscodeDir := ".vscode"
	settingsPath := filepath.Join(vscodeDir, "settings.json")

	// Create .vscode directory if it doesn't exist
	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .vscode directory: %w", err)
	}

	// VS Code settings
	settings := map[string]interface{}{
		"yaml.schemas": map[string]interface{}{
			"./schema/cluster.schema.json": []string{
				"**/clusters/**/*.yaml",
				"**/clusters/**/*-config.yaml",
				"**/.opencenter.yaml",
			},
		},
		"yaml.customTags": []string{
			"!vault",
			"!encrypted/pkcs1-oaep",
		},
		"yaml.format.enable": true,
		"yaml.validate":      true,
		"yaml.completion":    true,
		"yaml.hover":         true,
		"files.associations": map[string]string{
			"*-config.yaml":     "yaml",
			".opencenter.yaml":  "yaml",
			".sops.yaml":        "yaml",
		},
		"editor.quickSuggestions": map[string]bool{
			"strings": true,
		},
		"[yaml]": map[string]interface{}{
			"editor.defaultFormatter": "redhat.vscode-yaml",
			"editor.formatOnSave":     true,
			"editor.autoIndent":       "advanced",
		},
	}

	// Check if settings file exists
	var existingSettings map[string]interface{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		// Merge with existing settings
		if err := json.Unmarshal(data, &existingSettings); err == nil {
			// Merge settings
			for key, value := range settings {
				existingSettings[key] = value
			}
			settings = existingSettings
		}
	}

	// Write settings file
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// showIDEInstructions displays setup instructions for the specified IDE
func showIDEInstructions(cmd *cobra.Command, ide string) error {
	switch ide {
	case "vscode":
		fmt.Print(`
Visual Studio Code Setup Instructions
======================================

1. Install the YAML extension by Red Hat:
   code --install-extension redhat.vscode-yaml

2. Run the IDE setup command:
   openCenter config ide --ide=vscode

3. Restart VS Code

4. Open any cluster configuration file and enjoy autocomplete!

Features:
  • Autocomplete: Press Ctrl+Space for suggestions
  • Validation: Real-time error detection
  • Hover Documentation: Hover over keys for descriptions
  • Format Document: Press Shift+Alt+F to format

For more information, see: docs/ide-integration.md
`)

	case "jetbrains":
		fmt.Print(`
JetBrains IDEs Setup Instructions
==================================

1. Generate the JSON schema:
   openCenter cluster schema --out schema/cluster.schema.json

2. Open Settings/Preferences → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings

3. Add a new mapping:
   - Name: openCenter Cluster Configuration
   - Schema file: schema/cluster.schema.json
   - Schema version: JSON Schema version 7

4. Add file patterns:
   - **/clusters/**/*.yaml
   - **/clusters/**/*-config.yaml
   - **/.opencenter.yaml

5. Apply and restart your IDE

Features:
  • Autocomplete: Press Ctrl+Space for code completion
  • Validation: Real-time validation with error highlighting
  • Quick Documentation: Press Ctrl+Q for documentation
  • Reformat Code: Press Ctrl+Alt+L to format

For more information, see: docs/ide-integration.md
`)

	case "vim":
		fmt.Print(`
Vim/Neovim Setup Instructions
==============================

Option 1: Using coc.nvim
-------------------------
1. Install coc.nvim: https://github.com/neoclide/coc.nvim

2. Install the YAML language server:
   :CocInstall coc-yaml

3. Configure coc-settings.json:
   {
     "yaml.schemas": {
       "./schema/cluster.schema.json": [
         "**/clusters/**/*.yaml",
         "**/clusters/**/*-config.yaml",
         "**/.opencenter.yaml"
       ]
     },
     "yaml.validate": true,
     "yaml.completion": true
   }

4. Generate the schema:
   openCenter cluster schema --out schema/cluster.schema.json

Option 2: Using nvim-lspconfig
-------------------------------
1. Install nvim-lspconfig: https://github.com/neovim/nvim-lspconfig

2. Install yaml-language-server:
   npm install -g yaml-language-server

3. Configure in init.lua:
   require'lspconfig'.yamlls.setup{
     settings = {
       yaml = {
         schemas = {
           ["./schema/cluster.schema.json"] = {
             "**/clusters/**/*.yaml",
             "**/clusters/**/*-config.yaml",
             "**/.opencenter.yaml"
           }
         },
         validate = true,
         completion = true
       }
     }
   }

4. Generate the schema:
   openCenter cluster schema --out schema/cluster.schema.json

For more information, see: docs/ide-integration.md
`)

	case "emacs":
		fmt.Print(`
Emacs Setup Instructions
=========================

1. Install lsp-mode: https://github.com/emacs-lsp/lsp-mode

2. Install yaml-language-server:
   npm install -g yaml-language-server

3. Configure in init.el:
   (use-package lsp-mode
     :hook (yaml-mode . lsp)
     :config
     (setq lsp-yaml-schemas
           '(:cluster "./schema/cluster.schema.json")))

4. Add file associations:
   (add-to-list 'auto-mode-alist '("\\*-config\\.yaml\\'" . yaml-mode))
   (add-to-list 'auto-mode-alist '("\\.opencenter\\.yaml\\'" . yaml-mode))

5. Generate the schema:
   openCenter cluster schema --out schema/cluster.schema.json

6. Restart Emacs

For more information, see: docs/ide-integration.md
`)

	default:
		fmt.Printf("⚠️  Unknown IDE: %s\n", ide)
		fmt.Println("💡 Supported IDEs: vscode, jetbrains, vim, emacs")
		fmt.Println("📚 For more information, see: docs/ide-integration.md")
	}

	return nil
}
