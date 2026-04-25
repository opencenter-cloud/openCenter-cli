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
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// newClusterCredentialsExportCmd creates the hidden legacy credentials export command.
func newClusterCredentialsExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [cluster-name]",
		Short: "Export cloud provider credentials from cluster configuration",
		Long: `Export cloud provider credentials from cluster configuration in various formats.

This command reads the cluster's SOPS-encrypted configuration and extracts
cloud provider credentials for use with other tools. The credentials are
sourced from both the infrastructure configuration and secrets sections.

Credential Sources:
  AWS:
    • opencenter.infrastructure.cloud.aws (profile, region, vpc, subnets)
    • secrets.global.aws.infrastructure (access_key, secret_access_key, region)

  OpenStack:
    • opencenter.infrastructure.cloud.openstack (auth_url, region, etc.)
    • Application credentials from the configuration

Output Formats:
  • env (default): Shell environment variable exports
  • json: JSON format for programmatic use
  • terraform: Terraform provider configuration
  • clouds-yaml: OpenStack clouds.yaml format (OpenStack only)

The exported credentials can be used with:
  • eval "$(opencenter cluster env)"
  • Terraform/OpenTofu infrastructure provisioning
  • Ansible playbooks and inventory
  • Direct cloud CLI tools (aws, openstack)`,
		Example: `  # Export current cluster environment
  eval "$(opencenter cluster env)"

  # Export a specific cluster environment
  eval "$(opencenter cluster env my-cluster)"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				// No name provided, try to use active cluster
				activeName, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if activeName == "" {
					return fmt.Errorf("no cluster name provided and no active cluster set; specify a cluster name or use 'opencenter cluster use <name>' to set an active cluster")
				}
				name = activeName
			}

			// Get flags
			provider, _ := cmd.Flags().GetString("provider")
			format, _ := cmd.Flags().GetString("format")

			// Validate provider
			validProviders := []string{"aws", "openstack", "all"}
			if !contains(validProviders, provider) {
				return fmt.Errorf("invalid provider '%s', must be one of: %s", provider, strings.Join(validProviders, ", "))
			}

			// Validate format
			validFormats := []string{"env", "json", "terraform", "clouds-yaml"}
			if !contains(validFormats, format) {
				return fmt.Errorf("invalid format '%s', must be one of: %s", format, strings.Join(validFormats, ", "))
			}

			// Validate format-provider combinations
			if format == "clouds-yaml" && provider != "openstack" {
				return fmt.Errorf("clouds-yaml format is only supported for OpenStack provider")
			}

			// Load cluster configuration
			_, err := loadConfig(cmd.Context(), name)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			nativeCfg, _, _, _, err := loadNativeV2ConfigWithIdentifier(cmd.Context(), name)
			if err != nil {
				return fmt.Errorf("failed to load native cluster configuration: %w", err)
			}

			// Create credentials extractor
			extractor := credentials.NewExtractor(*nativeCfg)

			// Export credentials based on provider
			switch provider {
			case "aws":
				return exportAWSCredentials(cmd, extractor, format)
			case "openstack":
				return exportOpenStackCredentials(cmd, extractor, format)
			case "all":
				return exportAllCredentials(cmd, extractor, format)
			default:
				return fmt.Errorf("unsupported provider: %s", provider)
			}
		},
	}

	// Add flags
	cmd.Flags().String("provider", "all", "cloud provider to export (aws, openstack, all)")
	cmd.Flags().String("format", "env", "output format (env, json, terraform, clouds-yaml)")

	return cmd
}

// exportAWSCredentials exports AWS credentials in the specified format
func exportAWSCredentials(cmd *cobra.Command, extractor *credentials.Extractor, format string) error {
	awsCreds, err := extractor.ExtractAWS()
	if err != nil {
		return fmt.Errorf("failed to extract AWS credentials: %w", err)
	}

	if awsCreds.IsEmpty() {
		return fmt.Errorf("no AWS credentials found in cluster configuration")
	}

	switch format {
	case "env":
		output := awsCreds.ToEnvVarsForShell("bash")
		fmt.Fprint(cmd.OutOrStdout(), output)
	case "json":
		jsonData, err := json.MarshalIndent(awsCreds.ToMap(), "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal AWS credentials to JSON: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(jsonData))
	case "terraform":
		output := awsCreds.ToTerraform()
		fmt.Fprint(cmd.OutOrStdout(), output)
	default:
		return fmt.Errorf("unsupported format '%s' for AWS provider", format)
	}

	return nil
}

// exportOpenStackCredentials exports OpenStack credentials in the specified format
func exportOpenStackCredentials(cmd *cobra.Command, extractor *credentials.Extractor, format string) error {
	osCreds, err := extractor.ExtractOpenStack()
	if err != nil {
		return fmt.Errorf("failed to extract OpenStack credentials: %w", err)
	}

	if osCreds.IsEmpty() {
		return fmt.Errorf("no OpenStack credentials found in cluster configuration")
	}

	switch format {
	case "env":
		output := osCreds.ToEnvVarsForShell("bash")
		fmt.Fprint(cmd.OutOrStdout(), output)
	case "json":
		jsonData, err := json.MarshalIndent(osCreds.ToMap(), "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal OpenStack credentials to JSON: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(jsonData))
	case "terraform":
		output := osCreds.ToTerraform()
		fmt.Fprint(cmd.OutOrStdout(), output)
	case "clouds-yaml":
		output := osCreds.ToCloudsYAML()
		fmt.Fprint(cmd.OutOrStdout(), output)
	default:
		return fmt.Errorf("unsupported format '%s' for OpenStack provider", format)
	}

	return nil
}

// exportAllCredentials exports all available credentials in the specified format
func exportAllCredentials(cmd *cobra.Command, extractor *credentials.Extractor, format string) error {
	// Extract both AWS and OpenStack credentials
	awsCreds, awsErr := extractor.ExtractAWS()
	osCreds, osErr := extractor.ExtractOpenStack()

	// Check if we have any credentials
	hasAWS := awsErr == nil && !awsCreds.IsEmpty()
	hasOS := osErr == nil && !osCreds.IsEmpty()

	if !hasAWS && !hasOS {
		return fmt.Errorf("no credentials found in cluster configuration")
	}

	switch format {
	case "env":
		var output strings.Builder
		if hasAWS {
			output.WriteString(awsCreds.ToEnvVarsForShell("bash"))
		}
		if hasOS {
			if hasAWS {
				output.WriteString("\n")
			}
			output.WriteString(osCreds.ToEnvVarsForShell("bash"))
		}
		fmt.Fprint(cmd.OutOrStdout(), output.String())
	case "json":
		allCreds := make(map[string]interface{})
		if hasAWS {
			allCreds["aws"] = awsCreds.ToMap()
		}
		if hasOS {
			allCreds["openstack"] = osCreds.ToMap()
		}
		jsonData, err := json.MarshalIndent(allCreds, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal credentials to JSON: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(jsonData))
	case "terraform":
		var output strings.Builder
		if hasAWS {
			output.WriteString(awsCreds.ToTerraform())
		}
		if hasOS {
			if hasAWS {
				output.WriteString("\n\n")
			}
			output.WriteString(osCreds.ToTerraform())
		}
		fmt.Fprint(cmd.OutOrStdout(), output.String())
	case "clouds-yaml":
		return fmt.Errorf("clouds-yaml format is only supported for OpenStack provider")
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
