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
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// newClusterCredentialsUnsetCmd creates the hidden legacy credentials unset command.
func newClusterCredentialsUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Generate commands to unset cloud provider credentials from environment",
		Long: `Generate shell commands to unset cloud provider credentials from the environment.

This command outputs shell commands that can be evaluated to remove cloud
provider environment variables from the current shell session. This is useful
for cleaning up credentials after use or switching between different sets
of credentials.

The command generates 'unset' statements for the standard environment variables
used by each cloud provider:

AWS Variables:
  • AWS_ACCESS_KEY_ID
  • AWS_SECRET_ACCESS_KEY
  • AWS_DEFAULT_REGION
  • AWS_SESSION_TOKEN

OpenStack Variables:
  • OS_AUTH_URL
  • OS_USERNAME
  • OS_PASSWORD
  • OS_PROJECT_NAME
  • OS_USER_DOMAIN_NAME
  • OS_PROJECT_DOMAIN_NAME
  • OS_APPLICATION_CREDENTIAL_ID
  • OS_APPLICATION_CREDENTIAL_SECRET

For current GA workflows, start a fresh shell or unset the provider variables
directly after using opencenter cluster env.

This approach ensures that the unset commands are executed in the current
shell context, properly removing the environment variables.`,
		Example: `  # Export current cluster environment in a new shell
  eval "$(opencenter cluster env)"

  # Export a specific cluster environment in a new shell
  eval "$(opencenter cluster env my-cluster)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flags
			provider, _ := cmd.Flags().GetString("provider")

			// Validate provider
			validProviders := []string{"aws", "openstack", "all"}
			if !contains(validProviders, provider) {
				return fmt.Errorf("invalid provider '%s', must be one of: %s", provider, strings.Join(validProviders, ", "))
			}

			// Generate unset commands based on provider
			switch provider {
			case "aws":
				return unsetAWSCredentials(cmd)
			case "openstack":
				return unsetOpenStackCredentials(cmd)
			case "all":
				return unsetAllCredentials(cmd)
			default:
				return fmt.Errorf("unsupported provider: %s", provider)
			}
		},
	}

	// Add flags
	cmd.Flags().String("provider", "all", "cloud provider to unset (aws, openstack, all)")

	return cmd
}

// unsetAWSCredentials generates unset commands for AWS credentials
func unsetAWSCredentials(cmd *cobra.Command) error {
	unsetVars := credentials.GetAWSEnvVars()

	var output strings.Builder
	for _, envVar := range unsetVars {
		output.WriteString(fmt.Sprintf("unset %s\n", envVar))
	}

	fmt.Fprint(cmd.OutOrStdout(), output.String())
	return nil
}

// unsetOpenStackCredentials generates unset commands for OpenStack credentials
func unsetOpenStackCredentials(cmd *cobra.Command) error {
	unsetVars := credentials.GetOpenStackEnvVars()

	var output strings.Builder
	for _, envVar := range unsetVars {
		output.WriteString(fmt.Sprintf("unset %s\n", envVar))
	}

	fmt.Fprint(cmd.OutOrStdout(), output.String())
	return nil
}

// unsetAllCredentials generates unset commands for all cloud provider credentials
func unsetAllCredentials(cmd *cobra.Command) error {
	var output strings.Builder

	// AWS credentials
	awsVars := credentials.GetAWSEnvVars()
	for _, envVar := range awsVars {
		output.WriteString(fmt.Sprintf("unset %s\n", envVar))
	}

	// OpenStack credentials
	osVars := credentials.GetOpenStackEnvVars()
	for _, envVar := range osVars {
		output.WriteString(fmt.Sprintf("unset %s\n", envVar))
	}

	fmt.Fprint(cmd.OutOrStdout(), output.String())
	return nil
}
