package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/plugins"
)

func TestPluginsListUsesGlobalJSONOutput(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	pluginDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	pluginPath := filepath.Join(pluginDir, "opencenter-demo")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	t.Setenv("OPENCENTER_PLUGINS_DIR", pluginDir)
	t.Setenv("PATH", filepath.Join(dir, "empty-path"))

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"plugins", "list", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("plugins list --output json failed: %v", err)
	}

	var discovered map[string]plugins.PluginInfo
	if err := json.Unmarshal(out.Bytes(), &discovered); err != nil {
		t.Fatalf("expected JSON plugin list, got %q: %v", out.String(), err)
	}

	info, ok := discovered["opencenter-demo"]
	if !ok {
		t.Fatalf("expected opencenter-demo in output, got %#v", discovered)
	}
	if info.Path != pluginPath || info.Status != plugins.VerificationStatusUnverified || info.Message == "" {
		t.Fatalf("unexpected plugin info: %#v", info)
	}
}

func TestPluginsListTextIncludesVerificationMessage(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	pluginDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	pluginPath := filepath.Join(pluginDir, "opencenter-demo")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	t.Setenv("OPENCENTER_PLUGINS_DIR", pluginDir)
	t.Setenv("PATH", filepath.Join(dir, "empty-path"))

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"plugins", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("plugins list failed: %v", err)
	}

	output := out.String()
	for _, want := range []string{"demo", pluginPath, plugins.VerificationStatusUnverified, "no checksum entry"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in plugins list output, got %q", want, output)
		}
	}
}

func TestPluginsListRejectsDryRun(t *testing.T) {
	root := newOutputRootForCommandTest()
	root.SetArgs([]string{"plugins", "list", "--dry-run"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected read-only plugins list to reject --dry-run")
	}
	if !strings.Contains(err.Error(), `--dry-run has no effect for read-only command "opencenter plugins list"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
