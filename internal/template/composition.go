package template

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
)

// TemplateComposition represents a composition of a base template with overlays and patches
type TemplateComposition struct {
	BaseTemplate string
	Overlays     []TemplateOverlay
	Patches      []TemplatePatch
	Conditions   []RenderCondition
}

// TemplateOverlay represents an overlay that can be applied to a base template
type TemplateOverlay struct {
	Name       string
	Path       string
	Priority   int
	Conditions []RenderCondition
}

// TemplatePatch represents a targeted modification to a template
type TemplatePatch struct {
	Operation string      // add, remove, replace
	Path      string      // JSONPath to target
	Value     interface{} // New value
	Condition RenderCondition
}

// OverlayOrderingStrategy defines how overlays should be ordered
type OverlayOrderingStrategy string

const (
	// OrderByPriorityDesc orders overlays by priority (highest first), then by name
	OrderByPriorityDesc OverlayOrderingStrategy = "priority-desc"

	// OrderByPriorityAsc orders overlays by priority (lowest first), then by name
	OrderByPriorityAsc OverlayOrderingStrategy = "priority-asc"

	// OrderByName orders overlays alphabetically by name
	OrderByName OverlayOrderingStrategy = "name"

	// OrderByRegistration orders overlays in the order they were registered
	OrderByRegistration OverlayOrderingStrategy = "registration"
)

// OverlayOrderingConfig configures how overlays are ordered during composition
type OverlayOrderingConfig struct {
	Strategy OverlayOrderingStrategy
	// CustomSort allows providing a custom sorting function
	CustomSort func(overlays []TemplateOverlay) []TemplateOverlay
}

// DefaultOverlayOrderingConfig returns the default ordering configuration
func DefaultOverlayOrderingConfig() OverlayOrderingConfig {
	return OverlayOrderingConfig{
		Strategy: OrderByPriorityDesc,
	}
}

// TemplateComposer handles composition of templates with overlays and patches
type TemplateComposer interface {
	// Compose combines a base template with overlays and patches
	Compose(ctx context.Context, composition TemplateComposition, data interface{}) ([]byte, error)

	// ApplyOverlays applies overlays to a base template in priority order
	ApplyOverlays(baseContent string, overlays []TemplateOverlay, data interface{}) (string, error)

	// ApplyPatches applies patches to template content
	ApplyPatches(content string, patches []TemplatePatch, data interface{}) (string, error)

	// ValidateComposition validates that a composition is valid
	ValidateComposition(composition TemplateComposition) error

	// SetOrderingConfig sets the overlay ordering configuration
	SetOrderingConfig(config OverlayOrderingConfig)

	// GetOrderingConfig returns the current overlay ordering configuration
	GetOrderingConfig() OverlayOrderingConfig
}

// DefaultTemplateComposer is the default implementation of TemplateComposer
type DefaultTemplateComposer struct {
	engine         TemplateEngine
	registry       TemplateRegistry
	orderingConfig OverlayOrderingConfig
}

// NewDefaultTemplateComposer creates a new default template composer
func NewDefaultTemplateComposer(engine TemplateEngine, registry TemplateRegistry) *DefaultTemplateComposer {
	return &DefaultTemplateComposer{
		engine:         engine,
		registry:       registry,
		orderingConfig: DefaultOverlayOrderingConfig(),
	}
}

// SetOrderingConfig sets the overlay ordering configuration
func (c *DefaultTemplateComposer) SetOrderingConfig(config OverlayOrderingConfig) {
	c.orderingConfig = config
}

// GetOrderingConfig returns the current overlay ordering configuration
func (c *DefaultTemplateComposer) GetOrderingConfig() OverlayOrderingConfig {
	return c.orderingConfig
}

