package sops

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	utilerrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
)

func TestEncryptServiceOverrideValues_ReportsMissingSOPSBinary(t *testing.T) {
	// Force PATH to an empty directory so exec.LookPath("sops") fails
	// deterministically regardless of the developer's local environment.
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))

	cfg := newSOPSTestConfig("missing-sops", "baremetal", "")
	manager := NewSOPSManager()

	err := manager.EncryptServiceOverrideValues(context.Background(), t.TempDir(), cfg)
	if err == nil {
		t.Fatal("expected error when sops is not on PATH, got nil")
	}

	var se *utilerrors.StructuredError
	if !errors.As(err, &se) {
		t.Fatalf("expected StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message, "sops binary not found") {
		t.Fatalf("expected message to mention missing sops binary, got %q", se.Message)
	}
	if len(se.Suggestions) == 0 {
		t.Fatalf("expected install suggestions, got none")
	}
}
