package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// extractKeysFromSchema recursively extracts all field keys from a schema section
func extractKeysFromSchema(schema map[string]any, prefix string, keys *[]string) {
	if properties, ok := schema["properties"].(map[string]any); ok {
		for key, value := range properties {
			fullKey := key
			if prefix != "" {
				fullKey = prefix + "." + key
			}
			*keys = append(*keys, fullKey)

			if valueMap, ok := value.(map[string]any); ok {
				extractKeysFromSchema(valueMap, fullKey, keys)
			}
		}
	}
}

// getIACKeys returns all available iac keys excluding those that exist in opencenter
func getIACKeys() []string {
	schema, err := config.GenerateSchema(false)
	if err != nil {
		return []string{"Error loading schema"}
	}

	var schemaData map[string]any
	if err := json.Unmarshal(schema, &schemaData); err != nil {
		return []string{"Error parsing schema"}
	}

	properties, ok := schemaData["properties"].(map[string]any)
	if !ok {
		return []string{"Error accessing schema properties"}
	}

	// Extract all iac keys
	var iacKeys []string
	if iacSchema, ok := properties["iac"].(map[string]any); ok {
		extractKeysFromSchema(iacSchema, "", &iacKeys)
	}

	// Extract all opencenter keys (flattened, just key names without paths)
	var opencenterKeys []string
	if opencenterSchema, ok := properties["opencenter"].(map[string]any); ok {
		extractKeysFromSchema(opencenterSchema, "", &opencenterKeys)
	}

	// Create a set of opencenter key names (just the final part after last dot)
	opencenterKeySet := make(map[string]bool)
	for _, key := range opencenterKeys {
		parts := strings.Split(key, ".")
		keyName := parts[len(parts)-1]
		opencenterKeySet[keyName] = true
	}

	// Filter iac keys to exclude those that exist in opencenter
	var filteredIACKeys []string
	for _, key := range iacKeys {
		parts := strings.Split(key, ".")
		keyName := parts[len(parts)-1]
		if !opencenterKeySet[keyName] {
			filteredIACKeys = append(filteredIACKeys, "iac."+key)
		}
	}

	sort.Strings(filteredIACKeys)
	return filteredIACKeys
}

// showUpdateHelp displays enhanced help with available IAC schema values
func showUpdateHelp(cmd *cobra.Command) error {
	// Show standard help first
	fmt.Fprint(cmd.OutOrStdout(), cmd.Long)
	if cmd.Long == "" {
		fmt.Fprint(cmd.OutOrStdout(), cmd.Short)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintf(cmd.OutOrStdout(), "Usage:\n  %s\n\n", cmd.UseLine())

	// Show flags
	if cmd.Flags().HasFlags() {
		fmt.Fprintln(cmd.OutOrStdout(), "Flags:")
		fmt.Fprintln(cmd.OutOrStdout(), cmd.Flags().FlagUsages())
	}

	// Show global flags
	if cmd.HasParent() && cmd.Parent().PersistentFlags().HasFlags() {
		fmt.Fprintln(cmd.OutOrStdout(), "Global Flags:")
		fmt.Fprintln(cmd.OutOrStdout(), cmd.Parent().PersistentFlags().FlagUsages())
	}

	// Show available IAC schema values
	fmt.Fprintln(cmd.OutOrStdout(), "Available IAC Configuration Keys:")
	fmt.Fprintln(cmd.OutOrStdout(), "  Use any of the following keys with --<key>=<value> format:")
	fmt.Fprintln(cmd.OutOrStdout())

	iacKeys := getIACKeys()
	for _, key := range iacKeys {
		fmt.Fprintf(cmd.OutOrStdout(), "  --%s=<value>\n", key)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Examples:")
	fmt.Fprintln(cmd.OutOrStdout(), "  openCenter cluster update --iac.main.master_count=5")
	fmt.Fprintln(cmd.OutOrStdout(), "  openCenter cluster update my-cluster --iac.main.worker_count=3")
	fmt.Fprintln(cmd.OutOrStdout(), "  openCenter cluster update --iac.main.kubernetes_version=1.30.4")

	return nil
}

// newClusterUpdateCmd updates fields in an existing cluster configuration using
// dynamic dotted flags (e.g., --iac.counts.master=3). If a name is not
// provided, the active cluster is used.
func newClusterUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update fields in an existing cluster configuration",
		Args:  cobra.MaximumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if any update flags are provided by inspecting os.Args
			hasUpdateFlags := false
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "--") {
					parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
					if len(parts) >= 1 {
						key := parts[0]
						// Skip built-in flags
						if cmd.Flags().Lookup(key) == nil && key != "help" && key != "config-dir" {
							hasUpdateFlags = true
							break
						}
					}
				}
			}

			// If no cluster name provided and no update flags, show enhanced help
			if len(args) == 0 && !hasUpdateFlags {
				return showUpdateHelp(cmd)
			}

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