// Compose combines a base template with overlays and patches
// This implements Property 28: Overlay Application Correctness
// **Validates: Requirements 7.1**
func (c *DefaultTemplateComposer) Compose(ctx context.Context, composition TemplateComposition, data interface{}) ([]byte, error) {
	// Validate composition first
	if err := c.ValidateComposition(composition); err != nil {
		return nil, fmt.Errorf("invalid composition: %w", err)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("composition cancelled: %w", ctx.Err())
	default:
	}

	// Render base template
	baseContent, err := c.engine.Render(ctx, composition.BaseTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render base template %s: %w", composition.BaseTemplate, err)
	}

	// Apply overlays if any
	if len(composition.Overlays) > 0 {
		overlayedContent, err := c.ApplyOverlays(string(baseContent), composition.Overlays, data)
		if err != nil {
			return nil, fmt.Errorf("failed to apply overlays: %w", err)
		}
		baseContent = []byte(overlayedContent)
	}

	// Apply patches if any
	if len(composition.Patches) > 0 {
		patchedContent, err := c.ApplyPatches(string(baseContent), composition.Patches, data)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patches: %w", err)
		}
		baseContent = []byte(patchedContent)
	}

	return baseContent, nil
}

// ApplyOverlays applies overlays to a base template in priority order
// This implements Property 29: Deterministic Overlay Ordering
// **Validates: Requirements 7.2**
func (c *DefaultTemplateComposer) ApplyOverlays(baseContent string, overlays []TemplateOverlay, data interface{}) (string, error) {
	if len(overlays) == 0 {
		return baseContent, nil
	}

	// Sort overlays according to the configured strategy
	sortedOverlays := c.sortOverlays(overlays)

	// Apply each overlay in order
	result := baseContent
	for _, overlay := range sortedOverlays {
		// Check if overlay conditions are met
		if len(overlay.Conditions) > 0 {
			conditionsMet, err := c.evaluateConditions(overlay.Conditions, data)
			if err != nil {
				return "", fmt.Errorf("failed to evaluate conditions for overlay %s: %w", overlay.Name, err)
			}
			if !conditionsMet {
				continue // Skip this overlay
			}
		}

		// Render overlay template
		overlayContent, err := c.engine.Render(context.Background(), overlay.Path, data)
		if err != nil {
			return "", fmt.Errorf("failed to render overlay %s: %w", overlay.Name, err)
		}

		// Merge overlay with current result
		result, err = c.mergeTemplateContent(result, string(overlayContent))
		if err != nil {
			return "", fmt.Errorf("failed to merge overlay %s: %w", overlay.Name, err)
		}
	}

	return result, nil
}

// sortOverlays sorts overlays according to the configured ordering strategy
func (c *DefaultTemplateComposer) sortOverlays(overlays []TemplateOverlay) []TemplateOverlay {
	// Make a copy to avoid modifying the original slice
	sortedOverlays := make([]TemplateOverlay, len(overlays))
	copy(sortedOverlays, overlays)

	// If custom sort function is provided, use it
	if c.orderingConfig.CustomSort != nil {
		return c.orderingConfig.CustomSort(sortedOverlays)
	}

	// Apply the configured strategy
	switch c.orderingConfig.Strategy {
	case OrderByPriorityDesc:
		// Sort by priority (higher priority first), then by name for determinism
		sort.Slice(sortedOverlays, func(i, j int) bool {
			if sortedOverlays[i].Priority != sortedOverlays[j].Priority {
				return sortedOverlays[i].Priority > sortedOverlays[j].Priority
			}
			return sortedOverlays[i].Name < sortedOverlays[j].Name
		})

	case OrderByPriorityAsc:
		// Sort by priority (lower priority first), then by name for determinism
		sort.Slice(sortedOverlays, func(i, j int) bool {
			if sortedOverlays[i].Priority != sortedOverlays[j].Priority {
				return sortedOverlays[i].Priority < sortedOverlays[j].Priority
			}
			return sortedOverlays[i].Name < sortedOverlays[j].Name
		})

	case OrderByName:
		// Sort alphabetically by name
		sort.Slice(sortedOverlays, func(i, j int) bool {
			return sortedOverlays[i].Name < sortedOverlays[j].Name
		})

	case OrderByRegistration:
		// Keep original order (no sorting needed)
		// This is the order they were provided in the composition

	default:
		// Default to priority descending if strategy is unknown
		sort.Slice(sortedOverlays, func(i, j int) bool {
			if sortedOverlays[i].Priority != sortedOverlays[j].Priority {
				return sortedOverlays[i].Priority > sortedOverlays[j].Priority
			}
			return sortedOverlays[i].Name < sortedOverlays[j].Name
		})
	}

	return sortedOverlays
}

