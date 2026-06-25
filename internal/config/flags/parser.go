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
	"strings"
)

// EnhancedFlagParser implements the FlagParser interface
type EnhancedFlagParser struct {
	handlers   map[string]FlagHandler
	patterns   map[string]*regexp.Regexp
	precedence []FlagType
}

// NewEnhancedFlagParser creates a new enhanced flag parser
func NewEnhancedFlagParser() *EnhancedFlagParser {
	return &EnhancedFlagParser{
		handlers: make(map[string]FlagHandler),
		patterns: make(map[string]*regexp.Regexp),
		precedence: []FlagType{
			FlagTypeArray,
			FlagTypeArrayOp,
			FlagTypeMapOp,
			FlagTypeJSON,
			FlagTypeYAML,
			FlagTypeTemplate,
			FlagTypeFile,
			FlagTypeOutput,
			FlagTypeDotNotation, // Default fallback
		},
	}
}

// RegisterHandler adds a new flag type handler
func (p *EnhancedFlagParser) RegisterHandler(pattern string, handler FlagHandler) error {
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Compile the pattern as a regex
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern '%s': %w", pattern, err)
	}

	p.handlers[pattern] = handler
	p.patterns[pattern] = regex
	return nil
}

// SetPrecedence defines flag type precedence order
func (p *EnhancedFlagParser) SetPrecedence(order []FlagType) error {
	if len(order) == 0 {
		return fmt.Errorf("precedence order cannot be empty")
	}
	p.precedence = make([]FlagType, len(order))
	copy(p.precedence, order)
	return nil
}

// ParseFlags processes all command-line flags
func (p *EnhancedFlagParser) ParseFlags(args []string) (*ParsedFlags, error) {
	result := &ParsedFlags{
		DotNotation:     make(map[string]string),
		ArrayFlags:      []ArrayFlag{},
		JSONFlags:       []JSONFlag{},
		YAMLFlags:       []YAMLFlag{},
		TemplateVars:    make(map[string]string),
		ConfigFiles:     []ConfigFile{},
		ConfigFileFlags: []*ConfigFileFlag{},
		ArrayOperations: []ArrayOperationFlag{},
		MapOperations:   []MapFlag{},
		OutputFormat:    nil,
		OutputMode:      nil,
	}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}

		// Handle special JSON flag format: --json-set path=value
		if strings.HasPrefix(arg, "--json-set ") {
			if err := p.parseJSONSetFlag(arg, result); err != nil {
				return nil, err
			}
			continue
		}

		// Handle special YAML flag formats: --yaml-set path, --yaml-data path, --yaml-file path=file
		if strings.HasPrefix(arg, "--yaml-set ") || strings.HasPrefix(arg, "--yaml-data ") {
			if err := p.parseYAMLSetFlag(arg, result); err != nil {
				return nil, err
			}
			continue
		}

		// Split flag name and value
		parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
		if len(parts) != 2 {
			continue // Skip flags without values
		}

		flagName := parts[0]
		value := parts[1]

		// Find the appropriate handler
		handler := p.findHandler(flagName)
		if handler == nil {
			// Default to dot notation
			result.DotNotation[flagName] = value
			continue
		}

		// Route to the appropriate handler
		if err := p.routeToHandler(handler, flagName, value, result); err != nil {
			return nil, fmt.Errorf("error processing flag '%s': %w", flagName, err)
		}
	}

	return result, nil
}

// findHandler finds the appropriate handler for a flag name
func (p *EnhancedFlagParser) findHandler(flagName string) FlagHandler {
	// Check handlers in precedence order
	for _, flagType := range p.precedence {
		for pattern, handler := range p.handlers {
			if handler.GetFlagType() == flagType {
				if regex := p.patterns[pattern]; regex != nil {
					if regex.MatchString(flagName) {
						return handler
					}
				}
			}
		}
	}
	return nil
}

