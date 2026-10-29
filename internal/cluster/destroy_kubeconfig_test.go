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

package cluster

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestResolveClusterKubeconfigPath_PrefersSecretsZone guards the silent-skip
// regression: bootstrap writes the kubeconfig to the secrets zone, and the
// destroy CSI capture must look there first. Before the fix it only looked at
// <gitops>/infrastructure/clusters/<name>/kubeconfig.yaml, never found it,
// and silently skipped volume capture forever.
func TestResolveClusterKubeconfigPath_PrefersSecretsZone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)
	t.Setenv("OPENCENTER_CLUSTERS_DIR", "")
	t.Setenv("OPENCENTER_SECRETS_DIR", "")

	cfg := makeOpenStackCfg(t)
	cfg.OpenCenter.Meta.Organization = "acme"
	clusterDir := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName())

	secretsKubeconfig := filepath.Join(dir, "clusters", "secrets", "acme", cfg.ClusterName(), "kubernetes", "kubeconfig.yaml")
	if err := os.MkdirAll(filepath.Dir(secretsKubeconfig), 0o700); err != nil {
		t.Fatalf("mkdir secrets zone: %v", err)
	}
	if err := os.WriteFile(secretsKubeconfig, []byte("secrets-zone"), 0o600); err != nil {
		t.Fatalf("write secrets kubeconfig: %v", err)
	}

	// Also place a stale legacy file: the secrets zone must still win.
	legacy := filepath.Join(clusterDir, "kubeconfig.yaml")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy kubeconfig: %v", err)
	}

	got := resolveClusterKubeconfigPath(cfg, clusterDir)
	if got != secretsKubeconfig {
		t.Fatalf("resolveClusterKubeconfigPath() = %q, want secrets zone %q", got, secretsKubeconfig)
	}
}

// TestResolveClusterKubeconfigPath_FallsBackToLegacy asserts the pre-migration
// layout keeps working when no secrets-zone kubeconfig exists.
func TestResolveClusterKubeconfigPath_FallsBackToLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)
	t.Setenv("OPENCENTER_CLUSTERS_DIR", "")
	t.Setenv("OPENCENTER_SECRETS_DIR", "")

	cfg := makeOpenStackCfg(t)
	clusterDir := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName())

	got := resolveClusterKubeconfigPath(cfg, clusterDir)
	want := filepath.Join(clusterDir, "kubeconfig.yaml")
	if got != want {
		t.Fatalf("resolveClusterKubeconfigPath() = %q, want legacy %q", got, want)
	}
}

// TestKubeconfigForDestroy covers the absent-vs-present contract the capture
// step relies on: an existing candidate is returned; total absence yields an
// error wrapping errKubeconfigNotFound naming every candidate tried.
func TestKubeconfigForDestroy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)
	t.Setenv("OPENCENTER_CLUSTERS_DIR", "")
	t.Setenv("OPENCENTER_SECRETS_DIR", "")

	cfg := makeOpenStackCfg(t)
	clusterDir := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName())

	// Absent: error must wrap errKubeconfigNotFound.
	_, err := kubeconfigForDestroy(cfg, clusterDir)
	if err == nil {
		t.Fatal("expected error when no kubeconfig candidate exists")
	}
	if !errors.Is(err, errKubeconfigNotFound) {
		t.Fatalf("error %v does not wrap errKubeconfigNotFound", err)
	}

	// Legacy file present: returned.
	legacy := filepath.Join(clusterDir, "kubeconfig.yaml")
	if err := os.WriteFile(legacy, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write legacy kubeconfig: %v", err)
	}
	got, err := kubeconfigForDestroy(cfg, clusterDir)
	if err != nil {
		t.Fatalf("kubeconfigForDestroy() with legacy file: %v", err)
	}
	if got != legacy {
		t.Fatalf("kubeconfigForDestroy() = %q, want %q", got, legacy)
	}
}
