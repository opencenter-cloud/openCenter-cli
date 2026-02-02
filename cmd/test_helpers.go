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

import "github.com/rackerlabs/opencenter-cli/internal/config"

// minimalTestConfig returns a minimal Config for testing.
// This is a helper function for cmd package tests.
func minimalTestConfig(name string) config.Config {
	cfg := config.Config{
		SchemaVersion: config.SchemaVersion,
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         name,
				Organization: "opencenter",
			},
			Cluster: config.ClusterConfig{
				ClusterName: name,
			},
			GitOps: config.GitOpsConfig{
				GitDir: "./testdata/test-git-repo-" + name,
			},
		},
		OpenTofu: config.SimplifiedOpenTofu{
			Enabled: true,
		},
		Secrets: config.Secrets{
			SSHKey: config.SSHKey{
				Private: "./testdata/test-git-repo-" + name + "/" + name + "/secrets/ssh/" + name,
				Public:  "./testdata/test-git-repo-" + name + "/" + name + "/secrets/ssh/" + name + ".pub",
			},
		},
		Metadata: config.NewConfigMetadata(),
	}

	return cfg
}
