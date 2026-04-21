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
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

// TestFeatures runs the BDD scenarios. It uses Godog’s suite to
// register steps defined in helpers.go. Running `go test` in this
// package will execute the feature files automatically.
func TestFeatures(t *testing.T) {
	opts := defaultGodogOptions(t)
	parseGodogArgs(t, &opts)
	assertReadableFeatureFiles(t, opts.Paths)

	w, err := newWorld()
	if err != nil {
		t.Fatalf("failed to create world: %v", err)
	}

	suite := godog.TestSuite{
		Name: "opencenter",
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			s.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
				tmp, err := newScratchDir("opencenter-test-")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}
				w.tmpDir = tmp
				if err := w.isolateConfigDir(); err != nil {
					t.Fatalf("failed to isolate config dir: %v", err)
				}
				return ctx, nil
			})

			RegisterSteps(s, t, w)

			s.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
				// Clean up environment variables
				if w.oldConfigEnv != "" {
					os.Setenv("OPENCENTER_CONFIG_DIR", w.oldConfigEnv)
				} else {
					os.Unsetenv("OPENCENTER_CONFIG_DIR")
				}
				if w.oldStateEnv != "" {
					os.Setenv("OPENCENTER_STATE_DIR", w.oldStateEnv)
				} else {
					os.Unsetenv("OPENCENTER_STATE_DIR")
				}
				os.Unsetenv("OPENCENTER_TEST_TMP")
				if w.oldClusterEnv != "" {
					os.Setenv("OPENCENTER_CLUSTER", w.oldClusterEnv)
				} else {
					os.Unsetenv("OPENCENTER_CLUSTER")
				}
				if w.oldSessionEnv != "" {
					os.Setenv("OPENCENTER_SESSION_FILE", w.oldSessionEnv)
				} else {
					os.Unsetenv("OPENCENTER_SESSION_FILE")
				}
				if w.oldSessionID != "" {
					os.Setenv("OPENCENTER_SESSION_ID", w.oldSessionID)
				} else {
					os.Unsetenv("OPENCENTER_SESSION_ID")
				}

				// Clean up temporary directories
				if w.tmpDir != "" {
					os.RemoveAll(w.tmpDir)
				}
				if w.configDir != "" && w.configDir != w.tmpDir {
					os.RemoveAll(w.configDir)
				}
				if w.remoteGitDir != "" {
					os.RemoveAll(w.remoteGitDir)
				}

				// Reset world state
				w.tmpDir = ""
				w.configDir = ""
				w.stateDir = ""
				w.remoteGitDir = ""
				w.lastOut = ""
				w.lastErr = ""
				w.lastExit = 0
				w.lastFile = ""
				w.pendingCmd = ""
				w.answers = nil
				w.pendingChoice = ""
				w.cwd = ""
				w.oldConfigEnv = ""
				w.oldStateEnv = ""
				w.oldClusterEnv = ""
				w.oldSessionEnv = ""
				w.oldSessionID = ""

				return ctx, err
			})
		},
		Options: &opts,
	}
	if status := suite.Run(); status != 0 {
		t.Fatalf("godog suite returned status %d", status)
	}
}

func defaultGodogOptions(t *testing.T) godog.Options {
	return godog.Options{
		Output:   colors.Colored(os.Stdout),
		Format:   "pretty",
		Paths:    []string{filepath.Join(repoRoot(), "tests", "features")},
		Tags:     "~@wip",
		Strict:   true,
		TestingT: t,
	}
}

func parseGodogArgs(t *testing.T, opts *godog.Options) {
	t.Helper()

	flagSet := flag.NewFlagSet("godog", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	godog.BindFlags("godog.", flagSet, opts)

	if err := flagSet.Parse(extractGodogArgs(os.Args[1:])); err != nil {
		t.Fatalf("failed to parse godog flags: %v", err)
	}

	for i, path := range opts.Paths {
		if strings.HasPrefix(path, "tests/") {
			opts.Paths[i] = filepath.Join(repoRoot(), path)
		}
	}
}

func extractGodogArgs(args []string) []string {
	filtered := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--godog.") {
			continue
		}

		filtered = append(filtered, arg)

		if strings.Contains(arg, "=") || i+1 >= len(args) {
			continue
		}

		next := args[i+1]
		if strings.HasPrefix(next, "-") {
			continue
		}

		filtered = append(filtered, next)
		i++
	}

	return filtered
}

func assertReadableFeatureFiles(t *testing.T, paths []string) {
	t.Helper()

	for _, target := range paths {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat feature path %q: %v", target, err)
		}

		if info.IsDir() {
			err = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() || filepath.Ext(path) != ".feature" {
					return nil
				}
				return validateFeatureSpacing(path)
			})
			if err != nil {
				t.Fatalf("feature file formatting check failed: %v", err)
			}
			continue
		}

		if err := validateFeatureSpacing(target); err != nil {
			t.Fatalf("feature file formatting check failed: %v", err)
		}
	}
}

func validateFeatureSpacing(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read feature file %q: %w", path, err)
	}

	for lineNo, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimLeft(rawLine, " \t")
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") || line == `"""` {
			continue
		}

		for _, keyword := range []string{"Feature:", "Scenario:", "Scenario Outline:", "Background:", "Examples:"} {
			if strings.HasPrefix(line, keyword) && !hasSeparator(line, keyword) {
				return fmt.Errorf("%s:%d has corrupted Gherkin spacing: %q", path, lineNo+1, rawLine)
			}
		}

		for _, keyword := range []string{"Given", "When", "Then", "And", "But"} {
			if strings.HasPrefix(line, keyword) && !hasSeparator(line, keyword) {
				return fmt.Errorf("%s:%d has corrupted Gherkin spacing: %q", path, lineNo+1, rawLine)
			}
		}
	}

	return nil
}

func hasSeparator(line, keyword string) bool {
	if len(line) == len(keyword) {
		return strings.HasSuffix(keyword, ":")
	}

	next := line[len(keyword)]
	return next == ' ' || next == '\t'
}
