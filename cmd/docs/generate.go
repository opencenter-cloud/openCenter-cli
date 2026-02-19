//go:build tools

// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/opencenter-cli/cmd"
	"github.com/rackerlabs/opencenter-cli/internal/plugins"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	// Get the root command from your application
	rootCmd := cmd.GetRootCmd()

	// Manually add all subcommands to build the complete command tree.
	rootCmd.AddCommand(cmd.NewClusterCmd())
	rootCmd.AddCommand(cmd.NewConfigCmd())
	rootCmd.AddCommand(cmd.NewSecretsCmd())
	rootCmd.AddCommand(cmd.NewPluginsCmd())
	rootCmd.AddCommand(cmd.NewVersionCmd())
	// Discover and attach external plugins as subcommands
	plugins.LoadExternalPlugins(rootCmd)

	// Set the output directory for the documentation
	outputDir := "docs/reference/opencenter"

	// Remove the existing directory to ensure a clean slate
	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("Failed to remove existing documentation directory: %v", err)
	}

	// Create the documentation directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create documentation directory: %v", err)
	}

	// Generate the markdown documentation tree
	if err := doc.GenMarkdownTree(rootCmd, outputDir); err != nil {
		log.Fatalf("Failed to generate documentation: %v", err)
	}

	log.Println("Documentation generated successfully in", outputDir)
}
