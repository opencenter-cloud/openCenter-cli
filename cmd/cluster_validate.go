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

    "github.com/rackerlabs/openCenter/internal/config"
    "github.com/spf13/cobra"
)

// newClusterValidateCmd creates the command for validating a cluster's configuration.
//
// This command loads a cluster's configuration and runs a series of validation
// checks defined in the `config.Validate` function. If any validation rules
// are violated, it prints the errors to standard error and exits with a non-zero
// status code. If the configuration is valid, it prints a success message to
// standard output.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `validate` command.
func newClusterValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [name]",
		Short: "Validate cluster configuration invariants",
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
			errs := config.Validate(cfg)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(cmd.ErrOrStderr(), e)
				}
				return fmt.Errorf("validation failed")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Validation successful.")
			return nil
		},
	}
}
