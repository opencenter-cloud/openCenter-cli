# Tutorials


## Table of Contents

- [Who These Are For](#who-these-are-for)
- [Available Tutorials](#available-tutorials)
- [Tutorial Structure](#tutorial-structure)
- [Prerequisites](#prerequisites)
- [Getting Help](#getting-help)
- [After Tutorials](#after-tutorials)
- [Contributing](#contributing)
**doc_type: tutorial**

Learn openCenter through hands-on, step-by-step guides. Each tutorial builds confidence by walking you through a complete workflow with a clear outcome.

## Who These Are For

Tutorials are for anyone learning openCenter. You don't need prior experience—just follow along and you'll have a working cluster by the end.

## Available Tutorials

### Getting Started (15 minutes)
**[Getting Started](getting-started.md)** - Deploy your first Kubernetes cluster

Your first openCenter experience. Initialize a cluster configuration, validate it, and understand the basic workflow. Perfect for evaluating openCenter.

**You'll learn:**
- Install openCenter
- Initialize a cluster configuration
- Validate configuration
- Understand the GitOps structure

### OpenStack Deployment (45 minutes)
**[OpenStack Deployment](openstack-deployment.md)** - Deploy a production cluster on OpenStack

Deploy a production-ready Kubernetes cluster on OpenStack. Configure networking, set up secrets management, and bootstrap the cluster.

**You'll learn:**
- Configure OpenStack provider
- Set up SOPS encryption
- Generate GitOps repository
- Bootstrap infrastructure
- Deploy Kubernetes

### Local Development with Kind (20 minutes)
**[Kind Local Development](kind-local-dev.md)** - Test configurations locally

Set up a local development environment using Kind. Test configurations and workflows before deploying to production.

**You'll learn:**
- Install Kind and dependencies
- Create local test cluster
- Test configuration changes
- Debug issues locally

### Multi-Cluster Management (30 minutes)
**[Multi-Cluster Management](multi-cluster.md)** - Manage multiple clusters

Learn to manage multiple Kubernetes clusters across different environments and providers using openCenter's organization structure.

**You'll learn:**
- Organize clusters by environment
- Switch between clusters
- Share configurations
- Manage secrets across clusters

### AWS Deployment (45 minutes)
**[AWS Deployment](aws-deployment.md)** - Deploy a production cluster on AWS

Deploy a production Kubernetes cluster on AWS with proper IAM configuration, VPC setup, and GitOps integration.

**You'll learn:**
- Configure AWS provider
- Set up IAM roles
- Design VPC networking
- Deploy to AWS

### GitOps Workflow (30 minutes)
**[GitOps Workflow](gitops-workflow.md)** - Master the GitOps workflow

Understand how openCenter generates GitOps repositories and how to work with FluxCD for continuous delivery.

**You'll learn:**
- GitOps repository structure
- FluxCD integration
- Deploy applications
- Manage updates through Git

## Tutorial Structure

Each tutorial follows this format:

1. **What You'll Build** - Clear outcome
2. **Prerequisites** - What you need before starting
3. **Step-by-Step Instructions** - Numbered steps with commands
4. **Verify Your Work** - Check that it worked
5. **What You Learned** - Recap key concepts
6. **Next Steps** - Where to go from here

## Prerequisites

Most tutorials assume:
- Basic command-line familiarity
- Text editor installed
- Internet connection
- Appropriate cloud provider access (for cloud tutorials)

Specific prerequisites are listed in each tutorial.

## Getting Help

If you get stuck:
1. Check the [Troubleshooting Guide](../how-to/troubleshooting.md)
2. Review the [FAQ](../explanation/faq.md)
3. Search [GitHub Issues](https://github.com/rackerlabs/openCenter-cli/issues)
4. Ask in [Discussions](https://github.com/rackerlabs/openCenter-cli/discussions)

## After Tutorials

Once you've completed tutorials, explore:
- **[How-To Guides](../how-to/README.md)** - Solve specific problems
- **[Reference](../reference/README.md)** - Look up technical details
- **[Explanation](../explanation/README.md)** - Understand concepts deeply

## Contributing

Found an issue in a tutorial? Have an idea for a new one? See our [Contributing Guide](../../contributing.md).
