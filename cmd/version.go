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
	"runtime"

	"github.com/spf13/cobra"
)

// Build information variables set at compile time via ldflags
var (
	// Version is the semantic version (e.g., "1.0.0" or "0.0.1")
	Version = "dev"
	// GitCommit is the git commit SHA
	GitCommit = "unknown"
	// GitBranch is the git branch name
	GitBranch = "unknown"
	// GitTag is the git tag (if building from a tag)
	GitTag = ""
	// BuildDate is the build timestamp
	BuildDate = "unknown"
)

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version and build information",
		Long: `Display version and build information for openCenter.

Shows the version, git commit, branch, tag (if applicable), build date,
Go version, and platform information.`,
		Example: `  # Show full version information
  openCenter version

  # Show short version only
  openCenter version --short`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if short {
				// Short format: just the version string
				fmt.Fprintln(cmd.OutOrStdout(), getVersionString())
			} else {
				// Full format: all build information
				fmt.Fprintln(cmd.OutOrStdout(), getFullVersionInfo())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&short, "short", false, "Display short version only")

	return cmd
}

// getVersionString returns the version string
// If built from a tag, use the tag; otherwise use version-commit
func getVersionString() string {
	if GitTag != "" {
		return GitTag
	}
	if GitCommit != "unknown" && len(GitCommit) >= 7 {
		return fmt.Sprintf("%s-%s", Version, GitCommit[:7])
	}
	return Version
}

// getFullVersionInfo returns formatted version information
func getFullVersionInfo() string {
	versionStr := getVersionString()
	
	info := fmt.Sprintf("openCenter version: %s\n", versionStr)
	
	if GitCommit != "unknown" {
		info += fmt.Sprintf("Git commit:         %s\n", GitCommit)
	}
	
	if GitBranch != "unknown" {
		info += fmt.Sprintf("Git branch:         %s\n", GitBranch)
	}
	
	if GitTag != "" {
		info += fmt.Sprintf("Git tag:            %s\n", GitTag)
	}
	
	if BuildDate != "unknown" {
		info += fmt.Sprintf("Build date:         %s\n", BuildDate)
	}
	
	info += fmt.Sprintf("Go version:         %s\n", runtime.Version())
	info += fmt.Sprintf("Platform:           %s/%s\n", runtime.GOOS, runtime.GOARCH)
	
	return info
}