// ApplyPatches applies patches to template content
//
// Patch System Overview:
// The patch system supports three operations: add, remove, and replace.
// Each operation supports multiple path strategies for flexible content manipulation.
//
// Path Strategies:
//
// 1. ADD Operation:
//   - Path "." or empty: Append to end of content
//   - Path "line:N": Insert after line number N (0-indexed)
//   - Path "pattern": Insert after first line containing pattern
//   - Example: {Operation: "add", Path: "line:5", Value: "new content"}
//
// 2. REMOVE Operation:
//   - Path "line:N": Remove line number N (0-indexed)
//   - Path "lines:N-M": Remove line range from N to M (inclusive)
//   - Path "pattern": Remove all lines containing pattern
//   - Example: {Operation: "remove", Path: "lines:10-15"}
//
// 3. REPLACE Operation:
//   - Path "line:N": Replace line number N (0-indexed)
//   - Path "pattern": Replace first line containing pattern
//   - For YAML key-value lines, preserves indentation and key
//   - Example: {Operation: "replace", Path: "replicas", Value: "5"}
//
// Conditional Patches:
// All patches support optional conditions that must be met for the patch to apply.
// Example: {Operation: "add", Path: ".", Value: "debug: true", Condition: {Type: "equals", Field: "env", Value: "dev"}}
func (c *DefaultTemplateComposer) ApplyPatches(content string, patches []TemplatePatch, data interface{}) (string, error) {
	result := content

	for _, patch := range patches {
		// Check if patch condition is met
		if patch.Condition.Type != "" {
			conditionMet, err := c.evaluateCondition(patch.Condition, data)
			if err != nil {
				return "", fmt.Errorf("failed to evaluate patch condition: %w", err)
			}
			if !conditionMet {
				continue // Skip this patch
			}
		}

		// Apply patch based on operation
		var err error
		switch patch.Operation {
		case "add":
			result, err = c.applyAddPatch(result, patch)
		case "remove":
			result, err = c.applyRemovePatch(result, patch)
		case "replace":
			result, err = c.applyReplacePatch(result, patch)
		default:
			return "", fmt.Errorf("unknown patch operation: %s", patch.Operation)
		}

		if err != nil {
			return "", fmt.Errorf("failed to apply %s patch at %s: %w", patch.Operation, patch.Path, err)
		}
	}

	return result, nil
}

