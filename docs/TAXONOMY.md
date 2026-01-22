# openCenter Documentation Taxonomy 1.0.0


## Table of Contents

- [Purpose](#purpose)
- [Documentation Principles](#documentation-principles)
- [Complete Documentation Structure](#complete-documentation-structure)
- [Content Status Legend](#content-status-legend)
- [Priority Levels](#priority-levels)
- [Content Requirements by Type](#content-requirements-by-type)
- [Documentation Maintenance](#documentation-maintenance)
- [Migration Plan](#migration-plan)
- [Success Metrics](#success-metrics)
- [Tools and Automation](#tools-and-automation)
- [Appendix: File Naming Conventions](#appendix-file-naming-conventions)
- [Appendix: Cross-Referencing Guidelines](#appendix-cross-referencing-guidelines)
- [Version History](#version-history)
**doc_type: reference**

This document defines the complete documentation structure for openCenter 1.0.0 release. It serves as the master plan for documentation organization, content requirements, and maintenance guidelines.

## Purpose

This taxonomy ensures:
- Consistent documentation structure following Diátaxis framework
- Clear content ownership and maintenance responsibilities
- Comprehensive coverage of all features and use cases
- Easy navigation for all user personas
- Scalable structure for future growth

## Documentation Principles

### 1. Diátaxis Framework Compliance
All documentation must fit into one of four categories:
- **Tutorials**: Learning-oriented, hands-on guides
- **How-To Guides**: Task-oriented, problem-solving instructions
- **Reference**: Information-oriented, technical specifications
- **Explanation**: Understanding-oriented, conceptual background

### 2. User-Centric Organization
Content organized by:
- User role (operator, engineer, developer, security)
- Task type (setup, operation, troubleshooting, development)
- Provider (OpenStack, AWS, Kind, Talos)
- Feature area (cluster, secrets, GitOps, configuration)

### 3. Quality Standards
- Clear purpose statement in first paragraph
- `doc_type` metadata in first 60 lines
- Tested code examples
- Current version compatibility
- Regular review and updates

## Complete Documentation Structure

```
docs/
├── README.md                           # Main documentation index [EXISTS - UPDATED]
├── TAXONOMY.md                         # This file [NEW]
├── CONTENT_CHECKLIST.md               # Content audit and tracking [NEW]
│
├── tutorials/                          # Learning-oriented guides
│   ├── README.md                      # Tutorials index [NEW]
│   ├── getting-started.md             # 15-min first cluster [NEW - PRIORITY 1]
│   ├── openstack-deployment.md        # Production OpenStack [NEW - PRIORITY 1]
│   ├── kind-local-dev.md              # Local development setup [NEW - PRIORITY 2]
│   ├── multi-cluster.md               # Managing multiple clusters [NEW - PRIORITY 2]
│   ├── aws-deployment.md              # AWS production cluster [NEW - PRIORITY 3]
│   └── gitops-workflow.md             # GitOps workflow tutorial [NEW - PRIORITY 3]
│
├── how-to/                            # Task-oriented guides
│   ├── README.md                      # How-to index [NEW]
│   ├── troubleshooting.md             # Diagnostic procedures [EXISTS - NEEDS UPDATE]
│   ├── secrets-management.md          # SOPS workflows [RENAME from secrets.md]
│   ├── adding-services.md             # Service integration [EXISTS - NEEDS UPDATE]
│   ├── upgrading-clusters.md          # Upgrade procedures [NEW - PRIORITY 1]
│   ├── backup-recovery.md             # Backup and restore [NEW - PRIORITY 1]
│   ├── provider-setup.md              # Cloud provider configuration [NEW - PRIORITY 1]
│   ├── deploying-changes.md           # Deploy workflow [NEW - PRIORITY 2]
│   ├── monitoring.md                  # Monitoring setup [NEW - PRIORITY 2]
│   ├── custom-templates.md            # Template customization [NEW - PRIORITY 3]
│   ├── plugin-development.md          # Creating plugins [NEW - PRIORITY 3]
│   ├── cicd-integration.md            # CI/CD pipelines [NEW - PRIORITY 3]
│   ├── audit-compliance.md            # Compliance workflows [NEW - PRIORITY 3]
│   ├── migration.md                   # Migration procedures [NEW - PRIORITY 3]
│   └── ide-integration.md             # IDE setup [EXISTS - NEEDS UPDATE]
│
├── reference/                         # Information-oriented specs
│   ├── README.md                      # Reference index [NEW]
│   ├── cli-commands.md                # Complete CLI reference [EXISTS - NEEDS UPDATE]
│   ├── configuration.md               # Config schema [EXISTS - NEEDS UPDATE]
│   ├── api.md                         # Go package API [NEW - PRIORITY 2]
│   ├── error-codes.md                 # Error reference [NEW - PRIORITY 1]
│   ├── secrets.md                     # Secrets reference [NEW - PRIORITY 2]
│   ├── templates.md                   # Template reference [NEW - PRIORITY 2]
│   ├── environment-variables.md       # Env var reference [NEW - PRIORITY 2]
│   ├── file-formats.md                # File format specs [NEW - PRIORITY 3]
│   ├── shell-integration.md           # Shell completion [EXISTS - NEEDS UPDATE]
│   ├── glossary.md                    # Terms and definitions [NEW - PRIORITY 3]
│   └── cluster/                       # Cluster command details
│       ├── README.md                  # Cluster commands index [EXISTS - NEEDS UPDATE]
│       ├── init.md                    # cluster init [EXISTS - NEEDS UPDATE]
│       ├── validate.md                # cluster validate [EXISTS - NEEDS UPDATE]
│       ├── setup.md                   # cluster setup [EXISTS - NEEDS UPDATE]
│       ├── bootstrap.md               # cluster bootstrap [EXISTS - NEEDS UPDATE]
│       ├── list.md                    # cluster list [EXISTS - NEEDS UPDATE]
│       ├── select.md                  # cluster select [EXISTS - NEEDS UPDATE]
│       ├── current.md                 # cluster current [EXISTS - NEEDS UPDATE]
│       ├── info.md                    # cluster info [EXISTS - NEEDS UPDATE]
│       ├── edit.md                    # cluster edit [EXISTS - NEEDS UPDATE]
│       ├── render.md                  # cluster render [EXISTS - NEEDS UPDATE]
│       ├── schema.md                  # cluster schema [EXISTS - NEEDS UPDATE]
│       ├── update.md                  # cluster update [EXISTS - NEEDS UPDATE]
│       ├── migrate.md                 # cluster migrate [EXISTS - NEEDS UPDATE]
│       ├── preflight.md               # cluster preflight [EXISTS - NEEDS UPDATE]
│       └── destroy.md                 # cluster destroy [EXISTS - NEEDS UPDATE]
│
├── explanation/                       # Understanding-oriented docs
│   ├── README.md                      # Explanation index [NEW]
│   ├── architecture.md                # System architecture [NEW - PRIORITY 1]
│   ├── gitops-workflow.md             # GitOps concepts [NEW - PRIORITY 1]
│   ├── security-model.md              # Security architecture [NEW - PRIORITY 1]
│   ├── provider-comparison.md         # Provider selection guide [NEW - PRIORITY 2]
│   ├── configuration-system.md        # Config architecture [NEW - PRIORITY 2]
│   ├── template-engine.md             # Template system [NEW - PRIORITY 2]
│   ├── validation-pipeline.md         # Validation design [NEW - PRIORITY 2]
│   ├── plugin-system.md               # Plugin architecture [NEW - PRIORITY 3]
│   ├── faq.md                         # Frequently asked questions [NEW - PRIORITY 2]
│   ├── known-issues.md                # Current limitations [NEW - PRIORITY 2]
│   ├── design-decisions.md            # ADRs and rationale [NEW - PRIORITY 3]
│   └── roadmap.md                     # Future plans [NEW - PRIORITY 3]
│
├── providers/                         # Provider-specific documentation
│   ├── README.md                      # Providers overview [NEW]
│   ├── openstack/
│   │   ├── README.md                  # OpenStack overview [NEW]
│   │   ├── setup.md                   # Setup guide [NEW - PRIORITY 1]
│   │   ├── networking.md              # Network configuration [NEW - PRIORITY 2]
│   │   ├── troubleshooting.md         # OpenStack issues [NEW - PRIORITY 2]
│   │   ├── best-practices.md          # Production recommendations [NEW - PRIORITY 3]
│   │   └── readme.md                  # [EXISTS - CONSOLIDATE]
│   ├── aws/
│   │   ├── README.md                  # AWS overview [NEW]
│   │   ├── setup.md                   # Setup guide [NEW - PRIORITY 2]
│   │   ├── iam.md                     # IAM configuration [NEW - PRIORITY 2]
│   │   ├── vpc.md                     # VPC design [NEW - PRIORITY 3]
│   │   └── troubleshooting.md         # AWS issues [NEW - PRIORITY 3]
│   ├── kind/
│   │   ├── README.md                  # Kind overview [NEW]
│   │   ├── local-dev.md               # Development setup [NEW - PRIORITY 1]
│   │   ├── testing.md                 # Testing workflows [NEW - PRIORITY 2]
│   │   └── limitations.md             # Kind limitations [NEW - PRIORITY 3]
│   ├── talos/
│   │   ├── README.md                  # Talos overview [EXISTS - NEEDS UPDATE]
│   │   ├── architecture.md            # Talos architecture [EXISTS - NEEDS UPDATE]
│   │   ├── design.md                  # Design decisions [EXISTS - NEEDS UPDATE]
│   │   ├── implementation.md          # Implementation details [EXISTS - NEEDS UPDATE]
│   │   ├── requirements.md            # Requirements [EXISTS - NEEDS UPDATE]
│   │   └── readme.md                  # [EXISTS - CONSOLIDATE]
│   └── kubespray/
│       ├── README.md                  # Kubespray overview [NEW]
│       └── readme.md                  # [EXISTS - CONSOLIDATE]
│
├── operations/                        # Operational documentation
│   ├── README.md                      # Operations index [NEW]
│   ├── disaster-recovery.md           # DR procedures [EXISTS - NEEDS UPDATE]
│   ├── monitoring.md                  # Monitoring and observability [NEW - PRIORITY 2]
│   ├── security.md                    # Security operations [NEW - PRIORITY 2]
│   ├── capacity-planning.md           # Resource planning [NEW - PRIORITY 3]
│   ├── incident-response.md           # Incident handling [NEW - PRIORITY 3]
│   └── runbooks/                      # Operational runbooks
│       ├── README.md                  # Runbooks index [NEW]
│       ├── cluster-upgrade.md         # Upgrade runbook [NEW - PRIORITY 2]
│       ├── certificate-renewal.md     # Cert renewal [NEW - PRIORITY 3]
│       └── node-replacement.md        # Node replacement [NEW - PRIORITY 3]
│
└── dev/                               # Developer documentation
    ├── README.md                      # Developer guide [EXISTS - NEEDS UPDATE]
    ├── architecture.md                # Code architecture [NEW - PRIORITY 2]
    ├── contributing.md                # Contribution guide [NEW - PRIORITY 2]
    ├── release-process.md             # Release procedures [NEW - PRIORITY 2]
    ├── coding-standards.md            # Code style guide [NEW - PRIORITY 3]
    ├── configuration-system.md        # [EXISTS - MOVE TO explanation/]
    ├── dependency-injection.md        # [EXISTS - KEEP]
    ├── error-handling.md              # [EXISTS - KEEP]
    ├── feature-flag-logging.md        # [EXISTS - KEEP]
    ├── logging-migration.md           # [EXISTS - KEEP]
    ├── metrics-implementation.md      # [EXISTS - KEEP]
    ├── performance-characteristics.md # [EXISTS - KEEP]
    ├── performance-optimization-analysis.md # [EXISTS - KEEP]
    ├── validation-pipeline.md         # [EXISTS - MOVE TO explanation/]
    ├── cluster/
    │   ├── init.md                    # [EXISTS - KEEP]
    │   └── readme.md                  # [EXISTS - KEEP]
    ├── testing/
    │   ├── README.md                  # Testing guide [EXISTS - NEEDS UPDATE]
    │   ├── bdd-tests.md               # BDD testing [EXISTS - KEEP]
    │   └── sandbox-setup.md           # Test environment [EXISTS - KEEP]
    ├── internal/                      # Internal package docs
    │   ├── README.md                  # [EXISTS - KEEP]
    │   ├── config/                    # [EXISTS - KEEP]
    │   ├── gitops/                    # [EXISTS - KEEP]
    │   ├── services/                  # [EXISTS - KEEP]
    │   ├── template/                  # [EXISTS - KEEP]
    │   └── testing/                   # [EXISTS - KEEP]
    └── completed-tasks/               # Historical records
        └── README.md                  # [EXISTS - ARCHIVE]

```

## Content Status Legend

- **[NEW]**: Content needs to be created
- **[EXISTS]**: Content exists and is current
- **[NEEDS UPDATE]**: Content exists but requires updates for 1.0.0
- **[RENAME]**: File should be renamed for clarity
- **[CONSOLIDATE]**: Multiple files should be merged
- **[MOVE]**: Content should move to different location
- **[ARCHIVE]**: Content should be moved to archive
- **[KEEP]**: Content is current and correctly placed

## Priority Levels

### Priority 1 (Pre-Release Blockers)
Must be complete before 1.0.0 release:
- Getting Started tutorial
- OpenStack deployment tutorial
- Upgrading clusters how-to
- Backup and recovery how-to
- Provider setup how-to
- Error codes reference
- Architecture explanation
- GitOps workflow explanation
- Security model explanation
- OpenStack provider setup

### Priority 2 (Release Targets)
Should be complete for 1.0.0 release:
- Kind local dev tutorial
- Multi-cluster tutorial
- Deploying changes how-to
- Monitoring how-to
- API reference
- Secrets reference
- Templates reference
- Environment variables reference
- FAQ
- Known issues
- Provider comparison
- Configuration system explanation
- Template engine explanation
- Validation pipeline explanation

### Priority 3 (Post-Release)
Can be completed after 1.0.0 release:
- AWS deployment tutorial
- GitOps workflow tutorial
- Custom templates how-to
- Plugin development how-to
- CI/CD integration how-to
- Audit compliance how-to
- Migration how-to
- Glossary
- File formats reference
- Design decisions
- Roadmap
- Provider-specific best practices and advanced topics

## Content Requirements by Type

### Tutorials
Each tutorial must include:
- Clear learning outcome
- Time estimate
- Prerequisites list
- Step-by-step instructions
- Verification steps
- Next steps/further reading
- Tested on current version

### How-To Guides
Each how-to must include:
- Task summary (what problem it solves)
- Prerequisites
- Numbered steps
- Expected outcomes
- Troubleshooting section
- Related tasks

### Reference
Each reference doc must include:
- One-paragraph overview
- Complete specification
- Syntax/usage examples
- Parameter descriptions
- Return values/outputs
- Error conditions
- Version compatibility

### Explanation
Each explanation must include:
- Concept summary
- Why it works this way
- Design rationale
- Trade-offs and alternatives
- Common misconceptions
- Related concepts

## Documentation Maintenance

### Review Schedule
- **Tutorials**: Review every minor release
- **How-To Guides**: Review every minor release
- **Reference**: Update with every feature change
- **Explanation**: Review every major release

### Ownership
- **Product Team**: Tutorials, How-To Guides
- **Engineering Team**: Reference, Developer docs
- **Architecture Team**: Explanation, Design decisions
- **Operations Team**: Operations docs, Runbooks

### Quality Checks
Before marking content as complete:
1. ✅ Correct `doc_type` metadata
2. ✅ Clear purpose in first paragraph
3. ✅ All code examples tested
4. ✅ Links verified
5. ✅ Spelling and grammar checked
6. ✅ Follows Diátaxis principles
7. ✅ Peer reviewed
8. ✅ Version compatibility noted

## Migration Plan

### Phase 1: Structure (Week 1)
- Create all README.md index files
- Set up directory structure
- Create CONTENT_CHECKLIST.md

### Phase 2: Priority 1 Content (Weeks 2-4)
- Create all Priority 1 new content
- Update all Priority 1 existing content
- Review and test all examples

### Phase 3: Priority 2 Content (Weeks 5-7)
- Create all Priority 2 new content
- Update all Priority 2 existing content
- Cross-link related documents

### Phase 4: Cleanup (Week 8)
- Consolidate duplicate content
- Move misplaced content
- Archive historical content
- Final review and polish

### Phase 5: Priority 3 Content (Post-Release)
- Create remaining content
- Expand advanced topics
- Add community contributions

## Success Metrics

### Quantitative
- 100% of Priority 1 content complete
- 90% of Priority 2 content complete
- All code examples tested
- Zero broken links
- All docs have correct metadata

### Qualitative
- Users can find answers in < 2 minutes
- Reduced support tickets for documented features
- Positive community feedback
- Contributors can onboard using docs alone

## Tools and Automation

### Documentation Tools
- **Linting**: Vale for style checking
- **Link Checking**: markdown-link-check
- **Code Testing**: Extract and test code blocks
- **Spell Checking**: cspell
- **Format**: Prettier for markdown

### Automation
- Pre-commit hooks for linting
- CI checks for broken links
- Automated code example testing
- Version compatibility checks

## Appendix: File Naming Conventions

### General Rules
- Use lowercase with hyphens: `getting-started.md`
- Be descriptive: `openstack-deployment.md` not `os-deploy.md`
- Use consistent terminology across files
- Avoid abbreviations unless widely known

### Index Files
- Always named `README.md` (uppercase)
- Provide overview and navigation
- Link to all child documents

### Provider Files
- Prefix with provider name when ambiguous
- Use consistent structure across providers
- Keep provider-agnostic content in main docs

## Appendix: Cross-Referencing Guidelines

### Internal Links
- Use relative paths: `[text](../how-to/troubleshooting.md)`
- Link to specific sections: `[text](file.md#section-name)`
- Verify links in CI

### External Links
- Use full URLs for external resources
- Include link text that describes destination
- Check links regularly for rot

### Related Content
- Add "See Also" section at end of documents
- Link to related tutorials, how-tos, and explanations
- Create bidirectional links where appropriate

## Version History

- **1.0.0** (2026-01-19): Initial taxonomy for 1.0.0 release
