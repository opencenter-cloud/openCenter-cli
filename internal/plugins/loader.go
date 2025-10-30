package plugins

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	icfg "github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// BinaryPrefix is the expected prefix for external plugin executables.
const BinaryPrefix = "openCenter-"

var binaryPrefixLower = strings.ToLower(BinaryPrefix)

// LoadExternalPlugins discovers external plugin binaries and attaches them as
// cobra Commands to the provided root command. A plugin is any executable whose
// name starts with "openCenter-" located either in PATH or in the plugins dir.
//
// Discovery locations (in order):
//  1. OPENCENTER_PLUGINS_DIR (if set)
//  2. <configDir>/plugins where configDir is resolved from env or default
//  3. PATH entries
func LoadExternalPlugins(root *cobra.Command) {
	// Build a set of built-in command names to avoid conflicts
	builtIns := map[string]struct{}{}
	for _, c := range root.Commands() {
		builtIns[c.Name()] = struct{}{}
	}

	// Discover executables
	discovered := Discover()

	for name, full := range discovered {
		if !hasBinaryPrefix(name) {
			continue
		}
		use := trimBinaryPrefix(name)
		if use == "" {
			continue
		}
		if _, exists := builtIns[use]; exists {
			// Do not shadow built-in commands
			continue
		}

		cmd := &cobra.Command{
			Use:                use,
			Short:              fmt.Sprintf("external plugin: %s", use),
			DisableFlagParsing: true, // forward flags transparently
			Args:               cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runExternal(full, args)
			},
		}

		root.AddCommand(cmd)
	}
}

// Discover returns a map of discovered plugin binary basenames to their full paths,
// using the same discovery rules as LoadExternalPlugins.
func Discover() map[string]string {
	pluginBins := discoverPluginBinaries()
	seen := map[string]string{}
	for _, bin := range pluginBins {
		name := filepath.Base(bin)
		seen[name] = bin
	}
	return seen
}

func runExternal(path string, args []string) error {
	// Prepend the subcommand name to args is NOT needed: we map it already.
	c := exec.Command(path, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// Preserve the plugin's exit code and output
			return fmt.Errorf("plugin exited with code %d", ee.ExitCode())
		}
		return err
	}
	return nil
}

func discoverPluginBinaries() []string {
	var results []string

	// 1) explicit plugins dir
	if p := os.Getenv("OPENCENTER_PLUGINS_DIR"); p != "" {
		results = append(results, findPrefixedExecutables(p)...)
	}

	// 2) configDir/plugins
	if cfgDir, err := icfg.ResolveConfigDir(); err == nil && cfgDir != "" {
		results = append(results, findPrefixedExecutables(filepath.Join(cfgDir, "plugins"))...)
	}

	// 3) PATH entries
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		results = append(results, findPrefixedExecutables(dir)...)
	}

	return results
}

func findPrefixedExecutables(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !hasBinaryPrefix(name) {
			continue
		}
		full := filepath.Join(dir, name)
		if isExecutable(full) {
			out = append(out, full)
		}
	}
	return out
}

func hasBinaryPrefix(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), binaryPrefixLower)
}

func trimBinaryPrefix(name string) string {
	if !hasBinaryPrefix(name) {
		return name
	}
	return name[len(binaryPrefixLower):]
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	mode := info.Mode()
	return mode&0111 != 0 // any execute bit set
}
