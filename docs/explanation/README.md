# Explanation Documentation


## Table of Contents

- [Purpose](#purpose)
- [What You'll Find Here](#what-youll-find-here)
- [How to Use This Section](#how-to-use-this-section)
- [Related Documentation](#related-documentation)
- [Contributing to Explanations](#contributing-to-explanations)
- [Document Status](#document-status)
**doc_type: explanation**

This section helps you understand how openCenter works and why it's designed the way it is. These documents explain concepts, architecture, and design decisions rather than teaching specific tasks.

## Purpose

Explanation documentation builds mental models. It covers the reasoning behind openCenter's architecture, the trade-offs in its design, and the concepts that tie everything together. Read these when you want to understand the "why" behind the system.

## What You'll Find Here

### Architecture and Design

**[Architecture Overview](architecture.md)**  
How openCenter components fit together. Covers the CLI layer, configuration system, provider adapters, GitOps scaffolding, and secrets management. Read this first if you want a complete picture of the system.

**[Configuration System](configuration-system.md)**  
Why openCenter uses a single YAML file as the source of truth. Explains schema validation, path resolution, defaults, and migration between versions.

**[Template Engine](template-engine.md)**  
How openCenter generates GitOps repositories from templates. Covers the Go template system, Sprig functions, embedded resources, and rendering pipeline.

**[Validation Pipeline](validation-pipeline.md)**  
The multi-layered validation approach: JSON schema, business rules, provider-specific checks, and connectivity tests. Explains why each layer exists and what it catches.

**[Plugin System](plugin-system.md)**  
How plugins extend openCenter with custom commands and providers. Covers the plugin interface, discovery mechanism, and isolation model.

### Workflows and Concepts

**[GitOps Workflow](gitops-workflow.md)**  
What GitOps means in openCenter's context. Explains repository structure, FluxCD/ArgoCD integration, and how configuration changes flow through Git.

**[Security Model](security-model.md)**  
How openCenter handles secrets, credentials, and sensitive data. Covers SOPS encryption, Age keys, SSH key generation, and the principle of least privilege.

**[Provider Comparison](provider-comparison.md)**  
When to use OpenStack, AWS, Kind, Talos, or Kubespray. Compares provisioning approaches, maturity levels, and operational characteristics.

### Reference and Planning

**[FAQ](faq.md)**  
Common questions about openCenter's design, capabilities, and limitations. Organized by topic: configuration, providers, secrets, GitOps, and troubleshooting.

**[Known Issues](known-issues.md)**  
Current limitations and workarounds. Includes provider-specific constraints, planned improvements, and migration paths.

**[Design Decisions](design-decisions.md)**  
Architecture Decision Records (ADRs) documenting key choices. Explains why openCenter uses Cobra, embeds templates, requires organization directories, and other structural decisions.

**[Roadmap](roadmap.md)**  
Planned features and improvements. Covers upcoming providers, configuration enhancements, and operational tooling.

## How to Use This Section

### If You're New to openCenter

Start with [Architecture Overview](architecture.md) to understand the big picture, then read [GitOps Workflow](gitops-workflow.md) and [Security Model](security-model.md) to grasp the core concepts.

### If You're Choosing a Provider

Read [Provider Comparison](provider-comparison.md) to understand the trade-offs, then check the provider-specific documentation in `docs/providers/`.

### If You're Debugging or Extending

Read [Configuration System](configuration-system.md), [Template Engine](template-engine.md), and [Validation Pipeline](validation-pipeline.md) to understand how data flows through the system.

### If You're Planning a Deployment

Check [Security Model](security-model.md) for compliance considerations, [Known Issues](known-issues.md) for current limitations, and [FAQ](faq.md) for common concerns.

## Related Documentation

### Learn by Doing
See [Tutorials](../tutorials/README.md) for hands-on walkthroughs that build confidence with openCenter.

### Solve Specific Problems
See [How-To Guides](../how-to/README.md) for task-focused instructions on common operations.

### Look Up Details
See [Reference](../reference/README.md) for complete technical specifications of commands, configuration, and APIs.

### Provider-Specific Context
See [Providers](../providers/README.md) for detailed documentation on OpenStack, AWS, Kind, Talos, and Kubespray.

### Operational Context
See [Operations](../operations/README.md) for disaster recovery, monitoring, and security operations.

## Contributing to Explanations

Explanation documents should:
- Focus on concepts and reasoning, not step-by-step instructions
- Explain trade-offs and alternatives, not just what openCenter does
- Address common misconceptions and clarify confusing aspects
- Link to related tutorials, how-tos, and reference docs
- Avoid prescriptive language ("you should", "you must")

See the [Contributing Guide](../dev/contributing.md) for documentation standards and review process.

## Document Status

| Document | Status | Priority |
|----------|--------|----------|
| architecture.md | 📝 Needs creation | P1 |
| gitops-workflow.md | ✅ Complete | P1 |
| security-model.md | 📝 Needs creation | P1 |
| provider-comparison.md | 📝 Needs creation | P2 |
| configuration-system.md | 📝 Needs creation | P2 |
| template-engine.md | 📝 Needs creation | P2 |
| validation-pipeline.md | 📝 Needs creation | P2 |
| faq.md | 📝 Needs creation | P2 |
| known-issues.md | 📝 Needs creation | P2 |
| plugin-system.md | 📝 Needs creation | P3 |
| design-decisions.md | 📝 Needs creation | P3 |
| roadmap.md | 📝 Needs creation | P3 |

**Legend**: P1 = Pre-release blocker, P2 = Release target, P3 = Post-release

---

**Last Updated**: January 19, 2026  
**openCenter Version**: 1.0.0