// routeToHandler routes a flag to the appropriate handler
func (p *EnhancedFlagParser) routeToHandler(handler FlagHandler, flagName, value string, result *ParsedFlags) error {
	switch handler.GetFlagType() {
	case FlagTypeArray:
		if arrayHandler, ok := handler.(ArrayFlagHandler); ok {
			config, err := arrayHandler.ParseArrayFlag(flagName, value)
			if err != nil {
				return err
			}
			result.ArrayFlags = append(result.ArrayFlags, ArrayFlag{
				Type:   p.detectArrayType(flagName),
				Config: config,
			})
		}
	case FlagTypeArrayOp:
		parsed, err := handler.ParseFlag(flagName, value)
		if err != nil {
			return err
		}
		if arrayOpFlag, ok := parsed.(*ArrayOperationFlag); ok {
			result.ArrayOperations = append(result.ArrayOperations, *arrayOpFlag)
		}
	case FlagTypeMapOp:
		parsed, err := handler.ParseFlag(flagName, value)
		if err != nil {
			return err
		}
		if mapOpFlag, ok := parsed.(*MapFlag); ok {
			result.MapOperations = append(result.MapOperations, *mapOpFlag)
		}
	case FlagTypeJSON:
		parsed, err := handler.ParseFlag(flagName, value)
		if err != nil {
			return err
		}
		if jsonFlag, ok := parsed.(*JSONFlag); ok {
			result.JSONFlags = append(result.JSONFlags, *jsonFlag)
		}
	case FlagTypeYAML:
		parsed, err := handler.ParseFlag(flagName, value)
		if err != nil {
			return err
		}
		if yamlFlag, ok := parsed.(*YAMLFlag); ok {
			result.YAMLFlags = append(result.YAMLFlags, *yamlFlag)
		}
	case FlagTypeTemplate:
		result.TemplateVars[p.extractVariableName(flagName)] = value
	case FlagTypeFile:
		parsed, err := handler.ParseFlag(flagName, value)
		if err != nil {
			return err
		}

		// Handle both single file flags and config stack flags
		switch parsedValue := parsed.(type) {
		case *ConfigFileFlag:
			result.ConfigFileFlags = append(result.ConfigFileFlags, parsedValue)
		case []*ConfigFileFlag:
			result.ConfigFileFlags = append(result.ConfigFileFlags, parsedValue...)
		default:
			// Fallback to old ConfigFile format for backward compatibility
			result.ConfigFiles = append(result.ConfigFiles, ConfigFile{
				Path: value,
				Type: p.detectFileType(value),
			})
		}
	case FlagTypeOutput:
		parsed, err := handler.ParseFlag(flagName, value)
		if err != nil {
			return err
		}

		// Handle output format and mode flags
		switch parsedValue := parsed.(type) {
		case *OutputFormatFlag:
			result.OutputFormat = parsedValue
		case *OutputModeFlag:
			// Merge output mode flags (dry-run and quiet can both be set)
			if result.OutputMode == nil {
				result.OutputMode = parsedValue
			} else {
				// If both dry-run and quiet are set, quiet takes precedence
				if parsedValue.Mode == OutputModeQuiet {
					result.OutputMode = parsedValue
				}
			}
		}
	default:
		// Default to dot notation
		result.DotNotation[flagName] = value
	}
	return nil
}