// ValidateComposition validates that a composition is valid
// This implements Property 30: Overlay Compatibility Validation
// **Validates: Requirements 7.3**
func (c *DefaultTemplateComposer) ValidateComposition(composition TemplateComposition) error {
	// Validate base template exists
	if composition.BaseTemplate == "" {
		return fmt.Errorf("base template is required")
	}

	var baseTemplateDef *TemplateDefinition

	// Check if base template exists in registry (only if registry is available)
	// Note: We allow file paths as well, so we only check registry if the template name doesn't look like a path
	if c.registry != nil && !strings.Contains(composition.BaseTemplate, "/") && !strings.Contains(composition.BaseTemplate, "\\") {
		baseDef, err := c.registry.GetTemplate(composition.BaseTemplate)
		if err != nil {
			return fmt.Errorf("base template %s not found: %w", composition.BaseTemplate, err)
		}
		baseTemplateDef = &baseDef
	}

	// Validate overlays
	for i, overlay := range composition.Overlays {
		if overlay.Name == "" {
			return fmt.Errorf("overlay %d: name is required", i)
		}
		if overlay.Path == "" {
			return fmt.Errorf("overlay %s: path is required", overlay.Name)
		}

		// Check if overlay template exists in registry (only if registry is available and name doesn't look like a path)
		var overlayDef *TemplateDefinition
		if c.registry != nil && !strings.Contains(overlay.Name, "/") && !strings.Contains(overlay.Name, "\\") {
			def, err := c.registry.GetTemplate(overlay.Name)
			if err != nil {
				// Don't fail if template not found in registry - it might be a file path
				// Just log or skip this check
			} else {
				overlayDef = &def
			}
		}

		// Validate overlay compatibility with base template
		if baseTemplateDef != nil && overlayDef != nil {
			if err := c.validateOverlayCompatibility(*baseTemplateDef, *overlayDef, overlay.Name); err != nil {
				return fmt.Errorf("overlay %s is incompatible with base template: %w", overlay.Name, err)
			}
		}

		// Validate overlay conditions
		for j, condition := range overlay.Conditions {
			if err := ValidateRenderCondition(condition); err != nil {
				return fmt.Errorf("overlay %s condition %d: %w", overlay.Name, j, err)
			}
		}
	}

	// Validate overlay conflicts with each other
	if err := c.validateOverlayConflicts(composition.Overlays); err != nil {
		return fmt.Errorf("overlay conflicts detected: %w", err)
	}

	// Validate patches
	for i, patch := range composition.Patches {
		if patch.Operation == "" {
			return fmt.Errorf("patch %d: operation is required", i)
		}
		if patch.Path == "" {
			return fmt.Errorf("patch %d: path is required", i)
		}

		// Validate operation type
		validOps := map[string]bool{"add": true, "remove": true, "replace": true}
		if !validOps[patch.Operation] {
			return fmt.Errorf("patch %d: invalid operation %s (must be add, remove, or replace)", i, patch.Operation)
		}

		// Validate patch condition if present
		if patch.Condition.Type != "" {
			if err := ValidateRenderCondition(patch.Condition); err != nil {
				return fmt.Errorf("patch %d condition: %w", i, err)
			}
		}
	}

	// Validate composition conditions
	for i, condition := range composition.Conditions {
		if err := ValidateRenderCondition(condition); err != nil {
			return fmt.Errorf("composition condition %d: %w", i, err)
		}
	}

	return nil
}

