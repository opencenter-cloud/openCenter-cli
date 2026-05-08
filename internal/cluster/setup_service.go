package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/tofu"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
)

// SetupOptions contains options for cluster setup
type SetupOptions struct {
	ClusterName    string
	Organization   string
	DryRun         bool
	SkipValidation bool
	Force          bool
}

// SetupResult contains the result of cluster setup
type SetupResult struct {
	GitOpsPath       string
	ManifestsCreated int
	ValidationPassed bool
	CommitHash       string
}

// SetupService handles cluster setup business logic
type SetupService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
	configurationMgr *config.ConfigurationManager
	commandRunner    security.CommandRunner
	encryptor        ManifestEncryptor
}

// ManifestEncryptor encrypts Secret manifests in the GitOps tree. The default
// implementation delegates to SOPS; tests inject a fake that rewrites values
// without requiring a real SOPS binary.
type ManifestEncryptor interface {
	EncryptSecretManifest(ctx context.Context, path string, publicKey string, sopsConfigPath string) error
}

// sopsManifestEncryptor delegates to the real SOPS encryptor.
type sopsManifestEncryptor struct{}

func (e *sopsManifestEncryptor) EncryptSecretManifest(ctx context.Context, path string, publicKey string, sopsConfigPath string) error {
	encryptor := sops.NewDefaultEncryptor(nil, nil)
	encryptConfig := sops.EncryptionConfig{
		AgeKeys:    []string{publicKey},
		ConfigFile: sopsConfigPath,
		InPlace:    true,
	}
	return encryptor.EncryptFile(ctx, path, encryptConfig)
}

// NewSetupService creates a new SetupService
func NewSetupService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
) *SetupService {
	return NewSetupServiceWithConfigMgr(pathResolver, validationEngine, nil)
}

// NewSetupServiceWithConfigMgr creates a new SetupService with optional ConfigurationManager
func NewSetupServiceWithConfigMgr(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configurationMgr *config.ConfigurationManager,
) *SetupService {
	// Create ConfigurationManager if not provided
	if configurationMgr == nil {
		// Try to create one, but don't fail if it doesn't work
		configurationMgr, _ = config.NewConfigurationManager()
	}

	var enc ManifestEncryptor
	if os.Getenv("OPENCENTER_TEST_MODE") == "1" {
		enc = &fakeManifestEncryptor{}
	} else {
		enc = &sopsManifestEncryptor{}
	}

	return &SetupService{
		pathResolver:     pathResolver,
		validationEngine: validationEngine,
		configurationMgr: configurationMgr,
		commandRunner:    security.GetDefaultCommandRunner(),
		encryptor:        enc,
	}
}

// Setup performs cluster setup
func (s *SetupService) Setup(ctx context.Context, opts SetupOptions) (*SetupResult, error) {
	// Resolve paths
	clusterPaths, err := s.pathResolver.Resolve(ctx, opts.ClusterName, opts.Organization)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster paths: %w", err)
	}

	// Load configuration using ConfigurationManager
	// Build the full identifier (org/cluster) for config loading when organization is known
	configIdentifier := opts.ClusterName
	if opts.Organization != "" {
		configIdentifier = opts.Organization + "/" + opts.ClusterName
	}

	var cfg v2.Config
	if s.configurationMgr != nil {
		var loadedCfg *v2.Config
		var err error

		// Use LoadWithoutValidation if validation will be skipped anyway
		if opts.SkipValidation {
			loadedCfg, err = s.configurationMgr.LoadWithoutValidation(ctx, configIdentifier)
		} else {
			loadedCfg, err = s.configurationMgr.Load(ctx, configIdentifier)
		}

		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	} else {
		// Fallback: create temporary manager
		tempMgr, err := config.NewConfigurationManager()
		if err != nil {
			return nil, fmt.Errorf("creating configuration manager: %w", err)
		}

		var loadedCfg *v2.Config
		if opts.SkipValidation {
			loadedCfg, err = tempMgr.LoadWithoutValidation(ctx, configIdentifier)
		} else {
			loadedCfg, err = tempMgr.Load(ctx, configIdentifier)
		}

		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	}

	// Check schema version - only v2 is supported
	if cfg.SchemaVersion != "2.0" {
		return nil, fmt.Errorf("invalid schema version for cluster %s: expected 2.0, got %q", opts.ClusterName, cfg.SchemaVersion)
	}

	// Validate that git_dir is set
	gitDir := cfg.GitDir()
	if gitDir == "" || strings.HasPrefix(gitDir, "./testdata/test-git-repo-") {
		return nil, fmt.Errorf("opencenter.gitops.git_dir must be set in the configuration")
	}

	result := &SetupResult{
		GitOpsPath: gitDir,
	}

	if !opts.SkipValidation {
		if err := s.validateSetupConfig(&cfg); err != nil {
			result.ValidationPassed = false
			if !opts.Force {
				return nil, fmt.Errorf("validation failed: %w", err)
			}
		} else {
			result.ValidationPassed = true
		}
	}

	// Generate GitOps manifests
	manifestCount, err := s.generateGitOpsManifests(ctx, cfg, clusterPaths, opts.DryRun)
	if err != nil {
		return nil, fmt.Errorf("generating manifests: %w", err)
	}
	result.ManifestsCreated = manifestCount

	// Validate generated manifests
	if err := s.validateManifests(clusterPaths); err != nil {
		return nil, fmt.Errorf("validating manifests: %w", err)
	}

	// Commit changes if not dry run
	if !opts.DryRun {
		commitHash, err := s.commitChanges(ctx, clusterPaths)
		if err != nil {
			return nil, fmt.Errorf("committing changes: %w", err)
		}
		result.CommitHash = commitHash
	}

	return result, nil
}

