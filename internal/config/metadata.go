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

import (
	"os"
	"time"
)

// ConfigMetadata contains metadata about the configuration file including
// creation and update timestamps, creator information, and custom tags/annotations.
// This metadata is used for tracking configuration lifecycle and provenance.
type ConfigMetadata struct {
	CreatedAt   time.Time         `yaml:"created_at" json:"created_at"`
	UpdatedAt   time.Time         `yaml:"updated_at" json:"updated_at"`
	CreatedBy   string            `yaml:"created_by" json:"created_by"`
	Tags        map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// NewConfigMetadata creates a new ConfigMetadata instance with current timestamp
// and the current user as the creator.
//
// Outputs:
//   - ConfigMetadata: A new metadata instance with initialized timestamps.
func NewConfigMetadata() ConfigMetadata {
	now := time.Now()
	creator := getCurrentUser()

	return ConfigMetadata{
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   creator,
		Tags:        make(map[string]string),
		Annotations: make(map[string]string),
	}
}

// Touch updates the UpdatedAt timestamp to the current time.
// This should be called whenever the configuration is modified.
func (m *ConfigMetadata) Touch() {
	m.UpdatedAt = time.Now()
}

// getCurrentUser returns the current system user name.
// Falls back to "unknown" if the user cannot be determined.
func getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}
