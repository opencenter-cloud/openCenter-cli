package plugins

import (
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
    script := filepath.Join(dir, "openCenter-hello")
    content := "#!/usr/bin/env sh\necho plugin-ok\n"
    if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
        t.Fatalf("write plugin: %v", err)
    }

    // Point discovery to the temp dir only
    t.Setenv("OPENCENTER_PLUGINS_DIR", dir)

    root := &cobra.Command{Use: "openCenter-test"}
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

