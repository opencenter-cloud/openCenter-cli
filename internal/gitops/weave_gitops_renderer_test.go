package gitops

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWeaveGitOpsRendererGeneratesHashFromPassword(t *testing.T) {
	cfg := newDefault("weave-password")
	cfg.Secrets.WeaveGitOps.Password = "test-password"
	cfg.Secrets.WeaveGitOps.PasswordHash = ""

	rendered, err := weaveGitOpsRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, rendered, "passwordHash: $2")
	require.NotContains(t, rendered, "test-password")
}

func TestWeaveGitOpsRendererPreservesConfiguredHash(t *testing.T) {
	cfg := newDefault("weave-password-hash")
	cfg.Secrets.WeaveGitOps.Password = "ignored-password"
	cfg.Secrets.WeaveGitOps.PasswordHash = "$2a$10$already-configured"

	rendered, err := weaveGitOpsRenderer(cfg)
	require.NoError(t, err)
	require.True(t, strings.Contains(rendered, "passwordHash: $2a$10$already-configured"))
}
