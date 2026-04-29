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

import "testing"

// TestAdminEmailDefault verifies that admin_email has a default value
func TestAdminEmailDefault(t *testing.T) {
	cfg := NewDefault("test-cluster")

	if cfg.OpenCenter.Cluster.AdminEmail == "" {
		t.Error("AdminEmail should not be empty - it should have a default value")
	}

	if cfg.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
		t.Errorf("expected AdminEmail 'admin@example.com', got '%s'", cfg.OpenCenter.Cluster.AdminEmail)
	}
}
