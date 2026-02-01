# SuggestionEngine Implementation

## Overview

The SuggestionEngine enhances validation results with helpful, context-aware suggestions to guide users in fixing configuration errors.

## Components

### 1. SuggestionEngine

Core engine that manages suggestion rules and enhances validation results.

**Key Features:**
- Pluggable rule architecture
- Automatic deduplication of suggestions
- Sorted output for consistency
- Thread-safe operations

**Usage:**
```go
engine := NewSuggestionEngine()
result := &ValidationResult{...}
context := map[string]interface{}{
    "valid_values": []string{"openstack", "aws", "gcp"},
}
engine.EnhanceResult(result, context)
```

### 2. TypoSuggestionRule

Detects typos using Levenshtein distance algorithm and suggests corrections.

**Features:**
- Levenshtein distance calculation (edit distance)
- Maximum distance threshold of 3
- Top 3 suggestions returned
- Case-insensitive matching

**Example:**
```
Input: "openstck"
Valid values: ["openstack", "aws", "gcp"]
Suggestion: "Did you mean \"openstack\"?"
```

### 3. ContextSuggestionRule

Generates context-aware suggestions based on field names and error codes.

**Field Pattern Matching:**
- `email` → "Ensure email is in format: user@example.com"
- `url/endpoint` → "Ensure URL includes protocol (http:// or https://)"
- `cidr/subnet` → "Use CIDR notation (e.g., 10.0.0.0/16)"
- `ip/address` → "Ensure IP address is in valid format (e.g., 192.168.1.1)"
- `port` → "Port must be between 1 and 65535"
- `name/cluster` → "Use alphanumeric characters, hyphens, and underscores only"
- `version` → "Use semantic versioning format (e.g., 1.2.3)"
- `count/size` → "Value must be a positive number"
- `enabled/enable` → "Value must be true or false"

**Error Code Mapping:**
- `E001` → "This field is required and cannot be empty"
- `E002` → "Check the allowed values for this field"
- `E003` → "Verify CIDR notation is correct"
- `E004` → "Verify IP address format"
- `E005` → "Ensure IP is within the specified subnet range"
- `E006` → "Check field dependencies and requirements"
- `E007` → "Verify value is within acceptable range"

### 4. Levenshtein Distance Algorithm

Calculates the minimum number of single-character edits (insertions, deletions, substitutions) required to change one string into another.

**Implementation:**
- Dynamic programming approach
- O(m*n) time complexity
- O(m*n) space complexity
- Used for typo detection

**Examples:**
- `levenshteinDistance("openstack", "openstck")` → 1 (missing 'a')
- `levenshteinDistance("kubernetes", "kuberntes")` → 1 (missing 'e')
- `levenshteinDistance("aws", "azs")` → 1 (substitution)

## Architecture

```
SuggestionEngine
├── rules []SuggestionRule
│   ├── TypoSuggestionRule
│   │   └── Uses Levenshtein distance
│   └── ContextSuggestionRule
│       ├── Field pattern matching
│       └── Error code mapping
└── EnhanceResult(result, context)
    └── For each issue:
        ├── Generate suggestions from all rules
        ├── Deduplicate suggestions
        └── Sort for consistency
```

## Integration with ValidationEngine

The SuggestionEngine is designed to work seamlessly with the ValidationEngine:

```go
// Create engines
validationEngine := NewValidationEngine()
suggestionEngine := NewSuggestionEngine()

// Perform validation
result, err := validationEngine.Validate(ctx, "cluster-name", clusterName)

// Enhance with suggestions
context := map[string]interface{}{
    "valid_values": []string{"openstack", "aws", "gcp"},
}
suggestionEngine.EnhanceResult(result, context)

// Display enhanced result
for _, err := range result.Errors {
    fmt.Printf("Error: %s\n", err.Message)
    for _, suggestion := range err.Suggestions {
        fmt.Printf("  Suggestion: %s\n", suggestion)
    }
}
```

## Testing

Comprehensive test coverage includes:

- **Unit Tests:**
  - `TestNewSuggestionEngine` - Engine creation
  - `TestSuggestionEngine_AddRule` - Rule registration
  - `TestSuggestionEngine_EnhanceResult` - Result enhancement
  - `TestTypoSuggestionRule_Generate` - Typo detection
  - `TestContextSuggestionRule_Generate` - Context suggestions
  - `TestLevenshteinDistance` - Distance calculation

- **Demo Tests:**
  - `TestSuggestionEngine_Demo` - Complete workflow demonstration
  - `TestLevenshteinDistance_Demo` - Algorithm demonstration

- **Example Tests:**
  - `ExampleSuggestionEngine_complete` - Usage example

**Coverage:** 87.5% of statements

## Performance

- **Typo Detection:** O(n*m*k) where n=valid values, m=avg string length, k=max distance
- **Context Suggestions:** O(1) - simple pattern matching
- **Enhancement:** O(r*i) where r=rules, i=issues
- **Memory:** Minimal - uses deduplication map

## Extensibility

Add custom suggestion rules by implementing the `SuggestionRule` interface:

```go
type CustomRule struct{}

func (r *CustomRule) Name() string {
    return "custom"
}

func (r *CustomRule) Generate(issue *ValidationIssue, context map[string]interface{}) []string {
    // Custom logic here
    return []string{"Custom suggestion"}
}

// Register the rule
engine := NewSuggestionEngine()
engine.AddRule(&CustomRule{})
```

## Future Enhancements

1. **Machine Learning:** Train on common errors to improve suggestions
2. **Contextual History:** Learn from user corrections
3. **Multi-language Support:** Suggestions in different languages
4. **Confidence Scores:** Rank suggestions by confidence
5. **Interactive Mode:** Allow users to select from suggestions
6. **Suggestion Templates:** Configurable suggestion templates
7. **Provider-Specific Rules:** Custom rules per cloud provider

## References

- [Levenshtein Distance Algorithm](https://en.wikipedia.org/wiki/Levenshtein_distance)
- [ValidationEngine Documentation](./README.md)
- [Validator Implementation Guide](./validators/README.md)
