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
Package config provides functionality for managing cluster configurations.

This package defines the data structures for the cluster configuration, as well as functions for loading, saving, and validating configurations. It also includes functionality for generating a JSON schema for the configuration.

# When to use

This package is used internally by openCenter to manage cluster configurations. It is not intended for direct use by end-users.

# Configuration structure

The main data structure in this package is the `Config` struct, which represents the root configuration for a cluster. It groups GitOps, Kubernetes, and Cloud configuration under nested fields.
*/
package config
