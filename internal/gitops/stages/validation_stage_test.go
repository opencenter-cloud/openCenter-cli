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

package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationStage_Execute_ValidStructure(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a valid repository structure
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files including kustomization files
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"infrastructure/clusters/test-cluster/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/overlays/test-cluster/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.NoError(t, err, "Validation should pass for valid structure")
}

func TestValidationStage_Execute_MissingBaseDirectories(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create only partial structure (missing some required directories)
	dirs := []string{
		"applications",
		"infrastructure",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.Error(t, err, "Validation should fail for missing directories")
	assert.Contains(t, err.Error(), "base structure validation failed")
}

func TestValidationStage_Execute_MissingRequiredFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create directory structure but no files
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.Error(t, err, "Validation should fail for missing files")
	assert.Contains(t, err.Error(), "required files validation failed")
}

func TestValidationStage_Execute_MissingClusterDirectories(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create base structure but not cluster-specific directories
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files including kustomization files (but not cluster-specific ones)
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.Error(t, err, "Validation should fail for missing cluster directories")
	// The error could be about missing kustomization files or cluster directories
	assert.True(t,
		strings.Contains(err.Error(), "cluster structure validation failed") ||
			strings.Contains(err.Error(), "kustomization files validation failed"),
		"Error should mention cluster structure or kustomization files")
}

func TestValidationStage_Execute_MissingSOPSConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create complete structure
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files including kustomization files but NOT SOPS config
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"infrastructure/clusters/test-cluster/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/overlays/test-cluster/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration with secrets backend configured
	cfg := createTestConfigWithSecrets()

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.Error(t, err, "Validation should fail for missing SOPS config when secrets backend is configured")
	assert.Contains(t, err.Error(), "organization structure validation failed")
	assert.Contains(t, err.Error(), ".sops.yaml")
}

func TestValidationStage_Execute_WithSOPSConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create complete structure
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files including SOPS config and kustomization files
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		".sops.yaml":                        "creation_rules:\n  - age: test-key\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"infrastructure/clusters/test-cluster/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/overlays/test-cluster/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration with secrets backend configured
	cfg := createTestConfigWithSecrets()

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.NoError(t, err, "Validation should pass with SOPS config when secrets backend is configured")
}

func TestValidationStage_Execute_MissingKustomizationFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create complete structure
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files but NOT kustomization files
	files := map[string]string{
		".gitignore": "*.tmp\n",
		"README.md":  "# Test Repository\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.Error(t, err, "Validation should fail for missing kustomization files")
	assert.Contains(t, err.Error(), "kustomization files validation failed")
}

func TestValidationStage_Execute_WithKustomizationFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create complete structure
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files including kustomization files
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"infrastructure/clusters/test-cluster/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/overlays/test-cluster/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.NoError(t, err, "Validation should pass with all required files including kustomization files")
}

func TestValidationStage_Execute_FluxSystemValidation(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create complete structure
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
		".flux-system",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files including Flux system kustomization
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"infrastructure/clusters/test-cluster/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/overlays/test-cluster/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		".flux-system/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.NoError(t, err, "Validation should pass with Flux system directory and kustomization")
}

func TestValidationStage_Execute_FluxSystemMissingKustomization(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create complete structure including .flux-system directory
	dirs := []string{
		"applications",
		"applications/base",
		"applications/overlays",
		"applications/overlays/test-cluster",
		"infrastructure",
		"infrastructure/base",
		"infrastructure/clusters",
		"infrastructure/clusters/test-cluster",
		".flux-system", // Flux system directory exists
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755)
		require.NoError(t, err)
	}

	// Create required files but NOT Flux system kustomization
	files := map[string]string{
		".gitignore":                        "*.tmp\n",
		"README.md":                         "# Test Repository\n",
		"infrastructure/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"infrastructure/clusters/test-cluster/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/kustomization.yaml":                         "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		"applications/overlays/test-cluster/kustomization.yaml":   "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n",
		// Note: .flux-system/kustomization.yaml is intentionally missing
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Create workspace
	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: tmpDir,
		Config:  cfg,
	}

	// Create validation stage
	stage := NewValidationStage([]string{})

	// Execute validation
	err := stage.Execute(context.Background(), workspace)
	assert.Error(t, err, "Validation should fail when Flux system directory exists but kustomization is missing")
	assert.Contains(t, err.Error(), "flux system")
}

func TestValidationStage_Rollback(t *testing.T) {
	// Rollback should be a no-op for validation stage
	stage := NewValidationStage([]string{})
	cfg := createTestConfig("openstack")

	workspace := &gitops.GitOpsWorkspace{
		ID:      "test-workspace",
		RootDir: t.TempDir(),
		Config:  cfg,
	}

	err := stage.Rollback(context.Background(), workspace)
	assert.NoError(t, err, "Rollback should always succeed for validation stage")
}

func TestValidationStage_DryRun(t *testing.T) {
	stage := NewValidationStage([]string{"init", "infrastructure"})
	cfg := createTestConfig("openstack")

	plan, err := stage.DryRun(context.Background(), cfg)
	require.NoError(t, err)

	assert.Equal(t, "validation", plan.Name)
	assert.Equal(t, "Validate generated repository structure", plan.Description)
	assert.Empty(t, plan.Files, "Validation stage should not create files")
	assert.Empty(t, plan.Directories, "Validation stage should not create directories")
	assert.Equal(t, []string{"init", "infrastructure"}, plan.Dependencies)
}

// createTestConfigWithSecrets creates a test configuration with secrets backend configured.
func createTestConfigWithSecrets() v2.Config {
	cfg := createTestConfig("openstack")
	cfg.OpenCenter.Secrets.Backend = "sops"
	return cfg
}