// validateOverlayCompatibility checks if an overlay is compatible with the base template
func (c *DefaultTemplateComposer) validateOverlayCompatibility(baseTemplate, overlay TemplateDefinition, overlayName string) error {
	// Check 1: Type compatibility - overlays should be of type overlay or compatible with base
	if overlay.Type != TemplateTypeOverlay && overlay.Type != "" {
		// Allow base templates to overlay other base templates, but warn about other types
		if overlay.Type != TemplateTypeBase && baseTemplate.Type != overlay.Type {
			return fmt.Errorf(
				"incompatible template types detected\n"+
					"Conflict: Overlay '%s' has type '%s', but base template '%s' has type '%s'\n"+
					"Reason: Overlays should be of type 'overlay' or compatible with the base template type\n"+
					"Resolution Options:\n"+
					"  1. Change overlay type to 'overlay' in its template definition\n"+
					"  2. Use a base template of type '%s' instead\n"+
					"  3. Convert the overlay to match the base template type\n"+
					"Impact: Type mismatches may cause rendering failures or unexpected output",
				overlayName, overlay.Type, baseTemplate.Name, baseTemplate.Type,
				overlay.Type,
			)
		}
	}

	// Check 2: Provider compatibility - if both specify providers, they must match
	if baseTemplate.Provider != "" && overlay.Provider != "" {
		if baseTemplate.Provider != overlay.Provider {
			return fmt.Errorf(
				"incompatible cloud providers detected\n"+
					"Conflict: Overlay '%s' targets provider '%s', but base template '%s' targets provider '%s'\n"+
					"Reason: Templates designed for different cloud providers have incompatible resource definitions\n"+
					"Resolution Options:\n"+
					"  1. Use an overlay designed for provider '%s'\n"+
					"  2. Use a base template designed for provider '%s'\n"+
					"  3. Create a provider-agnostic overlay (remove provider specification)\n"+
					"Examples:\n"+
					"  - For OpenStack: Use overlays with provider='openstack'\n"+
					"  - For AWS: Use overlays with provider='aws'\n"+
					"  - For multi-provider: Use overlays with provider=''\n"+
					"Impact: Provider mismatches will result in invalid infrastructure configurations",
				overlayName, overlay.Provider, baseTemplate.Name, baseTemplate.Provider,
				baseTemplate.Provider, overlay.Provider,
			)
		}
	}

	// Check 3: Dependency validation - overlay dependencies should be met
	if len(overlay.Dependencies) > 0 {
		// Check if overlay dependencies are satisfied
		// This would require checking against the registry or available templates
		// For now, we just validate that dependencies are specified correctly
		for _, dep := range overlay.Dependencies {
			if dep == "" {
				return fmt.Errorf(
					"invalid dependency in overlay '%s'\n"+
						"Conflict: Overlay has an empty dependency entry\n"+
						"Resolution: Remove the empty dependency or specify a valid template name\n"+
						"Impact: Empty dependencies indicate a configuration error",
					overlayName,
				)
			}
			// Check for circular dependencies
			if dep == overlay.Name {
				return fmt.Errorf(
					"circular dependency detected\n"+
						"Conflict: Overlay '%s' depends on itself\n"+
						"Reason: Circular dependencies create infinite loops during template resolution\n"+
						"Resolution: Remove the self-dependency from overlay '%s'\n"+
						"Impact: Circular dependencies will cause template rendering to fail",
					overlayName, overlayName,
				)
			}
			if dep == baseTemplate.Name {
				// This is OK - overlay can depend on base template
				continue
			}
		}
	}

	return nil
}

// validateOverlayConflicts checks for conflicts between overlays
func (c *DefaultTemplateComposer) validateOverlayConflicts(overlays []TemplateOverlay) error {
	if len(overlays) <= 1 {
		return nil // No conflicts possible with 0 or 1 overlay
	}

	// Check for duplicate overlay names
	namesSeen := make(map[string]int)
	for i, overlay := range overlays {
		if prevIdx, exists := namesSeen[overlay.Name]; exists {
			return fmt.Errorf(
				"duplicate overlay name detected: '%s' appears at positions %d and %d\n"+
					"Conflict: Each overlay must have a unique name within a composition\n"+
					"Resolution: Rename one of the overlays to a unique name\n"+
					"Example: Change '%s' to '%s-v2' or '%s-alt'\n"+
					"Impact: Duplicate names prevent proper overlay ordering and application",
				overlay.Name, prevIdx, i,
				overlay.Name, overlay.Name, overlay.Name,
			)
		}
		namesSeen[overlay.Name] = i
	}

	// If registry is available, check for provider conflicts
	if c.registry != nil {
		type overlayProviderInfo struct {
			name     string
			provider string
			position int
		}
		var overlayProviders []overlayProviderInfo

		for i, overlay := range overlays {
			// Try to get overlay definition from registry
			if !strings.Contains(overlay.Name, "/") && !strings.Contains(overlay.Name, "\\") {
				def, err := c.registry.GetTemplate(overlay.Name)
				if err == nil && def.Provider != "" {
					overlayProviders = append(overlayProviders, overlayProviderInfo{
						name:     overlay.Name,
						provider: def.Provider,
						position: i,
					})
				}
			}
		}

		// Check if all providers are compatible (all same or empty)
		if len(overlayProviders) > 1 {
			firstProvider := overlayProviders[0]
			for _, current := range overlayProviders[1:] {
				if current.provider != firstProvider.provider {
					return fmt.Errorf(
						"conflicting cloud providers detected in overlays\n"+
							"Conflict: Overlay '%s' (position %d) targets provider '%s', but overlay '%s' (position %d) targets provider '%s'\n"+
							"Reason: Templates designed for different cloud providers cannot be safely combined\n"+
							"Resolution Options:\n"+
							"  1. Remove one of the conflicting overlays from the composition\n"+
							"  2. Use provider-specific compositions (separate compositions for each provider)\n"+
							"  3. Create provider-agnostic overlays that work across all providers\n"+
							"Impact: Mixing provider-specific templates may result in invalid or incompatible configurations",
						firstProvider.name, firstProvider.position, firstProvider.provider,
						current.name, current.position, current.provider,
					)
				}
			}
		}
	}

	return nil
}

