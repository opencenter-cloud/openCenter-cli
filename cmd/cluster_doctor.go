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
	"os/exec"

	"github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
)

func newClusterDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [name]",
		Short: "Check local tools, credentials, and provider readiness",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}
			cfg, err := loadConfig(ctx, name)
			if err != nil {
				return err
			}
			// Tools: git, kubectl, talosctl
			check := func(bin string) string {
				if _, err := exec.LookPath(bin); err == nil {
					return "OK"
				}
				return "MISSING"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "git: %s\n", check("git"))
			fmt.Fprintf(cmd.OutOrStdout(), "kubectl: %s\n", check("kubectl"))
			fmt.Fprintf(cmd.OutOrStdout(), "talosctl: %s\n", check("talosctl"))
			// Provider-specific checks
			switch cfg.OpenCenter.Infrastructure.Provider {
			case "openstack", "":
				messages := openstack.PreflightOpenStack(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
				for _, m := range messages {
					fmt.Fprintln(cmd.OutOrStdout(), m)
				}
			default:
				// Unknown provider; no checks
			}
			// Update stage and status
			if err := config.UpdateStatus(name, config.StagePreflight, config.StatusSuccess); err != nil {
				// Don't fail the command if status update fails, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Doctor checks complete.")
			return nil
		},
	}
}
