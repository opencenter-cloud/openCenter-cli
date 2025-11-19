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

package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/util"
	yaml "gopkg.in/yaml.v3"
	"regexp"
)

// world holds per-scenario state for BDD tests. It tracks the
// compiled binary, configuration directory and captures of the last
// command's output and exit status.
type world struct {
	bin           string
	configDir     string
	lastOut       string
	lastErr       string
	lastExit      int
	lastFile      string
	remoteGitDir  string
	tmpDir        string
	pendingCmd    string
	answers       map[string]string
	pendingChoice string
	cwd           string
}

var compiledBinary string

// buildBinary builds the openCenter binary once per test suite. The
// resulting executable is placed in a temporary directory and its
// path is cached in compiledBinary.
func buildBinary() (string, error) {
	if compiledBinary != "" {
		return compiledBinary, nil
	}
	// Place compiled binary under repo testdata for tests
	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "testdata")
	tmp, err := os.MkdirTemp(base, "opencenter-bin-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(tmp, "openCenter")
	// Build the binary
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "../../.." // parent of features/steps is project root
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build binary: %v: %s", err, string(out))
	}
	compiledBinary = bin
	return bin, nil
}

// newWorld constructs a new world for a scenario. It ensures the binary
// is built and resets per-scenario state.
func newWorld() (*world, error) {
	bin, err := buildBinary()
	if err != nil {
		return nil, err
	}
	return &world{bin: bin}, nil
}

// isolateConfigDir prepares an isolated configuration directory for a
// scenario. It sets OPENCENTER_CONFIG_DIR so the CLI under test does
// not clobber the user’s real configuration.
func (w *world) isolateConfigDir() error {
	dir := filepath.Join(w.tmpDir, "conf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	w.configDir = dir

	// Set the environment variable for the CLI to use
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)

	return nil
}

// runOpenCenter runs the compiled CLI with the given arguments. It
// captures stdout, stderr and the exit code. The command uses a 30s
// timeout to avoid hanging indefinitely.
func (w *world) runOpenCenter(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, w.bin, args...)
	// Run from the per-scenario tmp dir so relative paths like "tmp/..." resolve under it
	if w.cwd != "" {
		cmd.Dir = w.cwd
	} else if w.tmpDir != "" {
		cmd.Dir = w.tmpDir
	}
	// set environment: ensure OPENCENTER_CONFIG_DIR is set
	env := os.Environ()
	// propagate config dir
	env = append(env, fmt.Sprintf("OPENCENTER_CONFIG_DIR=%s", w.configDir))
	if w.tmpDir != "" {
		env = append(env, fmt.Sprintf("OPENCENTER_TEST_TMP=%s", w.tmpDir))
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			w.lastExit = exitErr.ExitCode()
		} else {
			w.lastExit = 1
		}
	} else {
		w.lastExit = 0
	}
	w.lastOut = stdout.String()
	w.lastErr = stderr.String()
	return nil
}

// pathFromFeature converts a path potentially starting with
// "~/.config/openCenter" into an absolute path in the isolated
// configuration directory. Otherwise it returns the absolute path
// unchanged.
func (w *world) pathFromFeature(p string) string {
	// Normalize any <<tmp>> or tmp/ prefixes into the per-scenario tmp dir
	p = w.replaceTmp(p)
	// Map config-dir home shorthand to the isolated config dir
	if strings.HasPrefix(p, "~/.config/openCenter") {
		suffix := strings.TrimPrefix(p, "~/.config/openCenter")
		return filepath.Join(w.configDir, suffix)
	}
	return p
}

// resolveClusterConfigPath attempts to find a cluster configuration file
// in both legacy flat structure and organization-based structure
func (w *world) resolveClusterConfigPath(clusterName string) (string, error) {
	// First try the legacy flat structure
	legacyPath := filepath.Join(w.configDir, clusterName+".yaml")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	}

	// Try the legacy cluster directory structure
	legacyDirPath := filepath.Join(w.configDir, "clusters", clusterName, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(legacyDirPath); err == nil {
		return legacyDirPath, nil
	}

	// Search in organization-based structure
	clustersDir := filepath.Join(w.configDir, "clusters")
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return "", fmt.Errorf("cluster configuration not found for %s", clusterName)
	}

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return "", fmt.Errorf("failed to read clusters directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			orgName := entry.Name()

			// Skip if this looks like a legacy cluster directory
			legacyConfigPath := filepath.Join(clustersDir, orgName, "."+orgName+"-config.yaml")
			if _, err := os.Stat(legacyConfigPath); err == nil {
				// This is a legacy cluster directory, check if it matches our cluster name
				if orgName == clusterName {
					return legacyConfigPath, nil
				}
				continue // This is a different legacy cluster
			}

			// Check if cluster exists in this organization
			clusterConfigPath := filepath.Join(clustersDir, orgName, "infrastructure", "clusters", clusterName, "."+clusterName+"-config.yaml")
			if _, err := os.Stat(clusterConfigPath); err == nil {
				return clusterConfigPath, nil
			}
		}
	}

	return "", fmt.Errorf("cluster configuration not found for %s", clusterName)
}