// evaluateConditions evaluates multiple conditions and returns true if all are met
func (c *DefaultTemplateComposer) evaluateConditions(conditions []RenderCondition, data interface{}) (bool, error) {
	for _, condition := range conditions {
		met, err := c.evaluateCondition(condition, data)
		if err != nil {
			return false, err
		}
		if !met {
			return false, nil
		}
	}
	return true, nil
}

// evaluateCondition evaluates a single condition against the data
func (c *DefaultTemplateComposer) evaluateCondition(condition RenderCondition, data interface{}) (bool, error) {
	// Extract field value from data
	fieldValue, err := extractFieldValue(data, condition.Field)
	if err != nil {
		return false, fmt.Errorf("failed to extract field %s: %w", condition.Field, err)
	}

	// Evaluate based on condition type
	switch condition.Type {
	case ConditionTypeEquals:
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", condition.Value), nil

	case ConditionTypeNotEquals:
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", condition.Value), nil

	case ConditionTypeContains:
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", condition.Value)
		return strings.Contains(fieldStr, valueStr), nil

	case ConditionTypeExists:
		return fieldValue != nil, nil

	case ConditionTypeGreaterThan:
		return compareValues(fieldValue, condition.Value) > 0, nil

	case ConditionTypeLessThan:
		return compareValues(fieldValue, condition.Value) < 0, nil

	default:
		return false, fmt.Errorf("unknown condition type: %s", condition.Type)
	}
}

