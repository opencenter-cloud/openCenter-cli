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
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 1: Flag categorization consistency
// For any set of CLI flags with different types, the flag parser should consistently
// categorize each flag into the correct type category and maintain proper precedence.
// Validates: Requirements 2.1, 10.1
func TestProperty_FlagCategorizationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("flag parser categorizes flags consistently", prop.ForAll(
		func(dotFlag string, arrayFlag string, jsonFlag string, yamlFlag string, templateFlag string) bool {
			parser := NewEnhancedFlagParser()
			
			// Register mock handlers for testing
			if err := registerMockHandlers(parser); err != nil {
				return false
			}

			// Build command line arguments - only include non-empty flags
			var args []string
			expectedCounts := 0
			
			// Add dot notation flag (should go to DotNotation category)
			if dotFlag != "" {
				args = append(args, fmt.Sprintf("--%s=value", dotFlag))
				expectedCounts++
			}
			
			// Add array flag (should go to ArrayFlags category)  
			if arrayFlag != "" {
				args = append(args, fmt.Sprintf("--%s=name=test,worker_count=3", arrayFlag))
				expectedCounts++
			}
			
			// Add JSON flag (should go to JSONFlags category)
			if jsonFlag != "" {
				args = append(args, fmt.Sprintf("--%s={\"key\":\"value\"}", jsonFlag))
				expectedCounts++
			}
			
			// Add YAML flag (should go to YAMLFlags category)
			if yamlFlag != "" {
				args = append(args, fmt.Sprintf("--%s=key: value", yamlFlag))
				expectedCounts++
			}
			
			// Add template flag (should go to TemplateVars category)
			if templateFlag != "" {
				args = append(args, fmt.Sprintf("--%s=testvalue", templateFlag))
				expectedCounts++
			}

			// Skip test if no flags to test
			if expectedCounts == 0 {
				return true
			}

			// Parse the flags
			parsed, err := parser.ParseFlags(args)
			if err != nil {
				return false
			}

			// Verify categorization consistency
			actualCounts := 0
			
			// Check dot notation flags
			if dotFlag != "" {
				if len(parsed.DotNotation) != 1 {
					return false
				}
				actualCounts++
			}
			
			// Check array flags
			if arrayFlag != "" {
				if len(parsed.ArrayFlags) != 1 {
					return false
				}
				actualCounts++
			}
			
			// Check JSON flags
			if jsonFlag != "" {
				if len(parsed.JSONFlags) != 1 {
					return false
				}
				actualCounts++
			}
			
			// Check YAML flags
			if yamlFlag != "" {
				if len(parsed.YAMLFlags) != 1 {
					return false
				}
				actualCounts++
			}
			
			// Check template flags
			if templateFlag != "" {
				if len(parsed.TemplateVars) != 1 {
					return false
				}
				actualCounts++
			}

			// Verify total count matches expected
			return actualCounts == expectedCounts
		},
		genDotNotationFlag(),
		genArrayFlag(),
		genJSONFlag(),
		genYAMLFlag(),
		genTemplateFlag(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// registerMockHandlers registers mock handlers for testing flag categorization
func registerMockHandlers(parser *EnhancedFlagParser) error {
	// Register array flag handler
	arrayHandler := &mockArrayFlagHandler{}
	if err := parser.RegisterHandler("server-pool|ssh-key|dns-server|subnet", arrayHandler); err != nil {
		return err
	}
	
	// Register JSON flag handler
	jsonHandler := &mockJSONFlagHandler{}
	if err := parser.RegisterHandler("json-set.*", jsonHandler); err != nil {
		return err
	}
	
	// Register YAML flag handler
	yamlHandler := &mockYAMLFlagHandler{}
	if err := parser.RegisterHandler("yaml-set.*|yaml-data.*", yamlHandler); err != nil {
		return err
	}
	
	// Register template flag handler
	templateHandler := &mockTemplateFlagHandler{}
	if err := parser.RegisterHandler("template-var-.*", templateHandler); err != nil {
		return err
	}
	
	return nil
}

// Mock handlers for testing

type mockArrayFlagHandler struct{}

func (h *mockArrayFlagHandler) CanHandle(flagName string) bool {
	return strings.Contains(flagName, "server-pool") || 
		   strings.Contains(flagName, "ssh-key") || 
		   strings.Contains(flagName, "dns-server") || 
		   strings.Contains(flagName, "subnet")
}

func (h *mockArrayFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return value, nil
}

func (h *mockArrayFlagHandler) GetFlagType() FlagType {
	return FlagTypeArray
}

func (h *mockArrayFlagHandler) ParseArrayFlag(flagName, value string) (*ArrayConfig, error) {
	return &ArrayConfig{
		Path:   flagName,
		Index:  0,
		Fields: map[string]interface{}{"value": value},
		Type:   "mock",
	}, nil
}

func (h *mockArrayFlagHandler) SupportedTypes() []string {
	return []string{"server-pool", "ssh-key", "dns-server", "subnet"}
}

func (h *mockArrayFlagHandler) ValidateArrayConfig(config *ArrayConfig) error {
	return nil
}

type mockJSONFlagHandler struct{}

func (h *mockJSONFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "json-set")
}

func (h *mockJSONFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	// Return a JSONFlag structure like the real handler
	path := strings.TrimPrefix(flagName, "json-set-")
	if path == "" {
		path = "test.path"
	}
	return &JSONFlag{
		Path:  path,
		Value: map[string]interface{}{"parsed": value},
	}, nil
}

func (h *mockJSONFlagHandler) GetFlagType() FlagType {
	return FlagTypeJSON
}

type mockYAMLFlagHandler struct{}

func (h *mockYAMLFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "yaml-set") || strings.HasPrefix(flagName, "yaml-data")
}

func (h *mockYAMLFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	// Return a YAMLFlag structure like the real handler
	path := strings.TrimPrefix(flagName, "yaml-set-")
	if path == flagName {
		path = strings.TrimPrefix(flagName, "yaml-data-")
	}
	if path == "" {
		path = "test.path"
	}
	return &YAMLFlag{
		Path:  path,
		Value: map[string]interface{}{"parsed": value},
	}, nil
}

func (h *mockYAMLFlagHandler) GetFlagType() FlagType {
	return FlagTypeYAML
}

type mockTemplateFlagHandler struct{}

func (h *mockTemplateFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "template-var-")
}

func (h *mockTemplateFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return value, nil
}

func (h *mockTemplateFlagHandler) GetFlagType() FlagType {
	return FlagTypeTemplate
}

// Generators for different flag types

func genDotNotationFlag() gopter.Gen {
	return gen.OneConstOf("", "config.value", "meta.env", "cluster.name", "infrastructure.provider")
}

func genArrayFlag() gopter.Gen {
	return gen.OneConstOf("", "server-pool", "ssh-key", "dns-server", "subnet")
}

func genJSONFlag() gopter.Gen {
	return gen.OneConstOf("", "json-set-config", "json-set-data", "json-set-values")
}

func genYAMLFlag() gopter.Gen {
	return gen.OneConstOf("", "yaml-set-config", "yaml-set-data", "yaml-set-values")
}

func genTemplateFlag() gopter.Gen {
	return gen.OneConstOf("", "template-var-ENV", "template-var-REGION", "template-var-CLUSTER")
}