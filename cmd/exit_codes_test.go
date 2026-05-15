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
	"errors"
	"fmt"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	utilErrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
)

// determineExitCode mirrors the exit code mapping logic from main.go.
// Exit code 3: ConfigNotFoundError (missing cluster configuration)
// Exit code 2: validation failures (StructuredError with ValidationError type)
// Exit code 1: all other errors (general failures)
//
// Requirements: 17.1, 17.2
func determineExitCode(err error) int {
	if err == nil {
		return 0
	}

	// Exit code 3: missing cluster configuration
	var cnfErr *v2.ConfigNotFoundError
	if errors.As(err, &cnfErr) {
		return 3
	}

	// Exit code 2: validation failure
	if v2.IsValidationError(err) {
		return 2
	}

	// Exit code 1: general error (default)
	return 1
}

// TestExitCode_ConfigNotFound verifies that ConfigNotFoundError maps to exit code 3.
// Requirements: 17.1
func TestExitCode_ConfigNotFound(t *testing.T) {
	err := v2.NewConfigNotFoundError("test-cluster", fmt.Errorf("no such directory"))
	got := determineExitCode(err)
	if got != 3 {
		t.Errorf("ConfigNotFoundError: expected exit code 3, got %d", got)
	}
}

// TestExitCode_WrappedConfigNotFound verifies that a wrapped ConfigNotFoundError
// still maps to exit code 3 via errors.As unwrapping.
// Requirements: 17.1
func TestExitCode_WrappedConfigNotFound(t *testing.T) {
	inner := v2.NewConfigNotFoundError("prod-cluster", nil)
	wrapped := fmt.Errorf("command failed: %w", inner)
	got := determineExitCode(wrapped)
	if got != 3 {
		t.Errorf("wrapped ConfigNotFoundError: expected exit code 3, got %d", got)
	}
}

// TestExitCode_ValidationFailure verifies that validation errors map to exit code 2.
// Requirements: 17.2
func TestExitCode_ValidationFailure(t *testing.T) {
	err := v2.NewValidationError("cluster.name", "cluster name cannot be empty", nil)
	got := determineExitCode(err)
	if got != 2 {
		t.Errorf("validation error: expected exit code 2, got %d", got)
	}
}

// TestExitCode_GeneralError verifies that non-specific errors map to exit code 1.
// Requirements: 17.2
func TestExitCode_GeneralError(t *testing.T) {
	err := fmt.Errorf("network timeout connecting to API")
	got := determineExitCode(err)
	if got != 1 {
		t.Errorf("general error: expected exit code 1, got %d", got)
	}
}

// TestExitCode_NilError verifies that nil error maps to exit code 0 (success).
func TestExitCode_NilError(t *testing.T) {
	got := determineExitCode(nil)
	if got != 0 {
		t.Errorf("nil error: expected exit code 0, got %d", got)
	}
}

// TestExitCode_Consistency runs a table-driven test covering all exit code categories.
// Requirements: 17.1, 17.2
func TestExitCode_Consistency(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil returns 0",
			err:      nil,
			wantCode: 0,
		},
		{
			name:     "ConfigNotFoundError returns 3",
			err:      v2.NewConfigNotFoundError("missing-cluster", nil),
			wantCode: 3,
		},
		{
			name:     "ConfigNotFoundError with cause returns 3",
			err:      v2.NewConfigNotFoundError("gone-cluster", fmt.Errorf("ENOENT")),
			wantCode: 3,
		},
		{
			name:     "wrapped ConfigNotFoundError returns 3",
			err:      fmt.Errorf("load: %w", v2.NewConfigNotFoundError("deep", nil)),
			wantCode: 3,
		},
		{
			name: "validation StructuredError returns 2",
			err: &utilErrors.StructuredError{
				Type:    utilErrors.ValidationError,
				Message: "field required",
			},
			wantCode: 2,
		},
		{
			name:     "NewValidationError helper returns 2",
			err:      v2.NewValidationError("provider", "invalid provider", nil),
			wantCode: 2,
		},
		{
			name:     "plain error returns 1",
			err:      fmt.Errorf("something broke"),
			wantCode: 1,
		},
		{
			name: "file error returns 1",
			err: &utilErrors.StructuredError{
				Type:    utilErrors.FileError,
				Message: "permission denied",
			},
			wantCode: 1,
		},
		{
			name: "network error returns 1",
			err: &utilErrors.StructuredError{
				Type:    utilErrors.NetworkError,
				Message: "connection refused",
			},
			wantCode: 1,
		},
		{
			name: "config error (non-validation) returns 1",
			err: &utilErrors.StructuredError{
				Type:    utilErrors.ConfigError,
				Message: "corrupt YAML",
			},
			wantCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := determineExitCode(tc.err)
			if got != tc.wantCode {
				t.Errorf("determineExitCode() = %d, want %d", got, tc.wantCode)
			}
		})
	}
}

// TestExitCode_ConfigNotFoundPreservesClusterName verifies that the cluster name
// is accessible from ConfigNotFoundError after exit code classification.
// Requirements: 17.1
func TestExitCode_ConfigNotFoundPreservesClusterName(t *testing.T) {
	clusterName := "my-important-cluster"
	err := v2.NewConfigNotFoundError(clusterName, nil)

	// Verify exit code
	if got := determineExitCode(err); got != 3 {
		t.Fatalf("expected exit code 3, got %d", got)
	}

	// Verify cluster name is extractable (as main.go does for the help message)
	var cnfErr *v2.ConfigNotFoundError
	if !errors.As(err, &cnfErr) {
		t.Fatal("errors.As failed to extract ConfigNotFoundError")
	}
	if cnfErr.ClusterName != clusterName {
		t.Errorf("ClusterName = %q, want %q", cnfErr.ClusterName, clusterName)
	}
}

// TestExitCode_MutualExclusivity verifies that ConfigNotFoundError takes
// precedence — it cannot simultaneously be classified as a validation error.
// Requirements: 17.1, 17.2
func TestExitCode_MutualExclusivity(t *testing.T) {
	// ConfigNotFoundError is not a StructuredError, so IsValidationError is false.
	err := v2.NewConfigNotFoundError("test", nil)

	if v2.IsValidationError(err) {
		t.Error("ConfigNotFoundError should not be classified as a validation error")
	}

	if got := determineExitCode(err); got != 3 {
		t.Errorf("ConfigNotFoundError should map to exit code 3, got %d", got)
	}
}
