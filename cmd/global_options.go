package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type OutputFormat string

const (
	OutputText OutputFormat = "text"
	OutputJSON OutputFormat = "json"
	OutputYAML OutputFormat = "yaml"
)

type GlobalOptions struct {
	ConfigDir string
	LogLevel  string
	Output    OutputFormat
	Quiet     bool
	Yes       bool
	DryRun    bool
}

type globalOptionsContextKey struct{}

const readOnlyAnnotation = "opencenter.readOnly"

func markReadOnlyCommand(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[readOnlyAnnotation] = "true"
}

func commandIsReadOnly(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Annotations == nil {
		return false
	}
	return cmd.Annotations[readOnlyAnnotation] == "true"
}

func parseGlobalOptions(cmd *cobra.Command) (GlobalOptions, error) {
	configDir := globalFlagValue(cmd, "config-dir")
	logLevel := globalFlagValue(cmd, "log-level")
	outputRaw := globalFlagValue(cmd, "output")
	quiet := globalBoolFlagValue(cmd, "quiet")
	yes := globalBoolFlagValue(cmd, "yes")
	dryRun := globalBoolFlagValue(cmd, "dry-run")

	if strings.TrimSpace(outputRaw) == "" {
		outputRaw = string(OutputText)
	}
	output := OutputFormat(strings.ToLower(strings.TrimSpace(outputRaw)))
	switch output {
	case OutputText, OutputJSON, OutputYAML:
	default:
		return GlobalOptions{}, fmt.Errorf("unsupported output format %q (supported: text, json, yaml)", outputRaw)
	}

	return GlobalOptions{
		ConfigDir: configDir,
		LogLevel:  logLevel,
		Output:    output,
		Quiet:     quiet,
		Yes:       yes,
		DryRun:    dryRun,
	}, nil
}

func globalFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if cmd == nil {
		return nil
	}
	return cmd.Root().PersistentFlags().Lookup(name)
}

func globalFlagValue(cmd *cobra.Command, name string) string {
	if flag := globalFlag(cmd, name); flag != nil {
		return flag.Value.String()
	}
	return ""
}

func globalBoolFlagValue(cmd *cobra.Command, name string) bool {
	return globalFlagValue(cmd, name) == "true"
}

func globalFlagChanged(cmd *cobra.Command, name string) bool {
	if flag := globalFlag(cmd, name); flag != nil {
		return flag.Changed
	}
	return false
}

func applyGlobalOptions(cmd *cobra.Command, args []string) error {
	opts, err := parseGlobalOptions(cmd)
	if err != nil {
		return err
	}

	if opts.LogLevel == "warn" && !globalFlagChanged(cmd, "log-level") {
		if envLevel := os.Getenv("OPENCENTER_LOG_LEVEL"); envLevel != "" {
			opts.LogLevel = envLevel
		}
	}
	if opts.LogLevel != "" {
		if err := config.SetLogLevel(opts.LogLevel); err != nil {
			return fmt.Errorf("failed to set log level: %w", err)
		}
	}

	ctx := context.WithValue(cmd.Context(), globalOptionsContextKey{}, opts)
	cmd.SetContext(ctx)

	if opts.DryRun {
		if err := rejectMeaninglessDryRun(cmd); err != nil {
			return err
		}
	}

	config.Debugf("Command: %s", cmd.CommandPath())
	config.Debugf("Arguments: %v", args)
	config.Debugf("Global output: %s", opts.Output)
	config.Debugf("Global dry-run: %v", opts.DryRun)
	return nil
}

func getGlobalOptions(cmd *cobra.Command) GlobalOptions {
	if opts, ok := cmd.Context().Value(globalOptionsContextKey{}).(GlobalOptions); ok {
		return opts
	}
	return GlobalOptions{LogLevel: "warn", Output: OutputText}
}

func rejectMeaninglessDryRun(cmd *cobra.Command) error {
	if !commandIsReadOnly(cmd) {
		return nil
	}
	return fmt.Errorf("--dry-run has no effect for read-only command %q", cmd.CommandPath())
}
