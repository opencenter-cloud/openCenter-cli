package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteStructuredOutputJSON(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	err := writeStructuredOutput(cmd, OutputJSON, map[string]string{"name": "prod"})
	if err != nil {
		t.Fatalf("writeStructuredOutput() error = %v", err)
	}
	if !strings.Contains(out.String(), `"name": "prod"`) {
		t.Fatalf("expected JSON output, got %q", out.String())
	}
}

func TestWriteStructuredOutputYAML(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	err := writeStructuredOutput(cmd, OutputYAML, map[string]string{"name": "prod"})
	if err != nil {
		t.Fatalf("writeStructuredOutput() error = %v", err)
	}
	if !strings.Contains(out.String(), "name: prod") {
		t.Fatalf("expected YAML output, got %q", out.String())
	}
}

func TestWriteStructuredOutputJSONReturnsWriteError(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(failingWriter{})

	err := writeStructuredOutput(cmd, OutputJSON, map[string]string{"name": "prod"})
	if err == nil {
		t.Fatal("writeStructuredOutput() error = nil, want write error")
	}
	if !strings.Contains(err.Error(), "write json output") {
		t.Fatalf("expected JSON write error context, got %q", err.Error())
	}
}

func TestWriteStructuredOutputYAMLReturnsWriteError(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(failingWriter{})

	err := writeStructuredOutput(cmd, OutputYAML, map[string]string{"name": "prod"})
	if err == nil {
		t.Fatal("writeStructuredOutput() error = nil, want write error")
	}
	if !strings.Contains(err.Error(), "write yaml output") {
		t.Fatalf("expected YAML write error context, got %q", err.Error())
	}
}
