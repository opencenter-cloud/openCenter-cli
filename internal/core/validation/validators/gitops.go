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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// GitOpsValidator validates GitOps repository structure and configuration.
//
// Requirements (from Phase 2 Validation Consolidation):
//   - Validates GitOps repository structure
//   - Checks for required files and directories
//   - Validates manifest structure
//   - Provides actionable suggestions for fixing issues
//
// Validates: Requirements 2.6, 2.10
type GitOpsValidator struct {
	requiredDirs  []string
	requiredFiles []string
}

// NewGitOpsValidator creates a new GitOps validator.
func NewGitOpsValidator() *GitOpsValidator {
	return &GitOpsValidator{
		requiredDirs: []string{
			"applications",
			"applications/base",
			"applications/overlays",
			"infrastructure",
			"infrastructure/clusters",
		},
		requiredFiles: []string{
			"applications/base/kustomization.yaml",
			"infrastructure/clusters/kustomization.yaml",
		},
	}
}

// Name returns the validator name.
func (v *GitOpsValidator) Name() string {
	return "gitops"
}

// Priority returns the validator priority.
// GitOps validation involves file I/O, so it has low priority.
func (v *GitOpsValidator) Priority() int {
	return validation.PriorityLow
}

// Validate validates GitOps configuration.
//
// The value should be a map with the following keys:
//   - "git_dir": Path to the GitOps repository directory
//   - "git_url": URL of the GitOps repository (optional)
//   - "gitops_base_repo": URL of the GitOps base repository (optional)
//
// Returns a ValidationResult with errors and actionable suggestions.
func (v *GitOpsValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	gitopsMap, ok := value.(map[string]interface{})
	if !ok {
		result.AddError("gitops", "value must be a map with 'git_dir' and optional 'git_url' keys",
			"Provide a map with GitOps configuration")
		return result, nil
	}

	// Validate git_dir
	gitDirVal, ok := gitopsMap["git_dir"]
	if !ok {
		result.AddError("gitops.git_dir", "git_dir is required",
			"Specify the path to the GitOps repository directory",
			"Example: git_dir: /path/to/gitops-repo")
		return result, nil
	}

	gitDir, ok := gitDirVal.(string)
	if !ok {
		result.AddError("gitops.git_dir", "git_dir must be a string")
		return result, nil
	}

	if gitDir == "" {
		result.AddError("gitops.git_dir", "git_dir cannot be empty",
			"Specify the path to the GitOps repository directory")
		return result, nil
	}

	// Validate repository structure if directory exists
	if _, err := os.Stat(gitDir); err == nil {
		v.validateRepositoryStructure(result, gitDir)
	} else if os.IsNotExist(err) {
		result.AddWarning("gitops.git_dir",
			fmt.Sprintf("GitOps directory does not exist: %s", gitDir),
			"Run 'opencenter cluster setup' to create the GitOps repository structure",
			"Or create the directory manually and populate it with required files")
	} else {
		result.AddError("gitops.git_dir",
			fmt.Sprintf("cannot access GitOps directory: %v", err),
			"Check directory permissions",
			"Ensure the path is correct")
	}

	// Validate git_url if provided
	if gitURLVal, ok := gitopsMap["git_url"]; ok {
		if gitURL, ok := gitURLVal.(string); ok && gitURL != "" {
			v.validateGitURL(result, gitURL, "gitops.git_url")
		}
	}

	// Validate gitops_base_repo if provided
	if baseRepoVal, ok := gitopsMap["gitops_base_repo"]; ok {
		if baseRepo, ok := baseRepoVal.(string); ok && baseRepo != "" {
			v.validateGitURL(result, baseRepo, "gitops.gitops_base_repo")
		}
	}

	return result, nil
}

// validateRepositoryStructure validates the GitOps repository structure.
func (v *GitOpsValidator) validateRepositoryStructure(result *validation.ValidationResult, gitDir string) {
	// Check for required directories
	for _, dir := range v.requiredDirs {
		dirPath := filepath.Join(gitDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			result.AddError("gitops.structure",
				fmt.Sprintf("required directory missing: %s", dir),
				fmt.Sprintf("Create the directory: mkdir -p %s", dirPath),
				"Run 'opencenter cluster setup' to generate the complete structure")
		}
	}

	// Check for required files
	for _, file := range v.requiredFiles {
		filePath := filepath.Join(gitDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			result.AddError("gitops.structure",
				fmt.Sprintf("required file missing: %s", file),
				fmt.Sprintf("Create the file: touch %s", filePath),
				"Run 'opencenter cluster setup' to generate required files")
		} else if err == nil {
			// File exists, validate it's a valid kustomization file
			v.validateKustomizationFile(result, filePath)
		}
	}

	// Check for .git directory (indicates it's a git repository)
	gitDirPath := filepath.Join(gitDir, ".git")
	if _, err := os.Stat(gitDirPath); os.IsNotExist(err) {
		result.AddWarning("gitops.git",
			"GitOps directory is not a git repository",
			fmt.Sprintf("Initialize git repository: cd %s && git init", gitDir),
			"GitOps requires version control for proper operation")
	}
}