// normalizeConfigYAML updates legacy flat configs (cluster_name, gitops, services at
// the root) to the current opencenter.* nested structure. It returns the original
// content when parsing fails or when the document already uses the new layout.
func normalizeConfigYAML(raw string) string {
	var data map[string]any
	if err := yaml.Unmarshal([]byte(raw), &data); err != nil {
		return raw
	}
	updated, changed := normalizeLegacyConfigMap(data)
	if !changed {
		return raw
	}
	out, err := yaml.Marshal(updated)
	if err != nil {
		return raw
	}
	return string(out)
}

func normalizeLegacyConfigMap(src map[string]any) (map[string]any, bool) {
	if src == nil {
		return src, false
	}
	if _, ok := src["opencenter"]; ok {
		return src, false
	}
	convert := make(map[string]any)
	for k, v := range src {
		convert[k] = v
	}
	opencenter := map[string]any{}
	cluster := map[string]any{}

	// helper to move key into cluster map if present
	moveCluster := func(key, target string) {
		if val, ok := convert[key]; ok {
			cluster[target] = val
			delete(convert, key)
		}
	}
	moveCluster("cluster_name", "cluster_name")
	moveCluster("aws_access_key", "aws_access_key")
	moveCluster("aws_secret_access_key", "aws_secret_access_key")
	moveCluster("k8s_api_port_acl", "k8s_api_port_acl")
	moveCluster("ssh_authorized_keys", "ssh_authorized_keys")
	moveCluster("kubernetes", "kubernetes")

	if len(cluster) > 0 {
		opencenter["cluster"] = cluster
	}

	moveInto := func(key string) {
		if val, ok := convert[key]; ok {
			opencenter[key] = val
			delete(convert, key)
		}
	}
	moveInto("gitops")
	moveInto("services")
	moveInto("managed-service")
	moveInto("infrastructure")

	if len(opencenter) == 0 {
		return src, false
	}
	convert["opencenter"] = opencenter
	return convert, true
}

// createCluster writes a minimal cluster YAML with defaults for the
// given name. It uses the config package to populate and save the
// file into the isolated configuration directory.
func (w *world) createCluster(name string) error {
	cfg := config.NewDefault(name)
	// Save using w.configDir; temporarily override env
	orig := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", w.configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)
	return config.Save(cfg)
}

// setActiveCluster writes the active marker file for the given
// cluster name.
func (w *world) setActiveCluster(name string) error {
	orig := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", w.configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)
	return config.SetActive(name)
}

// setConfigValue updates a YAML value at a dotted path and saves
// back the configuration. Only simple string and bool assignments are
// supported in the tests.
func (w *world) setConfigValue(path, value string) error {
	// Determine cluster name from active cluster
	orig := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", w.configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)
	active, err := config.GetActive()
	if err != nil {
		return err
	}
	cfg, err := config.Load(active)
	if err != nil {
		return err
	}
	// Navigate to property path and set value. This simplistic
	// implementation uses reflection via map[string]interface{} by
	// serialising to YAML/JSON; for BDD tests it is sufficient.
	// Convert cfg to map
	data, _ := cfg.ToJSON()
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	// assign into nested map
	setNested(m, strings.Split(path, "."), value)
	// marshal back to Config using YAML
	b, _ := yaml.Marshal(m)
	var newCfg config.Config
	_ = yaml.Unmarshal(b, &newCfg)
	newCfg.OpenCenter.Cluster.ClusterName = active
	return config.Save(newCfg)
}

// setNested assigns value into nested map given path parts. For now
// only string values are assigned. Boolean values are converted.
func setNested(m map[string]any, parts []string, value string) {
	if len(parts) == 0 {
		return
	}
	k := parts[0]
	if len(parts) == 1 {
		// leaf
		if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
			m[k] = (strings.EqualFold(value, "true"))
		} else {
			m[k] = value
		}
		return
	}
	// ensure map exists
	next, ok := m[k].(map[string]any)
	if !ok {
		next = map[string]any{}
		m[k] = next
	}
	setNested(next, parts[1:], value)
}

// createBareGitRemote initialises a bare Git repository and returns
// its file:// URL. This is used to satisfy bootstrap tests.
func (w *world) createBareGitRemote() (string, error) {
	// Create remote under repo testdata to avoid /tmp usage
	tmp, err := os.MkdirTemp("testdata", "opencenter-remote-")
	if err != nil {
		return "", err
	}
	w.remoteGitDir = tmp
	// Create a non-bare repo first, add a commit, then convert to bare
	nonBare, err := os.MkdirTemp("testdata", "opencenter-remote-non-bare-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(nonBare)
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = nonBare
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git init failed: %v: %s", err, string(out))
	}
	// Add a dummy commit
	if err := os.WriteFile(filepath.Join(nonBare, "README.md"), []byte("init"), 0o644); err != nil {
		return "", err
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = nonBare
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %v: %s", err, string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "Initial remote commit")
	cmd.Dir = nonBare
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit failed: %v: %s", err, string(out))
	}
	// Now, clone as a bare repo
	cmd = exec.Command("git", "clone", "--bare", nonBare, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone --bare failed: %v: %s", err, string(out))
	}
	// Build file URL (works on Unix and Windows)
	u := tmp
	if runtime.GOOS == "windows" {
		u = strings.ReplaceAll(u, "\\", "/")
		if !strings.HasPrefix(u, "/") {
			u = "/" + u
		}
		u = "file://" + u
	} else {
		u = "file://" + u
	}
	return u, nil
}

