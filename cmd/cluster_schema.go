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
    "os"
    "path/filepath"

    "github.com/rackerlabs/openCenter-cli/internal/config"
    "github.com/spf13/cobra"
)

func newClusterSchemaCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "schema",
        Short: "Export cluster JSON schema",
        RunE: func(cmd *cobra.Command, args []string) error {
            outPath, _ := cmd.Flags().GetString("out")
            pretty, _ := cmd.Flags().GetBool("pretty")
            data, err := config.GenerateSchema(pretty)
            if err != nil {
                return err
            }
            if outPath == "" {
                // Print to stdout
                fmt.Fprintln(cmd.OutOrStdout(), string(data))
                return nil
            }
            // Ensure directory exists
            if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
                return err
            }
            if err := os.WriteFile(outPath, data, 0o644); err != nil {
                return err
            }
            fmt.Fprintf(cmd.OutOrStdout(), "Schema written to %s\n", outPath)
            return nil
        },
    }
    cmd.Flags().String("out", "", "output file path (default stdout)")
    cmd.Flags().Bool("pretty", false, "pretty print JSON schema")
    return cmd
}
