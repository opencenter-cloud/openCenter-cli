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

package validators

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

func TestGitOpsValidator_Name(t *testing.T) {
	validator := NewGitOpsValidator()
	if validator.Name() != "gitops" {
		t.Errorf("expected name 'gitops', got %s", validator.Name())
	}
}

func TestGitOpsValidator_InvalidInput(t *testing.T) {
	validator := NewGitOpsValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
		errorField  string
	}{
		{
			name:        "not a map",
			value:       "invalid",
			expectError: true,
			errorField:  "gitops",
		},
		{
			name:        "missing git_dir",
			value:       map[string]interface{}{},
			expectError: true,
			errorField:  "gitops.git_dir",
		},
		{
			name: "git_dir not a string",
			value: map[string]interface{}{
				"git_dir": 123,
			},
			expectError: true,
			errorField:  "gitops.git_dir",
		},
		{
			name: "empty git_dir",
			value: map[string]interface{}{
				"git_dir": "",
			},
			expectError: true,
			errorField:  "gitops.git_dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if tt.expectError && len(result.Errors) > 0 {
				found := false
				for _, e := range result.Errors {
					if e.Field == tt.errorField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %s, but didn't find it", tt.errorField)
				}
			}
		})
	}
}

func TestGitOpsValidator_NonExistentDirectory(t *testing.T) {
	validator := NewGitOpsValidator()
	ctx := context.Background()

	value := map[string]interface{}{
		"git_dir": "/nonexistent/directory/path",
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have a warning about non-existent directory
	if len(result.Warnings) == 0 {
		t.Error("expected warning about non-existent directory")
	}
}

func TestGitOpsValidator_ValidRepositoryStructure(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create required directories
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"infrastructure",
		"infrastructure/clusters",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
	}

	// Create required files
	files := []string{
		"applications/base/kustomization.yaml",
		"infrastructure/clusters/kustomization.yaml",
	}

	for _, file := range files {
		filePath := filepath.Join(tmpDir, file)
		content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
`
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Create .git directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	validator := NewGitOpsValidator()
	ctx := context.Background()

	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected validation to pass, but it failed with errors: %v", result.Errors)
	}
}

func TestGitOpsValidator_MissingRequiredDirectories(t *testing.T) {
	// Create temporary directory with incomplete structure
	tmpDir := t.TempDir()

	// Only create some directories
	if err := os.MkdirAll(filepath.Join(tmpDir, "applications"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	validator := NewGitOpsValidator()
	ctx := context.Background()

	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected validation to fail due to missing directories")
	}

	// Should have errors about missing directories
	if len(result.Errors) == 0 {
		t.Error("expected errors about missing directories")
	}
}

func TestGitOpsValidator_InvalidGitURL(t *testing.T) {
	validator := NewGitOpsValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		gitURL      string
		expectError bool
		expectWarn  bool
	}{
		{
			name:        "invalid URL format",
			gitURL:      "not a url",
			expectError: true,
		},
		{
			name:        "unsupported scheme",
			gitURL:      "ftp://example.com/repo.git",
			expectError: true,
		},
		{
			name:        "http scheme (insecure)",
			gitURL:      "http://github.com/org/repo.git",
			expectWarn:  true,
		},
		{
			name:        "missing .git suffix",
			gitURL:      "https://github.com/org/repo",
			expectWarn:  true,
		},
		{
			name:   "valid https URL",
			gitURL: "https://github.com/org/repo.git",
		},
		{
			name:   "valid ssh URL",
			gitURL: "ssh://git@github.com/org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory that doesn't exist to avoid structure validation
			tmpDir := filepath.Join(t.TempDir(), "nonexistent")
			
			value := map[string]interface{}{
				"git_dir": tmpDir,
				"git_url": tt.gitURL,
			}

			result, err := validator.Validate(ctx, value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Filter out warnings about non-existent directory
			var relevantErrors []*validation.ValidationIssue
			var relevantWarnings []*validation.ValidationIssue
			
			for _, e := range result.Errors {
				if e.Field != "gitops.git_dir" {
					relevantErrors = append(relevantErrors, e)
				}
			}
			
			for _, w := range result.Warnings {
				if w.Field != "gitops.git_dir" {
					relevantWarnings = append(relevantWarnings, w)
				}
			}

			if tt.expectError && len(relevantErrors) == 0 {
				t.Error("expected errors, but got none")
			}

			if tt.expectWarn && len(relevantWarnings) == 0 {
				t.Error("expected warnings, but got none")
			}

			if !tt.expectError && !tt.expectWarn && len(relevantErrors) > 0 {
				t.Errorf("expected validation to pass, but it failed: %v", relevantErrors)
			}
		})
	}
}

func TestGitOpsValidator_EmptyKustomizationFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	baseDir := filepath.Join(tmpDir, "applications", "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create empty kustomization file
	kustomizationPath := filepath.Join(baseDir, "kustomization.yaml")
	if err := os.WriteFile(kustomizationPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	validator := NewGitOpsValidator()
	ctx := context.Background()

	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected validation to fail due to empty kustomization file")
	}

	// Should have error about empty file
	found := false
	for _, e := range result.Errors {
		if e.Field == "gitops.kustomization" && e.Message != "" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected error about empty kustomization file")
	}
}

func TestGitOpsValidator_InvalidKustomizationFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	baseDir := filepath.Join(tmpDir, "applications", "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create kustomization file without required fields
	kustomizationPath := filepath.Join(baseDir, "kustomization.yaml")
	content := `# Missing apiVersion and kind
resources: []
`
	if err := os.WriteFile(kustomizationPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	validator := NewGitOpsValidator()
	ctx := context.Background()

	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected validation to fail due to invalid kustomization file")
	}

	// Should have errors about missing fields
	if len(result.Errors) == 0 {
		t.Error("expected errors about missing kustomization fields")
	}
}

func TestGitOpsValidator_NotGitRepository(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure without .git
	if err := os.MkdirAll(filepath.Join(tmpDir, "applications"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	validator := NewGitOpsValidator()
	ctx := context.Background()

	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have warning about not being a git repository
	found := false
	for _, w := range result.Warnings {
		if w.Field == "gitops.git" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected warning about directory not being a git repository")
	}
}

func TestGitOpsValidator_SetRequiredDirectories(t *testing.T) {
	validator := NewGitOpsValidator()

	customDirs := []string{"custom-dir1", "custom-dir2"}
	validator.SetRequiredDirectories(customDirs)

	// Verify the directories were set (we can't directly access the field,
	// but we can test the behavior)
	tmpDir := t.TempDir()

	// Create only one of the custom directories
	if err := os.MkdirAll(filepath.Join(tmpDir, "custom-dir1"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx := context.Background()
	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have error about missing custom-dir2
	if result.Valid {
		t.Error("expected validation to fail due to missing custom directory")
	}
}

func TestGitOpsValidator_SetRequiredFiles(t *testing.T) {
	validator := NewGitOpsValidator()

	customFiles := []string{"custom-file.yaml"}
	validator.SetRequiredFiles(customFiles)

	tmpDir := t.TempDir()

	ctx := context.Background()
	value := map[string]interface{}{
		"git_dir": tmpDir,
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have error about missing custom file
	if result.Valid {
		t.Error("expected validation to fail due to missing custom file")
	}
}