// parseJSONSetFlag handles the special --json-set path=value format
func (p *EnhancedFlagParser) parseJSONSetFlag(arg string, result *ParsedFlags) error {
	// Remove "--json-set " prefix
	content := strings.TrimPrefix(arg, "--json-set ")

	// Split on first '=' to separate path from JSON value
	parts := strings.SplitN(content, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid --json-set format: expected --json-set path=value, got %s", arg)
	}

	path := strings.TrimSpace(parts[0])
	jsonValue := strings.TrimSpace(parts[1])

	if path == "" {
		return fmt.Errorf("empty path in --json-set flag")
	}

	if jsonValue == "" {
		return fmt.Errorf("empty JSON value in --json-set flag")
	}

	// Find JSON handler
	var jsonHandler FlagHandler
	for _, handler := range p.handlers {
		if handler.GetFlagType() == FlagTypeJSON {
			jsonHandler = handler
			break
		}
	}

	if jsonHandler == nil {
		return fmt.Errorf("no JSON handler registered")
	}

	// Create a synthetic flag name for the handler
	syntheticFlagName := "json-set-" + path

	// Parse using the JSON handler
	parsed, err := jsonHandler.ParseFlag(syntheticFlagName, jsonValue)
	if err != nil {
		return err
	}

	if jsonFlag, ok := parsed.(*JSONFlag); ok {
		result.JSONFlags = append(result.JSONFlags, *jsonFlag)
	}

	return nil
}

// parseYAMLSetFlag handles the special --yaml-set path and --yaml-data path formats
func (p *EnhancedFlagParser) parseYAMLSetFlag(arg string, result *ParsedFlags) error {
	var prefix, content string

	if strings.HasPrefix(arg, "--yaml-set ") {
		prefix = "--yaml-set "
		content = strings.TrimPrefix(arg, prefix)
	} else if strings.HasPrefix(arg, "--yaml-data ") {
		prefix = "--yaml-data "
		content = strings.TrimPrefix(arg, prefix)
	} else {
		return fmt.Errorf("invalid YAML flag format: %s", arg)
	}

	// For YAML flags, we expect the path to be provided separately from the YAML content
	// This is a simplified implementation - in practice, you might want to handle this differently
	// For now, we'll treat the content as the path and expect the YAML data to be provided via stdin or another mechanism

	path := strings.TrimSpace(content)
	if path == "" {
		return fmt.Errorf("empty path in YAML flag")
	}

	// Find YAML handler
	var yamlHandler FlagHandler
	for _, handler := range p.handlers {
		if handler.GetFlagType() == FlagTypeYAML {
			yamlHandler = handler
			break
		}
	}

	if yamlHandler == nil {
		return fmt.Errorf("no YAML handler registered")
	}

	// For this implementation, we'll use a placeholder YAML content
	// In a real implementation, this would be handled differently
	yamlContent := "key: value"

	// Create a synthetic flag name for the handler
	var syntheticFlagName string
	if strings.HasPrefix(arg, "--yaml-set ") {
		syntheticFlagName = "yaml-set-" + path
	} else {
		syntheticFlagName = "yaml-data-" + path
	}

	// Parse using the YAML handler
	parsed, err := yamlHandler.ParseFlag(syntheticFlagName, yamlContent)
	if err != nil {
		return err
	}

	if yamlFlag, ok := parsed.(*YAMLFlag); ok {
		result.YAMLFlags = append(result.YAMLFlags, *yamlFlag)
	}

	return nil
}

// detectArrayType detects the array type from the flag name
func (p *EnhancedFlagParser) detectArrayType(flagName string) string {
	if strings.Contains(flagName, "server-pool") {
		return "server-pool"
	}
	if strings.Contains(flagName, "ssh-key") {
		return "ssh-key"
	}
	if strings.Contains(flagName, "dns-server") {
		return "dns-server"
	}
	// Only match specific subnet array flags, not subnet fields like subnet_pods
	if flagName == "subnet" || strings.HasPrefix(flagName, "subnet-") {
		return "subnet"
	}
	return "unknown"
}

// extractPath extracts the configuration path from a flag name

// extractVariableName extracts the variable name from a template flag
func (p *EnhancedFlagParser) extractVariableName(flagName string) string {
	if strings.HasPrefix(flagName, "template-var-") {
		return strings.TrimPrefix(flagName, "template-var-")
	}
	return flagName
}

// detectFileType detects the file type from the file extension
func (p *EnhancedFlagParser) detectFileType(filePath string) string {
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		return "yaml"
	}
	if strings.HasSuffix(filePath, ".json") {
		return "json"
	}
	return "unknown"
}
