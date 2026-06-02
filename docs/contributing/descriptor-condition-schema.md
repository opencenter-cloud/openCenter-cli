---
id: descriptor-condition-schema
title: "Descriptor Condition Schema"
sidebar_label: Descriptor Condition Schema
description: Formal schema for overlay descriptor conditions used by the renderer to control conditional file inclusion.
doc_type: reference
audience: "developers, platform engineers"
tags: [descriptors, conditions, rendering, schema]
---
# Descriptor Condition Schema

**Purpose:** For developers, defines the formal schema for overlay descriptor conditions, including allowed operators, field path syntax, error handling, and the extension review process.

## Condition Structure

```yaml
when:
  field: <dotted.field.path>
  operator: <equals|exists|true|false>
  value: <string>  # required only for "equals"
```

A condition evaluates a single field from the typed `config.Config` against a simple predicate. Conditions appear on:

* `enabled_when` (descriptor-level): controls whether the entire descriptor is active
* `roots[].when` (root-level): controls whether a template root is expanded
* `files[].when` (file-level): controls whether an individual file is rendered

## Allowed Operators

| Operator | Semantics | `value` field | Behavior when field is absent |
| --- | --- | --- | --- |
| `equals` | `fmt.Sprint(fieldValue) == value` | required, non-empty | returns `false` |
| `exists` | field is present and non-nil in config | must be empty | returns `false` |
| `true` | field is a boolean and is `true` | must be empty | returns `false` |
| `false` | field is a boolean and is `false` | must be empty | returns `false` |

No logical combinators (AND, OR, NOT) are supported. Each condition is a single predicate. If a descriptor needs multiple conditions, split it into multiple descriptors or restructure the config to expose a single controlling field.

## Field Path Syntax

Field paths use dot-separated segments that resolve against `config.Config` via JSON/YAML struct tags.

```
opencenter.services.keycloak.enabled
opencenter.gitops.overlay_units.customer_managed.enabled
opencenter.infrastructure.provider
```

Resolution rules:

1. Start from the root `config.Config` struct.
2. For each segment, match against `json` tag, then `yaml` tag, then case-insensitive field name.
3. For struct fields, descend into the struct.
4. For map fields (`map[string]any`), use the segment as a map key.
5. If any segment fails to resolve, the field is considered absent.

## Validation

Field paths are validated at descriptor load time against a default `config.Config` instance using reflection (`internal/services/descriptors/loader.go:validateFieldPath`). This catches:

* typos in field names
* references to fields that don’t exist in the config struct
* malformed path syntax (must match `^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

Invalid conditions cause a load error. The renderer refuses to start with invalid descriptors. This is fail-closed behavior.

## Error Handling

| Error case | Behavior |
| --- | --- |
| Unsupported operator | Load error: descriptor rejected |
| Invalid field path syntax | Load error: descriptor rejected |
| Unknown field path (not in config struct) | Load error: descriptor rejected |
| `equals` with empty value | Load error: descriptor rejected |
| `exists`/`true`/`false` with non-empty value | Load error: descriptor rejected |
| Field absent at render time | Condition evaluates to `false` (not an error) |
| Field present but wrong type for `true`/`false` | Condition evaluates to `false` |

All error cases are fail-closed: invalid conditions prevent rendering, they never silently skip.

## Extension Review Process

Adding new operators or capabilities to the condition model requires:

1. A written proposal describing the new operator, its semantics, and why existing operators are insufficient.
2. An inventory of the specific per-service file variance that cannot be expressed with current operators.
3. Review and approval from the architecture owner.
4. Updates to this schema document, the `types.go` definition, the `loader.go` validator, and the `descriptor_renderer.go` evaluator.
5. Negative tests for the new operator’s error cases.

The bar for extension is intentionally high. The condition model is meant to stay simple. If a rendering decision requires complex logic, that logic belongs in Go code (typed config defaults, validation, or a dedicated rendering function), not in descriptor conditions.

## Implementation References

* Type definitions: `internal/services/descriptors/types.go` (`Condition`, `ConditionOperator`)
* Validation: `internal/services/descriptors/loader.go` (`validateCondition`, `validateFieldPath`)
* Evaluation: `internal/gitops/descriptor_renderer.go` (`evaluateDescriptorCondition`)
* Negative tests: `internal/services/descriptors/loader_test.go`