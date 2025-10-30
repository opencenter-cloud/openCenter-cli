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
    "github.com/spf13/cobra"
)

// newClusterCmd creates the top-level "cluster" command. It has
// several subcommands defined in separate files. Running "openCenter
// cluster" without subcommand prints help.
func newClusterCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "cluster",
        Short: "Manage cluster configurations",
        RunE: func(cmd *cobra.Command, args []string) error {
            return cmd.Help()
        },
    }
    // Add subcommands
    cmd.AddCommand(newClusterListCmd())
    cmd.AddCommand(newClusterSelectCmd())
    cmd.AddCommand(newClusterCurrentCmd())
    cmd.AddCommand(newClusterInfoCmd())
    cmd.AddCommand(newClusterInitCmd())
    cmd.AddCommand(newClusterValidateCmd())
    cmd.AddCommand(newClusterPreflightCmd())
    cmd.AddCommand(newClusterSetupCmd())
    cmd.AddCommand(newClusterRenderCmd())
    cmd.AddCommand(newClusterBootstrapCmd())
    cmd.AddCommand(newClusterSchemaCmd())
    cmd.AddCommand(newClusterDestroyCmd())
    cmd.AddCommand(newClusterUpdateCmd())
    return cmd
}
