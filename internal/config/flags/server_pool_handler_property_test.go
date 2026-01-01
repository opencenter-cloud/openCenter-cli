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

package flags

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 7: Dedicated flag handler consistency
// For any dedicated array flag (server-pool, ssh-key, dns-server), multiple instances
// should create multiple configuration entries without interference.
// Validates: Requirements 2.1, 2.2, 2.4, 2.5
func TestProperty_DedicatedFlagHandlerConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("server pool flag handler creates consistent configurations", prop.ForAll(
		func(name1 string, workerCount1 int, flavor1 string, node1 string,
			 name2 string, workerCount2 int, flavor2 string, node2 string) bool {
			
			handler := NewServerPoolFlagHandler()
			
			// Create two different server pool configurations
			value1 := fmt.Sprintf("name=%s,worker_count=%d,flavor_worker=%s,node_worker=%s",
				name1, workerCount1, flavor1, node1)
			value2 := fmt.Sprintf("name=%s,worker_count=%d,flavor_worker=%s,node_worker=%s",
				name2, workerCount2, flavor2, node2)
			
			// Parse both configurations
			config1, err1 := handler.ParseArrayFlag("server-pool", value1)
			if err1 != nil {
				return false
			}
			
			config2, err2 := handler.ParseArrayFlag("server-pool", value2)
			if err2 != nil {
				return false
			}
			
			// Verify both configurations are valid and independent
			if config1 == nil || config2 == nil {
				return false
			}
			
			// Verify configurations have correct type
			if config1.Type != "server-pool" || config2.Type != "server-pool" {
				return false
			}
			
			// Verify configurations have correct path
			expectedPath := "opencenter.infrastructure.server_pools"
			if config1.Path != expectedPath || config2.Path != expectedPath {
				return false
			}
			
			// Verify configurations contain the expected fields
			if !hasRequiredFields(config1.Fields) || !hasRequiredFields(config2.Fields) {
				return false
			}
			
			// Verify field values match input
			if config1.Fields["name"] != name1 || config1.Fields["worker_count"] != workerCount1 {
				return false
			}
			if config1.Fields["flavor_worker"] != flavor1 || config1.Fields["node_worker"] != node1 {
				return false
			}
			
			if config2.Fields["name"] != name2 || config2.Fields["worker_count"] != workerCount2 {
				return false
			}
			if config2.Fields["flavor_worker"] != flavor2 || config2.Fields["node_worker"] != node2 {
				return false
			}
			
			// Verify configurations are independent (changing one doesn't affect the other)
			config1.Fields["test_field"] = "test_value"
			if _, exists := config2.Fields["test_field"]; exists {
				return false
			}
			
			return true
		},
		genServerPoolName(),
		genWorkerCount(),
		genFlavorName(),
		genNodeName(),
		genServerPoolName(),
		genWorkerCount(),
		genFlavorName(),
		genNodeName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// hasRequiredFields checks if all required fields are present
func hasRequiredFields(fields map[string]interface{}) bool {
	requiredFields := []string{"name", "worker_count", "flavor_worker", "node_worker"}
	for _, field := range requiredFields {
		if _, exists := fields[field]; !exists {
			return false
		}
	}
	return true
}

// Generators for server pool configuration values

func genServerPoolName() gopter.Gen {
	return gen.OneConstOf("compute", "storage", "network", "database", "cache", "worker")
}

func genWorkerCount() gopter.Gen {
	return gen.IntRange(1, 10) // Keep within valid range for testing
}

func genFlavorName() gopter.Gen {
	return gen.OneConstOf("small", "medium", "large", "xlarge", "m1.small", "m1.medium", "m1.large")
}

func genNodeName() gopter.Gen {
	return gen.OneConstOf("worker", "compute", "storage", "control", "edge")
}