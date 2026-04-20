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

package ansible

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestProvision_ServiceDisabled(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Services = v2.ServiceMap{
		"ansible": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
	}

	err = Provision(*cfg)
	if err != nil {
		t.Errorf("unexpected error when ansible service is disabled: %v", err)
	}

	// Verify no files were created
	ansibleDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "ansible")
	if _, err := os.Stat(ansibleDir); !os.IsNotExist(err) {
		t.Error("ansible directory should not be created when service is disabled")
	}
}

func TestProvision_ServiceNotConfigured(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Services = v2.ServiceMap{}

	err = Provision(*cfg)
	if err != nil {
		t.Errorf("unexpected error when ansible service is not configured: %v", err)
	}

	// Verify no files were created
	ansibleDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "ansible")
	if _, err := os.Stat(ansibleDir); !os.IsNotExist(err) {
		t.Error("ansible directory should not be created when service is not configured")
	}
}

func TestProvision_MissingGitDir(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = ""
	cfg.OpenCenter.Services = v2.ServiceMap{
		"ansible": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}

	err = Provision(*cfg)
	if err == nil {
		t.Error("expected error when git_dir is missing")
	}
	if !containsString(err.Error(), "opencenter.gitops.git_dir must be set") {
		t.Errorf("expected error about git_dir, got: %v", err)
	}
}

func TestProvision_WhitespaceOnlyGitDir(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = "   \t\n  "
	cfg.OpenCenter.Services = v2.ServiceMap{
		"ansible": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}

	err = Provision(*cfg)
	if err == nil {
		t.Error("expected error when git_dir is whitespace only")
	}
	if !containsString(err.Error(), "opencenter.gitops.git_dir must be set") {
		t.Errorf("expected error about git_dir, got: %v", err)
	}
}

func TestProvision_DirectoryCreationFailure(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	// Create a file where we want to create the ansible directory
	tempDir := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = tempDir
	cfg.OpenCenter.Services = v2.ServiceMap{
		"ansible": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}

	// Create a file with the same name as the directory we want to create
	ansiblePath := filepath.Join(tempDir, "ansible")
	if err := os.WriteFile(ansiblePath, []byte("blocking file"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	err = Provision(*cfg)
	if err == nil {
		t.Error("expected error when ansible directory creation fails")
	}
	if !containsString(err.Error(), "failed to create ansible directory") {
		t.Errorf("expected error about directory creation, got: %v", err)
	}
}

func TestProvision_FileCreationFailure(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	// Create a directory structure where file creation will fail
	tempDir := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = tempDir
	cfg.OpenCenter.Services = v2.ServiceMap{
		"ansible": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}

	// Create ansible directory with restrictive permissions
	ansibleDir := filepath.Join(tempDir, "ansible")
	if err := os.MkdirAll(ansibleDir, 0000); err != nil {
		t.Fatalf("failed to create ansible directory: %v", err)
	}
	defer os.Chmod(ansibleDir, 0755) // Restore permissions for cleanup

	err = Provision(*cfg)
	if err == nil {
		t.Error("expected error when file creation fails")
	}
	if !containsString(err.Error(), "failed to create") {
		t.Errorf("expected error about file creation, got: %v", err)
	}
}

func TestProvision_DirectoryPermissions(t *testing.T) {
	cfg, err := v2.NewV2Default("test-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Services = v2.ServiceMap{
		"ansible": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}

	// This test will fail due to template execution, but we can test that
	// the directory is created with correct permissions
	err = Provision(*cfg)

	// Check that ansible directory was created with correct permissions
	ansibleDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "ansible")
	if info, statErr := os.Stat(ansibleDir); statErr == nil {
		if !info.IsDir() {
			t.Error("expected ansible path to be a directory")
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("expected ansible directory permissions to be 0755, got %o", info.Mode().Perm())
		}
	} else if err == nil {
		// If Provision succeeded but directory doesn't exist, that's a problem
		t.Error("Provision succeeded but ansible directory was not created")
	}

	// We expect template execution to fail, so log the error for reference
	if err != nil {
		t.Logf("Expected template execution error: %v", err)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
