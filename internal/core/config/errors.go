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

import "fmt"

// V1ConfigError is returned when attempting to load a v1 configuration in v2.0.0.
// This error provides clear guidance on how to migrate from v1 to v2.
type V1ConfigError struct {
	// Path is the file path of the v1 configuration
	Path string

	// Message is the primary error message
	Message string

	// Suggestion provides actionable guidance for the user
	Suggestion string
}

// Error implements the error interface.
func (e *V1ConfigError) Error() string {
	return fmt.Sprintf("%s\n\nFile: %s\n\n%s", e.Message, e.Path, e.Suggestion)
}

// Is implements error matching for errors.Is().
func (e *V1ConfigError) Is(target error) bool {
	_, ok := target.(*V1ConfigError)
	return ok
}

// NewV1ConfigError creates a new V1ConfigError with standard messaging.
func NewV1ConfigError(path string) *V1ConfigError {
	return &V1ConfigError{
		Path:    path,
		Message: "v1 configurations are not supported in v2.0.0",
		Suggestion: `To upgrade to v2.0.0:
1. Install opencenter v1.x (latest version)
2. Run: opencenter cluster migrate-config <cluster-name>
3. Verify the migrated configuration
4. Upgrade to opencenter v2.0.0

For more information, see: https://docs.opencenter.io/migration/v1-to-v2`,
	}
}

// UnsupportedVersionError is returned when a configuration uses an unsupported schema version.
type UnsupportedVersionError struct {
	// Version is the unsupported schema version found
	Version string

	// Path is the file path of the configuration
	Path string
}

// Error implements the error interface.
func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("unsupported schema version %q in %s (only v2.0 is supported)", e.Version, e.Path)
}

// Is implements error matching for errors.Is().
func (e *UnsupportedVersionError) Is(target error) bool {
	_, ok := target.(*UnsupportedVersionError)
	return ok
}