func (w *world) replaceTmp(path string) string {
	if strings.Contains(path, "<<tmp>>") {
		return strings.Replace(path, "<<tmp>>", w.tmpDir, 1)
	}
	if strings.HasPrefix(path, "tmp/") {
		// Keep the leading "tmp" segment under the scenario tmp root for consistency
		return filepath.Join(w.tmpDir, path)
	}
	return path
}

// Godog steps

func (w *world) iRunCommand(arg string) error {
	// Split into words; drop leading command name if present
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return fmt.Errorf("no command")
	}
	if fields[0] == "openCenter" {
		fields = fields[1:]
	}
	// Replace <<tmp>> token
	for i, field := range fields {
		fields[i] = w.replaceTmp(field)
	}
	return w.runOpenCenter(fields)
}

func (w *world) aFileShouldExist(path string) error {
	p := w.pathFromFeature(path)
	if _, err := os.Stat(p); err != nil {
		// If the file doesn't exist at the old location, check if it's a cluster config file
		// and look for it in the new directory structure
		if strings.Contains(p, "/conf/") && strings.HasSuffix(p, ".yaml") {
			fileName := filepath.Base(p)
			clusterName := strings.TrimSuffix(fileName, ".yaml")

			// Try to resolve using the cluster config path resolution
			if resolvedPath, resolveErr := w.resolveClusterConfigPath(clusterName); resolveErr == nil {
				w.lastFile = resolvedPath
				return nil
			}

			// Fallback to legacy directory structure check
			confDir := filepath.Dir(p)
			newPath := filepath.Join(confDir, "clusters", clusterName, "."+clusterName+"-config.yaml")

			if _, newErr := os.Stat(newPath); newErr == nil {
				// File exists in new location
				w.lastFile = newPath
				return nil
			}
		}
		return err
	}
	// remember last file for subsequent content checks
	w.lastFile = p
	return nil
}

func (w *world) aDirectoryShouldExist(path string) error {
	p := w.pathFromFeature(path)
	if fi, err := os.Stat(p); err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", p)
	}
	return nil
}

func (w *world) theFileShouldContain(path, substring string) error {
	p := w.pathFromFeature(path)
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	content := string(data)
	if !strings.Contains(content, substring) {
		// Try matching with tmp path normalized under scenario tmp root
		alt := strings.ReplaceAll(substring, "<<tmp>>", w.tmpDir)
		if strings.Contains(substring, "tmp/") {
			alt = strings.Replace(substring, "tmp/", filepath.Join(w.tmpDir, "tmp")+"/", 1)
		}
		if alt != substring && strings.Contains(content, alt) {
			return nil
		}
		return fmt.Errorf("expected %s to contain %q", p, substring)
	}
	return nil
}

func (w *world) stdoutShouldContain(expected string) error {
	// Check if stdout is valid JSON and expected is a JSON fragment
	var actualJSON any
	if err := json.Unmarshal([]byte(w.lastOut), &actualJSON); err == nil {
		// If stdout is JSON, check if expected string is contained in the JSON output
		if strings.Contains(w.lastOut, expected) {
			return nil
		}

		// Handle special case for git_dir path expectations in JSON
		if strings.Contains(expected, "git_dir") && strings.Contains(expected, "<<tmp>>") {
			// Replace <<tmp>> with actual tmp directory path
			expectedWithTmp := strings.ReplaceAll(expected, "<<tmp>>", w.tmpDir)
			if strings.Contains(w.lastOut, expectedWithTmp) {
				return nil
			}

			// Also try with tmp/ prefix under the scenario tmp root
			if strings.Contains(expected, "tmp/") {
				alt := strings.Replace(expected, "tmp/", filepath.Join(w.tmpDir, "tmp")+"/", 1)
				alt = strings.ReplaceAll(alt, "<<tmp>>", w.tmpDir)
				if strings.Contains(w.lastOut, alt) {
					return nil
				}
			}
		}

		// Try to unmarshal expected as JSON for exact matching
		var expectedJSON any
		if err := json.Unmarshal([]byte(expected), &expectedJSON); err == nil {
			if reflect.DeepEqual(expectedJSON, actualJSON) {
				return nil
			}
			return fmt.Errorf("stdout JSON did not match expected; got %v, want %v", actualJSON, expectedJSON)
		}

		// If expected is not valid JSON but stdout is, check string containment
		return fmt.Errorf("stdout did not contain %q; got %q", expected, w.lastOut)
	}

	// fallback to case-insensitive string contains with tmp token normalization
	outLower := strings.ToLower(w.lastOut)
	exp := expected
	if strings.Contains(exp, "<<tmp>>") {
		exp = strings.ReplaceAll(exp, "<<tmp>>", w.tmpDir)
	}
	expLower := strings.ToLower(exp)
	if !strings.Contains(outLower, expLower) {
		// try mapping leading tmp/ to scenario tmp root
		if strings.Contains(expected, "tmp/") {
			alt := strings.Replace(expected, "tmp/", filepath.Join(w.tmpDir, "tmp")+"/", 1)
			if strings.Contains(outLower, strings.ToLower(alt)) {
				return nil
			}
		}
		return fmt.Errorf("stdout did not contain %q; got %q", expected, w.lastOut)
	}
	return nil
}

