package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// writeOpenStackConfigWithStaleOrg writes an openstack blueprint directly into
// the blueprints zone for the given organization, while meta.organization
// names a different organization. This reproduces the on-disk state left by
// cluster init in a non-default org whose org assignment did not reach the
// config file: the blueprint lives under blueprints/<org>/ but the metadata
// still says "opencenter".
func writeOpenStackConfigWithStaleOrg(t *testing.T, dir, clusterName, zoneOrg, metaOrg string) {
	t.Helper()

	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		t.Fatalf("create openstack config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = metaOrg
	// Point the GitOps working directory at a vanished path (e.g. a checkout
	// under a deleted temp directory), so infrastructure destruction has
	// nothing to run against.
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(dir, "gone", "phase6-repo")

	data, err := v2.MarshalPublicConfig(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configPath := filepath.Join(dir, "clusters", "blueprints", zoneOrg, clusterName, clusterName+"-config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir blueprint dir: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
}

// TestClusterDestroyOrgPrefixedWithStaleMetaOrg covers the stuck-destroy bug:
// a blueprint stored under blueprints/test-org/ whose meta.organization still
// names the default org. `cluster list` shows it as test-org/<cluster> (zone
// location wins), so `cluster destroy test-org/<cluster> --remove-files` must
// resolve it via the zone location, treat the vanished infrastructure
// directory as already destroyed, and still complete local file removal.
func TestClusterDestroyOrgPrefixedWithStaleMetaOrg(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	const clusterName = "stale-org-destroy"
	writeOpenStackConfigWithStaleOrg(t, dir, clusterName, "test-org", "opencenter")

	configPath := filepath.Join(dir, "clusters", "blueprints", "test-org", clusterName, clusterName+"-config.yaml")

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"test-org/" + clusterName, "--force", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected blueprint config file to be removed, stat err: %v", err)
	}
	// The blueprint directory must be gone too: saveConfig leaves a
	// .backup sidecar that would otherwise survive as a phantom directory.
	if _, err := os.Stat(filepath.Dir(configPath)); !os.IsNotExist(err) {
		t.Fatalf("expected blueprint directory to be removed, stat err: %v", err)
	}
	if !strings.Contains(stdout.String(), "destroyed successfully") {
		t.Fatalf("expected success output, got:\n%s", stdout.String())
	}
}

// TestClusterDestroySkipClusterAccessConflictsWithRequireAccess asserts the
// opposing flags are rejected before any lock or infrastructure work.
func TestClusterDestroySkipClusterAccessConflictsWithRequireAccess(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterDestroyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"any-cluster", "--force", "--skip-cluster-access", "--require-cluster-access"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutual-exclusion error, got %v", err)
	}
}

// TestClusterDestroySkipClusterAccessWithUnreachableEndpoint covers the
// unreachable-kube-endpoint destroy: an openstack blueprint whose cluster API
// endpoint is dead must still complete with --skip-cluster-access --remove-
// files, without ever invoking kubectl.
func TestClusterDestroySkipClusterAccessWithUnreachableEndpoint(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	const clusterName = "dead-endpoint"
	writeOpenStackConfigWithStaleOrg(t, dir, clusterName, "opencenter", "opencenter")

	// A fake kubectl that records invocations and always fails: if the
	// capture step ran, the log file would exist.
	binDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	writeFakeExecutable(t, filepath.Join(binDir, "kubectl"), `#!/bin/sh
set -eu
echo "kubectl $*" >> "${FAKE_KIND_STATE_DIR}/kubectl.log"
exit 1
`)
	prependTestPath(t, binDir)

	configPath := filepath.Join(dir, "clusters", "blueprints", "opencenter", clusterName, clusterName+"-config.yaml")

	cmd := newClusterDestroyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{clusterName, "--force", "--skip-cluster-access", "--remove-files"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected blueprint config file to be removed, stat err: %v", err)
	}
	if !strings.Contains(stdout.String(), "Skipping CSI volume capture (--skip-cluster-access)") {
		t.Fatalf("expected skip notice in output, got:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "kubectl.log")); !os.IsNotExist(err) {
		t.Fatalf("kubectl must never run under --skip-cluster-access")
	}
}

// TestLoadConfigWithIdentifierHealsStaleMetaOrg asserts that loading via an
// org-qualified identifier trusts the blueprint zone location over stale
// metadata instead of failing with "not found in organization".
func TestLoadConfigWithIdentifierHealsStaleMetaOrg(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	const clusterName = "stale-org-load"
	writeOpenStackConfigWithStaleOrg(t, dir, clusterName, "test-org", "opencenter")

	cfg, gotCluster, gotOrg, err := loadConfigWithIdentifier(context.Background(), "test-org/"+clusterName)
	if err != nil {
		t.Fatalf("loadConfigWithIdentifier: %v", err)
	}
	if gotCluster != clusterName {
		t.Fatalf("cluster name = %q, want %q", gotCluster, clusterName)
	}
	if gotOrg != "test-org" {
		t.Fatalf("organization = %q, want test-org", gotOrg)
	}
	if cfg.OpenCenter.Meta.Organization != "test-org" {
		t.Fatalf("meta.organization = %q, want healed to test-org", cfg.OpenCenter.Meta.Organization)
	}
}