// validateKustomizationFile validates a kustomization.yaml file.
func (v *GitOpsValidator) validateKustomizationFile(result *validation.ValidationResult, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.AddWarning("gitops.kustomization",
			fmt.Sprintf("cannot read kustomization file: %s", filePath),
			"Check file permissions")
		return
	}

	// Basic validation: check if file is not empty
	if len(content) == 0 {
		result.AddError("gitops.kustomization",
			fmt.Sprintf("kustomization file is empty: %s", filePath),
			"Add kustomization content to the file",
			"Example: apiVersion: kustomize.config.k8s.io/v1beta1\\nkind: Kustomization")
		return
	}

	// Check for required kustomization fields
	contentStr := string(content)
	if !strings.Contains(contentStr, "apiVersion") {
		result.AddError("gitops.kustomization",
			fmt.Sprintf("kustomization file missing apiVersion: %s", filePath),
			"Add apiVersion field: apiVersion: kustomize.config.k8s.io/v1beta1")
	}

	if !strings.Contains(contentStr, "kind: Kustomization") {
		result.AddError("gitops.kustomization",
			fmt.Sprintf("kustomization file missing kind: %s", filePath),
			"Add kind field: kind: Kustomization")
	}
}

// validateGitURL validates a Git repository URL.
func (v *GitOpsValidator) validateGitURL(result *validation.ValidationResult, gitURL, field string) {
	if gitURL == "" {
		return
	}

	// Parse URL
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		result.AddError(field,
			fmt.Sprintf("invalid Git URL format: %s", gitURL),
			"Use a valid URL format",
			"Examples: https://github.com/org/repo.git, ssh://git@github.com/org/repo.git")
		return
	}

	// Check for supported schemes
	supportedSchemes := map[string]bool{
		"https": true,
		"http":  true,
		"ssh":   true,
		"git":   true,
	}

	if !supportedSchemes[parsedURL.Scheme] {
		result.AddError(field,
			fmt.Sprintf("unsupported Git URL scheme: %s", parsedURL.Scheme),
			"Use https, http, ssh, or git scheme",
			"Example: https://github.com/org/repo.git")
		return
	}

	// Warn about http (insecure)
	if parsedURL.Scheme == "http" {
		result.AddWarning(field,
			"Git URL uses insecure http scheme",
			"Consider using https for secure communication",
			"Example: https://github.com/org/repo.git")
	}

	// Check for .git suffix
	if !strings.HasSuffix(parsedURL.Path, ".git") {
		result.AddWarning(field,
			"Git URL should end with .git",
			"Add .git suffix to the URL",
			fmt.Sprintf("Example: %s.git", gitURL))
	}

	// Validate hostname for common Git hosting services
	hostname := parsedURL.Hostname()
	if hostname == "" {
		result.AddError(field,
			"Git URL missing hostname",
			"Provide a valid Git repository URL with hostname")
		return
	}

	// Check for common typos in Git hosting services
	commonHosts := map[string]string{
		"github.com":    "github.com",
		"gitlab.com":    "gitlab.com",
		"bitbucket.org": "bitbucket.org",
	}

	lowerHostname := strings.ToLower(hostname)
	for typo, correct := range commonHosts {
		if strings.Contains(lowerHostname, typo) && lowerHostname != typo {
			result.AddWarning(field,
				fmt.Sprintf("possible typo in hostname: %s", hostname),
				fmt.Sprintf("Did you mean: %s?", correct))
		}
	}
}

// SetRequiredDirectories sets the list of required directories.
func (v *GitOpsValidator) SetRequiredDirectories(dirs []string) {
	v.requiredDirs = dirs
}

// SetRequiredFiles sets the list of required files.
func (v *GitOpsValidator) SetRequiredFiles(files []string) {
	v.requiredFiles = files
}
