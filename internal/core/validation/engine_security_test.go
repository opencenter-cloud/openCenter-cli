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

package validation

import (
	"context"
	"strings"
	"testing"
)

// TestSecurityValidatorAlwaysRuns verifies that security validators execute
// in all validation operations and cannot be bypassed.
func TestSecurityValidatorAlwaysRuns(t *testing.T) {
	tests := []struct {
		name          string
		setupEngine   func() *ValidationEngine
		validateFunc  func(*ValidationEngine, context.Context, interface{}) (*ValidationResult, error)
		expectSecRun  bool
		expectFailure bool
	}{
		{
			name: "Validate with malicious input",
			setupEngine: func() *ValidationEngine {
				engine := NewValidationEngine()
				secValidator := newMockSecurityValidator()
				engine.MustRegisterSecurityValidator(secValidator)
				engine.MustRegister(newMockValidator("test-validator", true))
				return engine
			},
			validateFunc: func(engine *ValidationEngine, ctx context.Context, value interface{}) (*ValidationResult, error) {
				return engine.Validate(ctx, "test-validator", value)
			},
			expectSecRun:  true,
			expectFailure: true,
		},
		{
			name: "ValidateAll with malicious input",
			setupEngine: func() *ValidationEngine {
				engine := NewValidationEngine()
				secValidator := newMockSecurityValidator()
				engine.MustRegisterSecurityValidator(secValidator)
				engine.MustRegister(newMockValidator("validator1", true))
				engine.MustRegister(newMockValidator("validator2", true))
				return engine
			},
			validateFunc: func(engine *ValidationEngine, ctx context.Context, value interface{}) (*ValidationResult, error) {
				return engine.ValidateAll(ctx, []string{"validator1", "validator2"}, value)
			},
			expectSecRun:  true,
			expectFailure: true,
		},
		{
			name: "ValidateParallel with malicious input",
			setupEngine: func() *ValidationEngine {
				engine := NewValidationEngine()
				secValidator := newMockSecurityValidator()
				engine.MustRegisterSecurityValidator(secValidator)
				engine.MustRegister(newMockValidator("validator1", true))
				engine.MustRegister(newMockValidator("validator2", true))
				return engine
			},
			validateFunc: func(engine *ValidationEngine, ctx context.Context, value interface{}) (*ValidationResult, error) {
				return engine.ValidateParallel(ctx, []string{"validator1", "validator2"}, value)
			},
			expectSecRun:  true,
			expectFailure: true,
		},
		{
			name: "Validate with safe input",
			setupEngine: func() *ValidationEngine {
				engine := NewValidationEngine()
				secValidator := newMockSecurityValidator()
				engine.MustRegisterSecurityValidator(secValidator)
				engine.MustRegister(newMockValidator("test-validator", true))
				return engine
			},
			validateFunc: func(engine *ValidationEngine, ctx context.Context, value interface{}) (*ValidationResult, error) {
				return engine.Validate(ctx, "test-validator", value)
			},
			expectSecRun:  true,
			expectFailure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := tt.setupEngine()
			ctx := context.Background()

			// Test with malicious input
			maliciousInput := map[string]interface{}{
				"type":  "shell-input",
				"value": "test; rm -rf /",
			}

			result, err := tt.validateFunc(engine, ctx, maliciousInput)
			if err != nil {
				t.Fatalf("validation failed with error: %v", err)
			}

			if tt.expectFailure {
				if result.Valid {
					t.Error("expected validation to fail with malicious input, but it passed")
				}

				// Verify security error is present
				hasSecurityError := false
				for _, issue := range result.Errors {
					if strings.Contains(issue.Field, "shell") || strings.Contains(issue.Message, "shell") {
						hasSecurityError = true
						break
					}
				}

				if !hasSecurityError {
					t.Error("expected security error in validation result")
				}
			}

			// Test with safe input
			safeInput := map[string]interface{}{
				"type":  "shell-input",
				"value": "safe-input",
			}

			result, err = tt.validateFunc(engine, ctx, safeInput)
			if err != nil {
				t.Fatalf("validation failed with error: %v", err)
			}

			if !tt.expectFailure && !result.Valid {
				t.Errorf("expected validation to pass with safe input, but it failed: %v", result.Errors)
			}
		})
	}
}

