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

package ansible

import (
	"fmt"
	"os"
	"path/filepath"
    "strings"

	"github.com/rackerlabs/openCenter/internal/config"
	"github.com/rackerlabs/openCenter/internal/provision"
)

// Provision generates Ansible files from templates. It creates the
// `ansible.cfg` and inventory files in the directory specified by
// `cfg.Ansible.Path` within the GitOps repository.
//
// Inputs:
//   - cfg: The cluster configuration.
//
// Outputs:
//   - error: An error if one occurred during file generation.
func Provision(cfg config.Config) error {
    if !cfg.Ansible.Enabled {
        return nil
    }

    // When using raw iac.main_tf content, skip ansible provisioning to avoid
    // relying on legacy IAC fields that no longer exist.
    if strings.TrimSpace(cfg.IAC.MainTF) != "" {
        return nil
    }

	ansibleDir := filepath.Join(cfg.GitOps.GitDir, cfg.Ansible.Path)
	if err := os.MkdirAll(ansibleDir, 0755); err != nil {
		return fmt.Errorf("failed to create ansible directory: %w", err)
	}

	// Create ansible.cfg
	ansibleCfgPath := filepath.Join(ansibleDir, "ansible.cfg")
	ansibleCfgFile, err := os.Create(ansibleCfgPath)
	if err != nil {
		return fmt.Errorf("failed to create ansible.cfg: %w", err)
	}
	defer ansibleCfgFile.Close()

	if err := provision.Templates.ExecuteTemplate(ansibleCfgFile, "ansible.cfg.tmpl", cfg); err != nil {
		return fmt.Errorf("failed to execute ansible.cfg template: %w", err)
	}

	// Create inventory
	inventoryPath := filepath.Join(ansibleDir, "inventory")
	inventoryFile, err := os.Create(inventoryPath)
	if err != nil {
		return fmt.Errorf("failed to create inventory: %w", err)
	}
	defer inventoryFile.Close()

    if err := provision.Templates.ExecuteTemplate(inventoryFile, "inventory.tmpl", cfg); err != nil {
        return fmt.Errorf("failed to execute inventory template: %w", err)
    }

	return nil
}