// mergeTemplateContent merges overlay content with base content
// For now, this is a simple concatenation, but could be enhanced with more sophisticated merging
func (c *DefaultTemplateComposer) mergeTemplateContent(base, overlay string) (string, error) {
	// Simple merge strategy: append overlay to base
	// In a more sophisticated implementation, this could parse YAML/JSON and merge structures
	var buf bytes.Buffer
	buf.WriteString(base)
	if !strings.HasSuffix(base, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString("---\n") // YAML document separator
	buf.WriteString(overlay)
	return buf.String(), nil
}

// applyAddPatch adds content at the specified path
// Supports multiple strategies:
// 1. If path is empty or ".", append to end of content
// 2. If path starts with "line:", insert after specified line number
// 3. If path contains a key pattern, insert after matching key
// 4. Otherwise, append to end with path as comment
func (c *DefaultTemplateComposer) applyAddPatch(content string, patch TemplatePatch) (string, error) {
	valueStr := fmt.Sprintf("%v", patch.Value)

	// Strategy 1: Empty path or "." means append to end
	if patch.Path == "" || patch.Path == "." {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + valueStr + "\n", nil
	}

	// Strategy 2: Line number insertion (e.g., "line:5")
	if strings.HasPrefix(patch.Path, "line:") {
		lineNumStr := strings.TrimPrefix(patch.Path, "line:")
		var lineNum int
		if _, err := fmt.Sscanf(lineNumStr, "%d", &lineNum); err != nil {
			return "", fmt.Errorf("invalid line number in path %s: %w", patch.Path, err)
		}

		lines := strings.Split(content, "\n")
		if lineNum < 0 || lineNum > len(lines) {
			return "", fmt.Errorf("line number %d out of range (0-%d)", lineNum, len(lines))
		}

		// Insert after the specified line
		result := make([]string, 0, len(lines)+1)
		result = append(result, lines[:lineNum]...)
		result = append(result, valueStr)
		result = append(result, lines[lineNum:]...)
		return strings.Join(result, "\n"), nil
	}

	// Strategy 3: Key-based insertion (insert after first line containing the key)
	lines := strings.Split(content, "\n")
	inserted := false
	var result []string

	for i, line := range lines {
		result = append(result, line)
		if !inserted && strings.Contains(line, patch.Path) {
			// Insert after this line
			result = append(result, valueStr)
			inserted = true
			// Add remaining lines
			if i+1 < len(lines) {
				result = append(result, lines[i+1:]...)
			}
			break
		}
	}

	// If not inserted, append to end with comment
	if !inserted {
		if !strings.HasSuffix(content, "\n") {
			result = append(result, "")
		}
		result = append(result, fmt.Sprintf("# Added by patch at path: %s", patch.Path))
		result = append(result, valueStr)
	}

	return strings.Join(result, "\n"), nil
}

// applyRemovePatch removes content at the specified path
// Supports multiple strategies:
// 1. If path starts with "line:", remove specified line number
// 2. If path starts with "lines:", remove range of lines (e.g., "lines:5-10")
// 3. If path contains a key pattern, remove all lines containing that pattern
// 4. If path is a YAML key (e.g., "metadata.name"), remove that key and its value
func (c *DefaultTemplateComposer) applyRemovePatch(content string, patch TemplatePatch) (string, error) {
	lines := strings.Split(content, "\n")

	// Strategy 1: Single line removal (e.g., "line:5")
	if strings.HasPrefix(patch.Path, "line:") {
		lineNumStr := strings.TrimPrefix(patch.Path, "line:")
		var lineNum int
		if _, err := fmt.Sscanf(lineNumStr, "%d", &lineNum); err != nil {
			return "", fmt.Errorf("invalid line number in path %s: %w", patch.Path, err)
		}

		if lineNum < 0 || lineNum >= len(lines) {
			return "", fmt.Errorf("line number %d out of range (0-%d)", lineNum, len(lines)-1)
		}

		// Remove the specified line
		result := make([]string, 0, len(lines)-1)
		result = append(result, lines[:lineNum]...)
		if lineNum+1 < len(lines) {
			result = append(result, lines[lineNum+1:]...)
		}
		return strings.Join(result, "\n"), nil
	}

	// Strategy 2: Line range removal (e.g., "lines:5-10")
	if strings.HasPrefix(patch.Path, "lines:") {
		rangeStr := strings.TrimPrefix(patch.Path, "lines:")
		var startLine, endLine int
		if _, err := fmt.Sscanf(rangeStr, "%d-%d", &startLine, &endLine); err != nil {
			return "", fmt.Errorf("invalid line range in path %s: %w", patch.Path, err)
		}

		if startLine < 0 || endLine >= len(lines) || startLine > endLine {
			return "", fmt.Errorf("line range %d-%d out of range or invalid (0-%d)", startLine, endLine, len(lines)-1)
		}

		// Remove the specified range
		result := make([]string, 0, len(lines)-(endLine-startLine+1))
		result = append(result, lines[:startLine]...)
		if endLine+1 < len(lines) {
			result = append(result, lines[endLine+1:]...)
		}
		return strings.Join(result, "\n"), nil
	}

	// Strategy 3: Pattern-based removal (remove all lines containing the pattern)
	var result []string
	removed := false

	for _, line := range lines {
		if strings.Contains(line, patch.Path) {
			removed = true
			continue // Skip this line
		}
		result = append(result, line)
	}

	if !removed {
		return "", fmt.Errorf("no lines found matching path pattern: %s", patch.Path)
	}

	return strings.Join(result, "\n"), nil
}

// applyReplacePatch replaces content at the specified path
// Supports multiple strategies:
// 1. If path starts with "line:", replace specified line number
// 2. If path contains a key pattern, replace first matching line
// 3. If path is a YAML key pattern, replace the value for that key
func (c *DefaultTemplateComposer) applyReplacePatch(content string, patch TemplatePatch) (string, error) {
	valueStr := fmt.Sprintf("%v", patch.Value)
	lines := strings.Split(content, "\n")

	// Strategy 1: Line number replacement (e.g., "line:5")
	if strings.HasPrefix(patch.Path, "line:") {
		lineNumStr := strings.TrimPrefix(patch.Path, "line:")
		var lineNum int
		if _, err := fmt.Sscanf(lineNumStr, "%d", &lineNum); err != nil {
			return "", fmt.Errorf("invalid line number in path %s: %w", patch.Path, err)
		}

		if lineNum < 0 || lineNum >= len(lines) {
			return "", fmt.Errorf("line number %d out of range (0-%d)", lineNum, len(lines)-1)
		}

		// Replace the specified line
		lines[lineNum] = valueStr
		return strings.Join(lines, "\n"), nil
	}

	// Strategy 2: Pattern-based replacement (replace first matching line)
	replaced := false
	var result []string

	for _, line := range lines {
		if !replaced && strings.Contains(line, patch.Path) {
			// Check if this is a YAML key-value line
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && strings.Contains(parts[0], patch.Path) {
					// Preserve indentation and key, replace value
					indent := len(line) - len(strings.TrimLeft(line, " \t"))
					key := strings.TrimSpace(parts[0])
					result = append(result, fmt.Sprintf("%s%s: %s", strings.Repeat(" ", indent), key, valueStr))
					replaced = true
					continue
				}
			}
			// Otherwise, replace entire line
			result = append(result, valueStr)
			replaced = true
		} else {
			result = append(result, line)
		}
	}

	if !replaced {
		return "", fmt.Errorf("no lines found matching path pattern: %s", patch.Path)
	}

	return strings.Join(result, "\n"), nil
}

