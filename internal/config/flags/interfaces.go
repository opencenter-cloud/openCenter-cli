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
	FlagTypeArray       FlagType = "array"
	FlagTypeArrayOp     FlagType = "array_operation" // New type for array operations
	FlagTypeMapOp       FlagType = "map_operation"   // New type for map operations
	FlagTypeJSON        FlagType = "json"
	FlagTypeYAML        FlagType = "yaml"
	FlagTypeTemplate    FlagType = "template"
	FlagTypeFile        FlagType = "file"
	FlagTypeOutput      FlagType = "output"   // New type for output flags
	FlagTypeSecurity    FlagType = "security" // New type for security flags
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

// ArrayFlagHandler is a specialized handler for array-type flags
type ArrayFlagHandler interface {
	FlagHandler

	// ParseArrayFlag converts string to array configuration
	ParseArrayFlag(flagName, value string) (*ArrayConfig, error)
}

// ParsedFlag represents a parsed flag that can be merged into configuration
type ParsedFlag interface {
	// GetPath returns the configuration path this flag affects
	GetPath() string
}

// ParsedFlags contains all parsed flag information
type ParsedFlags struct {
	DotNotation     map[string]string
	ArrayFlags      []ArrayFlag
	JSONFlags       []JSONFlag
	YAMLFlags       []YAMLFlag
	TemplateVars    map[string]string
	ConfigFiles     []ConfigFile
	ConfigFileFlags []*ConfigFileFlag    // New configuration file flags with merge info
	ArrayOperations []ArrayOperationFlag // Array operations (append, insert, remove)
	MapOperations   []MapFlag            // Map operations (set, merge, remove)
	OutputFormat    *OutputFormatFlag    // Output format flag
	OutputMode      *OutputModeFlag      // Output mode flag
	SecurityFlags   []SecurityFlag       // Security-related flags
}

// ArrayFlag represents a parsed array flag (for dedicated handlers)
type ArrayFlag struct {
	Type   string
	Config *ArrayConfig
}

// ArrayConfig represents parsed array configuration
type ArrayConfig struct {
	Path   string
	Index  int
	Fields map[string]interface{}
	Type   string
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

// ConfigFileFlag represents a parsed configuration file flag with merge information
type ConfigFileFlag struct {
	Path      string
	Type      string
	Priority  int
	MergeType ConfigFileMergeType
}

// ConfigFileMergeType defines how configuration files should be merged
type ConfigFileMergeType string

const (
	ConfigFileMergeBase     ConfigFileMergeType = "base"     // Base configuration (lowest priority)
	ConfigFileMergeOverride ConfigFileMergeType = "override" // Override configuration (higher priority)
	ConfigFileMergeStack    ConfigFileMergeType = "stack"    // Stack configuration (priority by order)
)

// SecurityFlag represents a security-related flag
type SecurityFlag interface {
	GetSecurityType() SecurityFlagType
}

// SecurityFlagType represents the type of security flag
type SecurityFlagType string

const (
	SecurityFlagTypeSecureTemplateVar SecurityFlagType = "secure_template_var"
	SecurityFlagTypeMaskSensitive     SecurityFlagType = "mask_sensitive"
	SecurityFlagTypeSecurityWarnings  SecurityFlagType = "security_warnings"
	SecurityFlagTypeSOPSConfig        SecurityFlagType = "sops_config"
	SecurityFlagTypeEncryptedConfig   SecurityFlagType = "encrypted_config"
)

// SecureTemplateVarFlag represents a secure template variable flag
type SecureTemplateVarFlag struct {
	Key    string
	Value  string
	IsFile bool
}

func (f *SecureTemplateVarFlag) GetSecurityType() SecurityFlagType {
	return SecurityFlagTypeSecureTemplateVar
}

// MaskSensitiveFlag represents a flag to enable/disable sensitive data masking
type MaskSensitiveFlag struct {
	Enabled bool
}

func (f *MaskSensitiveFlag) GetSecurityType() SecurityFlagType {
	return SecurityFlagTypeMaskSensitive
}

// SecurityWarningsFlag represents a flag to enable/disable security warnings
type SecurityWarningsFlag struct {
	Enabled bool
}

func (f *SecurityWarningsFlag) GetSecurityType() SecurityFlagType {
	return SecurityFlagTypeSecurityWarnings
}

// SOPSConfigFlag represents a SOPS configuration flag
type SOPSConfigFlag struct {
	ConfigPath string
}

func (f *SOPSConfigFlag) GetSecurityType() SecurityFlagType {
	return SecurityFlagTypeSOPSConfig
}

// EncryptedConfigFlag represents an encrypted configuration file flag
type EncryptedConfigFlag struct {
	ConfigPath string
}

func (f *EncryptedConfigFlag) GetSecurityType() SecurityFlagType {
	return SecurityFlagTypeEncryptedConfig
}

// SecurityWarning represents a security warning
type SecurityWarning struct {
	Type       string
	Severity   string
	Message    string
	Field      string
	Suggestion string
}