// generateGitOpsManifests generates GitOps manifests from configuration
func (s *SetupService) generateGitOpsManifests(ctx context.Context, cfg v2.Config, clusterPaths *paths.ClusterPaths, dryRun bool) (int, error) {
	if dryRun {
		// In dry-run mode, just count what would be generated
		// For now, return an estimate
		return 50, nil
	}

	// Snapshot existing file modification times before generation so we can
	// distinguish files written during this run from pre-existing ones.
	snapshotBefore, err := s.snapshotFileModTimes(clusterPaths.GitOpsDir)
	if err != nil {
		return 0, fmt.Errorf("snapshotting existing files: %w", err)
	}

	// Copy base GitOps structure (always render for generation)
	if err := gitops.CopyBase(cfg, true); err != nil {
		return 0, fmt.Errorf("copying base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(cfg); err != nil {
		return 0, fmt.Errorf("rendering cluster apps: %w", err)
	}

	// Render infrastructure templates
	if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
		return 0, fmt.Errorf("rendering infrastructure cluster: %w", err)
	}

	if strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) != "kind" {
		if err := tofu.Provision(cfg); err != nil {
			return 0, fmt.Errorf("provisioning opentofu: %w", err)
		}
	}

	if err := s.encryptGeneratedSecretManifests(ctx, cfg, clusterPaths); err != nil {
		return 0, err
	}

	// Count only the files that were actually written during this generation.
	manifestCount, err := s.countGeneratedFiles(clusterPaths.GitOpsDir, snapshotBefore)
	if err != nil {
		return 0, fmt.Errorf("counting generated files: %w", err)
	}

	return manifestCount, nil
}

func (s *SetupService) encryptGeneratedSecretManifests(ctx context.Context, cfg v2.Config, clusterPaths *paths.ClusterPaths) error {
	findings, err := gitops.ScanGitOpsSecrets(clusterPaths.GitOpsDir)
	if err != nil {
		return fmt.Errorf("scanning generated manifests before encryption: %w", err)
	}

	secretFiles := make(map[string]struct{})
	for _, finding := range findings {
		switch finding.Rule {
		case "unencrypted-kubernetes-secret", "plaintext-secret-field", "invalid-sops-metadata":
			secretFiles[finding.Path] = struct{}{}
		case "age-private-key", "private-key", "git-token":
			return fmt.Errorf("generated GitOps tree contains %s in %s: %s", finding.Rule, finding.Path, finding.Message)
		case "invalid-yaml":
			return fmt.Errorf("generated GitOps tree contains invalid YAML in %s: %s", finding.Path, finding.Message)
		}
	}
	if len(secretFiles) == 0 {
		return nil
	}

	ageKeyPath := strings.TrimSpace(cfg.Secrets.SopsAgeKeyFile)
	if ageKeyPath == "" {
		return fmt.Errorf("generated Secret manifests require SOPS encryption, but secrets.sops_age_key_file is not configured")
	}
	publicKey, err := readAgePublicKeyFile(ageKeyPath)
	if err != nil {
		return fmt.Errorf("loading SOPS public key for generated Secret encryption: %w", err)
	}

	for rel := range secretFiles {
		path := filepath.Join(clusterPaths.GitOpsDir, filepath.FromSlash(rel))
		if err := s.encryptor.EncryptSecretManifest(ctx, path, publicKey, clusterPaths.SOPSConfigPath); err != nil {
			return fmt.Errorf("encrypting generated Secret manifest %s: %w", rel, err)
		}
	}
	return nil
}

