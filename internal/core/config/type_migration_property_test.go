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

package config_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	coreconfig "github.com/rackerlabs/opencenter-cli/internal/core/config"
	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
)

// TestProperty_TypeMigrationPreservesBehavior verifies that moving types to
// internal/core/config preserves all behavior through type aliases.
//
// **Validates: Requirements 2.1, 2.2, 2.3**
//
// Property: Type aliases in internal/core/config are equivalent to types in internal/config
func TestProperty_TypeMigrationPreservesBehavior(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Config type is equivalent", prop.ForAll(
		func(schemaVersion string, clusterName string) bool {
			// Create config using internal/config types
			oldCfg := internalconfig.Config{
				SchemaVersion: schemaVersion,
			}
			oldCfg.OpenCenter.Cluster.ClusterName = clusterName

			// Create config using internal/core/config types (which are aliases)
			newCfg := coreconfig.Config{
				SchemaVersion: schemaVersion,
			}
			newCfg.OpenCenter.Cluster.ClusterName = clusterName

			// Type names should be identical (they're aliases)
			oldType := reflect.TypeOf(oldCfg)
			newType := reflect.TypeOf(newCfg)

			// Since they're type aliases, they should be the same underlying type
			return oldType == newType &&
				oldCfg.SchemaVersion == newCfg.SchemaVersion &&
				oldCfg.ClusterName() == newCfg.ClusterName()
		},
		gen.AlphaString(),
		gen.Identifier(),
	))

	properties.Property("ConfigMetadata type is equivalent", prop.ForAll(
		func(createdBy string) bool {
			// Create metadata using internal/config
			oldMeta := internalconfig.NewConfigMetadata()
			oldMeta.CreatedBy = createdBy

			// Create metadata using internal/core/config
			newMeta := coreconfig.NewConfigMetadata()
			newMeta.CreatedBy = createdBy

			// Types should be identical
			oldType := reflect.TypeOf(oldMeta)
			newType := reflect.TypeOf(newMeta)

			return oldType == newType && oldMeta.CreatedBy == newMeta.CreatedBy
		},
		gen.Identifier(),
	))

	properties.Property("Constants are equivalent", prop.ForAll(
		func() bool {
			// Verify stage constants are equivalent
			stagesMatch := coreconfig.StageInit == internalconfig.StageInit &&
				coreconfig.StagePreflight == internalconfig.StagePreflight &&
				coreconfig.StageSetup == internalconfig.StageSetup &&
				coreconfig.StageBootstrap == internalconfig.StageBootstrap &&
				coreconfig.StageValidate == internalconfig.StageValidate &&
				coreconfig.StageDestroy == internalconfig.StageDestroy &&
				coreconfig.StageRender == internalconfig.StageRender &&
				coreconfig.StagePlan == internalconfig.StagePlan &&
				coreconfig.StageApply == internalconfig.StageApply

			// Verify status constants are equivalent
			statusesMatch := coreconfig.StatusPending == internalconfig.StatusPending &&
				coreconfig.StatusRunning == internalconfig.StatusRunning &&
				coreconfig.StatusSuccess == internalconfig.StatusSuccess &&
				coreconfig.StatusFailed == internalconfig.StatusFailed

			return stagesMatch && statusesMatch
		},
	))

	properties.TestingRun(t)
}

// TestProperty_TypeAliasesAreTransparent verifies that type aliases don't
// introduce any overhead or behavioral differences.
func TestProperty_TypeAliasesAreTransparent(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("type aliases are transparent", prop.ForAll(
		func(schemaVersion string) bool {
			// Create a config using the core package
			coreCfg := coreconfig.Config{
				SchemaVersion: schemaVersion,
			}

			// Assign it to a variable of the internal/config type
			// This should work seamlessly because they're aliases
			var internalCfg internalconfig.Config = coreCfg

			// They should be identical
			return coreCfg.SchemaVersion == internalCfg.SchemaVersion
		},
		gen.AlphaString(),
	))

	properties.Property("pointers to aliased types are compatible", prop.ForAll(
		func(schemaVersion string) bool {
			// Create a pointer using core package
			coreCfg := &coreconfig.Config{
				SchemaVersion: schemaVersion,
			}

			// Assign to internal/config pointer type
			var internalCfg *internalconfig.Config = coreCfg

			// They should point to the same data
			return coreCfg.SchemaVersion == internalCfg.SchemaVersion &&
				&coreCfg.SchemaVersion == &internalCfg.SchemaVersion
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
