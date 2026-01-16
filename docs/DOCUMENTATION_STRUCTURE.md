# Documentation Structure

This document describes the organization of documentation in the openCenter repository.

## Overview

All documentation is consolidated in the `docs/` directory, following the [Diátaxis framework](https://diataxis.fr/) for clear organization by purpose.

## Directory Structure

```
docs/
├── readme.md                    # Documentation index and navigation
├── overview.md                  # Product overview
├── architecture.md              # Technical architecture
├── current-status.md            # Implementation status
├── reference/                   # Reference documentation
│   ├── cli-commands.md         # CLI command reference
│   ├── configuration.md        # Configuration reference
│   ├── shell-integration.md    # Shell integration guide
│   └── cluster/                # Cluster command details
├── dev/                        # Developer documentation
│   ├── readme.md               # Developer guide
│   ├── cluster/                # Cluster command internals
│   ├── internal/               # Internal package documentation
│   │   ├── config/            # Configuration system docs
│   │   ├── gitops/            # GitOps implementation docs
│   │   ├── services/          # Service management docs
│   │   ├── template/          # Template engine docs
│   │   └── testing/           # Testing infrastructure docs
│   ├── completed-tasks/        # Historical task completion records
│   └── testing/                # Testing documentation
│       ├── bdd-tests.md       # BDD test suite guide
│       └── sandbox-setup.md   # Test sandbox setup
├── providers/                  # Provider-specific documentation
│   ├── kubespray/             # Kubespray provider
│   └── talos/                 # Talos provider
│       └── implementation.md  # Talos implementation details
├── migration/                  # Migration guides
└── templates/                  # Documentation templates
```

## Documentation Categories

### User-Facing Documentation

Located in the root `docs/` directory and organized by the Diátaxis framework:

- **Tutorials** - Learning-oriented, step-by-step guides
- **How-To Guides** - Task-oriented, problem-solving guides
- **Reference** - Information-oriented, technical specifications
- **Explanation** - Understanding-oriented, conceptual information

### Developer Documentation

Located in `docs/dev/`:

- **Developer Guide** (`dev/readme.md`) - CLI architecture and implementation overview
- **Cluster Commands** (`dev/cluster/`) - Internal implementation of cluster commands
- **Internal Packages** (`dev/internal/`) - Detailed documentation for internal packages
- **Testing** (`dev/testing/`) - Testing infrastructure and practices
- **Completed Tasks** (`dev/completed-tasks/`) - Historical records of completed work

### Provider Documentation

Located in `docs/providers/`:

- Provider-specific implementation details
- Configuration guides
- Integration documentation

## Files Kept Outside docs/

Some documentation files remain outside the `docs/` directory for specific reasons:

### Root Directory
- `README.md` - Project entry point, must be in root for GitHub
- `AGENTS.md` - Repository guidelines for Kiro AI assistant

### Embedded Documentation
- `internal/gitops/templates/**/README.md` - Embedded in binary, part of generated GitOps repos
- `testdata/*/README.md` - Test fixture documentation, kept with test data

### Configuration
- `.github/ISSUE_TEMPLATE/*.md` - GitHub issue templates
- `.kiro/specs/**/*.md` - Kiro specification files
- `.kiro/steering/*.md` - Kiro steering rules

## Navigation

The main entry point is `docs/readme.md`, which provides:

- Quick links to key documents
- Organization by documentation type (Diátaxis)
- Organization by topic
- Organization by user role
- Common tasks index

## Maintenance

When adding new documentation:

1. Place it in the appropriate `docs/` subdirectory
2. Follow the Diátaxis framework for categorization
3. Update `docs/readme.md` with links to the new content
4. Use relative links for cross-references
5. Keep embedded documentation (templates, test fixtures) in place

## Historical Records

Task completion summaries and implementation records are archived in `docs/dev/completed-tasks/`. These provide:

- Historical context for implementation decisions
- Validation evidence for major changes
- Reference for future maintenance

Current development tasks are tracked in `.kiro/specs/`.
