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
Package ansible provides functionality for generating Ansible files.

This package is used to create Ansible inventory and configuration files from templates. It is called when the `ansible.enabled` flag is set to `true` in the cluster configuration.

# When to use

This package is used internally by openCenter and is not intended for direct use by end-users. It is invoked as part of the `cluster setup` command.
*/
package ansible
