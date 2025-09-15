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

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfig(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	t.Run("Save and Load", func(t *testing.T) {
		cfg := NewDefault("test")
		if err := Save(cfg); err != nil {
			t.Fatal(err)
		}

		loaded, err := Load("test")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg, loaded) {
			t.Errorf("loaded config does not match saved config")
		}
	})

	t.Run("List", func(t *testing.T) {
		names, err := List()
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 1 || names[0] != "test" {
			t.Errorf("unexpected list result: %v", names)
		}
	})

	t.Run("SetActive and GetActive", func(t *testing.T) {
		if err := SetActive("test"); err != nil {
			t.Fatal(err)
		}
		active, err := GetActive()
		if err != nil {
			t.Fatal(err)
		}
		if active != "test" {
			t.Errorf("unexpected active cluster: %s", active)
		}
	})

    t.Run("Validate", func(t *testing.T) {
        cfg := NewDefault("test")
        // Missing git_dir should produce a validation error
        errs := Validate(cfg)
        if len(errs) == 0 {
            t.Error("expected validation error for missing gitops.git_dir")
        }
        // Provide minimal required fields
        cfg.GitOps.GitDir = "testdata/gitops"
        errs = Validate(cfg)
        if len(errs) != 0 {
            t.Errorf("unexpected validation errors: %v", errs)
        }
    })

	// New: OpenTofu S3 backend requires opencenter AWS credentials
	t.Run("Validate OpenTofu S3 requires credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.GitOps.GitDir = "testdata/gitops"
		cfg.OpenTofu.Enabled = true
		cfg.OpenTofu.Backend.Type = "s3"
		cfg.OpenTofu.Backend.S3.Bucket = "my-bucket"
		cfg.OpenTofu.Backend.S3.Key = "state.tfstate"
		cfg.OpenTofu.Backend.S3.Region = "us-east-1"

		errs := Validate(cfg)
		if len(errs) == 0 {
			t.Fatal("expected validation error for missing opencenter AWS credentials with s3 backend")
		}
		// Provide credentials, expect no error from this rule (other rules already satisfied)
		cfg.OpenCenter.AWSAccessKey = "AKIA..."
		cfg.OpenCenter.AWSSecretAccessKey = "secret"
		errs = Validate(cfg)
		if len(errs) != 0 {
			t.Errorf("unexpected validation errors with credentials set: %v", errs)
		}
	})
}

func TestResolveConfigDir(t *testing.T) {
	// Unset env var to test default behavior
	os.Unsetenv("OPENCENTER_CONFIG_DIR")

	dir, err := ResolveConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "openCenter")
	if dir != expected {
		t.Errorf("expected config dir %s, but got %s", expected, dir)
	}

	// Set env var to test override (use repo testdata)
    testDir := "testdata/openCenter-test"
    os.Setenv("OPENCENTER_CONFIG_DIR", testDir)
    defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

    dir, err = ResolveConfigDir()
    if err != nil {
        t.Fatal(err)
    }
    // ResolveConfigDir returns an absolute path; compare absolute forms.
    absExpected, _ := filepath.Abs(testDir)
    if dir != absExpected {
        t.Errorf("expected config dir %s, but got %s", absExpected, dir)
    }
}
