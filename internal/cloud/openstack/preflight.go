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

package openstack

import (
	"os/exec"
)

// PreflightOpenStack performs provider-specific preflight checks for
// OpenStack. It checks for the presence of the `openstack` CLI tool and
// verifies that the authentication URL is configured.
//
// Inputs:
//   - authURL: The OpenStack authentication URL.
//
// Outputs:
//   - []string: A list of warning messages. If the list is empty, all checks passed.
func PreflightOpenStack(authURL string) []string {
	var warnings []string
	// Check presence of openstack CLI
	if _, err := exec.LookPath("openstack"); err != nil {
		warnings = append(warnings, "openstack CLI not found: please install the OpenStack client tools and configure OS_* environment variables or clouds.yaml")
	}
	// Check auth URL configured
	if authURL == "" {
		warnings = append(warnings, "cloud.openstack.auth_url is empty; authentication may fail")
	}
	return warnings
}
