---
id: glossary
title: "Glossary"
sidebar_label: Glossary
description: Definitions of terms and acronyms used throughout openCenter CLI documentation.
doc_type: reference
audience: "all users"
tags: [glossary, terminology, definitions]
---
# Glossary

**Purpose:** Definitions of terms used throughout openCenter CLI documentation.

## A

* ***Age***\
Modern encryption tool using public-key cryptography. openCenter uses Age for SOPS encryption. Simpler than GPG with no key servers or expiration by default.
* ***Ansible***\
Automation tool used by Kubespray to deploy Kubernetes. openCenter generates Ansible inventory files for cluster provisioning.
* ***Application Credential***\
OpenStack authentication method using credential ID and secret instead of username/password. Recommended for automation.

## B

* ***Bootstrap***\
Process of deploying a cluster from configuration. Includes infrastructure provisioning, Kubernetes deployment, and GitOps setup.
* ***BDD (Behavior-Driven Development)***\
Testing methodology using Gherkin scenarios. openCenter uses Godog for BDD tests in `tests/features/`.

## C

* ***Calico***\
Container Network Interface (CNI) plugin for Kubernetes networking. Default CNI in openCenter.
* ***CNI (Container Network Interface)***\
Standard for configuring network interfaces in Linux containers. Kubernetes uses CNI plugins like Calico, Cilium, or Flannel.
* ***Cobra***\
Go library for building CLI applications. openCenter uses Cobra for command structure and flag parsing.
* ***Control Plane***\
Kubernetes components that manage the cluster (API server, scheduler, controller manager, etcd). openCenter deploys 3 control plane nodes for high availability.

## D

* ***Diátaxis***\
Documentation framework organizing content into four types: Tutorials, How-To Guides, Reference, and Explanation. openCenter documentation follows Diátaxis.
* ***Drift Detection***\
Process of identifying differences between desired configuration (Git) and actual cluster state. openCenter provides drift detection commands.

## F

* ***FluxCD***\
GitOps tool that continuously reconciles cluster state with Git repository. openCenter generates FluxCD manifests for GitOps management.
* ***Flavor***\
OpenStack term for VM size (CPU, RAM, disk). Similar to AWS instance types or VMware VM templates.

## G

* ***GitOps***\
Operational model where Git is the single source of truth for infrastructure and applications. Changes are made via Git commits, not direct cluster access.
* ***Godog***\
Go implementation of Cucumber for BDD testing. openCenter uses Godog for feature tests.

## H

* ***HelmRelease***\
FluxCD custom resource that deploys Helm charts. openCenter generates HelmRelease manifests for platform services.

## K

* ***Kind (Kubernetes in Docker)***\
Tool for running local Kubernetes clusters using Docker containers. Used for development and testing.
* ***Kubespray***\
Ansible playbooks for deploying production-ready Kubernetes clusters. openCenter uses Kubespray for cluster provisioning.
* ***Kustomize***\
Kubernetes configuration management tool using overlays. openCenter uses Kustomize for cluster-specific customization.
* ***Kustomization***\
FluxCD custom resource that applies Kustomize overlays. openCenter generates Kustomization manifests for services.

## M

* ***Mise***\
Tool version manager and task runner. openCenter uses Mise for managing Go, kubectl, kind, helm versions and build tasks.

## O

* ***Octavia***\
OpenStack load balancer service. When disabled, openCenter uses VRRP for control plane high availability.
* ***OpenTofu***\
Open-source Terraform fork. openCenter supports both Terraform and OpenTofu for infrastructure provisioning.
* ***Overlay***\
Kustomize pattern for customizing base manifests. openCenter uses overlays for cluster-specific configuration.

## P

* ***Pod Security Admission***\
Kubernetes admission controller enforcing security policies on pods. openCenter configures Pod Security Admission via Kubespray.
* ***Preflight Check***\
Validation performed before deployment to catch issues early. openCenter provides preflight commands for connectivity, quotas, and provider constraints.
* ***Provider***\
Infrastructure platform for cluster deployment. openCenter’s GA infrastructure providers are OpenStack, VMware, Baremetal, and Kind.

## S

* ***SOPS (Secrets OPerationS)***\
Tool for encrypting files with Age or GPG keys. openCenter uses SOPS for secrets management in Git.
* ***Sprig***\
Template function library for Go templates. openCenter uses Sprig functions in templates for string manipulation, encoding, etc.

## T

* ***Terraform***\
Infrastructure as Code tool for provisioning cloud resources. openCenter generates Terraform configurations for cluster infrastructure.

## V

* ***VRRP (Virtual Router Redundancy Protocol)***\
Protocol for high availability of network gateways. openCenter uses VRRP for control plane HA when Octavia is disabled.
* ***vSphere***\
VMware virtualization platform. openCenter supports deploying clusters on vSphere with pre-provisioned VMs.

## W

* ***Worker Node***\
Kubernetes node that runs application workloads. openCenter deploys 2+ worker nodes by default.

---

## Acronyms

| Acronym | Full Term | Description |
| --- | --- | --- |
| ADR | Architecture Decision Record | Document explaining architectural choices |
| API | Application Programming Interface | Interface for software interaction |
| AWS | Amazon Web Services | Cloud computing platform |
| BDD | Behavior-Driven Development | Testing methodology |
| CIDR | Classless Inter-Domain Routing | IP address notation (e.g., 10.0.0.0/16) |
| CLI | Command-Line Interface | Text-based user interface |
| CNI | Container Network Interface | Kubernetes networking standard |
| CR | Custom Resource | Kubernetes API extension |
| CRD | Custom Resource Definition | Schema for Custom Resources |
| CSI | Container Storage Interface | Kubernetes storage standard |
| DI | Dependency Injection | Design pattern for managing dependencies |
| DNS | Domain Name System | Internet naming system |
| HA | High Availability | System design for minimal downtime |
| IAM | Identity and Access Management | Authentication and authorization |
| IaC | Infrastructure as Code | Managing infrastructure via code |
| JSON | JavaScript Object Notation | Data interchange format |
| K8s | Kubernetes | Container orchestration platform |
| OIDC | OpenID Connect | Authentication protocol |
| RBAC | Role-Based Access Control | Authorization model |
| SSH | Secure Shell | Encrypted network protocol |
| TLS | Transport Layer Security | Encryption protocol |
| VM | Virtual Machine | Virtualized computer |
| VRRP | Virtual Router Redundancy Protocol | HA protocol for routers |
| YAML | YAML Ain’t Markup Language | Human-readable data format |

---

**Evidence:**

* Technical terms from codebase: `internal/` packages
* Provider terminology: `internal/cloud/` implementations
* GitOps concepts: `internal/gitops/` and ecosystem.md
* Kubernetes concepts: Standard Kubernetes documentation
* Tool names: `.mise.toml`, `go.mod` dependencies