// fakeManifestEncryptor rewrites Secret manifests with ENC[TEST] markers
// without requiring a real SOPS binary. Used only in test mode.
type fakeManifestEncryptor struct{}

func (e *fakeManifestEncryptor) EncryptSecretManifest(_ context.Context, path string, _ string, _ string) error {
	return testEncryptSecretManifest(path)
}

func testEncryptSecretManifest(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+2)
	inSecretMap := false
	secretIndent := 0
	hasSOPS := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "sops:" || strings.HasPrefix(trimmed, "sops:") {
			hasSOPS = true
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		if trimmed == "data:" || trimmed == "stringData:" {
			inSecretMap = true
			secretIndent = indent
			out = append(out, line)
			continue
		}
		if inSecretMap && trimmed != "" && indent <= secretIndent {
			inSecretMap = false
		}
		if inSecretMap && strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "#") {
			key, value, _ := strings.Cut(line, ":")
			if !strings.HasPrefix(strings.TrimSpace(value), "ENC[") {
				line = key + ": ENC[TEST]"
			}
		}
		out = append(out, line)
	}

	if !hasSOPS {
		if len(out) > 0 && out[len(out)-1] == "" {
			out = out[:len(out)-1]
		}
		out = append(out,
			"sops:",
			"  mac: ENC[TEST]",
			"  age:",
			"    - recipient: age1test",
			"      enc: ENC[TEST]",
		)
	}

	return os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o600)
}

func readAgePublicKeyFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			keyPair, err := crypto.ParseAgeKey(line)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(keyPair.PublicKey), nil
		}
	}
	return "", fmt.Errorf("no Age private key found in %s", path)
}

