package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseGlobalOptions(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addGlobalFlags(cmd)

	if err := cmd.ParseFlags([]string{
		"--config-dir", "/tmp/opencenter",
		"--log-level", "debug",
		"--output", "json",
		"--quiet",
		"--yes",
		"--dry-run",
	}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseGlobalOptions(cmd)
	if err != nil {
		t.Fatalf("parseGlobalOptions() error = %v", err)
	}

	if opts.ConfigDir != "/tmp/opencenter" {
		t.Fatalf("ConfigDir = %q, want /tmp/opencenter", opts.ConfigDir)
	}
	if opts.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", opts.LogLevel)
	}
	if opts.Output != OutputJSON {
		t.Fatalf("Output = %q, want %q", opts.Output, OutputJSON)
	}
	if !opts.Quiet || !opts.Yes || !opts.DryRun {
		t.Fatalf("expected quiet, yes, and dry-run to be true: %#v", opts)
	}
}

func TestParseGlobalOptionsRejectsInvalidOutput(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addGlobalFlags(cmd)

	if err := cmd.ParseFlags([]string{"--output", "xml"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	_, err := parseGlobalOptions(cmd)
	if err == nil {
		t.Fatal("expected invalid output format error")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("expected unsupported output format error, got %v", err)
	}
}

func TestApplyGlobalOptionsDoesNotOverrideExplicitWarnLogLevel(t *testing.T) {
	t.Setenv("OPENCENTER_LOG_LEVEL", "debug")

	cmd := &cobra.Command{Use: "test"}
	addGlobalFlags(cmd)
	cmd.SetContext(context.Background())

	if err := cmd.ParseFlags([]string{"--log-level", "warn"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	if err := applyGlobalOptions(cmd, nil); err != nil {
		t.Fatalf("applyGlobalOptions() error = %v", err)
	}

	opts := getGlobalOptions(cmd)
	if opts.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want explicit warn", opts.LogLevel)
	}
}

func TestParseGlobalOptionsIgnoresChildLocalOutputFlag(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	addGlobalFlags(root)
	child := &cobra.Command{Use: "child"}
	child.Flags().String("output", "", "local output file")
	root.AddCommand(child)

	if err := child.ParseFlags([]string{"--output", "/tmp/file.yaml"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseGlobalOptions(child)
	if err != nil {
		t.Fatalf("parseGlobalOptions() error = %v", err)
	}

	if opts.Output != OutputText {
		t.Fatalf("Output = %q, want %q", opts.Output, OutputText)
	}
}

func TestRejectDryRunForReadOnlyCommand(t *testing.T) {
	cmd := &cobra.Command{Use: "list"}
	addGlobalFlags(cmd)
	markReadOnlyCommand(cmd)

	if err := cmd.ParseFlags([]string{"--dry-run"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	opts, err := parseGlobalOptions(cmd)
	if err != nil {
		t.Fatalf("parseGlobalOptions() error = %v", err)
	}
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, opts))

	err = rejectMeaninglessDryRun(cmd)
	if err == nil {
		t.Fatal("expected read-only dry-run rejection")
	}
	if !strings.Contains(err.Error(), `--dry-run has no effect for read-only command "list"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
