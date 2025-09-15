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
	"github.com/rackerlabs/openCenter/internal/config"
)

// CopyBase copies or renders embedded files from gitops-base-dir into the target directory
// specified by cfg.GitOps.GitDir.
//
// When render is true, files ending with .tmpl are processed as Go text/templates
// with sprig functions available, and the cluster configuration is bound to the
// template under the dot context. The .tmpl extension is stripped from the
// destination filename.
//
// Non-template files are copied as-is. The directory structure under gitops-base-dir/
// is preserved. The target directory is created if it does not exist.
//
// Inputs:
//   - cfg: The cluster configuration.
//   - render: If true, template files will be rendered; otherwise, they will be copied.
//
// Outputs:
//   - error: An error if one occurred during the copy or render operation.
func CopyBase(cfg config.Config, render bool) error {
    target := cfg.GitOps.GitDir
    if target == "" {
        return fmt.Errorf("gitops.git_dir is empty")
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
        // If template file and render flag set, process template
        if strings.HasSuffix(d.Name(), ".tmpl") {
            // Strip .tmpl extension
            dst = strings.TrimSuffix(dst, ".tmpl")
            if render {
                return renderTemplate(path, dst, cfg)
            }
        }
        // Copy file as-is
        return copyFile(path, dst)
    })
    return err
}

// renderTemplate reads the embedded template file at path, executes
// it using the provided configuration, and writes the result to dst.
func renderTemplate(path, dst string, cfg config.Config) error {
    data, err := Files.ReadFile(path)
    if err != nil {
        return err
    }
    t, err := template.New(filepath.Base(path)).Funcs(sprig.TxtFuncMap()).Parse(string(data))
    if err != nil {
        return fmt.Errorf("failed to parse template %s: %w", path, err)
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
