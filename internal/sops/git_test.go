/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestNewGitIntegrator(t *testing.T) {
	repoPath := "/test/repo"
	encryptor := NewDefaultEncryptor(nil, nil)
	
	integrator := NewGitIntegrator(repoPath, encryptor)
	
	if integrator == nil {
		t.Error("NewGitIntegrator() should not return nil")
	}
}

func TestGitIntegrator_ValidateRepository(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	
	encryptor := NewDefaultEncryptor(nil, nil)
	integrator := NewGitIntegrator(tmpDir, encryptor)
	
	// Should fail because it's not a git repository
	err := integrator.ValidateRepository()
	if err == nil {
		t.Error("ValidateRepository() should fail for non-git directory")
	}
	
	// Create .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}
	
	// Should pass now
	err = integrator.ValidateRepository()
	if err != nil {
		t.Errorf("ValidateRepository() should pass for git directory: %v", err)
	}
}

func TestGitIntegrator_CreateGitIgnore(t *testing.T) {
	tmpDir := t.TempDir()
	
	encryptor := NewDefaultEncryptor(nil, nil)
	integrator := NewGitIntegrator(tmpDir, encryptor)
	
	err := integrator.CreateGitIgnore()
	if err != nil {
		t.Errorf("CreateGitIgnore() error = %v", err)
	}
	
	// Check if .gitignore was created
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Error("CreateGitIgnore() should create .gitignore file")
	}
}

func TestGitIntegrator_SetupGitAttributes(t *testing.T) {
	tmpDir := t.TempDir()
	
	encryptor := NewDefaultEncryptor(nil, nil)
	integrator := NewGitIntegrator(tmpDir, encryptor)
	
	err := integrator.SetupGitAttributes()
	if err != nil {
		t.Errorf("SetupGitAttributes() error = %v", err)
	}
	
	// Check if .gitattributes was created
	gitattributesPath := filepath.Join(tmpDir, ".gitattributes")
	if _, err := os.Stat(gitattributesPath); os.IsNotExist(err) {
		t.Error("SetupGitAttributes() should create .gitattributes file")
	}
}

func TestGitIntegrator_CreateCommitMessage(t *testing.T) {
	tmpDir := t.TempDir()
	
	encryptor := NewDefaultEncryptor(nil, nil)
	integrator := NewGitIntegrator(tmpDir, encryptor)
	
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
		},
	}
	
	tests := []struct {
		operation string
		contains  string
	}{
		{"bootstrap", "Bootstrap GitOps overlay"},
		{"update", "Update overlay configuration"},
		{"encrypt", "Encrypt sensitive files"},
		{"other", "Update overlay files"},
	}
	
	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			message := integrator.CreateCommitMessage(cfg, tt.operation)
			if message == "" {
				t.Error("CreateCommitMessage() should not return empty string")
			}
			if !contains(message, tt.contains) {
				t.Errorf("CreateCommitMessage() should contain '%s', got: %s", tt.contains, message)
			}
		})
	}
}

func TestGitIntegrator_getFilesToEncrypt(t *testing.T) {
	tmpDir := t.TempDir()
	
	encryptor := NewDefaultEncryptor(nil, nil)
	integrator := NewGitIntegrator(tmpDir, encryptor)
	
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
		},
	}
	
	files := integrator.getFilesToEncrypt(tmpDir, cfg)
	
	// Should contain standard files
	expectedFiles := []string{
		"flux-system/gotk-sync.yaml",
		"managed-services/sources/base-repo.yaml",
		"secrets/openstack-credentials.yaml",
	}
	
	if len(files) != len(expectedFiles) {
		t.Errorf("getFilesToEncrypt() returned %d files, expected %d", len(files), len(expectedFiles))
	}
	
	for _, expected := range expectedFiles {
		found := false
		for _, file := range files {
			if file == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("getFilesToEncrypt() should include %s", expected)
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}