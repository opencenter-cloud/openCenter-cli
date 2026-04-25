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

// newClusterCredentialsCmd creates the hidden legacy credentials command.
// It has subcommands for export and unset operations.
// Note: This command is now superseded by 'cluster env' but kept for backward compatibility.
func newClusterCredentialsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "credentials",
		Short:  "Manage cloud provider credentials from cluster configuration",
		Hidden: true, // Hidden from help - superseded by 'cluster env'
		Long: `Manage cloud provider credentials extracted from cluster configuration.

This command provides subcommands for exporting and unsetting cloud provider
credentials that are stored in the cluster's SOPS-encrypted configuration.

Note: This command is superseded by 'cluster env' which provides the same
functionality with a simpler interface.

Supported providers:
  • aws - Amazon Web Services credentials
  • openstack - OpenStack application credentials
  • all - All configured providers

The credentials are sourced from the cluster configuration file and can be
exported in various formats for use with other tools or unset from the
current environment.

Configuration sources:
  • AWS: opencenter.infrastructure.cloud.aws and secrets.global.aws.infrastructure
  • OpenStack: opencenter.infrastructure.cloud.openstack

Common workflow:
  1. Export credentials to environment variables
  2. Use with other cloud tools (terraform, ansible, etc.)
  3. Unset credentials when done`,
		Example: `  # Export current cluster environment
  eval "$(opencenter cluster env)"

  # Export a specific cluster environment
  eval "$(opencenter cluster env my-cluster)"

  # Export with organization
  eval "$(opencenter cluster env myorg/my-cluster)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newClusterCredentialsExportCmd())
	cmd.AddCommand(newClusterCredentialsUnsetCmd())

	return cmd
}
