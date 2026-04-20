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

package cmd

import (
	"fmt"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// minimalTestConfig returns a minimal v2 config for testing.
// This is a helper function for cmd package tests.
func minimalTestConfig(name string) v2.Config {
	cfg, err := v2.NewV2Default(name, "openstack")
	if err != nil {
		panic(fmt.Sprintf("minimalTestConfig: %v", err))
	}

	result := *cfg
	result.OpenCenter.Meta.Organization = "opencenter"
	result.OpenCenter.Cluster.ClusterName = name
	result.OpenCenter.Cluster.BaseDomain = "example.com"
	result.OpenCenter.Cluster.ClusterFQDN = fmt.Sprintf("%s.example.com", name)
	result.OpenCenter.GitOps.Repository.LocalDir = "./testdata/test-git-repo-" + name
	result.OpenTofu.Enabled = true
	result.Secrets.SSHKey = v2.SSHKeyConfig{
		Private: "./testdata/test-git-repo-" + name + "/" + name + "/secrets/ssh/" + name,
		Public:  "./testdata/test-git-repo-" + name + "/" + name + "/secrets/ssh/" + name + ".pub",
	}

	return result
}
