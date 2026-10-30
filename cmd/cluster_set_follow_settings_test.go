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

package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
)

// TestClusterSetFollowSettingsTagsManagedAndPersists pins that
// --follow-settings writes managed.base_domain: default (instead of pinning)
// and that the settings domain then propagates through a later load+generate.
func TestClusterSetFollowSettingsTagsManagedAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)

	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

	// A real init (with keys) so the later generate can encrypt: the point of
	// this test is the full settings-propagation loop through generate.
	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"follow-set", "--type", "kind"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}
	resetCommandStateForTests()
	configPath := filepath.Join(dir, "clusters", "blueprints", "opencenter", "follow-set", "follow-set-config.yaml")

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"follow-set", "opencenter.cluster.base_domain=follow.example.com", "--follow-settings"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set --follow-settings failed: %v\nstderr: %s", err, stderr.String())
	}

	updated := loadV2ConfigForTest(t, configPath)
	if updated.OpenCenter.Cluster.BaseDomain != "follow.example.com" {
		t.Fatalf("base_domain = %q", updated.OpenCenter.Cluster.BaseDomain)
	}
	if !updated.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		t.Fatal("--follow-settings must tag managed.base_domain: default")
	}
	if want := "follow-set.dfw3.follow.example.com"; updated.OpenCenter.Cluster.ClusterFQDN != want {
		t.Fatalf("cluster_fqdn = %q, want %q", updated.OpenCenter.Cluster.ClusterFQDN, want)
	}
	if !strings.Contains(stdout.String(), "follows cluster_defaults.base_domain") {
		t.Fatalf("output must confirm follow-settings:\n%s", stdout.String())
	}

	// The settings file then owns the value: a later settings change moves the
	// persisted config at the next load (generate), without --sync-base-domain.
	writeSettingsBaseDomain(t, "moved.example.org")
	genCmd := newClusterGenerateCmd()
	genCmd.SetOut(&bytes.Buffer{})
	genCmd.SetErr(&bytes.Buffer{})
	genCmd.SetArgs([]string{"follow-set", "--skip-validation"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("generate after settings change failed: %v", err)
	}
	resetCommandStateForTests()
	after := loadV2ConfigForTest(t, configPath)
	if after.OpenCenter.Cluster.BaseDomain != "moved.example.org" {
		t.Fatalf("settings change did not propagate: base_domain = %q", after.OpenCenter.Cluster.BaseDomain)
	}
	if after.OpenCenter.Cluster.ClusterFQDN != "follow-set.dfw3.moved.example.org" {
		t.Fatalf("cluster_fqdn did not follow: %q", after.OpenCenter.Cluster.ClusterFQDN)
	}
}

// TestClusterSetDefaultStillPins pins the unchanged default: a bare
// base_domain assignment stays cluster-owned.
func TestClusterSetDefaultStillPins(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "pin-set", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"pin-set", "opencenter.cluster.base_domain=mine.example.net"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v", err)
	}

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	if updated.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		t.Fatal("a bare base_domain assignment must stay pinned")
	}
	if updated.OpenCenter.Cluster.Managed.BaseDomain != v2.ProvenancePinned {
		t.Fatalf("managed.base_domain = %q, want pinned", updated.OpenCenter.Cluster.Managed.BaseDomain)
	}
}

// TestClusterSetFollowSettingsRejectsBootstrapStage pins the safety guard: a
// live cluster's DNS identity never switches back to settings ownership.
func TestClusterSetFollowSettingsRejectsBootstrapStage(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "live-set", "opencenter")
	cfg.OpenCenter.Meta.Stage = v2.StageBootstrap
	cfg.OpenCenter.Meta.Status = v2.StatusSuccess
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"live-set", "opencenter.cluster.base_domain=other.example.com", "--follow-settings"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("expected bootstrap-stage rejection, got %v", err)
	}

	// The rejection must leave the on-disk identity untouched.
	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	if updated.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		t.Fatal("rejected --follow-settings must not tag the field managed")
	}
}

// TestClusterSetFollowSettingsWithoutAssignmentIsInert pins that the flag
// only speaks to base_domain provenance: other assignments behave exactly as
// before.
func TestClusterSetFollowSettingsWithoutAssignmentIsInert(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "inert-set", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"inert-set", "opencenter.cluster.admin_email=ops@example.com", "--follow-settings"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v", err)
	}

	updated := loadV2ConfigForTest(t, clusterPaths.ConfigPath)
	if updated.OpenCenter.Cluster.AdminEmail != "ops@example.com" {
		t.Fatalf("admin_email = %q", updated.OpenCenter.Cluster.AdminEmail)
	}
	if updated.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		t.Fatal("--follow-settings without a base_domain assignment must not tag provenance")
	}
}

