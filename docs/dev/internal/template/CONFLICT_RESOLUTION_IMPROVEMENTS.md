# Conflict Resolution Error Message Improvements

## Overview

This document describes the improvements made to conflict resolution error messages in the template composition system. The enhancements provide clear, actionable guidance to users when template conflicts are detected.

## Changes Made

### 1. Enhanced Duplicate Overlay Name Errors

**Before:**
```
duplicate overlay name my-overlay at positions 0 and 1
```

**After:**
```
duplicate overlay name detected: 'my-overlay' appears at positions 0 and 1
Conflict: Each overlay must have a unique name within a composition
Resolution: Rename one of the overlays to a unique name
Example: Change 'my-overlay' to 'my-overlay-v2' or 'my-overlay-alt'
Impact: Duplicate names prevent proper overlay ordering and application
```

### 2. Enhanced Provider Conflict Errors

**Before:**
```
overlay provider aws does not match base template provider openstack
```

**After:**
```
incompatible cloud providers detected
Conflict: Overlay 'aws-overlay' targets provider 'aws', but base template 'base-template' targets provider 'openstack'
Reason: Templates designed for different cloud providers have incompatible resource definitions
Resolution Options:
  1. Use an overlay designed for provider 'openstack'
  2. Use a base template designed for provider 'aws'
  3. Create a provider-agnostic overlay (remove provider specification)
Examples:
  - For OpenStack: Use overlays with provider='openstack'
  - For AWS: Use overlays with provider='aws'
  - For multi-provider: Use overlays with provider=''
Impact: Provider mismatches will result in invalid infrastructure configurations
```

### 3. Enhanced Type Mismatch Errors

**Before:**
```
overlay type infrastructure is not compatible with base template type service
```

**After:**
```
incompatible template types detected
Conflict: Overlay 'infra-overlay' has type 'infrastructure', but base template 'service-base' has type 'service'
Reason: Overlays should be of type 'overlay' or compatible with the base template type
Resolution Options:
  1. Change overlay type to 'overlay' in its template definition
  2. Use a base template of type 'infrastructure' instead
  3. Convert the overlay to match the base template type
Impact: Type mismatches may cause rendering failures or unexpected output
```

### 4. Enhanced Circular Dependency Errors

**Before:**
```
overlay has empty dependency
```

**After:**
```
invalid dependency in overlay 'overlay-name'
Conflict: Overlay has an empty dependency entry
Resolution: Remove the empty dependency or specify a valid template name
Impact: Empty dependencies indicate a configuration error
```

**Before:**
```
overlay my-overlay has circular dependency on itself
```

**After:**
```
circular dependency detected
Conflict: Overlay 'my-overlay' depends on itself
Reason: Circular dependencies create infinite loops during template resolution
Resolution: Remove the self-dependency from overlay 'my-overlay'
Impact: Circular dependencies will cause template rendering to fail
```

### 5. Enhanced Multi-Overlay Provider Conflicts

**Before:**
```
conflicting providers in overlays: openstack and aws at positions 0 and 1
```

**After:**
```
conflicting cloud providers detected in overlays
Conflict: Overlay 'openstack-overlay' (position 0) targets provider 'openstack', but overlay 'aws-overlay' (position 1) targets provider 'aws'
Reason: Templates designed for different cloud providers cannot be safely combined
Resolution Options:
  1. Remove one of the conflicting overlays from the composition
  2. Use provider-specific compositions (separate compositions for each provider)
  3. Create provider-agnostic overlays that work across all providers
Impact: Mixing provider-specific templates may result in invalid or incompatible configurations
```

## Error Message Structure

All enhanced error messages follow a consistent structure:

1. **Conflict**: Clear description of what is conflicting
2. **Reason**: Explanation of why this is a problem
3. **Resolution Options**: Numbered list of ways to fix the issue
4. **Examples** (when applicable): Concrete examples of correct usage
5. **Impact**: Description of what happens if the conflict is not resolved

## Benefits

1. **Clarity**: Users immediately understand what went wrong
2. **Actionability**: Multiple resolution options guide users to a fix
3. **Education**: Explanations help users understand the system better
4. **Consistency**: All error messages follow the same structure
5. **Debugging**: Detailed context helps troubleshoot complex issues

## Testing

All enhanced error messages are covered by tests in:
- `composition_test.go`: Unit tests for each conflict type
- `conflict_demo_test.go`: Demonstration of error messages in action

## Related Requirements

This implementation satisfies:
- **Task 3.2**: Template Composition System
- **Acceptance Criteria**: "Conflict resolution provides clear error messages"
- **Property 30**: Overlay Compatibility Validation (Requirements 7.3)

## Future Enhancements

Potential future improvements:
1. Add suggestions based on available templates in the registry
2. Include links to documentation for common issues
3. Add color-coded output for terminal display
4. Provide automated fix suggestions (e.g., "Run: opencenter template fix-conflict")