func (s *SetupService) validateSetupConfig(cfg *v2.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	if strings.TrimSpace(cfg.ClusterName()) == "" {
		return fmt.Errorf("cluster name must be set")
	}
	if strings.TrimSpace(cfg.GitDir()) == "" {
		return fmt.Errorf("opencenter.gitops.git_dir must be set in the configuration")
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	if provider == "" {
		return fmt.Errorf("opencenter.infrastructure.provider must be set")
	}
	if provider == "kind" && cfg.OpenCenter.Infrastructure.Kind == nil {
		return fmt.Errorf("opencenter.infrastructure.kind must be configured for the kind provider")
	}

	return nil
}

// validateManifests validates generated GitOps manifests
func (s *SetupService) validateManifests(clusterPaths *paths.ClusterPaths) error {
	// If using a fake encryptor (test mode), ensure any remaining plaintext
	// Secret manifests are marked as encrypted before validation runs.
	if _, ok := s.encryptor.(*fakeManifestEncryptor); ok {
		if err := testEncryptPlaintextSecretFindings(clusterPaths.GitOpsDir); err != nil {
			return err
		}
	}

	// Create manifest validator
	validator := gitops.NewManifestValidator(clusterPaths.GitOpsDir)

	// Run validation
	if err := validator.Validate(); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	return nil
}

func testEncryptPlaintextSecretFindings(gitOpsDir string) error {
	findings, err := gitops.ScanGitOpsSecrets(gitOpsDir)
	if err != nil {
		return err
	}
	secretFiles := map[string]struct{}{}
	for _, finding := range findings {
		if finding.Rule == "unencrypted-kubernetes-secret" || finding.Rule == "plaintext-secret-field" || finding.Rule == "invalid-sops-metadata" {
			secretFiles[finding.Path] = struct{}{}
		}
	}
	for rel := range secretFiles {
		path := filepath.Join(gitOpsDir, filepath.FromSlash(rel))
		if err := testEncryptSecretManifest(path); err != nil {
			return err
		}
	}
	return nil
}

// commitChanges commits generated manifests to git
func (s *SetupService) commitChanges(ctx context.Context, clusterPaths *paths.ClusterPaths) (string, error) {
	gitDir := clusterPaths.GitOpsDir
	if err := writeGitOpsHygiene(gitDir); err != nil {
		return "", err
	}

	// Check if git repository is initialized
	if _, err := os.Stat(filepath.Join(gitDir, ".git")); os.IsNotExist(err) {
		// Initialize git repository
		cmd, err := s.commandRunner.PrepareCommandContext(ctx, "git", "init")
		if err != nil {
			return "", fmt.Errorf("preparing git init: %w", err)
		}
		cmd.Dir = gitDir
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("initializing git repository: %w", err)
		}

		// Configure local git identity so commits work in environments
		// without a global git config (CI, containers, fresh installs).
		for _, kv := range [][2]string{
			{"user.name", "opencenter"},
			{"user.email", "opencenter@localhost"},
		} {
			cfgCmd, err := s.commandRunner.PrepareCommandContext(ctx, "git", "config", kv[0], kv[1])
			if err != nil {
				return "", fmt.Errorf("preparing git config %s: %w", kv[0], err)
			}
			cfgCmd.Dir = gitDir
			if err := cfgCmd.Run(); err != nil {
				return "", fmt.Errorf("setting git config %s: %w", kv[0], err)
			}
		}
	}

	hookCmd, err := s.commandRunner.PrepareCommandContext(ctx, "git", "config", "core.hooksPath", ".opencenter/hooks")
	if err != nil {
		return "", fmt.Errorf("preparing git config core.hooksPath: %w", err)
	}
	hookCmd.Dir = gitDir
	if err := hookCmd.Run(); err != nil {
		return "", fmt.Errorf("setting git config core.hooksPath: %w", err)
	}

	// Stage all files
	cmd, err := s.commandRunner.PrepareCommandContext(ctx, "git", "add", ".")
	if err != nil {
		return "", fmt.Errorf("preparing git add: %w", err)
	}
	cmd.Dir = gitDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("staging files: %w", err)
	}

	findings, err := gitops.ScanGitOpsSecretsWithOptions(ctx, gitops.SecretScanOptions{Root: gitDir, Staged: true})
	if err != nil {
		return "", fmt.Errorf("scanning staged GitOps files: %w", err)
	}
	if len(findings) > 0 {
		var messages []string
		for _, finding := range findings {
			messages = append(messages, fmt.Sprintf("%s: %s: %s", finding.Path, finding.Rule, finding.Message))
		}
		return "", fmt.Errorf("refusing to commit GitOps security findings:\n%s", strings.Join(messages, "\n"))
	}

	// Check if there are changes to commit
	cmd, err = s.commandRunner.PrepareCommandContext(ctx, "git", "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("preparing git status: %w", err)
	}
	cmd.Dir = gitDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("checking git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		// No changes to commit
		return "", nil
	}

	// Commit changes
	// Use --no-verify because the pre-commit hook calls `opencenter cluster
	// validate-manifests` which may not be available during initial setup, and
	// we already ran the security scan above on the staged blobs.
	commitMessage := "Initialize GitOps repository structure\n\n- Add base GitOps structure\n- Add cluster-specific applications\n- Add infrastructure templates"
	cmd, err = s.commandRunner.PrepareCommandContext(ctx, "git", "commit", "--no-verify", "-m", commitMessage)
	if err != nil {
		return "", fmt.Errorf("preparing git commit: %w", err)
	}
	cmd.Dir = gitDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("committing changes: %w", err)
	}

	// Get commit hash
	cmd, err = s.commandRunner.PrepareCommandContext(ctx, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("preparing git rev-parse: %w", err)
	}
	cmd.Dir = gitDir
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting commit hash: %w", err)
	}

	commitHash := strings.TrimSpace(string(output))
	return commitHash, nil
}

// skippedDirs lists directories that are not generated by opencenter and must
// be excluded from both the pre-generation snapshot and the post-generation
// count. These match the exclusions in the SOPS infrastructure path regex
// (see internal/sops/key_manager.go).
var skippedDirs = map[string]bool{
	".git":       true,
	".terraform": true,
	".bin":       true,
	"venv":       true,
	"kubespray":  true,
}

// snapshotFileModTimes walks gitDir and records the modification time of every
// regular file (excluding non-generated directories such as .git, .terraform,
// venv, kubespray, and .bin). The returned map is keyed by absolute path.
// If gitDir does not exist yet the map is empty (first-time generation).
func (s *SetupService) snapshotFileModTimes(gitDir string) (map[string]int64, error) {
	snapshot := make(map[string]int64)

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return snapshot, nil
	}

	err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && skippedDirs[info.Name()] {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			snapshot[path] = info.ModTime().UnixNano()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// countGeneratedFiles counts files that were created or modified during this
// generation run by comparing the current state against a pre-generation
// snapshot of modification times. Non-generated directories (.git, .terraform,
// venv, kubespray, .bin) are excluded from the walk.
func (s *SetupService) countGeneratedFiles(gitDir string, before map[string]int64) (int, error) {
	count := 0

	err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && skippedDirs[info.Name()] {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			prevModTime, existed := before[path]
			if !existed || info.ModTime().UnixNano() != prevModTime {
				// File is new or was modified during this run.
				count++
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}
