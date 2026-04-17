package cluster

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type gitAuthCapabilityHandler struct{}

func newGitAuthCapabilityHandler() orchestration.CapabilityHandler {
	return &gitAuthCapabilityHandler{}
}

func (h *gitAuthCapabilityHandler) Name() string {
	return "git-auth"
}

func (h *gitAuthCapabilityHandler) Applies(cfg *v2.Config, providerCtx orchestration.ProviderContext) bool {
	return cfg != nil && providerCtx.ClusterPaths != nil
}

func (h *gitAuthCapabilityHandler) Discover(ctx context.Context, cfg *v2.Config, providerCtx orchestration.ProviderContext) (orchestration.DiscoveryResult, error) {
	return orchestration.DiscoveryResult{}, nil
}

func (h *gitAuthCapabilityHandler) Prompts(cfg *v2.Config, providerCtx orchestration.ProviderContext, discovery orchestration.DiscoveryResult) []orchestration.PromptSpec {
	gitURL := strings.TrimSpace(cfg.ConfiguredGitURL())
	if gitURL == "" {
		return []orchestration.PromptSpec{
			{
				ID:       "git.url",
				Group:    configureGroupGit,
				Kind:     orchestration.PromptKindInput,
				Label:    "GitOps repository URL",
				Default:  strings.TrimSpace(cfg.OpenCenter.GitOps.GitURL),
				Required: true,
			},
		}
	}

	if isHTTPSGitURL(gitURL) {
		if strings.TrimSpace(cfg.OpenCenter.GitOps.GitTokenProvider) == "" || strings.TrimSpace(cfg.OpenCenter.GitOps.GitToken) == "" {
			return []orchestration.PromptSpec{
				{
					ID:       "git.token_provider",
					Group:    configureGroupGit,
					Kind:     orchestration.PromptKindSelect,
					Label:    "HTTPS token provider",
					Default:  firstNonEmptyString(strings.TrimSpace(cfg.OpenCenter.GitOps.GitTokenProvider), "github"),
					Required: true,
					Options: []orchestration.PromptOption{
						{Value: "github", Label: "GitHub"},
						{Value: "gitlab", Label: "GitLab"},
						{Value: "gitea", Label: "Gitea"},
						{Value: "generic", Label: "Generic token"},
					},
				},
				{
					ID:       "git.token",
					Group:    configureGroupGit,
					Kind:     orchestration.PromptKindSecret,
					Label:    "HTTPS Git token",
					Required: true,
				},
			}
		}
		return nil
	}

	if isSSHGitURL(gitURL) {
		defaultMode := "cluster"
		if strings.TrimSpace(cfg.OpenCenter.GitOps.GitSSHKey) != "" && strings.TrimSpace(cfg.OpenCenter.GitOps.GitSSHKey) != providerCtx.ClusterPaths.SSHKeyPath {
			defaultMode = "custom"
		}
		if strings.TrimSpace(cfg.OpenCenter.GitOps.GitSSHKey) == "" || strings.TrimSpace(cfg.OpenCenter.GitOps.GitSSHPub) == "" {
			return []orchestration.PromptSpec{
				{
					ID:       "git.ssh_mode",
					Group:    configureGroupGit,
					Kind:     orchestration.PromptKindSelect,
					Label:    "SSH key source",
					Default:  defaultMode,
					Required: true,
					Options: []orchestration.PromptOption{
						{Value: "cluster", Label: "Cluster key pair"},
						{Value: "custom", Label: "Custom key paths"},
					},
				},
				{ID: "git.ssh_key", Group: configureGroupGit, Kind: orchestration.PromptKindInput, Label: "SSH private key path", Default: providerCtx.ClusterPaths.SSHKeyPath},
				{ID: "git.ssh_pub", Group: configureGroupGit, Kind: orchestration.PromptKindInput, Label: "SSH public key path", Default: providerCtx.ClusterPaths.SSHKeyPath + ".pub"},
			}
		}
	}

	return nil
}

func (h *gitAuthCapabilityHandler) ApplyAnswers(cfg *v2.Config, answers orchestration.PromptAnswers, providerCtx orchestration.ProviderContext) (orchestration.ChangeSet, error) {
	changes := orchestration.ChangeSet{}

	if value := strings.TrimSpace(answers["git.url"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_url", Label: "Git URL", Value: value})
	}

	gitURL := strings.TrimSpace(answers["git.url"])
	if gitURL == "" {
		gitURL = strings.TrimSpace(cfg.ConfiguredGitURL())
	}
	if gitURL == "" {
		gitURL = strings.TrimSpace(cfg.OpenCenter.GitOps.GitURL)
	}

	if isHTTPSGitURL(gitURL) {
		tokenProvider := strings.TrimSpace(answers["git.token_provider"])
		if tokenProvider == "" {
			tokenProvider = firstNonEmptyString(strings.TrimSpace(cfg.OpenCenter.GitOps.GitTokenProvider), "github")
		}
		tokenValue := strings.TrimSpace(answers["git.token"])
		if tokenValue != "" {
			tokenPath := filepath.Join(providerCtx.ClusterPaths.SecretsDir, "git", "gitops-token.txt")
			changes.Files = append(changes.Files, orchestration.ManagedFile{
				Group:    configureGroupGit,
				Path:     tokenPath,
				Label:    "Managed Git token",
				Contents: tokenValue + "\n",
				Mode:     0o600,
				Masked:   true,
			})
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_token_provider", Label: "Git token provider", Value: tokenProvider},
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_token", Label: "Git token path", Value: tokenPath},
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_ssh_key", Label: "Git SSH private key", Value: ""},
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_ssh_pub", Label: "Git SSH public key", Value: ""},
			)
		}
		return changes, nil
	}

	if isSSHGitURL(gitURL) {
		mode := strings.TrimSpace(answers["git.ssh_mode"])
		if mode == "" {
			mode = "cluster"
		}
		privateKey := strings.TrimSpace(answers["git.ssh_key"])
		publicKey := strings.TrimSpace(answers["git.ssh_pub"])
		if mode == "cluster" {
			privateKey = providerCtx.ClusterPaths.SSHKeyPath
			publicKey = providerCtx.ClusterPaths.SSHKeyPath + ".pub"
		}
		if privateKey != "" && publicKey != "" {
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_ssh_key", Label: "Git SSH private key", Value: privateKey},
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_ssh_pub", Label: "Git SSH public key", Value: publicKey},
				orchestration.ConfigPatch{Group: configureGroupGit, Path: "opencenter.gitops.git_token", Label: "Git token path", Value: ""},
			)
		}
	}

	return changes, nil
}

func isHTTPSGitURL(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://")
}

func isSSHGitURL(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(trimmed, "ssh://") || strings.Contains(trimmed, "@")
}