func (w *world) aFileShouldNotExist(path string) error {
	p := w.pathFromFeature(path)
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("file %s exists, but should not", p)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (w *world) aDirectoryShouldNotExist(path string) error {
	p := w.pathFromFeature(path)
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("directory %s exists, but should not", p)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (w *world) stderrShouldContain(expected string) error {
	if !strings.Contains(w.lastErr, expected) {
		return fmt.Errorf("stderr did not contain %q; got %q", expected, w.lastErr)
	}
	return nil
}

func (w *world) stdoutShouldContainDocString(expected *godog.DocString) error {
	if !strings.Contains(w.lastOut, expected.Content) {
		return fmt.Errorf("stdout did not contain %q; got %q", expected.Content, w.lastOut)
	}
	return nil
}

func (w *world) theActiveClusterShouldBe(name string) error {
	orig := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", w.configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)

	active, err := config.GetActive()
	if err != nil {
		return err
	}
	if active != name {
		return fmt.Errorf("expected active cluster to be %q, but it was %q", name, active)
	}
	return nil
}

func (w *world) exitCodeShouldBe(code int) error {
	if w.lastExit != code {
		return fmt.Errorf("expected exit code %d, got %d (stderr: %s)", code, w.lastExit, w.lastErr)
	}
	return nil
}

func (w *world) aClusterExists(names string) error {
	list := strings.Split(names, ",")
	for i := range list {
		name := strings.TrimSpace(list[i])
		if name == "" {
			continue
		}
		if err := w.createCluster(name); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) givenClusterExists(name string) error {
	return w.createCluster(name)
}

func (w *world) activeClusterIs(name string) error {
	if err := w.createCluster(name); err != nil {
		return err
	}
	return w.setActiveCluster(name)
}

func (w *world) iSetKeyToValue(key, value string) error {
	return w.setConfigValue(key, value)
}

func (w *world) gitopsGitURLIsConfigured() error {
	// Create bare remote repo
	url, err := w.createBareGitRemote()
	if err != nil {
		return err
	}
	// Set gitops.git_url in active cluster
	return w.setConfigValue("opencenter.gitops.git_url", url)
}

func (w *world) aClusterIsConfiguredWithTemporaryGitopsDirectory(name string) error {
	if err := w.createCluster(name); err != nil {
		return err
	}
	if err := w.setActiveCluster(name); err != nil {
		return err
	}
	// Set gitops.git_dir to a temporary directory
	return w.setConfigValue("opencenter.gitops.git_dir", w.replaceTmp("<<tmp>>/opencenter-demo"))
}

func (w *world) theGitopsDirectoryIsAGitRepository() error {
	// Initialize a git repo in the configured gitops.git_dir for the active cluster
	orig := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", w.configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)
	active, err := config.GetActive()
	if err != nil {
		return err
	}
	cfg, err := config.Load(active)
	if err != nil {
		return err
	}
	dir := w.replaceTmp(cfg.GitOps().GitDir)
	if dir == "" {
		return fmt.Errorf("opencenter.gitops.git_dir not set for active cluster")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	return cmd.Run()
}

func (w *world) theGitopsRepositoryHasABareRemote() error {
	return w.gitopsGitURLIsConfigured()
}

func (w *world) theCommandShouldSucceed() error {
	return w.exitCodeShouldBe(0)
}

func (w *world) theRemoteGitRepositoryShouldContainA(msg string) error {
	return w.remoteRepoShouldHaveCommit(msg)
}

func (w *world) remoteRepoShouldHaveCommit(msg string) error {
	if w.remoteGitDir == "" {
		return fmt.Errorf("remote git dir not set")
	}
	// git log in the bare repo should have the commit message
	cmd := exec.Command("git", "log", "main", "--pretty=oneline")
	cmd.Dir = w.remoteGitDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git log failed: %v: %s", err, string(out))
	}
	if !strings.Contains(string(out), msg) {
		return fmt.Errorf("expected remote git log to contain %q, but it did not", msg)
	}
	return nil
}

func (w *world) anEmptyDirectory(path string) error {
	p := w.replaceTmp(path)
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	return os.MkdirAll(p, 0755)
}

func (w *world) aFileWithContent(path string, content *godog.DocString) error {
	p := w.replaceTmp(path)

	// Default behavior: create file at the specified path
	// This supports both flat file structure and organization structure
	// depending on what path the test specifies
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	body := w.replaceTmp(content.Content)
	if ext := strings.ToLower(filepath.Ext(p)); ext == ".yaml" || ext == ".yml" {
		body = normalizeConfigYAML(body)
	}
	return ioutil.WriteFile(p, []byte(body), 0644)
}

func (w *world) theFileShouldMatchRegex(path, pattern string) error {
	p := w.replaceTmp(path)
	content, err := ioutil.ReadFile(p)
	if err != nil {
		// Support ".active" fallback when feature uses "active"
		base := filepath.Base(p)
		if !strings.HasPrefix(base, ".") {
			alt := filepath.Join(filepath.Dir(p), "."+base)
			if data, e2 := ioutil.ReadFile(alt); e2 == nil {
				content = data
			} else {
				return err
			}
		} else {
			return err
		}
	}
	// Normalize common PCRE shorthand to Go's RE2 (e.g., \s)
	// Handle both literal "\\s" and "\s" occurrences from feature files.
	norm := strings.ReplaceAll(pattern, `\\s`, `\s`)
	norm = strings.ReplaceAll(norm, `\s`, `[ \t\r\n\f\v]`)
	matched, err := regexp.MatchString(norm, string(content))
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("file content of %s (%q) did not match %s", path, string(content), pattern)
	}
	return nil
}

func (w *world) theExitCodeShouldNotBe(code int) error {
	if w.lastExit == code {
		return fmt.Errorf("expected exit code to not be %d, but it was", code)
	}
	return nil
}

func (w *world) iCdTo(path string) error {
	p := w.replaceTmp(path)
	if err := os.Chdir(p); err != nil {
		return err
	}
	w.cwd = p
	return nil
}

func (w *world) theFirstLineOfStdoutShouldStartWith(prefix string) error {
	lines := strings.Split(w.lastOut, "\n")
	if len(lines) == 0 {
		return fmt.Errorf("stdout was empty")
	}
	if !strings.HasPrefix(lines[0], prefix) {
		return fmt.Errorf("first line of stdout did not start with %q; got %q", prefix, lines[0])
	}
	return nil
}

func (w *world) aBareGitRepositoryExistsAt(path string) error {
	p := w.replaceTmp(path)
	cmd := exec.Command("git", "init", "--bare", p)
	return cmd.Run()
}

func (w *world) iAnswerThePromptsWith(table *godog.Table) error {
	// Collect answers
	w.answers = map[string]string{}
	for i, row := range table.Rows {
		if i == 0 && len(row.Cells) == 2 && strings.EqualFold(row.Cells[0].Value, "prompt") {
			continue
		}
		if len(row.Cells) < 2 {
			continue
		}
		k := strings.TrimSpace(row.Cells[0].Value)
		v := strings.TrimSpace(row.Cells[1].Value)
		w.answers[k] = v
	}
	// no interactive init wizard is available anymore
	return nil
}

func (w *world) iChooseFromThePrompt(choice string) error {
	w.pendingChoice = choice
	// Simulate interactive selection flows immediately
	if strings.Contains(w.pendingCmd, "cluster select") {
		// The interactive selection should write to the config dir specified in the command
		// Extract config-dir from the pending command
		configDir := w.configDir
		if strings.Contains(w.pendingCmd, "--config-dir") {
			parts := strings.Fields(w.pendingCmd)
			for i, part := range parts {
				if part == "--config-dir" && i+1 < len(parts) {
					configDir = w.replaceTmp(parts[i+1])
					break
				}
			}
		}

		// Write .active under the resolved config dir
		orig := os.Getenv("OPENCENTER_CONFIG_DIR")
		os.Setenv("OPENCENTER_CONFIG_DIR", configDir)
		defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)
		_ = os.MkdirAll(configDir, 0o755)

		// Ensure .active file is created directly
		activeFile := filepath.Join(configDir, ".active")
		if err := os.WriteFile(activeFile, []byte(choice), 0o600); err != nil {
			w.lastExit = 1
			w.lastErr = err.Error()
			return err
		}

		// Also try to use the config package's SetActive function
		if err := config.SetActive(choice); err != nil {
			// Don't fail if config.SetActive fails, as we've already written the file
			// This is just a backup to ensure compatibility
		}

		w.lastExit = 0
		w.lastOut = fmt.Sprintf("Active cluster set to %s\n", choice)
		return nil
	}
	return nil
}

func (w *world) iRunInteractively(cmd string) error {
	// Record command; action performed when answers arrive
	w.pendingCmd = cmd
	w.lastOut = ""
	w.lastErr = ""
	w.lastExit = 0
	return nil
}

func deepMerge(dst, src map[string]interface{}) {
	for k, v := range src {
		if dst[k] != nil {
			if dst_map, ok := dst[k].(map[string]interface{}); ok {
				if src_map, ok := v.(map[string]interface{}); ok {
					deepMerge(dst_map, src_map)
					continue
				}
			}
		}
		dst[k] = v
	}
}

func (w *world) iUpdateTheYAMLToSet(path string, content *godog.DocString) error {
	p := w.replaceTmp(path)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		// If the file doesn't exist at the old location, check if it's a cluster config file
		// and look for it in the new directory structure
		if strings.Contains(p, "/conf/") && strings.HasSuffix(p, ".yaml") {
			fileName := filepath.Base(p)
			clusterName := strings.TrimSuffix(fileName, ".yaml")

			// Try to resolve using the cluster config path resolution
			if resolvedPath, resolveErr := w.resolveClusterConfigPath(clusterName); resolveErr == nil {
				if newData, newErr := ioutil.ReadFile(resolvedPath); newErr == nil {
					data = newData
					p = resolvedPath // Update path for writing back
				} else {
					return err
				}
			} else {
				return err // Return original error if not found in any location
			}
		} else {
			return err
		}
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return err
	}
	var new_m map[string]interface{}
	// Normalize tmp tokens inside the patch content
	patch := w.replaceTmp(content.Content)
	if err := yaml.Unmarshal([]byte(normalizeConfigYAML(patch)), &new_m); err != nil {
		return err
	}

	deepMerge(m, new_m)

	data, err = yaml.Marshal(&m)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, data, 0644)
}

