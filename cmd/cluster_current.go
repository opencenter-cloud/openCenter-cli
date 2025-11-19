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

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

func newClusterCurrentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Show the current active cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := config.GetActive()
			if err != nil {
				return err
			}
			if name == "" {
				// Nothing to show
				return nil
			}
			q, _ := cmd.Flags().GetBool("quiet")
			if q {
				fmt.Fprint(cmd.OutOrStdout(), strings.TrimSpace(name))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.TrimSpace(name))
			}
			return nil
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "quiet output (just the name)")
	return cmd
}
