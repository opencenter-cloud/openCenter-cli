package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

func findSubcommand(cmd interface{ Commands() []*cobra.Command }, name string) *cobra.Command {
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == name || subcmd.HasAlias(name) {
			return subcmd
		}
	}
	return nil
}

func TestSecretsKeysGACommandsExist(t *testing.T) {
	cmd := NewSecretsKeysCmd()

	for _, name := range []string{"generate", "rotate", "backup", "validate", "check", "revoke"} {
		if findSubcommand(cmd, name) == nil {
			t.Fatalf("expected secrets keys %s command to exist", name)
		}
	}
}

func TestSecretsKeysRotateAcceptsClusterScope(t *testing.T) {
	cmd := newSecretsKeysRotateCmd()
	if cmd.Flags().Lookup("cluster") == nil {
		t.Fatal("expected secrets keys rotate --cluster flag")
	}
	if cmd.Flags().Lookup("type") == nil {
		t.Fatal("expected secrets keys rotate --type flag")
	}
}

func TestSecretsKeysRevokeAcceptsClusterScope(t *testing.T) {
	cmd := newSecretsKeysRevokeCmd()
	if cmd.Flags().Lookup("cluster") == nil {
		t.Fatal("expected secrets keys revoke --cluster flag")
	}
	if cmd.Flags().Lookup("user") == nil {
		t.Fatal("expected secrets keys revoke --user flag")
	}
	if cmd.Flags().Lookup("key") == nil {
		t.Fatal("expected secrets keys revoke --key flag")
	}
}

func TestSecretsKeysCheckRecommendationsUseSecretsKeysRotate(t *testing.T) {
	testCases := []struct {
		name     string
		report   *secrets.ExpirationReport
		expected string
	}{
		{
			name: "expired",
			report: &secrets.ExpirationReport{
				Expired: []secrets.KeyExpirationInfo{
					{
						Cluster:       "expired-cluster",
						KeyType:       secrets.KeyTypeAge,
						Fingerprint:   "age-expired",
						DaysRemaining: -1,
						ExpiresAt:     time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			expected: "opencenter secrets keys rotate --cluster expired-cluster --type age",
		},
		{
			name: "warning",
			report: &secrets.ExpirationReport{
				Warning: []secrets.KeyExpirationInfo{
					{
						Cluster:       "warning-cluster",
						KeyType:       secrets.KeyTypeSSH,
						Fingerprint:   "ssh-warning",
						DaysRemaining: 7,
						ExpiresAt:     time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			expected: "opencenter secrets keys rotate --cluster warning-cluster --type ssh",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newClusterCheckKeysCmd()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			displayExpirationReportText(cmd, tc.report, 14)

			output := stdout.String() + stderr.String()
			if !strings.Contains(output, tc.expected) {
				t.Fatalf("expected recommendation %q in output:\n%s", tc.expected, output)
			}
			if strings.Contains(output, "opencenter cluster rotate-keys") {
				t.Fatalf("expected no stale cluster rotate-keys recommendation in output:\n%s", output)
			}
		})
	}
}
