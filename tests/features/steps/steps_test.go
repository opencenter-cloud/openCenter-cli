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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

// TestFeatures runs the BDD scenarios. It uses Godog’s suite to
// register steps defined in helpers.go. Running `go test` in this
// package will execute the feature files automatically.
func TestFeatures(t *testing.T) {
	opts := godog.Options{
		Output: colors.Colored(os.Stdout),
		Format: "pretty",
		Paths:  []string{".."},
		Tags:   "~@wip",
	}
	// Allow overriding via CLI flags passed after 'args' in go test invocation,
	// e.g. `go test ./... -v args --godog.paths=tests/features/foo.feature --godog.tags=@fast`.
	for i := 0; i < len(os.Args); i++ {
		a := os.Args[i]
		if a == "--godog.paths" && i+1 < len(os.Args) {
			p := os.Args[i+1]
			_, thisFile, _, _ := runtime.Caller(0)
			repo := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
			if strings.HasPrefix(p, "tests/") {
				p = filepath.Join(repo, p)
			}
			opts.Paths = []string{p}
		}
		if strings.HasPrefix(a, "--godog.paths=") {
			p := strings.TrimPrefix(a, "--godog.paths=")
			_, thisFile, _, _ := runtime.Caller(0)
			repo := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
			if strings.HasPrefix(p, "tests/") {
				p = filepath.Join(repo, p)
			}
			opts.Paths = []string{p}
		}
		if a == "--godog.tags" && i+1 < len(os.Args) {
			opts.Tags = os.Args[i+1]
		}
		if strings.HasPrefix(a, "--godog.tags=") {
			opts.Tags = strings.TrimPrefix(a, "--godog.tags=")
		}
		if a == "--godog.format" && i+1 < len(os.Args) {
			opts.Format = os.Args[i+1]
		}
		if strings.HasPrefix(a, "--godog.format=") {
			opts.Format = strings.TrimPrefix(a, "--godog.format=")
		}
	}

	w, err := newWorld()
	if err != nil {
		t.Fatalf("failed to create world: %v", err)
	}

	suite := godog.TestSuite{
		Name: "openCenter",
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			s.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
				// Create a per-scenario workspace under repo-level testdata
				// Resolve repo-root testdata directory based on this file's path
				_, thisFile, _, _ := runtime.Caller(0)
				base := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "testdata")
				tmp, err := os.MkdirTemp(base, "opencenter-test-")
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
				os.Unsetenv("OPENCENTER_CONFIG_DIR")
				os.Unsetenv("OPENCENTER_TEST_TMP")

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
				w.remoteGitDir = ""
				w.lastOut = ""
				w.lastErr = ""
				w.lastExit = 0
				w.lastFile = ""
				w.pendingCmd = ""
				w.answers = nil
				w.pendingChoice = ""
				w.cwd = ""

				return ctx, err
			})
		},
		Options: &opts,
	}
	if suite.Run() != 0 {
		t.Fail()
	}
}