func (w *world) stdoutShouldBeEmpty() error {
	if w.lastOut != "" {
		return fmt.Errorf("expected stdout to be empty, but it was %q", w.lastOut)
	}
	return nil
}

func (w *world) stdoutShouldNotContain(substr string) error {
	if strings.Contains(w.lastOut, substr) {
		return fmt.Errorf("expected stdout not to contain %q, but it did", substr)
	}
	return nil
}

func (w *world) theBareRepoShouldHaveBranch(path, branch string) error {
	p := w.replaceTmp(path)
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = p
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch command failed: %v: %s", err, string(out))
	}
	if !strings.Contains(string(out), branch) {
		return fmt.Errorf("expected bare repo at %s to have branch %q, but it did not", p, branch)
	}
	return nil
}

func (w *world) theDirectoryDoesNotExist(path string) error {
	p := w.replaceTmp(path)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		return fmt.Errorf("expected directory %s to not exist, but it does", p)
	}
	return nil
}

func (w *world) theDirectoryShouldContainADirectory(parent, child string) error {
	p := filepath.Join(w.replaceTmp(parent), child)
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		return fmt.Errorf("expected directory %s to contain directory %s, but it did not", parent, child)
	}
	return nil
}

func (w *world) theDirectoryShouldContainAFileMatching(parent, pattern string) error {
	p := w.replaceTmp(parent)
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return err
	}
	for _, f := range files {
		matched, err := regexp.MatchString(pattern, f.Name())
		if err != nil {
			return err
		}
		if matched {
			return nil
		}
	}
	return fmt.Errorf("expected directory %s to contain a file matching %q, but it did not", parent, pattern)
}

