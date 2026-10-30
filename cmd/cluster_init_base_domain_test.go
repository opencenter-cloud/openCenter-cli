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
	"context"
	"bytes"
	"strings"
	"testing"
)

// TestClusterInitBaseDomainFlagSeedsManagedDomain pins the --base-domain
// contract: the value lands on disk with managed.base_domain: default and the
// derived cluster_fqdn, so the cluster keeps following
// cluster_defaults.base_domain from settings at every load.
func TestClusterInitBaseDomainFlagSeedsManagedDomain(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"bdflag-init", "--type", "kind", "--no-keygen", "--base-domain", "K8S.Example.COM"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init --base-domain failed: %v\nstderr: %s", err, stderr.String())
	}

	configText := readClusterConfigText(t, "bdflag-init")
	if !strings.Contains(configText, "base_domain: k8s.example.com") {
		t.Fatalf("--base-domain value not written (or not lowercased):\n%s", configText)
	}
	if !strings.Contains(configText, "base_domain: default") {
		t.Fatalf("--base-domain must tag managed.base_domain: default:\n%s", configText)
	}
	if !strings.Contains(configText, "cluster_fqdn: bdflag-init.dfw3.k8s.example.com") {
		t.Fatalf("cluster_fqdn was not derived from --base-domain:\n%s", configText)
	}
	if !strings.Contains(stdout.String(), "Base domain: k8s.example.com") {
		t.Fatalf("result message must report the flag-sourced domain:\n%s", stdout.String())
	}
}

// TestClusterInitBaseDomainFlagFollowsSettingsChange proves the point of the
// managed tag: after init with --base-domain, a later settings change
// propagates through generate with no pin and no --sync-base-domain.
func TestClusterInitBaseDomainFlagFollowsSettingsChange(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)

	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

	writeSettingsBaseDomain(t, "first.example.com")

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"bdfollow", "--type", "kind", "--base-domain", "seed.example.com"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}
	resetCommandStateForTests()

	configText := readClusterConfigText(t, "bdfollow")
	if !strings.Contains(configText, "base_domain: seed.example.com") {
		t.Fatalf("--base-domain must win over the settings value at init:\n%s", configText)
	}

	// Settings move; the managed cluster must follow at the next load.
	writeSettingsBaseDomain(t, "moved.example.org")
	genCmd := newClusterGenerateCmd()
	var stdout, stderr bytes.Buffer
	genCmd.SetOut(&stdout)
	genCmd.SetErr(&stderr)
	genCmd.SetArgs([]string{"bdfollow", "--skip-validation"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("generate failed: %v\nstderr: %s", err, stderr.String())
	}
	configText = readClusterConfigText(t, "bdfollow")
	if !strings.Contains(configText, "base_domain: moved.example.org") {
		t.Fatalf("managed cluster did not follow the settings change:\n%s", configText)
	}
	if !strings.Contains(configText, "cluster_fqdn: bdfollow.dfw3.moved.example.org") {
		t.Fatalf("cluster_fqdn did not follow the settings change:\n%s", configText)
	}
}

// TestClusterInitBaseDomainFlagValidations pins the fail-fast contract: an
// invalid domain, and a --base-domain combined with the dotted override,
// error before any cluster state is written.
func TestClusterInitBaseDomainFlagValidations(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"invalid domain", []string{"bdvalid", "--no-keygen", "--base-domain", "not a domain"}, "invalid --base-domain"},
		{"consecutive dots", []string{"bdvalid", "--no-keygen", "--base-domain", "a..b.com"}, "consecutive dots"},
		{"empty value", []string{"bdvalid", "--no-keygen", "--base-domain", " "}, "cannot be empty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newClusterInitCmd()
			cmd.SetOut(&bytes.Buffer{})
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	// --base-domain is the last word: a duplicate dotted override is replaced,
	// not raced.
	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"bddup", "--type", "kind", "--no-keygen",
		"--opencenter.cluster.base_domain=other.example.net",
		"--base-domain", "winner.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init with duplicate overrides failed: %v\nstderr: %s", err, stderr.String())
	}
	configText := readClusterConfigText(t, "bddup")
	if !strings.Contains(configText, "base_domain: winner.example.com") {
		t.Fatalf("--base-domain must win over the dotted override:\n%s", configText)
	}
	if strings.Contains(configText, "other.example.net") {
		t.Fatalf("dotted override must be replaced:\n%s", configText)
	}
}
// TestClusterInitDryRunReportsBaseDomain pins that the dry-run preview names
// the base domain it would set.
func TestClusterInitDryRunReportsBaseDomain(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"bddry", "--type", "kind", "--no-keygen", "--base-domain", "dry.example.com"})
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{DryRun: true, Output: OutputText}))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run init failed: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Would set base domain: dry.example.com") {
		t.Fatalf("dry-run must preview the base domain:\n%s", stdout.String())
	}
}
