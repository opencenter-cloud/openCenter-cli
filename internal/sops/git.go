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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// GitIntegrator handles Git operations with SOPS-encrypted files
type GitIntegrator struct {
	repoPath  string
	encryptor *Encryptor
}

// NewGitIntegrator creates a new Git integrator
func NewGitIntegrator(repoPath string, encryptor *Encryptor) *GitIntegrator {
	return &GitIntegrator{
		repoPath:  repoPath,
		encryptor: encryptor,
	}
}

// CommitConfig represents Git commit configuration
type CommitConfig struct {
	Message     string
	Author      string
	Email       string
	SignCommits bool
	DryRun      bool
	Verbose     bool
}

// CommitEncryptedFiles commits SOPS-encrypted files to Git
func (g *GitIntegrator) CommitEncryptedFiles(ctx context.Context, cfg *config.Config, commitCfg CommitConfig) error {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Encrypt files before committing
	if err := g.encryptFilesForCommit(ctx, cfg); err != nil {
		return fmt.Errorf("failed to encrypt files: %w", err)
	}

	// Stage encrypted files
	if err := g.stageEncryptedFiles(ctx, cfg); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Commit changes
	if err := g.commitChanges(ctx, commitCfg); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

// encryptFilesForCommit encrypts files that should be encrypted before commit
func (g *GitIntegrator) encryptFilesForCommit(ctx context.Context, cfg *config.Config) error {
	filesToEncrypt := g.encryptor.getFilesToEncrypt(g.repoPath, cfg)

	// Get age key from configuration
	var ageKeys []string
	if cfg.Secrets.SopsAgeKeyFile != "" {
		// Load the age key from the specified file
		if keyPair, err := loadAgeKeyFromFile(cfg.Secrets.SopsAgeKeyFile); err == nil {
			ageKeys = []string{keyPair.PublicKey}
		}
	}
	
	encryptConfig := EncryptionConfig{
		AgeKeys: ageKeys,
		InPlace: true,
		Verbose: false,
	}

	for _, file := range filesToEncrypt {
		filePath := filepath.Join(g.repoPath, file)

		// Skip if file doesn't exist
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		// Check if already encrypted
		if isEncrypted, err := g.encryptor.IsFileEncrypted(filePath); err != nil {
			return fmt.Errorf("failed to check encryption status of %s: %w", file, err)
		} else if isEncrypted {
			continue // Already encrypted
		}

		// Encrypt the file
		if err := g.encryptor.EncryptFile(ctx, filePath, encryptConfig); err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", file, err)
		}
	}

	return nil
}

// stageEncryptedFiles stages encrypted files for commit
func (g *GitIntegrator) stageEncryptedFiles(ctx context.Context, cfg *config.Config) error {
	filesToStage := g.encryptor.getFilesToEncrypt(g.repoPath, cfg)

	for _, file := range filesToStage {
		filePath := filepath.Join(g.repoPath, file)

		// Skip if file doesn't exist
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		// Stage the file
		cmd := exec.CommandContext(ctx, "git", "add", file)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stage file %s: %w", file, err)
		}
	}

	return nil
}

// commitChanges commits the staged changes
func (g *GitIntegrator) commitChanges(ctx context.Context, commitCfg CommitConfig) error {
	args := []string{"commit"}

	// Add message
	if commitCfg.Message != "" {
		args = append(args, "-m", commitCfg.Message)
	} else {
		args = append(args, "-m", "Update SOPS-encrypted overlay files")
	}

	// Add author if specified
	if commitCfg.Author != "" && commitCfg.Email != "" {
		args = append(args, "--author", fmt.Sprintf("%s <%s>", commitCfg.Author, commitCfg.Email))
	}

	// Add signing if enabled
	if commitCfg.SignCommits {
		args = append(args, "-S")
	}

	// Add dry-run if specified
	if commitCfg.DryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.CommandContext(ctx, "git", args...)

	if commitCfg.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}