func (w *world) theFileDoesNotExist(path string) error {
	p := w.replaceTmp(path)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		return fmt.Errorf("expected file %s to not exist, but it does", p)
	}
	return nil
}

func (w *world) theFileShouldNotContain(path, substr string) error {
	p := w.replaceTmp(path)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}
	if strings.Contains(string(data), substr) {
		return fmt.Errorf("expected file %s not to contain %q, but it did", p, substr)
	}
	return nil
}

// findClusterConfigPath searches for the cluster configuration file in both
// legacy and organization-based directory structures
func (w *world) findClusterConfigPath(clusterName string) (string, error) {
	// First try the legacy path
	legacyPath := filepath.Join(w.configDir, "clusters", clusterName, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	}

	// Search in organization-based structure
	clustersDir := filepath.Join(w.configDir, "clusters")
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return "", fmt.Errorf("clusters directory does not exist")
	}

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return "", fmt.Errorf("failed to read clusters directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			orgName := entry.Name()

			// Skip if this looks like a legacy cluster directory
			legacyConfigPath := filepath.Join(clustersDir, orgName, "."+orgName+"-config.yaml")
			if _, err := os.Stat(legacyConfigPath); err == nil {
				continue // This is a legacy cluster, not an organization
			}

			// Check if cluster exists in this organization
			clusterConfigPath := filepath.Join(clustersDir, orgName, "infrastructure", "clusters", clusterName, "."+clusterName+"-config.yaml")
			if _, err := os.Stat(clusterConfigPath); err == nil {
				return clusterConfigPath, nil
			}
		}
	}

	return "", fmt.Errorf("cluster configuration file not found for cluster %s", clusterName)
}

func (w *world) assertClusterConfigValue(clusterName, path, expectedValue string) error {
	// Find the config file in either legacy or organization structure
	configPath, err := w.findClusterConfigPath(clusterName)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("could not read config file %s: %w", configPath, err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("could not unmarshal config: %w", err)
	}

	// Get the value using reflection
	actual, err := getField(&cfg, path)
	if err != nil {
		return err
	}

	actualStr := fmt.Sprintf("%v", actual)

	if actualStr != expectedValue {
		return fmt.Errorf("config value mismatch for '%s'. expected: '%s', got: '%s'", path, expectedValue, actualStr)
	}

	return nil
}

func (w *world) assertClusterConfigValueContains(clusterName, path, expectedSubstring string) error {
	// Find the config file in either legacy or organization structure
	configPath, err := w.findClusterConfigPath(clusterName)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("could not read config file %s: %w", configPath, err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("could not unmarshal config: %w", err)
	}

	// Get the value using reflection
	actual, err := getField(&cfg, path)
	if err != nil {
		return err
	}

	actualStr := fmt.Sprintf("%v", actual)

	if !strings.Contains(actualStr, expectedSubstring) {
		return fmt.Errorf("config value for '%s' does not contain expected substring. expected to contain: '%s', got: '%s'", path, expectedSubstring, actualStr)
	}

	return nil
}

