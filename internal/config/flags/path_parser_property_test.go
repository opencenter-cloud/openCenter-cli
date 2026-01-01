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

// Feature: cli-configuration-enhancement, Property 2: Syntax equivalence
// For any configuration path with array indices, bracket syntax `field[0].subfield` 
// and dot syntax `field.0.subfield` should produce identical configuration results
// Validates: Requirements 1.1, 1.2
func TestProperty_SyntaxEquivalence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("bracket and dot syntax produce equivalent structured paths", prop.ForAll(
		func(fieldName string, index int, subfield string) bool {
			parser := NewEnhancedPathParser()
			
			// Create bracket syntax path: field[index].subfield
			bracketPath := fmt.Sprintf("%s[%d].%s", fieldName, index, subfield)
			
			// Create dot syntax path: field.index.subfield  
			dotPath := fmt.Sprintf("%s.%d.%s", fieldName, index, subfield)
			
			// Parse both paths
			bracketStructured, err1 := parser.ParsePath(bracketPath)
			if err1 != nil {
				return false
			}
			
			dotStructured, err2 := parser.ParsePath(dotPath)
			if err2 != nil {
				return false
			}
			
			// Both should be valid
			if bracketStructured == nil || dotStructured == nil {
				return false
			}
			
			// Both should have array access
			if !bracketStructured.HasArrayAccess() || !dotStructured.HasArrayAccess() {
				return false
			}
			
			// Both should be marked as arrays
			if !bracketStructured.IsArray || !dotStructured.IsArray {
				return false
			}
			
			// The internal structure may differ, but the logical result should be the same
			
			// Get field names (excluding indices)
			bracketFields := bracketStructured.GetFieldNames()
			dotFields := dotStructured.GetFieldNames()
			
			if len(bracketFields) != len(dotFields) {
				return false
			}
			
			for i, field := range bracketFields {
				if field != dotFields[i] {
					return false
				}
			}
			
			// Get indices
			bracketIndices := bracketStructured.GetIndices()
			dotIndices := dotStructured.GetIndices()
			
			if len(bracketIndices) != len(dotIndices) {
				return false
			}
			
			for i, idx := range bracketIndices {
				if idx != dotIndices[i] {
					return false
				}
			}
			
			// Both should contain the expected field names
			expectedFields := []string{fieldName, subfield}
			if len(bracketFields) != len(expectedFields) {
				return false
			}
			
			for i, expected := range expectedFields {
				if bracketFields[i] != expected || dotFields[i] != expected {
					return false
				}
			}
			
			// Both should contain the expected index
			if len(bracketIndices) != 1 || len(dotIndices) != 1 {
				return false
			}
			
			if bracketIndices[0] != index || dotIndices[0] != index {
				return false
			}
			
			return true
		},
		genValidFieldName(),
		genArrayIndex(),
		genValidFieldName(),
	))

	properties.Property("nested array syntax equivalence", prop.ForAll(
		func(field1 string, index1 int, field2 string, index2 int, finalField string) bool {
			parser := NewEnhancedPathParser()
			
			// Create nested bracket syntax: field1[index1].field2[index2].finalField
			bracketPath := fmt.Sprintf("%s[%d].%s[%d].%s", field1, index1, field2, index2, finalField)
			
			// Create nested dot syntax: field1.index1.field2.index2.finalField
			dotPath := fmt.Sprintf("%s.%d.%s.%d.%s", field1, index1, field2, index2, finalField)
			
			// Parse both paths
			bracketStructured, err1 := parser.ParsePath(bracketPath)
			if err1 != nil {
				return false
			}
			
			dotStructured, err2 := parser.ParsePath(dotPath)
			if err2 != nil {
				return false
			}
			
			// Both should be valid
			if bracketStructured == nil || dotStructured == nil {
				return false
			}
			
			// Both should have array access
			if !bracketStructured.HasArrayAccess() || !dotStructured.HasArrayAccess() {
				return false
			}
			
			// Get field names and indices
			bracketFields := bracketStructured.GetFieldNames()
			dotFields := dotStructured.GetFieldNames()
			bracketIndices := bracketStructured.GetIndices()
			dotIndices := dotStructured.GetIndices()
			
			// Should have same number of fields and indices
			if len(bracketFields) != len(dotFields) || len(bracketIndices) != len(dotIndices) {
				return false
			}
			
			// Should have expected fields
			expectedFields := []string{field1, field2, finalField}
			if len(bracketFields) != len(expectedFields) {
				return false
			}
			
			for i, expected := range expectedFields {
				if bracketFields[i] != expected || dotFields[i] != expected {
					return false
				}
			}
			
			// Should have expected indices
			expectedIndices := []int{index1, index2}
			if len(bracketIndices) != len(expectedIndices) {
				return false
			}
			
			for i, expected := range expectedIndices {
				if bracketIndices[i] != expected || dotIndices[i] != expected {
					return false
				}
			}
			
			return true
		},
		genValidFieldName(),
		genArrayIndex(),
		genValidFieldName(),
		genArrayIndex(),
		genValidFieldName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Generators for path parsing property tests

func genValidFieldName() gopter.Gen {
	// Generate valid field names (start with letter/underscore, contain letters/numbers/underscores)
	return gen.OneConstOf(
		"field", "subfield", "config", "data", "value", "item", "element",
		"server", "client", "network", "storage", "compute", "worker",
		"_private", "meta_data", "config_value", "field_name",
	)
}

func genArrayIndex() gopter.Gen {
	// Generate valid array indices (non-negative integers)
	return gen.IntRange(0, 99) // Keep reasonable range for testing
}

func genSimplePath() gopter.Gen {
	// Generate simple paths without arrays for baseline testing
	return gen.SliceOfN(2, genValidFieldName()).Map(func(fields []string) string {
		return strings.Join(fields, ".")
	})
}

func genBracketPath() gopter.Gen {
	// Generate paths with bracket syntax
	return gopter.CombineGens(
		genValidFieldName(),
		genArrayIndex(),
		genValidFieldName(),
	).Map(func(values []interface{}) string {
		field := values[0].(string)
		index := values[1].(int)
		subfield := values[2].(string)
		return fmt.Sprintf("%s[%d].%s", field, index, subfield)
	})
}

func genDotPath() gopter.Gen {
	// Generate paths with dot syntax for arrays
	return gopter.CombineGens(
		genValidFieldName(),
		genArrayIndex(),
		genValidFieldName(),
	).Map(func(values []interface{}) string {
		field := values[0].(string)
		index := values[1].(int)
		subfield := values[2].(string)
		return fmt.Sprintf("%s.%d.%s", field, index, subfield)
	})
}