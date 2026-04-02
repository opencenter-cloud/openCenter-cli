package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadExternalPlugins_AddsAndRunsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows for shell script exec")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "opencenter-hello")
	content := "#!/usr/bin/env sh\necho plugin-ok\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	// Point discovery to the temp dir only
	t.Setenv("OPENCENTER_PLUGINS_DIR", dir)

	root := &cobra.Command{Use: "opencenter-test"}
	LoadExternalPlugins(root)

	var hello *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "hello" {
			hello = c
			break
		}
	}
	if hello == nil {
		t.Fatalf("expected plugin command 'hello' to be registered")
	}

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Run the plugin command
	if err := hello.RunE(hello, []string{}); err != nil {
		t.Fatalf("plugin run failed: %v", err)
	}

	// Restore stdout and read
	w.Close()
	os.Stdout = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "plugin-ok") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestLoadExternalPlugins_AcceptsLowercasePrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows for shell script exec")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "opencenter-lower")
	content := "#!/usr/bin/env sh\necho lower-plugin\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	t.Setenv("OPENCENTER_PLUGINS_DIR", dir)

	root := &cobra.Command{Use: "opencenter-test"}
	LoadExternalPlugins(root)

	var lower *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "lower" {
			lower = c
			break
		}
	}

	if lower == nil {
		t.Fatalf("expected lowercase plugin command to be registered")
	}

	if err := lower.RunE(lower, nil); err != nil {
		t.Fatalf("plugin run failed: %v", err)
	}
}

func TestDiscoverDetailed_VerificationStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows for shell script exec")
	}

	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins dir: %v", err)
	}

	script := filepath.Join(pluginsDir, "opencenter-verified")
	content := "#!/usr/bin/env sh\necho verified\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	sum := sha256.Sum256([]byte(content))
	checksumLine := hex.EncodeToString(sum[:]) + "  opencenter-verified\n"
	if err := os.WriteFile(filepath.Join(pluginsDir, "checksums.txt"), []byte(checksumLine), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	t.Setenv("OPENCENTER_CONFIG_DIR", root)
	t.Setenv("OPENCENTER_PLUGINS_DIR", pluginsDir)

	discovered := DiscoverDetailed()
	info, ok := discovered["opencenter-verified"]
	if !ok {
		t.Fatalf("expected plugin to be discovered")
	}

	if info.Status != VerificationStatusVerified {
		t.Fatalf("expected verified status, got %s", info.Status)
	}
}

func TestLoadExternalPlugins_RejectsChecksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows for shell script exec")
	}

	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins dir: %v", err)
	}

	script := filepath.Join(pluginsDir, "opencenter-bad")
	content := "#!/usr/bin/env sh\necho bad-plugin\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	if err := os.WriteFile(filepath.Join(pluginsDir, "checksums.txt"), []byte("deadbeef  opencenter-bad\n"), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	t.Setenv("OPENCENTER_CONFIG_DIR", root)
	t.Setenv("OPENCENTER_PLUGINS_DIR", pluginsDir)

	rootCmd := &cobra.Command{Use: "opencenter-test"}
	LoadExternalPlugins(rootCmd)

	var bad *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "bad" {
			bad = c
			break
		}
	}
	if bad == nil {
		t.Fatalf("expected plugin command 'bad' to be registered")
	}

	if err := bad.RunE(bad, nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}
