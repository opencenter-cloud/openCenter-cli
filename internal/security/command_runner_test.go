package security

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDefaultCommandRunner_PrepareCommandContext(t *testing.T) {
	runner := NewCommandRunner(nil)

	cmd, err := runner.PrepareCommandContext(context.Background(), "git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("PrepareCommandContext() error = %v", err)
	}

	if filepath.Base(cmd.Path) != "git" {
		t.Fatalf("PrepareCommandContext() path = %q, want basename git", cmd.Path)
	}

	if len(cmd.Args) != 3 {
		t.Fatalf("PrepareCommandContext() args length = %d, want 3", len(cmd.Args))
	}
}

func TestDefaultCommandRunner_RejectsShellExecution(t *testing.T) {
	runner := NewCommandRunner(nil)

	if _, err := runner.PrepareCommandContext(context.Background(), "sh", "-c", "echo nope"); err == nil {
		t.Fatal("PrepareCommandContext() should reject shell execution")
	}
}
