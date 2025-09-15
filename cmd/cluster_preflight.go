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

    "github.com/rackerlabs/openCenter/internal/cloud/openstack"
    "github.com/rackerlabs/openCenter/internal/config"
    "github.com/spf13/cobra"
)

func newClusterPreflightCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "preflight [name]",
        Short: "Run preflight checks for tools and provider requirements",
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
            switch cfg.Cloud.Provider {
            case "openstack", "":
                messages := openstack.PreflightOpenStack(cfg.Cloud.OpenStack.AuthURL)
                for _, m := range messages {
                    fmt.Fprintln(cmd.OutOrStdout(), m)
                }
            default:
                // Unknown provider; no checks
            }
            fmt.Fprintln(cmd.OutOrStdout(), "Preflight complete.")
            return nil
        },
    }
}
