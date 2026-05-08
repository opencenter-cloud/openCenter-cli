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

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newConfigExplainCmd creates the "config explain" command group.
func newConfigExplainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain how configuration values affect CLI behavior",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newConfigExplainClusterDefaultsCmd())
	return cmd
}

// clusterDefaultExplanation describes a single cluster_defaults field.
type clusterDefaultExplanation struct {
	Key       string
	AppliedTo string
	When      string
	During    string
}

var clusterDefaultExplanations = []clusterDefaultExplanation{
	{
		Key:       "cluster_defaults.provider",
		AppliedTo: "opencenter.infrastructure.provider",
		When:      "cluster config has no provider set",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.region",
		AppliedTo: "opencenter.meta.region",
		When:      "cluster config has no region set",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.environment",
		AppliedTo: "opencenter.meta.env",
		When:      "cluster config has no environment set",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.gitops_auth_method",
		AppliedTo: "opencenter.gitops.auth",
		When:      "cluster init chooses the GitOps bootstrap auth shape",
		During:    "cluster init",
	},
	{
		Key:       "cluster_defaults.ssh_authorized_keys",
		AppliedTo: "opencenter.cluster.ssh_authorized_keys",
		When:      "cluster config has no valid SSH keys",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.base_domain",
		AppliedTo: "opencenter.cluster.base_domain",
		When:      "cluster config uses the placeholder domain or is empty",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.admin_email",
		AppliedTo: "opencenter.cluster.admin_email",
		When:      "cluster config uses the placeholder email or is empty",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.kubernetes_version",
		AppliedTo: "opencenter.cluster.kubernetes.version",
		When:      "always (overrides the hardcoded default)",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.cni",
		AppliedTo: "opencenter.cluster.kubernetes.network_plugin",
		When:      "the specified CNI is not already enabled",
		During:    "cluster init, config load",
	},
	{
		Key:       "cluster_defaults.ssh_user",
		AppliedTo: "opencenter.infrastructure.ssh_user",
		When:      "cluster config uses the default 'ubuntu' or is empty",
		During:    "cluster init, config load",
	},
}

// newConfigExplainClusterDefaultsCmd creates the "config explain cluster-defaults" command.
func newConfigExplainClusterDefaultsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster-defaults",
		Short: "Show how cluster_defaults values are applied to new cluster configs",
		Long: `Display each cluster_defaults field, its current value, and exactly where
it is injected into cluster configurations during "opencenter cluster init".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			cd := cm.GetConfig().ClusterDefaults

			for _, e := range clusterDefaultExplanations {
				value := fieldValue(cd, e.Key)
				fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", e.Key, value)
				fmt.Fprintf(cmd.OutOrStdout(), "  → Applied to: %s\n", e.AppliedTo)
				fmt.Fprintf(cmd.OutOrStdout(), "  → When:       %s\n", e.When)
				fmt.Fprintf(cmd.OutOrStdout(), "  → During:     %s\n", e.During)
				fmt.Fprintln(cmd.OutOrStdout())
			}

			return nil
		},
	}
}

// fieldValue returns the string representation of a cluster defaults field.
func fieldValue(cd config.ClusterDefaultsConfig, key string) string {
	switch key {
	case "cluster_defaults.provider":
		return quote(cd.Provider)
	case "cluster_defaults.region":
		return quote(cd.Region)
	case "cluster_defaults.environment":
		return quote(cd.Environment)
	case "cluster_defaults.gitops_auth_method":
		return quote(cd.GitopsAuthMethod)
	case "cluster_defaults.ssh_authorized_keys":
		if len(cd.SSHAuthorizedKeys) == 0 {
			return "(not set)"
		}
		return fmt.Sprintf("[%d key(s)]", len(cd.SSHAuthorizedKeys))
	case "cluster_defaults.base_domain":
		return quote(cd.BaseDomain)
	case "cluster_defaults.admin_email":
		return quote(cd.AdminEmail)
	case "cluster_defaults.kubernetes_version":
		return quote(cd.KubernetesVersion)
	case "cluster_defaults.cni":
		return quote(cd.CNI)
	case "cluster_defaults.ssh_user":
		return quote(cd.SSHUser)
	default:
		return "(unknown)"
	}
}

func quote(s string) string {
	if s == "" {
		return "(not set)"
	}
	return fmt.Sprintf("%q", s)
}
