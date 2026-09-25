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

package gitops

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	gitopsAuthMethodToken = "token"
	gitopsAuthMethodSSH   = "ssh"
)

// SourceAuthParams holds the parameters needed to render a GitRepository
// source auth block with an active and a commented-out alternative variant.
type SourceAuthParams struct {
	AuthMethod string
	TokenURL   string
	SSHURL     string
	RefType    string
	RefValue   string
	SecretName string

	// Anonymous omits the secretRef entirely so Flux accesses the repository
	// without credentials. The shared openCenter-gitops-base repository is
	// public, so its GitRepository source must not reference a credential
	// Secret; attaching one (e.g. the token-derived opencenter-base Secret,
	// which in local mode holds the Gitea token) makes Flux authenticate to
	// public GitHub with the wrong credential and fail with HTTP 401.
	Anonymous bool
}

// RenderSourceAuthBlock renders the URL, ref, and (unless Anonymous) Secret
// reference for the selected Git authentication method, followed by a fully
// commented-out equivalent for the alternative method. The auth method is
// explicit rather than inferred from a repository URL.
func RenderSourceAuthBlock(params SourceAuthParams) string {
	authMethod := normalizedSourceAuthMethod(params.AuthMethod)
	secretName := strings.TrimSpace(params.SecretName)
	if secretName == "" {
		secretName = "opencenter-base"
	}

	activeLabel, activeURL := "token auth", params.TokenURL
	alternativeLabel, alternativeURL := "ssh auth", params.SSHURL
	if authMethod == gitopsAuthMethodSSH {
		activeLabel, activeURL = "ssh auth", params.SSHURL
		alternativeLabel, alternativeURL = "token auth", params.TokenURL
	}

	var b strings.Builder
	fmt.Fprintf(&b, "  # --- %s (active) ---\n", activeLabel)
	fmt.Fprintf(&b, "  url: %s\n", activeURL)
	b.WriteString("  ref:\n")
	fmt.Fprintf(&b, "    %s: \"%s\"\n", params.RefType, params.RefValue)
	if !params.Anonymous {
		b.WriteString("  secretRef:\n")
		fmt.Fprintf(&b, "    name: %s\n", secretName)
	}
	fmt.Fprintf(&b, "  # --- %s (alternative) ---\n", alternativeLabel)
	fmt.Fprintf(&b, "  # url: %s\n", alternativeURL)
	b.WriteString("  # ref:\n")
	fmt.Fprintf(&b, "  #   %s: \"%s\"", params.RefType, params.RefValue)
	if !params.Anonymous {
		b.WriteString("\n  # secretRef:\n")
		fmt.Fprintf(&b, "  #   name: %s", secretName)
	}

	return b.String()
}

// BuildSourceAuthParams derives token and SSH URL variants for repositoryURL
// and retains the explicit auth method selected for this generation run.
func BuildSourceAuthParams(authMethod, repositoryURL, refType, refValue, secretName string) (SourceAuthParams, error) {
	tokenURL, sshURL, err := GitRepositoryURLVariants(repositoryURL)
	if err != nil {
		return SourceAuthParams{}, err
	}

	return SourceAuthParams{
		AuthMethod: normalizedSourceAuthMethod(authMethod),
		TokenURL:   tokenURL,
		SSHURL:     sshURL,
		RefType:    refType,
		RefValue:   refValue,
		SecretName: secretName,
	}, nil
}

// GitRepositoryURLVariants converts a Git repository URL to equivalent HTTPS
// (token) and ssh:// (SSH key) URLs while preserving its host and path. It
// accepts https://, ssh://, and scp-style git@host:path input.
func GitRepositoryURLVariants(repositoryURL string) (tokenURL, sshURL string, err error) {
	repositoryURL = strings.TrimSpace(repositoryURL)
	if repositoryURL == "" {
		return "", "", fmt.Errorf("repository URL is required")
	}

	if at := strings.Index(repositoryURL, "@"); at > 0 && !strings.Contains(repositoryURL[:at], "://") {
		parts := strings.SplitN(repositoryURL[at+1:], ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.Trim(strings.TrimSpace(parts[1]), "/") == "" {
			return "", "", fmt.Errorf("invalid scp-style repository URL %q", repositoryURL)
		}
		host := strings.TrimSpace(parts[0])
		path := "/" + strings.Trim(strings.TrimSpace(parts[1]), "/")
		return "https://" + host + path, "ssh://git@" + host + path, nil
	}

	parsed, parseErr := url.Parse(repositoryURL)
	if parseErr != nil {
		return "", "", fmt.Errorf("parse repository URL %q: %w", repositoryURL, parseErr)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" && scheme != "ssh" {
		return "", "", fmt.Errorf("repository URL %q must use https://, ssh://, or git@host:path", repositoryURL)
	}
	if parsed.Hostname() == "" {
		return "", "", fmt.Errorf("repository URL %q does not include a host", repositoryURL)
	}
	if strings.Trim(parsed.Path, "/") == "" {
		return "", "", fmt.Errorf("repository URL %q does not include a repository path", repositoryURL)
	}

	host := parsed.Hostname()
	if port := parsed.Port(); port != "" {
		host += ":" + port
	}
	path := "/" + strings.Trim(parsed.EscapedPath(), "/")
	return "https://" + host + path, "ssh://git@" + host + path, nil
}

func normalizedSourceAuthMethod(authMethod string) string {
	if strings.EqualFold(strings.TrimSpace(authMethod), gitopsAuthMethodSSH) {
		return gitopsAuthMethodSSH
	}
	return gitopsAuthMethodToken
}
