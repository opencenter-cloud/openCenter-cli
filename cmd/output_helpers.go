package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func writeStructuredOutput(cmd *cobra.Command, format OutputFormat, value any) error {
	switch format {
	case OutputJSON:
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return fmt.Errorf("format json output: %w", err)
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(encoded)); err != nil {
			return fmt.Errorf("write json output: %w", err)
		}
		return nil
	case OutputYAML:
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return fmt.Errorf("format yaml output: %w", err)
		}
		if _, err := fmt.Fprint(cmd.OutOrStdout(), string(encoded)); err != nil {
			return fmt.Errorf("write yaml output: %w", err)
		}
		return nil
	case OutputText:
		return fmt.Errorf("writeStructuredOutput does not render text output")
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
