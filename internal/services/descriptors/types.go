package descriptors

// ConditionOperator defines the bounded set of allowed descriptor conditions.
type ConditionOperator string

const (
	ConditionOperatorEquals ConditionOperator = "equals"
	ConditionOperatorExists ConditionOperator = "exists"
	ConditionOperatorTrue   ConditionOperator = "true"
	ConditionOperatorFalse  ConditionOperator = "false"
)

// Condition applies a simple field-based predicate against typed config data.
type Condition struct {
	Field    string            `yaml:"field,omitempty" json:"field,omitempty"`
	Operator ConditionOperator `yaml:"operator,omitempty" json:"operator,omitempty"`
	Value    string            `yaml:"value,omitempty" json:"value,omitempty"`
}

// TemplateRoot expands every file beneath a template root into owned output.
type TemplateRoot struct {
	Path     string     `yaml:"path,omitempty" json:"path,omitempty"`
	Output   string     `yaml:"output,omitempty" json:"output,omitempty"`
	Excludes []string   `yaml:"excludes,omitempty" json:"excludes,omitempty"`
	When     *Condition `yaml:"when,omitempty" json:"when,omitempty"`
}

// File declares one explicitly owned template file.
type File struct {
	Template string     `yaml:"template,omitempty" json:"template,omitempty"`
	Output   string     `yaml:"output,omitempty" json:"output,omitempty"`
	Render   *bool      `yaml:"render,omitempty" json:"render,omitempty"`
	When     *Condition `yaml:"when,omitempty" json:"when,omitempty"`
}

// Descriptor defines one logical overlay unit and the renderer-owned files it
// is responsible for.
type Descriptor struct {
	Name             string         `yaml:"name,omitempty" json:"name,omitempty"`
	Layer            string         `yaml:"layer,omitempty" json:"layer,omitempty"`
	Service          string         `yaml:"service,omitempty" json:"service,omitempty"`
	ManagedService   string         `yaml:"managed_service,omitempty" json:"managed_service,omitempty"`
	AggregateTargets []string       `yaml:"aggregate_targets,omitempty" json:"aggregate_targets,omitempty"`
	EnabledWhen      *Condition     `yaml:"enabled_when,omitempty" json:"enabled_when,omitempty"`
	Roots            []TemplateRoot `yaml:"roots,omitempty" json:"roots,omitempty"`
	Files            []File         `yaml:"files,omitempty" json:"files,omitempty"`
}
