package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveSecretsLoginPasswordRequiresExplicitStdin(t *testing.T) {
	_, err := resolveSecretsLoginPassword(
		strings.NewReader("ignored"),
		false,
		false,
		0,
		&bytes.Buffer{},
		func(_ int) ([]byte, error) {
			t.Fatal("readPassword should not be called")
			return nil, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "--password-stdin") {
		t.Fatalf("expected non-interactive stdin error, got %v", err)
	}
}

func TestResolveSecretsLoginPasswordReadsExplicitStdin(t *testing.T) {
	password, err := resolveSecretsLoginPassword(
		strings.NewReader("  super-secret \n"),
		true,
		false,
		0,
		&bytes.Buffer{},
		func(_ int) ([]byte, error) {
			t.Fatal("readPassword should not be called")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("resolveSecretsLoginPassword() error = %v", err)
	}
	if password != "super-secret" {
		t.Fatalf("password = %q, want %q", password, "super-secret")
	}
}

func TestResolveSecretsLoginPasswordPromptsInteractively(t *testing.T) {
	var prompt bytes.Buffer

	password, err := resolveSecretsLoginPassword(
		strings.NewReader("ignored"),
		false,
		true,
		123,
		&prompt,
		func(fd int) ([]byte, error) {
			if fd != 123 {
				t.Fatalf("fd = %d, want 123", fd)
			}
			return []byte("prompted-secret"), nil
		},
	)
	if err != nil {
		t.Fatalf("resolveSecretsLoginPassword() error = %v", err)
	}
	if password != "prompted-secret" {
		t.Fatalf("password = %q, want %q", password, "prompted-secret")
	}
	if !strings.Contains(prompt.String(), "OpenStack password:") {
		t.Fatalf("expected prompt output, got %q", prompt.String())
	}
}
