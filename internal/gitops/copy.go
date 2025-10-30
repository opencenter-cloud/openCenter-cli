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

package gitops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// CopyBase copies or renders embedded files from gitops-base-dir into the target directory
// specified by cfg.GitOps().GitDir.
//
// Files ending with .tpl are always rendered with the cluster configuration bound
// under the dot context and the .tpl suffix stripped from the destination path.
// When render is true, .tmpl files are rendered using the same rules. When render
// is false, .tmpl files are copied verbatim (extension preserved) to allow manual
// customization workflows.
//
// Non-template files are copied as-is. The directory structure under gitops-base-dir/
// is preserved. The target directory is created if it does not exist.
//
// Inputs:
//   - cfg: The cluster configuration.
//   - render: If true, both .tpl and .tmpl files render; if false, only .tpl
//     files render while .tmpl files are copied as-is for manual editing.
//
// Outputs:
//   - error: An error if one occurred during the copy or render operation.
func CopyBase(cfg config.Config, render bool) error {
	target := cfg.GitOps().GitDir
	if target == "" {
		return fmt.Errorf("opencenter.gitops.git_dir must be set")
	}
	// Create target directory if missing
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	// Walk embedded files
	err := fs.WalkDir(Files, "gitops-base-dir", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("gitops-base-dir", path)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		name := d.Name()
		isTpl := strings.HasSuffix(name, ".tpl")
		isTmpl := strings.HasSuffix(name, ".tmpl")
		if isTpl || isTmpl {
			shouldRender := render || isTpl
			if shouldRender {
				if isTpl {
					dst = strings.TrimSuffix(dst, ".tpl")
				} else {
					dst = strings.TrimSuffix(dst, ".tmpl")
				}
				return renderTemplate(path, dst, cfg)
			}
			return copyFile(path, dst)
		}
		// Copy file as-is
		return copyFile(path, dst)
	})
	return err
}

// renderTemplate reads the embedded template file at path, executes
// it using the provided configuration, and writes the result to dst.
// It handles special cases where template files contain non-Go template syntax.
func renderTemplate(path, dst string, cfg config.Config) error {
	data, err := Files.ReadFile(path)
	if err != nil {
		return err
	}

	// Handle special cases for files that contain conflicting template syntax
	content := string(data)
	filename := filepath.Base(path)

	// For Makefile.tpl, escape Helm template syntax to prevent Go template parsing conflicts
	if filename == "Makefile.tpl" {
		// Replace Helm template syntax with escaped version for Go template processing
		content = strings.ReplaceAll(content, `--template="{{.Version}}"`, `--template="{{"{{"}}.Version{{"}}"}}"`)
	}

	t, err := template.New(filename).Funcs(sprig.TxtFuncMap()).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", path, err)
	}
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := t.Execute(f, cfg); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", path, err)
	}
	return nil
}

// copyFile copies an embedded file from src to dst without
// interpretation. The dst file is created with default permissions.
func copyFile(src, dst string) error {
	data, err := Files.ReadFile(src)
	if err != nil {
		return err
	}
	// Ensure directory
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// shouldSkipFile determines if a file should be skipped based on service configuration.
// It checks if the file belongs to a disabled service or managed service.
func shouldSkipFile(relPath string, cfg config.Config) bool {
	pathParts := strings.Split(relPath, string(filepath.Separator))
	
	// Skip files in disabled services directories
	if len(pathParts) >= 2 && pathParts[0] == "services" {
		serviceName := pathParts[1]
		if service, exists := cfg.OpenCenter.Services[serviceName]; exists && !service.Enabled {
			return true
		}
	}
	
	// Skip files in disabled managed services directories
	if len(pathParts) >= 2 && pathParts[0] == "managed-services" {
		serviceName := pathParts[1]
		if service, exists := cfg.OpenCenter.ManagedService[serviceName]; exists && !service.Enabled {
			return true
		}
	}
	
	return false
}

// RenderClusterApps renders cluster-apps-base template to applications/overlays/<cluster-name>/
// This function processes all files in the cluster-apps-base template directory,
// renders .tmpl files with the cluster configuration, and copies others as-is.
// It skips directories for disabled services and managed services.
func RenderClusterApps(cfg config.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	target := filepath.Join(cfg.GitOps().GitDir, "applications", "overlays", clusterName)

	// Create target directory
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Walk embedded cluster-apps-base files
	return fs.WalkDir(Files, "templates/cluster-apps-base", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("templates/cluster-apps-base", path)
		if err != nil {
			return err
		}

		// Skip files for disabled services
		if shouldSkipFile(rel, cfg) {
			return nil
		}

		dst := filepath.Join(target, rel)

		// If template file, process and strip template extension
		if strings.HasSuffix(d.Name(), ".tmpl") || strings.HasSuffix(d.Name(), ".tpl") {
			if strings.HasSuffix(d.Name(), ".tmpl") {
				dst = strings.TrimSuffix(dst, ".tmpl")
			} else {
				dst = strings.TrimSuffix(dst, ".tpl")
			}
			return renderTemplate(path, dst, cfg)
		}

		// Copy file as-is
		return copyFile(path, dst)
	})
}

// RenderInfrastructureCluster renders infrastructure-cluster-template to infrastructure/clusters/<cluster-name>/
// This function processes all files in the infrastructure-cluster-template directory,
// renders .tmpl and .tpl files with the cluster configuration, and copies others as-is.
func RenderInfrastructureCluster(cfg config.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	target := filepath.Join(cfg.GitOps().GitDir, "infrastructure", "clusters", clusterName)

	// Create target directory
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Walk embedded infrastructure-cluster-template files
	return fs.WalkDir(Files, "templates/infrastructure-cluster-template", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("templates/infrastructure-cluster-template", path)
		if err != nil {
			return err
		}

		dst := filepath.Join(target, rel)

		// If template file, process and strip template extension
		if strings.HasSuffix(d.Name(), ".tmpl") || strings.HasSuffix(d.Name(), ".tpl") {
			if strings.HasSuffix(d.Name(), ".tmpl") {
				dst = strings.TrimSuffix(dst, ".tmpl")
			} else {
				dst = strings.TrimSuffix(dst, ".tpl")
			}
			return renderTemplate(path, dst, cfg)
		}

		// Copy file as-is
		return copyFile(path, dst)
	})
}
