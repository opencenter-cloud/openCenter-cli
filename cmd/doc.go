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

/*
Package cmd implements the command-line interface for openCenter.

It provides commands for managing cluster configurations, including initialization, validation, and GitOps scaffolding.

When to use

Use the commands in this package to interact with openCenter from the command line. The main entry point is the `openCenter` command, which has several subcommands for managing clusters.

Examples

To list all available clusters:

	openCenter cluster list

To initialize a new cluster configuration:

	openCenter cluster init my-cluster

To validate a cluster configuration:

	openCenter cluster validate my-cluster
*/
package cmd
