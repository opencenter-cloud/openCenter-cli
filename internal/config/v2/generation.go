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

package v2

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// generationIDPattern is the canonical lowercase UUID form. Bootstrap embeds
// the generation identifier in a filesystem path segment, so anything that is
// not canonical must never reach a path.
var generationIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// NewGenerationID mints a fresh cluster generation identifier.
func NewGenerationID() string {
	return uuid.NewString()
}

// IsGenerationID reports whether value is a canonical lowercase UUID.
func IsGenerationID(value string) bool {
	return generationIDPattern.MatchString(value)
}

// EnsureGenerationID stamps a generation identifier on configurations that have
// none, and reports whether one was minted. Load and save call it so a config
// written before the field existed acquires an identifier on its first
// round-trip through the CLI; every later operation preserves the existing
// value untouched.
//
// A malformed identifier is replaced rather than propagated: it would otherwise
// reach a filesystem path segment. That only ever happens if a user hand-edited
// metadata.generation_id, since no other code path writes a non-canonical value.
func EnsureGenerationID(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	if IsGenerationID(cfg.Metadata.GenerationID) {
		return false
	}
	cfg.Metadata.GenerationID = NewGenerationID()
	return true
}

// GenerationID returns the cluster's generation identifier, or "" when the
// configuration predates the field and has not been round-tripped yet. Callers
// that need a usable identifier must resolve one through EnsureGenerationID
// rather than treating the empty string as a valid path segment.
func (c Config) GenerationID() string {
	return strings.TrimSpace(c.Metadata.GenerationID)
}
