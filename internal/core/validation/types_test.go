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

func TestValidationResult_ToError_ValidResult(t *testing.T) {
	result := NewValidationResult()
	result.AddWarning("field", "warning message")

	err := result.ToError()
	if err != nil {
		t.Errorf("Expected nil error for valid result, got %v", err)
	}
}

func TestValidationResult_ToError_NoErrors(t *testing.T) {
	result := &ValidationResult{
		Valid:    false,
		Errors:   []*ValidationIssue{},
		Warnings: []*ValidationIssue{},
	}

	err := result.ToError()
	if err != nil {
		t.Errorf("Expected nil error when no errors present, got %v", err)
	}
}

func TestValidationResult_ToError_SingleError(t *testing.T) {
	result := NewValidationResult()
	result.AddError("cluster.name", "name is required", "Provide a cluster name", "Use lowercase letters")

	err := result.ToError()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check error message
	expectedMsg := "cluster.name: name is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	// Check that it's a structured error
	structErr, ok := err.(*structuredError)
	if !ok {
		t.Fatal("Expected *structuredError type")
	}

	if structErr.Type != "validation" {
		t.Errorf("Expected type 'validation', got %q", structErr.Type)
	}

	if structErr.Field != "cluster.name" {
		t.Errorf("Expected field 'cluster.name', got %q", structErr.Field)
	}

	if structErr.Message != "name is required" {
		t.Errorf("Expected message 'name is required', got %q", structErr.Message)
	}

	if len(structErr.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(structErr.Suggestions))
	}

	if structErr.Retryable {
		t.Error("Expected Retryable to be false")
	}
}

func TestValidationResult_ToError_MultipleErrors(t *testing.T) {
	result := NewValidationResult()
	result.AddError("cluster.name", "name is required", "Provide a cluster name")
	result.AddError("cluster.region", "invalid region", "Use us-east-1 or us-west-2")
	result.AddError("network.cidr", "invalid CIDR format", "Use format: 10.0.0.0/16")

	err := result.ToError()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check that it's a structured error
	structErr, ok := err.(*structuredError)
	if !ok {
		t.Fatal("Expected *structuredError type")
	}

	// Check aggregated message
	expectedMsg := "cluster.name: name is required; cluster.region: invalid region; network.cidr: invalid CIDR format"
	if structErr.Message != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, structErr.Message)
	}

	// Check that all suggestions are included
	if len(structErr.Suggestions) != 3 {
		t.Errorf("Expected 3 suggestions, got %d", len(structErr.Suggestions))
	}

	// Verify suggestions are present
	expectedSuggestions := map[string]bool{
		"Provide a cluster name":      true,
		"Use us-east-1 or us-west-2":  true,
		"Use format: 10.0.0.0/16":     true,
	}

	for _, suggestion := range structErr.Suggestions {
		if !expectedSuggestions[suggestion] {
			t.Errorf("Unexpected suggestion: %q", suggestion)
		}
	}
}

func TestValidationResult_ToError_DuplicateSuggestions(t *testing.T) {
	result := NewValidationResult()
	result.AddError("field1", "error1", "Use lowercase", "Check format")
	result.AddError("field2", "error2", "Use lowercase", "Verify input")

	err := result.ToError()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	structErr, ok := err.(*structuredError)
	if !ok {
		t.Fatal("Expected *structuredError type")
	}

	// Check that duplicate suggestions are removed
	suggestionCount := make(map[string]int)
	for _, s := range structErr.Suggestions {
		suggestionCount[s]++
	}

	for suggestion, count := range suggestionCount {
		if count > 1 {
			t.Errorf("Duplicate suggestion %q found %d times", suggestion, count)
		}
	}

	// Should have 3 unique suggestions
	if len(structErr.Suggestions) != 3 {
		t.Errorf("Expected 3 unique suggestions, got %d", len(structErr.Suggestions))
	}
}

func TestValidationResult_ToError_WithContext(t *testing.T) {
	result := NewValidationResult()
	
	// Add error with context
	issue := &ValidationIssue{
		Severity: SeverityError,
		Field:    "cluster.name",
		Message:  "name is required",
		Context: map[string]interface{}{
			"validator": "cluster-name",
			"operation": "validate",
		},
	}
	result.Errors = append(result.Errors, issue)
	result.Valid = false

	err := result.ToError()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	structErr, ok := err.(*structuredError)
	if !ok {
		t.Fatal("Expected *structuredError type")
	}

	// Check context is preserved
	if structErr.Context == nil {
		t.Fatal("Expected context to be present")
	}

	if structErr.Context["validator"] != "cluster-name" {
		t.Errorf("Expected validator 'cluster-name', got %v", structErr.Context["validator"])
	}

	if structErr.Context["operation"] != "validate" {
		t.Errorf("Expected operation 'validate', got %v", structErr.Context["operation"])
	}
}

func TestValidationResult_ToError_ErrorWithoutField(t *testing.T) {
	result := NewValidationResult()
	
	// Add error without field
	issue := &ValidationIssue{
		Severity: SeverityError,
		Field:    "",
		Message:  "general validation error",
	}
	result.Errors = append(result.Errors, issue)
	result.Valid = false

	err := result.ToError()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Error message should not have field prefix
	expectedMsg := "general validation error"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestStructuredError_Error(t *testing.T) {
	// Test with field
	err := &structuredError{
		Field:   "cluster.name",
		Message: "name is required",
	}

	expected := "cluster.name: name is required"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}

	// Test without field
	err.Field = ""
	expected = "name is required"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