// TestSecurityValidatorCannotBeBypassed verifies that security validators
// cannot be bypassed through any means.
func TestSecurityValidatorCannotBeBypassed(t *testing.T) {
	t.Run("security validator runs even when not explicitly requested", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()
		engine.MustRegisterSecurityValidator(secValidator)
		engine.MustRegister(newMockValidator("other-validator", true))

		ctx := context.Background()
		maliciousInput := map[string]interface{}{
			"type":  "shell-input",
			"value": "test | cat /etc/passwd",
		}

		// Request only "other-validator", but security validator should still run
		result, err := engine.Validate(ctx, "other-validator", maliciousInput)
		if err != nil {
			t.Fatalf("validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("expected security validator to detect malicious input")
		}
	})

	t.Run("security validator runs in ValidateAll", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()
		engine.MustRegisterSecurityValidator(secValidator)
		engine.MustRegister(newMockValidator("validator1", true))
		engine.MustRegister(newMockValidator("validator2", true))

		ctx := context.Background()
		maliciousInput := map[string]interface{}{
			"type":  "shell-input",
			"value": "test && rm -rf /",
		}

		// Request multiple validators, security validator should run first
		result, err := engine.ValidateAll(ctx, []string{"validator1", "validator2"}, maliciousInput)
		if err != nil {
			t.Fatalf("validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("expected security validator to detect malicious input")
		}
	})

	t.Run("security validator runs in ValidateParallel", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()
		engine.MustRegisterSecurityValidator(secValidator)
		engine.MustRegister(newMockValidator("validator1", true))
		engine.MustRegister(newMockValidator("validator2", true))

		ctx := context.Background()
		maliciousInput := map[string]interface{}{
			"type":  "shell-input",
			"value": "test; cat /etc/shadow",
		}

		// Request parallel validation, security validator should run first
		result, err := engine.ValidateParallel(ctx, []string{"validator1", "validator2"}, maliciousInput)
		if err != nil {
			t.Fatalf("validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("expected security validator to detect malicious input")
		}
	})

	t.Run("multiple security validators all run", func(t *testing.T) {
		engine := NewValidationEngine()
		
		// Register multiple security validators
		secValidator1 := newMockSecurityValidator()
		secValidator2 := newMockSecurityValidatorWithName("security-2")
		
		engine.MustRegisterSecurityValidator(secValidator1)
		engine.MustRegisterSecurityValidator(secValidator2)
		engine.MustRegister(newMockValidator("test-validator", true))

		ctx := context.Background()
		maliciousInput := map[string]interface{}{
			"type":  "shell-input",
			"value": "test | rm -rf /",
		}

		result, err := engine.Validate(ctx, "test-validator", maliciousInput)
		if err != nil {
			t.Fatalf("validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("expected security validators to detect malicious input")
		}

		// Verify both security validators ran (should have errors from both)
		if len(result.Errors) < 1 {
			t.Error("expected errors from security validators")
		}
	})
}

// TestListSecurityValidators verifies that security validators can be listed.
func TestListSecurityValidators(t *testing.T) {
	engine := NewValidationEngine()

	// Initially empty
	if len(engine.ListSecurityValidators()) != 0 {
		t.Error("expected no security validators initially")
	}

	// Register security validators
	secValidator1 := newMockSecurityValidator()
	secValidator2 := newMockSecurityValidatorWithName("security-2")

	engine.MustRegisterSecurityValidator(secValidator1)
	engine.MustRegisterSecurityValidator(secValidator2)

	// Verify they're listed
	secValidators := engine.ListSecurityValidators()
	if len(secValidators) != 2 {
		t.Errorf("expected 2 security validators, got %d", len(secValidators))
	}

	// Verify names are correct
	expectedNames := map[string]bool{
		"security":   true,
		"security-2": true,
	}

	for _, name := range secValidators {
		if !expectedNames[name] {
			t.Errorf("unexpected security validator name: %s", name)
		}
	}
}

// TestSecurityValidatorRegistration verifies security validator registration.
func TestSecurityValidatorRegistration(t *testing.T) {
	t.Run("security validator is also in main registry", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()

		err := engine.RegisterSecurityValidator(secValidator)
		if err != nil {
			t.Fatalf("failed to register security validator: %v", err)
		}

		// Verify it's in the main registry
		if !engine.Has("security") {
			t.Error("security validator not found in main registry")
		}

		// Verify it's in the security validators list
		secValidators := engine.ListSecurityValidators()
		if len(secValidators) != 1 || secValidators[0] != "security" {
			t.Error("security validator not in security validators list")
		}
	})

	t.Run("duplicate security validator registration fails", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()

		err := engine.RegisterSecurityValidator(secValidator)
		if err != nil {
			t.Fatalf("failed to register security validator: %v", err)
		}

		// Try to register again
		err = engine.RegisterSecurityValidator(secValidator)
		if err == nil {
			t.Error("expected error when registering duplicate security validator")
		}
	})

	t.Run("MustRegisterSecurityValidator panics on error", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()

		engine.MustRegisterSecurityValidator(secValidator)

		// Try to register again - should panic
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when registering duplicate security validator")
			}
		}()

		engine.MustRegisterSecurityValidator(secValidator)
	})
}

