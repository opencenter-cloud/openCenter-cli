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
	"testing"
)

func TestValidationIssue_Error(t *testing.T) {
	issue := &ValidationIssue{
		Severity: SeverityError,
		Field:    "cluster.name",
		Message:  "cluster name is required",
		Code:     "E001",
	}

	expected := "[E001] cluster.name: cluster name is required"
	if issue.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, issue.Error())
	}

	// Test without code
	issue.Code = ""
	expected = "cluster.name: cluster name is required"
	if issue.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, issue.Error())
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	result := &ValidationResult{Valid: true}

	if result.HasErrors() {
		t.Error("Expected no errors")
	}

	result.AddError("field", "error message")

	if !result.HasErrors() {
		t.Error("Expected errors")
	}
}

func TestValidationResult_HasWarnings(t *testing.T) {
	result := &ValidationResult{Valid: true}

	if result.HasWarnings() {
		t.Error("Expected no warnings")
	}

	result.AddWarning("field", "warning message")

	if !result.HasWarnings() {
		t.Error("Expected warnings")
	}
}

func TestValidationResult_HasIssues(t *testing.T) {
	result := &ValidationResult{Valid: true}

	if result.HasIssues() {
		t.Error("Expected no issues")
	}

	result.AddWarning("field", "warning message")

	if !result.HasIssues() {
		t.Error("Expected issues")
	}
}

func TestValidationResult_AddError(t *testing.T) {
	result := &ValidationResult{Valid: true}

	result.AddError("field", "error message", "suggestion1", "suggestion2")

	if result.Valid {
		t.Error("Expected Valid to be false after adding error")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(result.Errors))
	}

	issue := result.Errors[0]
	if issue.Severity != SeverityError {
		t.Errorf("Expected severity %q, got %q", SeverityError, issue.Severity)
	}

	if issue.Field != "field" {
		t.Errorf("Expected field %q, got %q", "field", issue.Field)
	}

	if issue.Message != "error message" {
		t.Errorf("Expected message %q, got %q", "error message", issue.Message)
	}

	if len(issue.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(issue.Suggestions))
	}
}

func TestValidationResult_AddWarning(t *testing.T) {
	result := &ValidationResult{Valid: true}

	result.AddWarning("field", "warning message", "suggestion")

	if !result.Valid {
		t.Error("Expected Valid to remain true after adding warning")
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(result.Warnings))
	}

	issue := result.Warnings[0]
	if issue.Severity != SeverityWarning {
		t.Errorf("Expected severity %q, got %q", SeverityWarning, issue.Severity)
	}
}

func TestValidationResult_AddInfo(t *testing.T) {
	result := &ValidationResult{Valid: true}

	result.AddInfo("field", "info message")

	if !result.Valid {
		t.Error("Expected Valid to remain true after adding info")
	}

	if len(result.Info) != 1 {
		t.Fatalf("Expected 1 info, got %d", len(result.Info))
	}

	issue := result.Info[0]
	if issue.Severity != SeverityInfo {
		t.Errorf("Expected severity %q, got %q", SeverityInfo, issue.Severity)
	}
}

func TestValidationResult_Merge(t *testing.T) {
	result1 := &ValidationResult{Valid: true}
	result1.AddWarning("field1", "warning1")

	result2 := &ValidationResult{Valid: false}
	result2.AddError("field2", "error2")
	result2.AddWarning("field2", "warning2")

	result1.Merge(result2)

	if result1.Valid {
		t.Error("Expected Valid to be false after merging invalid result")
	}

	if len(result1.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result1.Errors))
	}

	if len(result1.Warnings) != 2 {
		t.Errorf("Expected 2 warnings, got %d", len(result1.Warnings))
	}
}

func TestValidationResult_MergeNil(t *testing.T) {
	result := &ValidationResult{Valid: true}
	result.AddWarning("field", "warning")

	result.Merge(nil)

	if !result.Valid {
		t.Error("Expected Valid to remain true after merging nil")
	}

	if len(result.Warnings) != 1 {
		t.Error("Expected warnings to remain unchanged")
	}
}

func TestDefaultValidationOptions(t *testing.T) {
	opts := DefaultValidationOptions()

	if opts == nil {
		t.Fatal("DefaultValidationOptions returned nil")
	}

	if opts.StopOnFirstError {
		t.Error("Expected StopOnFirstError to be false")
	}

	if !opts.IncludeWarnings {
		t.Error("Expected IncludeWarnings to be true")
	}

	if opts.Context == nil {
		t.Error("Expected Context to be initialized")
	}
}
