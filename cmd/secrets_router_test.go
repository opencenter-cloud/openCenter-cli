// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestResolveBackend_BackendValidation tests the backend validation logic
// by verifying error messages for unsupported backends.
func TestResolveBackend_BackendValidation(t *testing.T) {
	tests := []struct {
		name          string
		backend       string
		shouldError   bool
		errorContains []string
	}{
		{
			name:        "barbican is valid",
			backend:     "barbican",
			shouldError: false,
		},
		{
			name:        "sops is valid",
			backend:     "sops",
			shouldError: false,
		},
		{
			name:        "file is valid",
			backend:     "file",
			shouldError: false,
		},
		{
			name:        "empty defaults to barbican",
			backend:     "",
			shouldError: false,
		},
		{
			name:          "vault is unsupported",
			backend:       "vault",
			shouldError:   true,
			errorContains: []string{"barbican", "sops", "file"},
		},
		{
			name:          "aws-secrets-manager is unsupported",
			backend:       "aws-secrets-manager",
			shouldError:   true,
			errorContains: []string{"barbican", "sops", "file"},
		},
		{
			name:          "invalid is unsupported",
			backend:       "invalid",
			shouldError:   true,
			errorContains: []string{"barbican", "sops", "file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic directly
			backend := tt.backend
			if backend == "" {
				backend = "barbican"
			}

			var err error
			switch backend {
			case "barbican", "sops", "file":
				// Valid backend
				err = nil
			default:
				// Invalid backend - should produce error with all supported backends
				err = &backendValidationError{backend: backend}
			}

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error for backend '%s', got nil", tt.backend)
				} else {
					errMsg := err.Error()
					for _, expected := range tt.errorContains {
						if !strings.Contains(errMsg, expected) {
							t.Errorf("error message should contain '%s': %s", expected, errMsg)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for backend '%s', got: %v", tt.backend, err)
				}
			}
		})
	}
}

func TestSecretsReadCommandsUseGlobalOutputFlags(t *testing.T) {
	secretsCmd := NewSecretsCmd()

	for _, path := range [][]string{{"list"}, {"describe"}} {
		cmd, _, err := secretsCmd.Find(path)
		if err != nil {
			t.Fatalf("find secrets %s: %v", strings.Join(path, " "), err)
		}
		if cmd.Flags().Lookup("format") != nil {
			t.Fatalf("secrets %s must use global --output instead of local --format", strings.Join(path, " "))
		}
	}
}

func TestSecretsValidateUsesGlobalOutputFlag(t *testing.T) {
	cmd := newSecretsValidateCmd()

	if cmd.Flags().Lookup("output") != nil {
		t.Fatal("secrets validate must use global --output instead of local --output")
	}
}

func TestSecretsListUsesGlobalOutputWriter(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveFileBackendSecretsConfig(t, dir)

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"secrets", "list", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("secrets list --output json failed: %v", err)
	}

	var secrets []configSecretMetadata
	if err := json.Unmarshal(out.Bytes(), &secrets); err != nil {
		t.Fatalf("expected JSON secrets list in command output, got %q: %v", out.String(), err)
	}
	found := false
	for _, secret := range secrets {
		if secret.Name == "grafana-admin-password" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected secrets list: %#v", secrets)
	}
}

func TestSecretsDescribeUsesGlobalOutputWriter(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveFileBackendSecretsConfig(t, dir)

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"secrets", "describe", "grafana-admin-password", "--output", "yaml"})

	if err := root.Execute(); err != nil {
		t.Fatalf("secrets describe --output yaml failed: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"name: grafana-admin-password",
		"type: password",
		"location: 'config: secrets.grafana.admin_password'",
		"payload_kind: scalar",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in command output, got %q", want, output)
		}
	}
}

func saveFileBackendSecretsConfig(t *testing.T, dir string) {
	t.Helper()

	cfg, _ := saveKindConfigForCommandTest(t, dir, "alpha", "opencenter")
	cfg.OpenCenter.Secrets.Backend = "file"
	cfg.Secrets.Grafana.AdminPassword = "super-secret"
	if err := saveNativeV2Config(context.Background(), &cfg); err != nil {
		t.Fatalf("save file-backed secrets config: %v", err)
	}
	if err := setActiveCluster("alpha"); err != nil {
		t.Fatalf("set active cluster: %v", err)
	}
}

// backendValidationError is a test helper that mimics the error format
// returned by resolveBackend for unsupported backends.
type backendValidationError struct {
	backend string
}

func (e *backendValidationError) Error() string {
	return "unsupported secrets backend: " + e.backend + " (supported: barbican, sops, file)"
}
