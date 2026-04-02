package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/spf13/cobra"
)

// BinaryPrefix is the expected prefix for external plugin executables.
const BinaryPrefix = "opencenter-"

var binaryPrefixLower = strings.ToLower(BinaryPrefix)

const (
	VerificationStatusVerified         = "verified"
	VerificationStatusUnverified       = "unverified"
	VerificationStatusChecksumMismatch = "checksum-mismatch"
	VerificationStatusError            = "verification-error"
)

// PluginInfo contains the discovered path and verification metadata for a plugin.
type PluginInfo struct {
	Path    string
	Status  string
	Message string
}

// LoadExternalPlugins discovers external plugin binaries and attaches them as
// cobra Commands to the provided root command. A plugin is any executable whose
// name starts with "opencenter-" located either in PATH or in the plugins dir.
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
	discovered := DiscoverDetailed()

	for name, info := range discovered {
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
				switch info.Status {
				case VerificationStatusChecksumMismatch:
					return fmt.Errorf("refusing to run plugin %s: %s", use, info.Message)
				case VerificationStatusError:
					return fmt.Errorf("cannot verify plugin %s: %s", use, info.Message)
				case VerificationStatusUnverified:
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: plugin %s is unverified; add its checksum to %s\n", use, checksumsFilePath())
				}

				return runExternal(info.Path, args)
			},
		}

		root.AddCommand(cmd)
	}
}

// Discover returns a map of discovered plugin binary basenames to their full paths,
// using the same discovery rules as LoadExternalPlugins.
func Discover() map[string]string {
	seen := map[string]string{}
	for name, info := range DiscoverDetailed() {
		seen[name] = info.Path
	}
	return seen
}

// DiscoverDetailed returns the discovered plugin paths and verification state.
func DiscoverDetailed() map[string]PluginInfo {
	pluginBins := discoverPluginBinaries()
	checksums, checksumErr := loadPluginChecksums()
	seen := map[string]PluginInfo{}

	for _, bin := range pluginBins {
		name := filepath.Base(bin)
		info := PluginInfo{Path: bin}

		if checksumErr != nil {
			info.Status = VerificationStatusError
			info.Message = checksumErr.Error()
		} else {
			info.Status, info.Message = verifyPlugin(name, bin, checksums)
		}

		seen[name] = info
	}

	return seen
}

func runExternal(path string, args []string) error {
	// Prepend the subcommand name to args is NOT needed: we map it already.
	c, err := security.GetDefaultCommandRunner().PrepareCommand(path, args...)
	if err != nil {
		return fmt.Errorf("preparing plugin command: %w", err)
	}
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
	if cfgDir, err := config.ResolveConfigDir(); err == nil && cfgDir != "" {
		results = append(results, findPrefixedExecutables(filepath.Join(cfgDir, "plugins"))...)
	}

	// 3) PATH entries
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		results = append(results, findPrefixedExecutables(dir)...)
	}

	return results
}

func loadPluginChecksums() (map[string]string, error) {
	path := checksumsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read checksums file: %w", err)
	}

	checksums := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid checksum entry: %q", line)
		}

		filename := strings.TrimPrefix(fields[len(fields)-1], "*")
		checksums[filepath.Base(filename)] = fields[0]
	}

	return checksums, nil
}

func verifyPlugin(name, path string, checksums map[string]string) (string, string) {
	expected, ok := checksums[name]
	if !ok {
		return VerificationStatusUnverified, "no checksum entry"
	}

	actual, err := sha256ForFile(path)
	if err != nil {
		return VerificationStatusError, err.Error()
	}

	if !strings.EqualFold(actual, expected) {
		return VerificationStatusChecksumMismatch, fmt.Sprintf("checksum mismatch for %s", name)
	}

	return VerificationStatusVerified, "checksum verified"
}

func sha256ForFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open plugin %s: %w", path, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash plugin %s: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func checksumsFilePath() string {
	return filepath.Join(config.GetPluginsDir(), "checksums.txt")
}

// SortedPluginNames returns discovered plugin names in a stable order.
func SortedPluginNames(discovered map[string]PluginInfo) []string {
	names := make([]string, 0, len(discovered))
	for name := range discovered {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
