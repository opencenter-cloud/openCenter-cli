package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadExternalPlugins_RegistersLocalPluginName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows for shell script exec")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "opencenter-local")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	t.Setenv("OPENCENTER_PLUGINS_DIR", dir)

	root := &cobra.Command{Use: "opencenter-test"}
	LoadExternalPlugins(root)

	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "local" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(root.Commands()))
		for _, c := range root.Commands() {
			names = append(names, c.Name())
		}
		t.Fatalf("expected plugin command 'local' among registered commands %v", names)
	}
}