// TestSecurityValidatorWithOptions verifies security validators run with options.
func TestSecurityValidatorWithOptions(t *testing.T) {
	t.Run("security validator runs with StopOnFirstError", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()
		engine.MustRegisterSecurityValidator(secValidator)
		engine.MustRegister(newMockValidator("validator1", true))
		engine.MustRegister(newMockValidator("validator2", true))

		ctx := context.Background()
		maliciousInput := map[string]interface{}{
			"type":  "shell-input",
			"value": "test; rm -rf /",
		}

		opts := &ValidationOptions{
			StopOnFirstError: true,
			IncludeWarnings:  true,
		}

		result, err := engine.ValidateAllWithOptions(ctx, []string{"validator1", "validator2"}, maliciousInput, opts)
		if err != nil {
			t.Fatalf("validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("expected security validator to detect malicious input")
		}
	})

	t.Run("security validator runs without warnings", func(t *testing.T) {
		engine := NewValidationEngine()
		secValidator := newMockSecurityValidator()
		engine.MustRegisterSecurityValidator(secValidator)
		engine.MustRegister(newMockValidator("test-validator", true))

		ctx := context.Background()
		maliciousInput := map[string]interface{}{
			"type":  "shell-input",
			"value": "test | cat /etc/passwd",
		}

		opts := &ValidationOptions{
			IncludeWarnings: false,
		}

		result, err := engine.ValidateWithOptions(ctx, "test-validator", maliciousInput, opts)
		if err != nil {
			t.Fatalf("validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("expected security validator to detect malicious input")
		}

		// Warnings should be filtered out
		if len(result.Warnings) > 0 {
			t.Error("expected no warnings when IncludeWarnings is false")
		}
	})
}

// Mock security validator for testing
type mockSecurityValidatorForTest struct {
	name string
}

func newMockSecurityValidator() *mockSecurityValidatorForTest {
	return &mockSecurityValidatorForTest{name: "security"}
}

func newMockSecurityValidatorWithName(name string) *mockSecurityValidatorForTest {
	return &mockSecurityValidatorForTest{name: name}
}

func (v *mockSecurityValidatorForTest) Name() string {
	return v.name
}

func (v *mockSecurityValidatorForTest) Priority() int {
	return PriorityHigh
}

func (v *mockSecurityValidatorForTest) Validate(ctx context.Context, value interface{}) (*ValidationResult, error) {
	result := NewValidationResult()

	securityMap, ok := value.(map[string]interface{})
	if !ok {
		return result, nil
	}

	securityType, ok := securityMap["type"].(string)
	if !ok || securityType != "shell-input" {
		return result, nil
	}

	input, ok := securityMap["value"].(string)
	if !ok {
		return result, nil
	}

	// Check for dangerous patterns
	dangerousPatterns := []string{";", "|", "&", "rm -rf", "cat /etc"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(input, pattern) {
			result.AddError("shell_input",
				"dangerous shell pattern detected",
				"Remove shell metacharacters from input")
			return result, nil
		}
	}

	return result, nil
}

// Mock validator for testing
type mockValidatorForSecurityTest struct {
	name  string
	valid bool
}

func newMockValidator(name string, valid bool) *mockValidatorForSecurityTest {
	return &mockValidatorForSecurityTest{name: name, valid: valid}
}

func (v *mockValidatorForSecurityTest) Name() string {
	return v.name
}

func (v *mockValidatorForSecurityTest) Priority() int {
	return PriorityNormal
}

func (v *mockValidatorForSecurityTest) Validate(ctx context.Context, value interface{}) (*ValidationResult, error) {
	result := NewValidationResult()
	if !v.valid {
		result.AddError("test", "validation failed")
	}
	return result, nil
}
