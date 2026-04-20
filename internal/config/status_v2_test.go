package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

func TestUpdateStatusPreservesNativeV2Structure(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", baseDir)

	pathResolver := paths.NewPathResolver(filepath.Join(baseDir, "clusters"))
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "status-kind", "opencenter"); err != nil {
		t.Fatalf("CreateClusterDirectories() error = %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "status-kind", "opencenter")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	cfg, err := v2.NewV2Default("status-kind", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
	cfg.Secrets.SOPSConfig.AgeKeyFile = clusterPaths.SOPSKeyPath
	cfg.Secrets.SSHKey.Private = clusterPaths.SSHKeyPath
	cfg.Secrets.SSHKey.Public = clusterPaths.SSHKeyPath + ".pub"
	if cfg.OpenCenter.GitOps.Auth.SSH == nil {
		cfg.OpenCenter.GitOps.Auth.SSH = &v2.GitOpsSSHAuth{}
	}
	cfg.OpenCenter.GitOps.Auth.SSH.PrivateKey = clusterPaths.SSHKeyPath
	cfg.OpenCenter.GitOps.Auth.SSH.PublicKey = clusterPaths.SSHKeyPath + ".pub"
	cfg.OpenCenter.Infrastructure.SSH.KeyPath = clusterPaths.SSHKeyPath

	loader := v2.NewConfigLoader(defaults.NewRegistry())
	if err := loader.SaveToFile(cfg, clusterPaths.ConfigPath); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	if err := UpdateStatus("status-kind", StageBootstrap, StatusSuccess); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if strings.Contains(string(data), "ssh_user:") {
		t.Fatalf("expected native v2 file to stay in v2 shape, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "\n        compute:") && !strings.Contains(string(data), "\n    compute:") {
		t.Fatalf("expected native v2 compute section to remain present, got:\n%s", string(data))
	}

	updatedCfg, err := loader.LoadFromFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if updatedCfg.OpenCenter.Meta.Stage != StageBootstrap {
		t.Fatalf("stage = %q, want %q", updatedCfg.OpenCenter.Meta.Stage, StageBootstrap)
	}
	if updatedCfg.OpenCenter.Meta.Status != StatusSuccess {
		t.Fatalf("status = %q, want %q", updatedCfg.OpenCenter.Meta.Status, StatusSuccess)
	}
}