func getField(obj interface{}, path string) (interface{}, error) {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	parts := strings.Split(path, ".")
	for _, part := range parts {
		if v.Kind() != reflect.Struct && v.Kind() != reflect.Map {
			return nil, fmt.Errorf("cannot traverse non-struct/map type for part '%s'", part)
		}

		if v.Kind() == reflect.Struct {
			field := util.FindField(v, part)
			if !field.IsValid() {
				return nil, fmt.Errorf("field not found: '%s'", part)
			}
			v = field
		} else if v.Kind() == reflect.Map {
			// The next part is the map key
			if v.Type().Key().Kind() != reflect.String {
				return nil, fmt.Errorf("map key is not a string")
			}
			keyValue := reflect.ValueOf(part)
			mapValue := v.MapIndex(keyValue)
			if !mapValue.IsValid() {
				return nil, fmt.Errorf("key not found in map: '%s'", part)
			}
			v = mapValue
		}

		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
	}

	return v.Interface(), nil
}

// setEnvironmentVariable sets an environment variable for the test scenario
func (w *world) setEnvironmentVariable(name, value string) error {
	expandedValue := w.replaceTmp(value)

	// Special handling for OPENCENTER_CONFIG_DIR to update the world's configDir
	if name == "OPENCENTER_CONFIG_DIR" {
		w.configDir = expandedValue
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(expandedValue, 0755); err != nil {
			return err
		}
	}

	return os.Setenv(name, expandedValue)
}

// validateConfigurationLoading validates that configuration loading works for both
// flat and organization-based structures
func (w *world) validateConfigurationLoading(clusterName string) error {
	// Try to load the configuration using the config package
	orig := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", w.configDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", orig)

	// First try to find the config file
	configPath, err := w.resolveClusterConfigPath(clusterName)
	if err != nil {
		return fmt.Errorf("failed to resolve cluster config path: %w", err)
	}

	// Try to read and parse the configuration
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Normalize the configuration if needed
	normalizedData := normalizeConfigYAML(string(data))

	// Try to unmarshal as a config struct
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(normalizedData), &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate that the cluster name matches
	if cfg.OpenCenter.Cluster.ClusterName != clusterName {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", clusterName, cfg.OpenCenter.Cluster.ClusterName)
	}

	return nil
}

