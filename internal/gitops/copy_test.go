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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter/internal/config"
	"github.com/rackerlabs/openCenter/internal/provision"
)

func TestMain(m *testing.M) {
	if err := provision.Init(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestCopyBase(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("test")
	cfg.OpenCenter.GitOps.GitDir = dst

	if err := CopyBase(cfg, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, ".gitignore")); os.IsNotExist(err) {
		t.Error(".gitignore was not copied")
	}

	files, err := filepath.Glob(filepath.Join(dst, "*"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("files in dst: %v", files)
}

func TestRenderInfrastructureClusterRendersConfigValues(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("render-test")
	cfg.OpenCenter.Cluster.ClusterName = "render-test"
	cfg.OpenCenter.GitOps.GitDir = dst
	cfg.OpenCenter.Cluster.Kubernetes.Version = "9.9.9"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://auth.example.local/v3/"

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, cfg.OpenCenter.Cluster.Kubernetes.Version) {
		t.Fatalf("rendered main.tf missing kubernetes version %q\ncontent:\n%s", cfg.OpenCenter.Cluster.Kubernetes.Version, content)
	}
	if !strings.Contains(content, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL) {
		t.Fatalf("rendered main.tf missing auth_url %q\ncontent:\n%s", cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL, content)
	}
}

func TestRenderClusterAppsRendersClusterName(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("cluster-apps")
	cfg.OpenCenter.Cluster.ClusterName = "cluster-apps"
	cfg.OpenCenter.GitOps.GitDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps returned error: %v", err)
	}

	sourcesFile := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "managed-services", "fluxcd", "sources.yaml")
	data, err := os.ReadFile(sourcesFile)
	if err != nil {
		t.Fatalf("failed to read rendered sources.yaml: %v", err)
	}
	if !strings.Contains(string(data), cfg.ClusterName()) {
		t.Fatalf("rendered sources.yaml missing cluster name %q\ncontent:\n%s", cfg.ClusterName(), string(data))
	}
}
