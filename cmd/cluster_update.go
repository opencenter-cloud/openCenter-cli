package cmd

import (
    "fmt"
    "os"
    "strings"

    "github.com/rackerlabs/openCenter/internal/config"
    "github.com/spf13/cobra"
)

// newClusterUpdateCmd updates fields in an existing cluster configuration using
// dynamic dotted flags (e.g., --iac.counts.master=3). If a name is not
// provided, the active cluster is used.
func newClusterUpdateCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "update [name]",
        Short: "Update fields in an existing cluster configuration",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            var name string
            if len(args) > 0 {
                name = args[0]
            }
            // Resolve name from active if not provided
            if name == "" {
                active, err := config.GetActive()
                if err != nil {
                    return err
                }
                if active == "" {
                    return failf("no active cluster; specify a name or select a cluster")
                }
                name = active
            }

            cfg, err := config.Load(name)
            if err != nil {
                return fmt.Errorf("failed to load cluster %s: %w", name, err)
            }

            // Apply overrides from flags by inspecting os.Args similar to init
            for _, arg := range os.Args {
                if strings.HasPrefix(arg, "--") {
                    parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
                    if len(parts) == 2 {
                        key, value := parts[0], parts[1]
                        if cmd.Flags().Lookup(key) != nil {
                            continue
                        }
                        if err := setField(&cfg, key, value); err != nil {
                            return fmt.Errorf("error setting config from flag '%s': %w", key, err)
                        }
                    }
                }
            }

            // Optional strict validation
            strict, _ := cmd.Flags().GetBool("strict")
            if strict {
                if errs := config.Validate(cfg); len(errs) > 0 {
                    for _, e := range errs {
                        fmt.Fprintln(cmd.ErrOrStderr(), e)
                    }
                    return fmt.Errorf("validation failed")
                }
            }

            if err := config.Save(cfg); err != nil {
                return err
            }
            fmt.Fprintf(cmd.OutOrStdout(), "Updated cluster configuration %s\n", name)
            return nil
        },
    }
    cmd.Flags().Bool("strict", false, "fail if the resulting configuration is not valid")
    return cmd
}

