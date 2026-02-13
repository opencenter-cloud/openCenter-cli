# Temporary File Management

## Critical Rule: No Temporary Files in Repository

**NEVER create temporary tracking, summary, or progress files in the repository.**

This includes but is not limited to:
- Implementation summaries
- Progress tracking documents
- Sanitization reports
- Development notes
- Task completion summaries
- Any file ending in `_SUMMARY.md`, `_COMPLETE.md`, `_IMPLEMENTATION.md`

## Temporary File Location

If you need to create temporary tracking files during development, use:

```
~/.kiro/TEMP_TRACKING/
```

This directory is outside the repository and will not be committed.

## Examples of Prohibited Files

❌ **DO NOT CREATE:**
- `VMWARE_PROVIDER_SUMMARY.md`
- `VMWARE_IMPLEMENTATION_COMPLETE.md`
- `SANITIZATION_COMPLETE.md`
- `FEATURE_X_PROGRESS.md`
- `TASK_TRACKING.md`

✅ **INSTEAD CREATE:**
- `~/.kiro/TEMP_TRACKING/vmware-provider-summary.md`
- `~/.kiro/TEMP_TRACKING/implementation-notes.md`
- `~/.kiro/TEMP_TRACKING/progress-$(date +%Y%m%d).md`

## What Belongs in Repository

Only create files in the repository that are:
- Source code (`.go`, `.ts`, etc.)
- Tests (`*_test.go`, etc.)
- Documentation in `docs/` directory
- Configuration files (`.yaml`, `.toml`, etc.)
- Build scripts and tooling

## Rationale

Temporary tracking files:
- Clutter the repository
- Create noise in git history
- May contain sensitive information
- Are not useful to other developers
- Violate the principle of minimal repository content

## When You Need to Track Progress

If you need to maintain state across multiple interactions:
1. Create file in `~/.kiro/TEMP_TRACKING/`
2. Use descriptive names with dates if needed
3. Clean up when task is complete

## Exception: Legitimate Documentation

If implementation details are valuable for future reference, create proper documentation:
- Add to `docs/` directory with appropriate structure
- Follow documentation standards (TOC for >100 lines)
- Use proper file naming conventions
- Ensure content is sanitized and professional
