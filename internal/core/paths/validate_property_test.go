package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_ValidateAcceptsDisjointZones generates random valid layouts
// where GitOps, state, and secrets zones are disjoint and confirms Validate()
// accepts them all.
func TestProperty_ValidateAcceptsDisjointZones(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200
	properties := gopter.NewProperties(parameters)

	properties.Property("disjoint zones always pass validation", prop.ForAll(
		func(org, cluster string) bool {
			root := t.TempDir()
			gitopsDir := filepath.Join(root, "gitops", org)
			stateDir := filepath.Join(root, "state", org, cluster)
			secretsDir := filepath.Join(root, "secrets", org, cluster)

			if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
				return false
			}

			p := ClusterPaths{
				GitOpsDir:       gitopsDir,
				ClusterStateDir: stateDir,
				SecretsDir:      secretsDir,
				ConfigPath:      filepath.Join(stateDir, cluster+"-config.yaml"),
				SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", cluster+"-key.txt"),
				SSHKeyPath:      filepath.Join(secretsDir, "ssh", cluster),
			}
			return p.Validate() == nil
		},
		genOrgName(),
		genClusterName(),
	))

	properties.Property("overlapping zones always fail validation", prop.ForAll(
		func(org, cluster string, overlapTarget int) bool {
			root := t.TempDir()
			gitopsDir := filepath.Join(root, "gitops", org)
			stateDir := filepath.Join(root, "state", org, cluster)
			secretsDir := filepath.Join(root, "secrets", org, cluster)

			if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
				return false
			}

			p := ClusterPaths{
				GitOpsDir:       gitopsDir,
				ClusterStateDir: stateDir,
				SecretsDir:      secretsDir,
				ConfigPath:      filepath.Join(stateDir, cluster+"-config.yaml"),
				SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", cluster+"-key.txt"),
				SSHKeyPath:      filepath.Join(secretsDir, "ssh", cluster),
			}

			// Force one zone to overlap with gitops
			switch overlapTarget % 5 {
			case 0:
				p.ClusterStateDir = filepath.Join(gitopsDir, "state")
				p.ConfigPath = filepath.Join(p.ClusterStateDir, cluster+"-config.yaml")
			case 1:
				p.SecretsDir = filepath.Join(gitopsDir, "secrets")
				p.SOPSKeyPath = filepath.Join(p.SecretsDir, "age", "keys", cluster+"-key.txt")
				p.SSHKeyPath = filepath.Join(p.SecretsDir, "ssh", cluster)
			case 2:
				p.ConfigPath = filepath.Join(gitopsDir, cluster+"-config.yaml")
			case 3:
				p.SOPSKeyPath = filepath.Join(gitopsDir, "key.txt")
			case 4:
				p.SSHKeyPath = filepath.Join(gitopsDir, "ssh-key")
			}

			return p.Validate() != nil
		},
		genOrgName(),
		genClusterName(),
		gen.IntRange(0, 100),
	))

	properties.Property("equality of gitops and state is rejected", prop.ForAll(
		func(org, cluster string) bool {
			root := t.TempDir()
			gitopsDir := filepath.Join(root, "shared", org)
			if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
				return false
			}

			p := ClusterPaths{
				GitOpsDir:       gitopsDir,
				ClusterStateDir: gitopsDir, // same as gitops
				SecretsDir:      filepath.Join(root, "secrets", org, cluster),
				ConfigPath:      filepath.Join(gitopsDir, cluster+"-config.yaml"),
				SOPSKeyPath:     filepath.Join(root, "secrets", org, cluster, "age", "keys", cluster+"-key.txt"),
				SSHKeyPath:      filepath.Join(root, "secrets", org, cluster, "ssh", cluster),
			}
			return p.Validate() != nil
		},
		genOrgName(),
		genClusterName(),
	))

	properties.Property("sibling paths with common prefix are accepted", prop.ForAll(
		func(org, cluster string) bool {
			root := t.TempDir()
			gitopsDir := filepath.Join(root, "zones", org)
			if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
				return false
			}

			// Sibling: same prefix but different suffix (e.g. "zones/acme" vs "zones/acme-state")
			stateDir := filepath.Join(root, "zones", org+"-state", cluster)
			secretsDir := filepath.Join(root, "zones", org+"-secrets", cluster)

			p := ClusterPaths{
				GitOpsDir:       gitopsDir,
				ClusterStateDir: stateDir,
				SecretsDir:      secretsDir,
				ConfigPath:      filepath.Join(stateDir, cluster+"-config.yaml"),
				SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", cluster+"-key.txt"),
				SSHKeyPath:      filepath.Join(secretsDir, "ssh", cluster),
			}
			return p.Validate() == nil
		},
		genOrgName(),
		genClusterName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func genOrgName() gopter.Gen {
	return gen.RegexMatch("[a-z][a-z0-9-]{1,12}")
}

func genClusterName() gopter.Gen {
	return gen.RegexMatch("[a-z][a-z0-9-]{1,12}")
}
