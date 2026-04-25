package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClusterExportCommandShape(t *testing.T) {
	cmd := newClusterExportCmd()

	if cmd == nil {
		t.Fatal("export command should not be nil")
	}
	if cmd.Use != "export [name]" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "export [name]")
	}
	if cmd.Flags().Lookup("output-file") == nil {
		t.Fatal("expected --output-file flag")
	}
	if cmd.Flags().Lookup("output") != nil {
		t.Fatal("did not expect legacy --output flag")
	}
}

func TestClusterExportPrintsEffectiveConfigToStdout(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveKindConfigForCommandTest(t, dir, "export-stdout", "opencenter")

	cmd := newClusterExportCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export-stdout"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster export failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "schema_version: \"2.0\"") {
		t.Fatalf("expected effective config on stdout, got:\n%s", got)
	}
	if !strings.Contains(got, "name: export-stdout") {
		t.Fatalf("expected exported cluster name, got:\n%s", got)
	}
	if strings.Contains(got, "Effective configuration exported to") {
		t.Fatalf("did not expect file export summary on stdout export, got:\n%s", got)
	}
}

func TestClusterExportWritesOutputFile(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveKindConfigForCommandTest(t, dir, "export-file", "opencenter")

	outputPath := filepath.Join(dir, "effective.yaml")
	cmd := newClusterExportCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export-file", "--output-file", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster export --output-file failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(data), "schema_version: \"2.0\"") {
		t.Fatalf("expected effective config in output file, got:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "Effective configuration exported to:") {
		t.Fatalf("expected export summary, got:\n%s", out.String())
	}
}