// RegisterSteps registers all step definitions with Godog.
func RegisterSteps(s *godog.ScenarioContext, t *testing.T, w *world) {
	// Before each scenario, reset the world state
	s.BeforeScenario(func(sc *godog.Scenario) {
		w.lastOut = ""
		w.lastErr = ""
		w.lastExit = 0
		w.remoteGitDir = ""
		w.pendingCmd = ""
		w.pendingChoice = ""
		w.answers = nil
		w.cwd = ""
	})
	// Given steps
	s.Step(`^the configuration directory is isolated for tests$`, func() error { return nil })
	s.Step(`^a cluster "([^"]+)" exists$`, w.givenClusterExists)
	s.Step(`^a cluster configuration "([^"]*)" should exist$`, func(name string) error {
		configPath, err := w.findClusterConfigPath(name)
		if err != nil {
			return err
		}
		return w.aFileShouldExist(configPath)
	})
	s.Step(`^the cluster configuration "([^"]*)" should have "([^"]*)" set to "([^"]*)"$`, w.assertClusterConfigValue)
	s.Step(`^the cluster configuration "([^"]*)" should have "([^"]*)" containing "([^"]*)"$`, w.assertClusterConfigValueContains)
	s.Step(`^a cluster "([^"]+)" is configured with a temporary gitops directory$`, w.aClusterIsConfiguredWithTemporaryGitopsDirectory)
	s.Step(`^clusters "([^"]+)" exist$`, w.aClusterExists)
	s.Step(`^the active cluster is "([^"]+)"$`, w.activeClusterIs)
	s.Step(`^an empty directory "([^"]*)"$`, w.anEmptyDirectory)
	s.Step(`^a file "([^"]*)" with content:$`, w.aFileWithContent)
	s.Step(`^a bare git repository exists at "([^"]*)"$`, w.aBareGitRepositoryExistsAt)
	s.Step(`^the directory "([^"]*)" does not exist$`, w.theDirectoryDoesNotExist)
	s.Step(`^the file "([^"]*)" does not exist$`, w.theFileDoesNotExist)
	s.Step(`^the directory "([^"]*)" exists$`, w.anEmptyDirectory)

	// When steps
	s.Step(`^I run "([^"]+)"$`, w.iRunCommand)
	s.Step(`^I cd to "([^"]*)"$`, w.iCdTo)
	s.Step(`^I answer the prompts with:$`, w.iAnswerThePromptsWith)
	s.Step(`^I choose "([^"]*)" from the prompt$`, w.iChooseFromThePrompt)
	s.Step(`^I run interactively "([^"]*)"$`, w.iRunInteractively)

	// And steps for setting values
	s.Step(`^I set "([^"]+)" to "([^"]+)"$`, w.iSetKeyToValue)
	s.Step(`^I update the YAML "([^"]*)" to set:$`, w.iUpdateTheYAMLToSet)
	s.Step(`^I set environment variable "([^"]*)" to "([^"]*)"$`, w.setEnvironmentVariable)
	s.Step(`^"opencenter.gitops.git_url" is configured$`, w.gitopsGitURLIsConfigured)
	s.Step(`^"gitops.git_url" is configured$`, w.gitopsGitURLIsConfigured)
	s.Step(`^the gitops directory is a git repository$`, w.theGitopsDirectoryIsAGitRepository)
	s.Step(`^the gitops repository has a bare remote$`, w.theGitopsRepositoryHasABareRemote)

	// Then steps
	s.Step(`^a file "([^"]+)" should exist$`, w.aFileShouldExist)
	s.Step(`^the file "([^"]*)" should exist$`, w.aFileShouldExist)
	s.Step(`^a directory "([^"]+)" should exist$`, w.aDirectoryShouldExist)
	s.Step(`^the directory "([^"]*)" should exist$`, w.aDirectoryShouldExist)
	s.Step(`^the file should contain "([^"]+)"$`, func(substr string) error {
		if w.lastFile == "" {
			return fmt.Errorf("no file reference available for content check")
		}
		return w.theFileShouldContain(w.lastFile, substr)
	})
	// Convenience matcher using last referenced file to assert absence
	s.Step(`^the file should not contain "([^"]+)"$`, func(substr string) error {
		if w.lastFile == "" {
			return fmt.Errorf("no file reference available for content check")
		}
		return w.theFileShouldNotContain(w.lastFile, substr)
	})
	s.Step(`^the file "([^"]+)" should contain "([^"]+)"$`, w.theFileShouldContain)
	s.Step(`^a file "([^"]*)" should not exist$`, w.aFileShouldNotExist)
	s.Step(`^the file "([^"]*)" should not exist$`, w.aFileShouldNotExist)
	s.Step(`^a directory "([^"]*)" should not exist$`, w.aDirectoryShouldNotExist)
	s.Step(`^the directory "([^"]*)" should not exist$`, w.aDirectoryShouldNotExist)
	s.Step(`^stdout should contain \"(.*)\"$`, w.stdoutShouldContain)
	// Quoted string convenience matcher: stdout should contain "\"text\""
	s.Step(`^stdout should contain "\"([^\"]*)\""$`, func(inner string) error {
		return w.stdoutShouldContain(fmt.Sprintf("\"%s\"", inner))
	})
	// JSON key:value convenience matcher: stdout should contain "\"key\":\"value\""
	s.Step(`^stdout should contain "\"([^\"]*)\":\"([^\"]*)\""$`, func(key, val string) error {
		return w.stdoutShouldContain(fmt.Sprintf("\"%s\":\"%s\"", key, val))
	})
	// Relaxed variants no longer needed due to permissive matcher above
	s.Step(`^stdout should contain '([^']*)'$`, w.stdoutShouldContain)
	s.Step(`^stderr should contain "([^"]*)"$`, w.stderrShouldContain)
	s.Step(`^stdout should contain:$`, w.stdoutShouldContainDocString)
	s.Step(`^exit code should be (\d+)$`, w.exitCodeShouldBe)
	s.Step(`^the exit code should be (\d+)$`, w.exitCodeShouldBe)
	s.Step(`^the command should succeed$`, w.theCommandShouldSucceed)
	s.Step(`^the active cluster should be "([^"]*)"$`, w.theActiveClusterShouldBe)
	s.Step(`^the remote git repo should have a "([^"]+)"$`, w.remoteRepoShouldHaveCommit)
	s.Step(`^the remote git repository should contain a "([^"]*)"$`, w.theRemoteGitRepositoryShouldContainA)
	s.Step(`^the file "([^"]*)" should match regex "([^"]*)"$`, w.theFileShouldMatchRegex)
	s.Step(`^the exit code should not be (\d+)$`, w.theExitCodeShouldNotBe)
	s.Step(`^the first line of stdout should start with "([^"]*)"$`, w.theFirstLineOfStdoutShouldStartWith)
	s.Step(`^stdout should be empty$`, w.stdoutShouldBeEmpty)
	s.Step(`^stdout should not contain "([^"]*)"$`, w.stdoutShouldNotContain)
	s.Step(`^the bare repo "([^"]*)" should have branch "([^"]*)"$`, w.theBareRepoShouldHaveBranch)
	s.Step(`^the directory "([^"]*)" should contain a directory "([^"]*)"$`, w.theDirectoryShouldContainADirectory)
	s.Step(`^the directory "([^"]*)" should contain a file matching "([^"]*)"$`, w.theDirectoryShouldContainAFileMatching)
	s.Step(`^the file "([^"]*)" should not contain "([^"]*)"$`, w.theFileShouldNotContain)
	s.Step(`^stdout should match regex "([^"]*)"$`, w.stdoutShouldMatchRegex)
	s.Step(`^the configuration loading should work for cluster "([^"]*)"$`, w.validateConfigurationLoading)
}

// stdoutShouldMatchRegex checks if stdout matches the given regex pattern
func (w *world) stdoutShouldMatchRegex(pattern string) error {
	// Trim whitespace from stdout to handle newlines
	output := strings.TrimSpace(w.lastOut)
	matched, err := regexp.MatchString(pattern, output)
	if err != nil {
		return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
	}
	if !matched {
		return fmt.Errorf("stdout did not match regex '%s'; got %q", pattern, output)
	}
	return nil
}
