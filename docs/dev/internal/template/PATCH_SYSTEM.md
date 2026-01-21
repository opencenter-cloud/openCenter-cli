# Template Patch System

## Overview

The template patch system provides flexible, targeted modifications to template content through three core operations: **add**, **remove**, and **replace**. Each operation supports multiple path strategies for precise content manipulation.

## Operations

### 1. ADD Operation

Adds content at a specified location in the template.

**Path Strategies:**

- **Append to end**: Path `"."` or empty string
  ```go
  {Operation: "add", Path: ".", Value: "new content"}
  ```

- **Insert after line number**: Path `"line:N"` (0-indexed)
  ```go
  {Operation: "add", Path: "line:5", Value: "inserted content"}
  ```

- **Insert after pattern**: Path contains a search pattern
  ```go
  {Operation: "add", Path: "metadata:", Value: "  annotations:\n    key: value"}
  ```

**Examples:**

```go
// Append to end of template
patch := TemplatePatch{
    Operation: "add",
    Path:      ".",
    Value:     "footer: content",
}

// Insert after line 10
patch := TemplatePatch{
    Operation: "add",
    Path:      "line:10",
    Value:     "# New section",
}

// Insert after first line containing "labels:"
patch := TemplatePatch{
    Operation: "add",
    Path:      "labels:",
    Value:     "    version: 1.0.0",
}
```

### 2. REMOVE Operation

Removes content from the template.

**Path Strategies:**

- **Remove single line**: Path `"line:N"` (0-indexed)
  ```go
  {Operation: "remove", Path: "line:5"}
  ```

- **Remove line range**: Path `"lines:N-M"` (inclusive)
  ```go
  {Operation: "remove", Path: "lines:10-15"}
  ```

- **Remove by pattern**: Path contains a search pattern (removes all matching lines)
  ```go
  {Operation: "remove", Path: "debug:"}
  ```

**Examples:**

```go
// Remove line 5
patch := TemplatePatch{
    Operation: "remove",
    Path:      "line:5",
}

// Remove lines 10 through 15
patch := TemplatePatch{
    Operation: "remove",
    Path:      "lines:10-15",
}

// Remove all lines containing "debug:"
patch := TemplatePatch{
    Operation: "remove",
    Path:      "debug:",
}
```

### 3. REPLACE Operation

Replaces content at a specified location.

**Path Strategies:**

- **Replace line by number**: Path `"line:N"` (0-indexed)
  ```go
  {Operation: "replace", Path: "line:5", Value: "new content"}
  ```

- **Replace by pattern**: Path contains a search pattern (replaces first match)
  ```go
  {Operation: "replace", Path: "replicas:", Value: "5"}
  ```

- **YAML-aware replacement**: For YAML key-value lines, preserves indentation and key
  ```go
  {Operation: "replace", Path: "replicas", Value: "10"}
  // Transforms "  replicas: 3" to "  replicas: 10"
  ```

**Examples:**

```go
// Replace line 5
patch := TemplatePatch{
    Operation: "replace",
    Path:      "line:5",
    Value:     "replaced content",
}

// Replace first line containing "replicas:"
patch := TemplatePatch{
    Operation: "replace",
    Path:      "replicas",
    Value:     "10",
}
// Preserves YAML structure: "  replicas: 3" becomes "  replicas: 10"
```

## Conditional Patches

All patches support optional conditions that must be met for the patch to apply.

**Example:**

```go
patch := TemplatePatch{
    Operation: "add",
    Path:      ".",
    Value:     "debug: true",
    Condition: RenderCondition{
        Type:  ConditionTypeEquals,
        Field: "Environment",
        Value: "development",
    },
}
// Only applies in development environment
```

## Complete Example

```go
// Kubernetes deployment template with patches
composition := TemplateComposition{
    BaseTemplate: "deployment.yaml",
    Patches: []TemplatePatch{
        // Add a label
        {
            Operation: "add",
            Path:      "app: myapp",
            Value:     "        version: 1.0.0",
        },
        // Replace replica count
        {
            Operation: "replace",
            Path:      "replicas",
            Value:     "5",
        },
        // Add environment variables
        {
            Operation: "add",
            Path:      "image:",
            Value:     "        env:\n        - name: ENV\n          value: production",
        },
        // Remove debug configuration (conditional)
        {
            Operation: "remove",
            Path:      "debug:",
            Condition: RenderCondition{
                Type:  ConditionTypeEquals,
                Field: "Environment",
                Value: "production",
            },
        },
    },
}

result, err := composer.Compose(ctx, composition, data)
```

## Error Handling

The patch system provides detailed error messages:

- **Invalid line numbers**: Reports range violations
- **Pattern not found**: Indicates when remove/replace patterns don't match
- **Invalid path format**: Validates path syntax for line-based operations

**Example Error Messages:**

```
failed to apply add patch at line:999: line number 999 out of range (0-50)
failed to apply remove patch at nonexistent: no lines found matching path pattern: nonexistent
failed to apply replace patch at line:5: line number 5 out of range (0-3)
```

## Best Practices

1. **Use specific patterns**: Prefer unique patterns over generic ones to avoid unintended matches
2. **Test patches incrementally**: Apply patches one at a time during development
3. **Preserve structure**: For YAML/JSON, use pattern-based operations to maintain formatting
4. **Use conditions wisely**: Leverage conditional patches for environment-specific modifications
5. **Document intent**: Add comments explaining complex patch sequences

## Implementation Details

- **Line-based operations**: Use 0-indexed line numbers
- **Pattern matching**: Uses simple string containment (not regex)
- **YAML awareness**: Replace operation detects and preserves YAML key-value structure
- **Order matters**: Patches are applied sequentially in the order specified
- **Atomic operations**: Each patch either succeeds completely or fails with an error

## Testing

The patch system includes comprehensive tests covering:

- All three operations (add, remove, replace)
- All path strategies
- Conditional patches
- Edge cases (YAML structure, nested content, line ranges)
- Error conditions
- Multiple patches in sequence

See `internal/template/composition_test.go` for detailed examples.
