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
	"regexp"
	"strconv"
	"strings"
)

// PathParser handles different path syntax formats
type PathParser interface {
	// ParsePath converts path string to structured path
	ParsePath(path string) (*StructuredPath, error)
	
	// ValidatePath ensures path syntax is correct
	ValidatePath(path string) error
	
	// SupportedSyntax returns supported path syntax patterns
	SupportedSyntax() []string
}

// StructuredPath represents a parsed configuration path
type StructuredPath struct {
	Parts     []PathPart
	IsArray   bool
	HasIndex  bool
	RawPath   string
}

// PathPart represents a single part of a configuration path
type PathPart struct {
	Name     string
	Index    int
	IsArray  bool
	IsMap    bool
	HasIndex bool
}

// EnhancedPathParser implements PathParser with support for both bracket and dot syntax
type EnhancedPathParser struct {
	// Regex patterns for different syntax types
	bracketPattern *regexp.Regexp
	dotPattern     *regexp.Regexp
}

// NewEnhancedPathParser creates a new enhanced path parser
func NewEnhancedPathParser() *EnhancedPathParser {
	return &EnhancedPathParser{
		// Pattern to match field[index] syntax
		bracketPattern: regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\[(\d+)\]$`),
		// Pattern to match field.index syntax (where index is numeric)
		dotPattern: regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\.(\d+)$`),
	}
}

// ParsePath converts path string to structured path
func (p *EnhancedPathParser) ParsePath(path string) (*StructuredPath, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	
	// Split by dots first to handle nested paths
	parts := strings.Split(path, ".")
	var structuredParts []PathPart
	hasIndex := false
	isArray := false
	
	for i, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("empty path part at position %d in path '%s'", i, path)
		}
		
		pathPart, err := p.parseSinglePart(part)
		if err != nil {
			return nil, fmt.Errorf("invalid path part '%s' at position %d in path '%s': %w", part, i, path, err)
		}
		
		if pathPart.IsArray || pathPart.HasIndex {
			hasIndex = true
			isArray = true
		}
		
		structuredParts = append(structuredParts, pathPart)
	}
	
	return &StructuredPath{
		Parts:     structuredParts,
		IsArray:   isArray,
		HasIndex:  hasIndex,
		RawPath:   path,
	}, nil
}

// parseSinglePart parses a single part of the path (e.g., "field", "field[0]", "0")
func (p *EnhancedPathParser) parseSinglePart(part string) (PathPart, error) {
	// Check for bracket syntax: field[index]
	if matches := p.bracketPattern.FindStringSubmatch(part); matches != nil {
		fieldName := matches[1]
		indexStr := matches[2]
		
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return PathPart{}, fmt.Errorf("invalid array index '%s': %w", indexStr, err)
		}
		
		if index < 0 {
			return PathPart{}, fmt.Errorf("array index cannot be negative: %d", index)
		}
		
		return PathPart{
			Name:     fieldName,
			Index:    index,
			IsArray:  true,
			HasIndex: true,
		}, nil
	}
	
	// Check if this is just a numeric index (for dot syntax like field.0.subfield)
	if index, err := strconv.Atoi(part); err == nil {
		if index < 0 {
			return PathPart{}, fmt.Errorf("array index cannot be negative: %d", index)
		}
		
		return PathPart{
			Name:     "", // Empty name indicates this is just an index
			Index:    index,
			IsArray:  true,
			HasIndex: true,
		}, nil
	}
	
	// Regular field name
	if !isValidFieldName(part) {
		return PathPart{}, fmt.Errorf("invalid field name '%s': must start with letter or underscore and contain only letters, numbers, and underscores", part)
	}
	
	return PathPart{
		Name:     part,
		Index:    -1,
		IsArray:  false,
		HasIndex: false,
	}, nil
}

// ValidatePath ensures path syntax is correct
func (p *EnhancedPathParser) ValidatePath(path string) error {
	_, err := p.ParsePath(path)
	return err
}

// SupportedSyntax returns supported path syntax patterns
func (p *EnhancedPathParser) SupportedSyntax() []string {
	return []string{
		"field.subfield",           // Basic dot notation
		"field[0].subfield",        // Bracket syntax for arrays
		"field.0.subfield",         // Dot syntax for arrays (backward compatibility)
		"field[0].nested[1].value", // Nested array indexing
		"field.0.nested.1.value",   // Mixed syntax (backward compatibility)
	}
}

// isValidFieldName checks if a field name is valid (allows hyphens for map keys)
func isValidFieldName(name string) bool {
	if name == "" {
		return false
	}
	
	// Must start with letter or underscore
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}
	
	// Rest can be letters, numbers, underscores, or hyphens
	for i := 1; i < len(name); i++ {
		char := name[i]
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			return false
		}
	}
	
	return true
}

// String returns a string representation of the structured path
func (sp *StructuredPath) String() string {
	return sp.RawPath
}

// GetFieldNames returns all field names in the path (excluding indices)
func (sp *StructuredPath) GetFieldNames() []string {
	var names []string
	for _, part := range sp.Parts {
		if part.Name != "" {
			names = append(names, part.Name)
		}
	}
	return names
}

// GetIndices returns all array indices in the path
func (sp *StructuredPath) GetIndices() []int {
	var indices []int
	for _, part := range sp.Parts {
		if part.HasIndex {
			indices = append(indices, part.Index)
		}
	}
	return indices
}

// HasArrayAccess returns true if the path contains array access
func (sp *StructuredPath) HasArrayAccess() bool {
	return sp.HasIndex
}

// GetLastPart returns the last part of the path
func (sp *StructuredPath) GetLastPart() PathPart {
	if len(sp.Parts) == 0 {
		return PathPart{}
	}
	return sp.Parts[len(sp.Parts)-1]
}

// GetParentPath returns the path without the last part
func (sp *StructuredPath) GetParentPath() *StructuredPath {
	if len(sp.Parts) <= 1 {
		return nil
	}
	
	parentParts := make([]PathPart, len(sp.Parts)-1)
	copy(parentParts, sp.Parts[:len(sp.Parts)-1])
	
	hasIndex := false
	isArray := false
	for _, part := range parentParts {
		if part.HasIndex || part.IsArray {
			hasIndex = true
			isArray = true
		}
	}
	
	// Reconstruct the raw path for the parent
	var pathSegments []string
	for _, part := range parentParts {
		if part.Name != "" {
			if part.HasIndex {
				pathSegments = append(pathSegments, fmt.Sprintf("%s[%d]", part.Name, part.Index))
			} else {
				pathSegments = append(pathSegments, part.Name)
			}
		} else if part.HasIndex {
			// This is a numeric index part
			pathSegments = append(pathSegments, strconv.Itoa(part.Index))
		}
	}
	
	return &StructuredPath{
		Parts:     parentParts,
		IsArray:   isArray,
		HasIndex:  hasIndex,
		RawPath:   strings.Join(pathSegments, "."),
	}
}

// String returns a string representation of the path part
func (pp *PathPart) String() string {
	if pp.Name != "" {
		if pp.HasIndex {
			return fmt.Sprintf("%s[%d]", pp.Name, pp.Index)
		}
		return pp.Name
	} else if pp.HasIndex {
		return strconv.Itoa(pp.Index)
	}
	return ""
}