// PushChanges pushes committed changes to remote repository
func (g *GitIntegrator) PushChanges(ctx context.Context, remote, branch string) error {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Push changes
	args := []string{"push"}

	if remote != "" {
		args = append(args, remote)
	}

	if branch != "" {
		args = append(args, branch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

// CloneRepository clones a Git repository
func (g *GitIntegrator) CloneRepository(ctx context.Context, repoURL, targetDir, branch string) error {
	args := []string{"clone"}

	if branch != "" {
		args = append(args, "--branch", branch)
	}

	args = append(args, repoURL, targetDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Update repository path
	g.repoPath = targetDir

	return nil
}

// ValidateRepository validates that the directory is a valid Git repository
func (g *GitIntegrator) ValidateRepository() error {
	// Check if .git directory exists
	gitDir := filepath.Join(g.repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", g.repoPath)
	}

	return nil
}

// GetCurrentBranch returns the current Git branch
func (g *GitIntegrator) GetCurrentBranch(ctx context.Context) (string, error) {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return "", fmt.Errorf("failed to change to repository directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetRemoteURL returns the remote URL for the repository
func (g *GitIntegrator) GetRemoteURL(ctx context.Context, remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}

	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return "", fmt.Errorf("failed to change to repository directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", remote)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CheckForChanges checks if there are uncommitted changes
func (g *GitIntegrator) CheckForChanges(ctx context.Context) (bool, error) {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return false, fmt.Errorf("failed to change to repository directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// CreateGitIgnore creates a .gitignore file with SOPS-specific entries
func (g *GitIntegrator) CreateGitIgnore() error {
	gitignoreContent := `# SOPS-related files
.sops.yaml.bak
*.dec
*.dec.*

# Temporary files
*.tmp
*.temp
.DS_Store
Thumbs.db

# Editor files
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db
`

	gitignorePath := filepath.Join(g.repoPath, ".gitignore")

	// Check if .gitignore already exists
	if _, err := os.Stat(gitignorePath); err == nil {
		// Append to existing .gitignore
		file, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open .gitignore: %w", err)
		}
		defer file.Close()

		if _, err := file.WriteString("\n" + gitignoreContent); err != nil {
			return fmt.Errorf("failed to append to .gitignore: %w", err)
		}
	} else {
		// Create new .gitignore
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}

	return nil
}

// SetupGitAttributes creates a .gitattributes file for SOPS files
func (g *GitIntegrator) SetupGitAttributes() error {
	gitattributesContent := `# SOPS encrypted files
*.yaml diff=sopsdiffer
*.yml diff=sopsdiffer
*.json diff=sopsdiffer

# Ensure encrypted files are treated as binary
secrets/*.yaml binary
secrets/*.yml binary
**/secrets/*.yaml binary
**/secrets/*.yml binary
`

	gitattributesPath := filepath.Join(g.repoPath, ".gitattributes")

	if err := os.WriteFile(gitattributesPath, []byte(gitattributesContent), 0o644); err != nil {
		return fmt.Errorf("failed to create .gitattributes: %w", err)
	}

	return nil
}

// ConfigureSOPSDiff configures Git to use SOPS for diffing encrypted files
func (g *GitIntegrator) ConfigureSOPSDiff(ctx context.Context) error {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Configure SOPS diff tool
	cmd := exec.CommandContext(ctx, "git", "config", "diff.sopsdiffer.textconv", "sops -d")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure SOPS diff: %w", err)
	}

	return nil
}

// ValidateGitConfig validates Git configuration for SOPS operations
func (g *GitIntegrator) ValidateGitConfig(ctx context.Context) error {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Check if user.name is configured
	cmd := exec.CommandContext(ctx, "git", "config", "user.name")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git user.name not configured: %w", err)
	}

	// Check if user.email is configured
	cmd = exec.CommandContext(ctx, "git", "config", "user.email")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git user.email not configured: %w", err)
	}

	return nil
}

// CreateCommitMessage generates a commit message for overlay changes
func (g *GitIntegrator) CreateCommitMessage(cfg *config.Config, operation string) string {
	clusterName := cfg.OpenCenter.Cluster.ClusterName

	switch operation {
	case "bootstrap":
		return fmt.Sprintf("Bootstrap GitOps overlay for cluster %s\n\n- Initialize FluxCD overlay structure\n- Add SOPS-encrypted secrets\n- Configure managed and customer-managed layers", clusterName)
	case "update":
		return fmt.Sprintf("Update overlay configuration for cluster %s\n\n- Update SOPS-encrypted files\n- Sync with latest configuration", clusterName)
	case "encrypt":
		return fmt.Sprintf("Encrypt sensitive files for cluster %s\n\n- Apply SOPS encryption to secrets\n- Update encrypted overlay files", clusterName)
	default:
		return fmt.Sprintf("Update overlay files for cluster %s", clusterName)
	}
}

// GetLastCommitHash returns the hash of the last commit
func (g *GitIntegrator) GetLastCommitHash(ctx context.Context) (string, error) {
	// Change to repository directory
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.repoPath); err != nil {
		return "", fmt.Errorf("failed to change to repository directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get last commit hash: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
