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
    "gopkg.in/yaml.v3"

    "github.com/rackerlabs/openCenter-cli/internal/config"
    "github.com/spf13/cobra"
)

func newClusterInfoCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "info [name]",
        Short: "Show configuration for a cluster",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				var err error
				name, err = config.GetActive()
				if err != nil {
					return err
				}
				if name == "" {
					return fmt.Errorf("no active cluster; specify name")
				}
			}
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			// Handle --validate flag
			validate, _ := cmd.Flags().GetBool("validate")
			if validate {
				errs := config.Validate(cfg)
				if len(errs) > 0 {
					for _, e := range errs {
						fmt.Fprintln(cmd.ErrOrStderr(), e)
					}
					return fmt.Errorf("validation failed")
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Validation successful.")
				return nil
			}

			// Get the full path to the config file
			configPath, err := config.ConfigPath(name)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

            // Output format
            asJSON, _ := cmd.Flags().GetBool("json")
            if asJSON {
				// Print metadata and path in JSON format
				output := map[string]any{
					"config_path": configPath,
					"metadata":    cfg.OpenCenter.Meta,
				}
				b, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
            }

            // Print metadata and config path in human-readable format
            fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", name)
            fmt.Fprintf(cmd.OutOrStdout(), "Config Path: %s\n\n", configPath)
            fmt.Fprintln(cmd.OutOrStdout(), "Metadata:")
            
            // Create a combined metadata output that includes both Meta and cluster_name
            metadataOutput := map[string]any{
            	"name":         cfg.OpenCenter.Meta.Name,
            	"cluster_name": cfg.OpenCenter.Cluster.ClusterName,
            	"env":          cfg.OpenCenter.Meta.Env,
            	"region":       cfg.OpenCenter.Meta.Region,
            	"status":       cfg.OpenCenter.Meta.Status,
            	"organization": cfg.OpenCenter.Meta.Organization,
            }
            
            data, err := yaml.Marshal(metadataOutput)
            if err != nil {
                return err
            }
            fmt.Fprint(cmd.OutOrStdout(), string(data))
            return nil
        },
    }
    cmd.Flags().Bool("validate", false, "validate cluster configuration invariants")
    cmd.Flags().Bool("json", false, "output JSON instead of YAML")
    return cmd
}
