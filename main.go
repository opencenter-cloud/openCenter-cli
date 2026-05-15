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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/opencenter-cloud/opencenter-cli/cmd"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
)

// Build information variables set at compile time via ldflags
var (
	version   = "dev"
	gitCommit = "unknown"
	gitBranch = "unknown"
	gitTag    = ""
	buildDate = "unknown"
)

func main() {
	// Set build information in cmd package
	cmd.Version = version
	cmd.GitCommit = gitCommit
	cmd.GitBranch = gitBranch
	cmd.GitTag = gitTag
	cmd.BuildDate = buildDate

	// Get base directory for path resolver.
	baseDir := config.ResolveClustersDir()

	// Create and initialize DI container
	container, err := di.SetupContainer(baseDir)
	if err != nil {
		// If container setup fails, print error and exit
		// We can't use the logger here since it's in the container
		os.Stderr.WriteString("Failed to initialize application: " + err.Error() + "\n")
		os.Exit(1)
	}

	// Create a context with the container
	ctx := context.WithValue(context.Background(), cmd.ContainerKey, container)

	// Execute with version and context
	if err := cmd.ExecuteWithContext(ctx, version); err != nil {
		// Shutdown container before exiting
		if shutdownErr := container.Shutdown(); shutdownErr != nil {
			os.Stderr.WriteString("Failed to shutdown container: " + shutdownErr.Error() + "\n")
		}

		// Exit code 3 for missing cluster configuration
		var cnfErr *v2.ConfigNotFoundError
		if errors.As(err, &cnfErr) {
			fmt.Fprintf(os.Stderr, "\nCheck available clusters with: opencenter cluster list\n")
			fmt.Fprintf(os.Stderr, "Initialize a new cluster with: opencenter cluster init %s\n", cnfErr.ClusterName)
			os.Exit(3)
		}

		// Print the error since SilenceErrors is set on the root command
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Shutdown container on successful exit
	if err := container.Shutdown(); err != nil {
		os.Stderr.WriteString("Failed to shutdown container: " + err.Error() + "\n")
		os.Exit(1)
	}
}
