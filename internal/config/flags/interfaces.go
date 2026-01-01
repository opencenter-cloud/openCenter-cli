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

// FlagType represents the type of flag being processed
type FlagType string

const (
	FlagTypeDotNotation FlagType = "dot_notation"
	FlagTypeArray      FlagType = "array"
	FlagTypeArrayOp    FlagType = "array_operation"  // New type for array operations
	FlagTypeMapOp      FlagType = "map_operation"    // New type for map operations
	FlagTypeJSON       FlagType = "json"
	FlagTypeYAML       FlagType = "yaml"
	FlagTypeTemplate   FlagType = "template"
	FlagTypeFile       FlagType = "file"
)

// FlagParser handles all types of CLI flag parsing
type FlagParser interface {
	// ParseFlags processes all command-line flags
	ParseFlags(args []string) (*ParsedFlags, error)
	
	// RegisterHandler adds a new flag type handler
	RegisterHandler(pattern string, handler FlagHandler) error
	
	// SetPrecedence defines flag type precedence order
	SetPrecedence(order []FlagType) error
}

// FlagHandler processes specific types of flags
type FlagHandler interface {
	// CanHandle returns true if this handler can process the given flag
	CanHandle(flagName string) bool
	
	// ParseFlag processes a single flag and returns the parsed result
	ParseFlag(flagName, value string) (interface{}, error)
	
	// GetFlagType returns the type of flags this handler processes
	GetFlagType() FlagType
}

// ParsedFlag represents a parsed flag that can be merged into configuration
type ParsedFlag interface {
	// GetPath returns the configuration path this flag affects
	GetPath() string
}

// ConfigurationMergeable represents flags that can merge into configuration
type ConfigurationMergeable interface {
	ParsedFlag
	// MergeIntoConfiguration applies this flag to the configuration
	MergeIntoConfiguration(flag ParsedFlag, config map[string]interface{}) error
}

// ArrayFlagHandler processes array-specific flags (for dedicated handlers like server-pool)
type ArrayFlagHandler interface {
	FlagHandler
	
	// ParseArrayFlag converts string to array configuration
	ParseArrayFlag(flagName, value string) (*ArrayConfig, error)
	
	// SupportedTypes returns array types this handler supports
	SupportedTypes() []string
	
	// ValidateArrayConfig ensures array configuration is valid
	ValidateArrayConfig(config *ArrayConfig) error
}

// ParsedFlags contains all parsed flag information
type ParsedFlags struct {
	DotNotation      map[string]string
	ArrayFlags       []ArrayFlag
	JSONFlags        []JSONFlag
	YAMLFlags        []YAMLFlag
	TemplateVars     map[string]string
	ConfigFiles      []ConfigFile
	ArrayOperations  []ArrayOperationFlag  // Array operations (append, insert, remove)
	MapOperations    []MapFlag             // Map operations (set, merge, remove)
}

// ArrayFlag represents a parsed array flag (for dedicated handlers)
type ArrayFlag struct {
	Type   string
	Config *ArrayConfig
}

// ArrayConfig represents parsed array configuration
type ArrayConfig struct {
	Path     string
	Index    int
	Fields   map[string]interface{}
	Type     string
}

// JSONFlag represents a parsed JSON flag
type JSONFlag struct {
	Path  string
	Value interface{}
}

// YAMLFlag represents a parsed YAML flag
type YAMLFlag struct {
	Path  string
	Value interface{}
}

// ConfigFile represents a configuration file to be loaded
type ConfigFile struct {
	Path string
	Type string
}