// extractFieldValue extracts a field value from data using dot notation
func extractFieldValue(data interface{}, fieldPath string) (interface{}, error) {
	// Handle map data
	if dataMap, ok := data.(map[string]interface{}); ok {
		parts := strings.Split(fieldPath, ".")
		current := interface{}(dataMap)

		for _, part := range parts {
			if currentMap, ok := current.(map[string]interface{}); ok {
				var exists bool
				current, exists = currentMap[part]
				if !exists {
					return nil, fmt.Errorf("field %s not found", part)
				}
			} else {
				return nil, fmt.Errorf("cannot traverse non-map value at %s", part)
			}
		}

		return current, nil
	}

	// For other types, use reflection or template execution
	// For now, return error for unsupported types
	return nil, fmt.Errorf("unsupported data type for field extraction")
}

// compareValues compares two values numerically
func compareValues(a, b interface{}) int {
	// Convert to float64 for comparison
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if !aOk || !bOk {
		// Fall back to string comparison
		aStr := fmt.Sprintf("%v", a)
		bStr := fmt.Sprintf("%v", b)
		return strings.Compare(aStr, bStr)
	}

	if aFloat < bFloat {
		return -1
	} else if aFloat > bFloat {
		return 1
	}
	return 0
}

// toFloat64 attempts to convert a value to float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// ComposeWithEngine is a convenience function that creates a composer and composes a template
func ComposeWithEngine(ctx context.Context, engine TemplateEngine, registry TemplateRegistry, composition TemplateComposition, data interface{}) ([]byte, error) {
	composer := NewDefaultTemplateComposer(engine, registry)
	return composer.Compose(ctx, composition, data)
}

// RenderComposedTemplate renders a template composition using the provided engine and registry
func RenderComposedTemplate(ctx context.Context, engine TemplateEngine, registry TemplateRegistry, baseTemplate string, overlays []TemplateOverlay, data interface{}) ([]byte, error) {
	composition := TemplateComposition{
		BaseTemplate: baseTemplate,
		Overlays:     overlays,
	}

	return ComposeWithEngine(ctx, engine, registry, composition, data)
}
