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

package validators_test

import (
	"context"
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

// ExampleSecurityValidator_pathTraversal demonstrates path traversal detection.
func ExampleSecurityValidator_pathTraversal() {
	validator := validators.NewSecurityValidator()
	ctx := context.Background()

	// Attempt to validate input with path traversal
	result, _ := validator.Validate(ctx, map[string]interface{}{
		"type":  "shell-input",
		"value": "../../../etc/passwd",
	})

	fmt.Printf("Valid: %v\n", result.Valid)
	if len(result.Errors) > 0 {
		fmt.Printf("Error detected: contains path traversal pattern\n")
	}

	// Output:
	// Valid: false
	// Error detected: contains path traversal pattern
}

// ExampleSecurityValidator_commandInjection demonstrates command injection detection.
func ExampleSecurityValidator_commandInjection() {
	validator := validators.NewSecurityValidator()
	ctx := context.Background()

	// Attempt to validate input with command injection
	result, _ := validator.Validate(ctx, map[string]interface{}{
		"type":  "shell-input",
		"value": "cluster; rm -rf /",
	})

	fmt.Printf("Valid: %v\n", result.Valid)
	if len(result.Errors) > 0 {
		fmt.Printf("Error detected: dangerous shell metacharacter\n")
	}

	// Output:
	// Valid: false
	// Error detected: dangerous shell metacharacter
}

// ExampleSecurityValidator_safeInput demonstrates validation of safe input.
func ExampleSecurityValidator_safeInput() {
	validator := validators.NewSecurityValidator()
	ctx := context.Background()

	// Validate safe input
	result, _ := validator.Validate(ctx, map[string]interface{}{
		"type":  "shell-input",
		"value": "my-cluster-01",
	})

	fmt.Printf("Valid: %v\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))

	// Output:
	// Valid: true
	// Errors: 0
}

// ExampleSecurityValidator_withAuditLogging demonstrates audit logging integration.
func ExampleSecurityValidator_withAuditLogging() {
	validator := validators.NewSecurityValidator()
	ctx := context.Background()

	// Mock audit logger that prints violations
	mockLogger := &mockAuditLogger{
		logFunc: func(ctx context.Context, actor, inputType, reason string) error {
			fmt.Printf("Audit log: actor=%s, type=%s\n", actor, inputType)
			return nil
		},
	}

	validator.SetAuditLogger(mockLogger)
	validator.SetActor("admin-user")

	// Validate malicious input - will be logged
	result, _ := validator.Validate(ctx, map[string]interface{}{
		"type":  "command",
		"value": "echo $(whoami)",
	})

	fmt.Printf("Valid: %v\n", result.Valid)

	// Output:
	// Audit log: actor=admin-user, type=command
	// Valid: false
}

// mockAuditLogger is a mock implementation for examples
type mockAuditLogger struct {
	logFunc func(ctx context.Context, actor, inputType, reason string) error
}

func (m *mockAuditLogger) LogInputRejected(ctx context.Context, actor, inputType, reason string) error {
	if m.logFunc != nil {
		return m.logFunc(ctx, actor, inputType, reason)
	}
	return nil
}
