# Glossary


## Table of Contents

- [A](#a)
- [B](#b)
- [C](#c)
- [D](#d)
- [E](#e)
- [F](#f)
- [G](#g)
- [H](#h)
- [I](#i)
- [J](#j)
- [K](#k)
- [L](#l)
- [M](#m)
- [N](#n)
- [O](#o)
- [P](#p)
- [R](#r)
- [S](#s)
- [T](#t)
- [V](#v)
- [W](#w)
- [Y](#y)
- [Acronyms Quick Reference](#acronyms-quick-reference)
- [See Also](#see-also)
**Document Type:** Reference  
**Audience:** All users  
**Purpose:** Comprehensive terminology reference for openCenter-cli

This glossary defines all terms, concepts, and acronyms used throughout the openCenter-cli project. Terms are organized alphabetically with cross-references where applicable.

---

## A

### Age
A modern encryption tool and format used by SOPS for encrypting secrets. Age keys consist of a public/private key pair where the public key is used for encryption and the private key for decryption.

**Related:** [SOPS](#sops), [Encryption](#encryption)

### Ansible
Configuration management tool used by the Kubespray provider for deploying Kubernetes clusters. openCenter-cli generates Ansible inventory and playbooks for cluster provisioning.

**Related:** [Kubespray](#kubespray), [Provisioning](#provisioning)

### Application Credential
OpenStack authentication method that provides scoped, revocable credentials without requiring password storage. Preferred over username/password authentication.

**Related:** [OpenStack](#openstack), [Authentication](#authentication)

### ArgoCD
GitOps continuous delivery tool for Kubernetes. Alternative to FluxCD for managing cluster applications.

**Related:** [GitOps](#gitops), [FluxCD](#fluxcd)

---

## B

### Barbican
OpenStack Key Manager service for storing and managing secrets, encryption keys, and certificates. Used by openCenter-cli for secure secret storage in OpenStack environments.

**Related:** [OpenStack](#openstack), [Secrets Management](#secrets-management)

### Baremetal
Infrastructure provider type for deploying Kubernetes on physical servers without virtualization. Requires pre-configured nodes with IP addresses.

**Related:** [Provider](#provider), [Infrastructure](#infrastructure)

### Bastion Host
Jump server that provides secure access to cluster nodes. Acts as an intermediary for SSH connections to control plane and worker nodes.

**Related:** [SSH](#ssh), [Security](#security)

### BDD (Behavior-Driven Development)
Testing methodology using Gherkin scenarios. openCenter-cli uses Godog for BDD tests in the `tests/features/` directory.

**Related:** [Testing](#testing), [Gherkin](#gherkin)

### BFV (Boot From Volume)
OpenStack feature that boots instances from Cinder block storage volumes instead of ephemeral disks. Provides persistent storage and snapshot capabilities.

**Related:** [Cinder](#cinder), [Storage](#storage)

### Bootstrap
Process of initializing a Kubernetes cluster with GitOps tooling (FluxCD) and connecting it to the GitOps repository. Executed via `openCenter cluster bootstrap`.

**Related:** [GitOps](#gitops), [FluxCD](#fluxcd)

---

## C

### Calico
Container Network Interface (CNI) plugin providing network policy and security for Kubernetes. One of three CNI options supported by openCenter-cli.

**Related:** [CNI](#cni), [Cilium](#cilium), [Kube-OVN](#kube-ovn)

### CCM (Cloud Controller Manager)
Kubernetes component that integrates with cloud provider APIs for load balancers, storage, and networking. openCenter-cli deploys provider-specific CCMs.

**Related:** [OpenStack CCM](#openstack-ccm), [Cloud Provider](#cloud-provider)

### Cert-Manager
Kubernetes add-on for automated TLS certificate management using Let's Encrypt or other ACME providers. Requires AWS Route53 credentials for DNS validation.

**Related:** [TLS](#tls), [ACME](#acme)

### CIDR (Classless Inter-Domain Routing)
IP address range notation (e.g., `10.42.0.0/16`). Used for pod networks, service networks, and access control lists.

**Related:** [Networking](#networking), [Subnet](#subnet)

### Cilium
eBPF-based CNI plugin providing advanced networking, security, and observability. Supports kube-proxy replacement and service mesh features.

**Related:** [CNI](#cni), [eBPF](#ebpf)

### Cinder
OpenStack Block Storage service providing persistent volumes for instances. Used for boot volumes and additional block devices.

**Related:** [OpenStack](#openstack), [BFV](#bfv-boot-from-volume)

### CLI Config
User-level configuration file (`~/.config/openCenter/config.yaml`) controlling CLI behavior, logging, and default settings. Separate from cluster configurations.

**Related:** [Configuration](#configuration), [Config Manager](#config-manager)

### Cluster
A Kubernetes cluster managed by openCenter-cli. Each cluster has a unique name and configuration file stored in an organization-based directory structure.

**Related:** [Organization](#organization), [Configuration](#configuration)

### Cluster Configuration
YAML file (`.{cluster}-config.yaml`) defining all aspects of a Kubernetes cluster including infrastructure, networking, services, and secrets.

**Related:** [YAML](#yaml), [Schema](#schema)

### CNI (Container Network Interface)
Plugin specification for configuring network interfaces in containers. openCenter-cli supports Calico, Cilium, and Kube-OVN.

**Related:** [Calico](#calico), [Cilium](#cilium), [Kube-OVN](#kube-ovn)

### Cobra
Go library for building CLI applications. Provides command structure, flag parsing, and help generation for openCenter-cli.

**Related:** [CLI](#cli), [Commands](#commands)

### Config Manager
Internal component responsible for loading, validating, and managing CLI and cluster configurations. Handles path resolution and organization-based multi-tenancy.

**Related:** [Configuration](#configuration), [Organization](#organization)

### Configuration
Declarative YAML specification of cluster infrastructure, networking, services, and secrets. Serves as the single source of truth for cluster state.

**Related:** [YAML](#yaml), [Schema](#schema)

### Control Plane
Kubernetes master nodes running the API server, scheduler, controller manager, and etcd. Minimum 1 node, recommended 3 for high availability.

**Related:** [Master Node](#master-node), [etcd](#etcd)

### CSI (Container Storage Interface)
Plugin specification for storage providers in Kubernetes. openCenter-cli deploys OpenStack Cinder CSI and optionally vSphere CSI.

**Related:** [Cinder](#cinder), [Storage](#storage)

---

## D

### Deployment
Configuration section controlling automated deployment behavior. Includes `auto_deploy` flag for automatic infrastructure provisioning.

**Related:** [Bootstrap](#bootstrap), [Provisioning](#provisioning)

### Designate
OpenStack DNS service for managing DNS zones and records. Used for automated DNS configuration in cluster deployments.

**Related:** [OpenStack](#openstack), [DNS](#dns)

### Drift Detection
Process of comparing actual cluster state against the declared configuration to identify unmanaged changes. Executed via `openCenter cluster drift`.

**Related:** [GitOps](#gitops), [Validation](#validation)

### Dry Run
Execution mode that prints planned actions without making changes. Enabled via `--dry-run` flag or CLI configuration.

**Related:** [Validation](#validation), [Preflight](#preflight)

---

## E

### eBPF (Extended Berkeley Packet Filter)
Linux kernel technology enabling programmable packet processing. Used by Cilium for high-performance networking.

**Related:** [Cilium](#cilium), [CNI](#cni)

### Encryption
Process of securing sensitive data using cryptographic algorithms. openCenter-cli uses SOPS with Age encryption for secrets.

**Related:** [SOPS](#sops), [Age](#age), [Secrets](#secrets)

### etcd
Distributed key-value store used by Kubernetes for cluster state. Requires regular backups for disaster recovery.

**Related:** [Control Plane](#control-plane), [Backup](#backup)

### etcd-backup
Service for automated etcd backups to S3-compatible storage. Configured in the `services` section.

**Related:** [etcd](#etcd), [S3](#s3)

---

## F

### Flavor
OpenStack instance type defining CPU, memory, and disk resources. Specified separately for bastion, master, and worker nodes.

**Related:** [OpenStack](#openstack), [Instance](#instance)

### FluxCD
GitOps continuous delivery tool for Kubernetes. Automatically synchronizes cluster state with Git repository contents.

**Related:** [GitOps](#gitops), [Reconciliation](#reconciliation)

### FQDN (Fully Qualified Domain Name)
Complete domain name including hostname and domain (e.g., `my-cluster.sjc3.k8s.opencenter.cloud`). Used for cluster API endpoints and services.

**Related:** [DNS](#dns), [Domain](#domain)

---

## G

### Gateway API
Kubernetes API for configuring ingress and service mesh routing. Successor to Ingress API with enhanced capabilities.

**Related:** [Ingress](#ingress), [Service Mesh](#service-mesh)

### Gherkin
Language for writing BDD test scenarios using Given-When-Then syntax. Used in `tests/features/*.feature` files.

**Related:** [BDD](#bdd-behavior-driven-development), [Testing](#testing)

### GitOps
Operational model using Git as the single source of truth for declarative infrastructure and applications. Changes are made via Git commits and automatically applied.

**Related:** [FluxCD](#fluxcd), [ArgoCD](#argocd)

### GitOps Base Repository
Template repository containing standard Kubernetes manifests and FluxCD configurations. Cloned and customized during cluster setup.

**Related:** [GitOps](#gitops), [Template](#template)

### GitOps Repository
Git repository containing cluster-specific manifests, configurations, and secrets. Generated by `openCenter cluster setup`.

**Related:** [GitOps](#gitops), [Repository](#repository)

### Godog
Go implementation of Cucumber for BDD testing. Used for integration tests in openCenter-cli.

**Related:** [BDD](#bdd-behavior-driven-development), [Testing](#testing)

### Gopter
Property-based testing library for Go. Used for generative testing of critical logic.

**Related:** [Testing](#testing), [Property-Based Testing](#property-based-testing)

### Grafana
Visualization and analytics platform for metrics and logs. Part of kube-prometheus-stack service.

**Related:** [Prometheus](#prometheus), [Observability](#observability)

---

## H

### Headlamp
Kubernetes dashboard with OIDC authentication support. Provides web-based cluster management interface.

**Related:** [OIDC](#oidc-openid-connect), [Dashboard](#dashboard)

### Helm
Package manager for Kubernetes applications. Used internally by some services but not exposed directly to users.

**Related:** [Kubernetes](#kubernetes), [Package Manager](#package-manager)

---

## I

### Image ID
UUID identifying an OpenStack Glance image. Used for specifying OS images for cluster nodes.

**Related:** [OpenStack](#openstack), [Glance](#glance)

### Infrastructure
Configuration section defining cloud provider, networking, and compute resources. Includes provider-specific settings.

**Related:** [Provider](#provider), [Cloud](#cloud)

### Ingress
Kubernetes resource for exposing HTTP/HTTPS services externally. Managed via Gateway API in openCenter-cli.

**Related:** [Gateway API](#gateway-api), [Load Balancer](#load-balancer)

### Instance
Virtual machine in cloud infrastructure. Corresponds to Kubernetes nodes (master, worker, bastion).

**Related:** [Node](#node), [Flavor](#flavor)

---

## J

### JSON Schema
Formal specification of configuration structure and validation rules. Generated via `openCenter cluster schema`.

**Related:** [Schema](#schema), [Validation](#validation)

---

## K

### Keycloak
Open-source identity and access management solution. Provides OIDC authentication for Kubernetes and applications.

**Related:** [OIDC](#oidc-openid-connect), [Authentication](#authentication)

### Keystone
OpenStack Identity service for authentication and authorization. Provides API credentials and service catalog.

**Related:** [OpenStack](#openstack), [Authentication](#authentication)

### Kind (Kubernetes in Docker)
Tool for running local Kubernetes clusters using Docker containers. Supported as a provider for development/testing.

**Related:** [Provider](#provider), [Development](#development)

### Kube-OVN
CNI plugin based on Open Virtual Network (OVN). Provides advanced networking features with optional Cilium integration.

**Related:** [CNI](#cni), [OVN](#ovn)

### kube-prometheus-stack
Comprehensive monitoring solution including Prometheus, Grafana, and Alertmanager. Deployed as a managed service.

**Related:** [Prometheus](#prometheus), [Grafana](#grafana)

### KubeVIP
Virtual IP solution for Kubernetes control plane high availability. Provides floating IP for API server access.

**Related:** [High Availability](#high-availability), [Control Plane](#control-plane)

### Kubespray
Ansible-based Kubernetes deployment tool. Used by openCenter-cli for cluster provisioning.

**Related:** [Ansible](#ansible), [Provisioning](#provisioning)

### Kubernetes
Open-source container orchestration platform. The primary workload managed by openCenter-cli.

**Related:** [Cluster](#cluster), [Container](#container)

### Kyverno
Kubernetes policy engine for security, compliance, and automation. Enforces pod security standards and custom policies.

**Related:** [Policy](#policy), [Security](#security)

---

## L

### Let's Encrypt
Free, automated certificate authority providing TLS certificates. Used by cert-manager for HTTPS endpoints.

**Related:** [Cert-Manager](#cert-manager), [TLS](#tls)

### Load Balancer
Network component distributing traffic across multiple nodes. Providers include OVN, Octavia, and MetalLB.

**Related:** [OVN](#ovn), [Octavia](#octavia), [MetalLB](#metallb)

### Loki
Log aggregation system designed for Kubernetes. Stores logs in object storage (Swift or S3).

**Related:** [Logging](#logging), [Swift](#swift)

---

## M

### Managed Service
Service managed by Rackspace or external provider. Configured in `managed-service` section (e.g., alert-proxy).

**Related:** [Service](#service), [Alert Proxy](#alert-proxy)

### Master Node
Kubernetes control plane node running API server, scheduler, and controller manager. Also called control plane node.

**Related:** [Control Plane](#control-plane), [Node](#node)

### Metadata
Configuration section tracking lifecycle information including creation time, creator, tags, and annotations.

**Related:** [Configuration](#configuration), [Annotations](#annotations)

### MetalLB
Load balancer implementation for bare metal Kubernetes clusters. Provides LoadBalancer service type without cloud provider.

**Related:** [Load Balancer](#load-balancer), [Baremetal](#baremetal)

### Migration
Process of updating configuration from one schema version to another. Handled automatically by the migrator component.

**Related:** [Schema](#schema), [Versioning](#versioning)

### Mise
Tool version management and task automation system. Replaces Make for build tasks in openCenter-cli.

**Related:** [Build System](#build-system), [Tasks](#tasks)

---

## N

### Networking
Configuration section defining network topology, CIDR ranges, DNS, NTP, and security settings.

**Related:** [CIDR](#cidr-classless-inter-domain-routing), [Subnet](#subnet)

### Neutron
OpenStack Networking service providing virtual networks, routers, and security groups.

**Related:** [OpenStack](#openstack), [Networking](#networking)

### Node
Physical or virtual machine running Kubernetes workloads. Types include master, worker, and bastion nodes.

**Related:** [Instance](#instance), [Cluster](#cluster)

### Node Naming
Configuration defining hostname prefixes for different node types (e.g., `cp` for control plane, `wn` for worker).

**Related:** [Node](#node), [Naming Convention](#naming-convention)

### Nova
OpenStack Compute service for managing virtual machine instances.

**Related:** [OpenStack](#openstack), [Instance](#instance)

---

## O

### Observability
Practice of monitoring, logging, and tracing system behavior. Implemented via Prometheus, Loki, and Tempo.

**Related:** [Prometheus](#prometheus), [Loki](#loki), [Tempo](#tempo)

### Octavia
OpenStack Load Balancer service. Alternative to OVN for Kubernetes LoadBalancer services.

**Related:** [Load Balancer](#load-balancer), [OpenStack](#openstack)

### OIDC (OpenID Connect)
Authentication protocol built on OAuth 2.0. Used for Kubernetes API authentication and application SSO.

**Related:** [Keycloak](#keycloak), [Authentication](#authentication)

### OLM (Operator Lifecycle Manager)
Framework for managing Kubernetes operators. Handles installation, updates, and lifecycle of operators.

**Related:** [Operator](#operator), [Kubernetes](#kubernetes)

### OpenStack
Open-source cloud computing platform. Primary infrastructure provider supported by openCenter-cli.

**Related:** [Provider](#provider), [Cloud](#cloud)

### OpenStack CCM
Cloud Controller Manager for OpenStack integration. Provides load balancer and storage integration.

**Related:** [CCM](#ccm-cloud-controller-manager), [OpenStack](#openstack)

### OpenStack CSI
Container Storage Interface driver for OpenStack Cinder. Provides persistent volume support.

**Related:** [CSI](#csi-container-storage-interface), [Cinder](#cinder)

### OpenTofu
Open-source Infrastructure as Code tool (Terraform fork). Used for provisioning cloud infrastructure.

**Related:** [Terraform](#terraform), [IaC](#iac-infrastructure-as-code)

### Operator
Kubernetes controller implementing domain-specific operational knowledge. Manages complex applications.

**Related:** [OLM](#olm-operator-lifecycle-manager), [Kubernetes](#kubernetes)

### Organization
Top-level grouping for clusters enabling multi-tenancy. Clusters are stored in organization-based directories.

**Related:** [Multi-Tenancy](#multi-tenancy), [Cluster](#cluster)

### OVN (Open Virtual Network)
Software-defined networking system. Default load balancer provider for Kubernetes services.

**Related:** [Load Balancer](#load-balancer), [Networking](#networking)

### Overlay
Cluster-specific directory in GitOps repository containing customized manifests and secrets.

**Related:** [GitOps](#gitops), [Kustomize](#kustomize)

---

## P

### Path Resolver
Component resolving file paths for organization-based directory structures. Handles backward compatibility with legacy layouts.

**Related:** [Organization](#organization), [Configuration](#configuration)

### Persistent Volume
Kubernetes storage resource that persists beyond pod lifecycle. Backed by cloud storage (Cinder, EBS, etc.).

**Related:** [Storage](#storage), [CSI](#csi-container-storage-interface)

### Plugin
External command extending openCenter-cli functionality. Discovered automatically in `~/.config/openCenter/plugins/`.

**Related:** [CLI](#cli), [Extension](#extension)

### Pod
Smallest deployable unit in Kubernetes containing one or more containers.

**Related:** [Kubernetes](#kubernetes), [Container](#container)

### Pod Security
Kubernetes security standards enforcing security best practices for pods. Managed by Kyverno in openCenter-cli.

**Related:** [Security](#security), [Kyverno](#kyverno)

### Policy
Rule or constraint enforced by Kubernetes admission controllers. Implemented via Kyverno policies.

**Related:** [Kyverno](#kyverno), [Security](#security)

### Preflight
Pre-deployment validation checking cloud provider connectivity, quotas, and resource availability. Executed via `openCenter cluster preflight`.

**Related:** [Validation](#validation), [Provider](#provider)

### Prometheus
Open-source monitoring and alerting system. Collects metrics from Kubernetes and applications.

**Related:** [Observability](#observability), [Metrics](#metrics)

### Property-Based Testing
Testing methodology using generated inputs to verify properties. Implemented with Gopter.

**Related:** [Testing](#testing), [Gopter](#gopter)

### Provider
Infrastructure platform hosting Kubernetes clusters. Supported: OpenStack, AWS, VMware, Kind, Baremetal.

**Related:** [Infrastructure](#infrastructure), [Cloud](#cloud)

### Provisioning
Process of creating infrastructure resources (networks, instances, storage) for a Kubernetes cluster.

**Related:** [OpenTofu](#opentofu), [Infrastructure](#infrastructure)

### Pulumi
Infrastructure as Code tool using general-purpose programming languages. Used for Talos Linux deployments.

**Related:** [IaC](#iac-infrastructure-as-code), [Talos](#talos)

---

## R

### RBAC (Role-Based Access Control)
Kubernetes authorization mechanism controlling access to resources based on roles.

**Related:** [Security](#security), [Authorization](#authorization)

### RBAC Manager
Operator simplifying RBAC configuration via custom resources. Deployed as a service.

**Related:** [RBAC](#rbac-role-based-access-control), [Operator](#operator)

### Reconciliation
Process of continuously comparing desired state (Git) with actual state (cluster) and applying changes.

**Related:** [GitOps](#gitops), [FluxCD](#fluxcd)

### Region
Geographic location of cloud resources. Required for OpenStack and AWS providers.

**Related:** [Provider](#provider), [Availability Zone](#availability-zone)

### Registry
Container image registry storing Docker images. Can be public (Docker Hub) or private.

**Related:** [Container](#container), [Image](#image)

### Repository
Git repository containing cluster configurations and manifests. Central to GitOps workflow.

**Related:** [GitOps](#gitops), [Git](#git)

---

## S

### S3
Object storage API originally from AWS, now widely supported. Used for state backends and backups.

**Related:** [Swift](#swift), [Object Storage](#object-storage)

### Schema
Formal specification of configuration structure using JSON Schema. Defines required fields, types, and validation rules.

**Related:** [JSON Schema](#json-schema), [Validation](#validation)

### Schema Version
Version identifier for configuration format. Enables migration between schema versions.

**Related:** [Migration](#migration), [Versioning](#versioning)

### Secrets
Sensitive data like passwords, API keys, and certificates. Encrypted using SOPS before storage in Git.

**Related:** [SOPS](#sops), [Encryption](#encryption)

### Secrets Management
Practice of securely storing, accessing, and rotating sensitive data. Implemented via SOPS and Barbican.

**Related:** [SOPS](#sops), [Barbican](#barbican)

### Security Group
Firewall rules controlling network access to instances. Configured per node type.

**Related:** [Networking](#networking), [Firewall](#firewall)

### Server Group
OpenStack feature for controlling instance placement. Supports affinity and anti-affinity policies.

**Related:** [OpenStack](#openstack), [High Availability](#high-availability)

### Service
Kubernetes resource exposing applications internally or externally. Also refers to add-on applications deployed in clusters.

**Related:** [Kubernetes](#kubernetes), [Load Balancer](#load-balancer)

### Service Map
Configuration structure mapping service names to their configurations. Used for `services` and `managed-service` sections.

**Related:** [Service](#service), [Configuration](#configuration)

### Setup
Process of generating GitOps repository structure and manifests. Executed via `openCenter cluster setup`.

**Related:** [GitOps](#gitops), [Bootstrap](#bootstrap)

### SOPS (Secrets OPerationS)
Tool for encrypting files using Age, PGP, or cloud KMS. Integrates with Git for encrypted secrets storage.

**Related:** [Age](#age), [Encryption](#encryption)

### Sprig
Template function library providing 100+ functions for Go templates. Used in GitOps manifest generation.

**Related:** [Template](#template), [Go Templates](#go-templates)

### SSH (Secure Shell)
Protocol for secure remote access to servers. Used for node access via bastion host.

**Related:** [Bastion Host](#bastion-host), [Authentication](#authentication)

### Storage Class
Kubernetes resource defining storage provisioning parameters. Specifies volume type, replication, and performance.

**Related:** [Persistent Volume](#persistent-volume), [CSI](#csi-container-storage-interface)

### Subnet
IP address range within a network. Separate subnets for nodes, pods, and services.

**Related:** [CIDR](#cidr-classless-inter-domain-routing), [Networking](#networking)

### Swift
OpenStack Object Storage service. Used for backups, logs, and state storage.

**Related:** [OpenStack](#openstack), [S3](#s3)

---

## T

### Talos
Immutable Linux distribution designed for Kubernetes. Provides enhanced security and simplified operations.

**Related:** [Operating System](#operating-system), [Security](#security)

### Task
Automated command defined in `.mise.toml`. Replaces Makefile targets for build and test operations.

**Related:** [Mise](#mise), [Build System](#build-system)

### Template
Go template file using text/template syntax with Sprig functions. Used for generating GitOps manifests.

**Related:** [Go Templates](#go-templates), [Sprig](#sprig)

### Tempo
Distributed tracing backend for Kubernetes. Stores traces in object storage.

**Related:** [Observability](#observability), [Tracing](#tracing)

### Terraform
Infrastructure as Code tool for provisioning cloud resources. OpenTofu is the open-source fork.

**Related:** [OpenTofu](#opentofu), [IaC](#iac-infrastructure-as-code)

### TLS (Transport Layer Security)
Cryptographic protocol for secure communications. Used for HTTPS endpoints and internal cluster communication.

**Related:** [Cert-Manager](#cert-manager), [Encryption](#encryption)

---

## V

### Validation
Process of checking configuration correctness against schema and business rules. Multiple validation layers exist.

**Related:** [Schema](#schema), [Preflight](#preflight)

### Velero
Kubernetes backup and disaster recovery tool. Backs up cluster resources and persistent volumes.

**Related:** [Backup](#backup), [Disaster Recovery](#disaster-recovery)

### Versioning
Practice of tracking configuration schema versions for backward compatibility and migration.

**Related:** [Schema Version](#schema-version), [Migration](#migration)

### VLAN (Virtual Local Area Network)
Network segmentation technology. Configurable for OpenStack networking.

**Related:** [Networking](#networking), [OpenStack](#openstack)

### VMware
Virtualization platform. Supported as an infrastructure provider via vSphere.

**Related:** [Provider](#provider), [vSphere](#vsphere)

### VPC (Virtual Private Cloud)
Isolated network environment in AWS. Required for AWS-based clusters.

**Related:** [AWS](#aws), [Networking](#networking)

### VRRP (Virtual Router Redundancy Protocol)
Protocol providing high availability for network gateways. Optional for control plane HA.

**Related:** [High Availability](#high-availability), [Networking](#networking)

### vSphere
VMware's cloud computing virtualization platform. Supported infrastructure provider.

**Related:** [VMware](#vmware), [Provider](#provider)

### vSphere CSI
Container Storage Interface driver for VMware vSphere. Provides persistent volume support.

**Related:** [CSI](#csi-container-storage-interface), [vSphere](#vsphere)

---

## W

### Weave GitOps
Web UI for FluxCD providing visualization and management of GitOps workflows.

**Related:** [FluxCD](#fluxcd), [GitOps](#gitops)

### Windows Worker
Kubernetes worker node running Windows Server. Supports Windows container workloads.

**Related:** [Worker Node](#worker-node), [Windows](#windows)

### Worker Node
Kubernetes node running application workloads. Separate from control plane nodes.

**Related:** [Node](#node), [Cluster](#cluster)

### Workspace
Local directory containing GitOps repository and generated files. Typically `~/.config/openCenter/clusters/<org>/<cluster>/`.

**Related:** [GitOps](#gitops), [Organization](#organization)

---

## Y

### YAML (YAML Ain't Markup Language)
Human-readable data serialization format. Used for all configuration files in openCenter-cli.

**Related:** [Configuration](#configuration), [Serialization](#serialization)

---

## Acronyms Quick Reference

| Acronym | Full Term | Category |
|---------|-----------|----------|
| ACME | Automatic Certificate Management Environment | Security |
| API | Application Programming Interface | General |
| AWS | Amazon Web Services | Provider |
| BDD | Behavior-Driven Development | Testing |
| BFV | Boot From Volume | Storage |
| CCM | Cloud Controller Manager | Kubernetes |
| CIDR | Classless Inter-Domain Routing | Networking |
| CLI | Command-Line Interface | General |
| CNI | Container Network Interface | Networking |
| CSI | Container Storage Interface | Storage |
| DNS | Domain Name System | Networking |
| eBPF | Extended Berkeley Packet Filter | Networking |
| FQDN | Fully Qualified Domain Name | Networking |
| HA | High Availability | Architecture |
| HTTP | Hypertext Transfer Protocol | Networking |
| HTTPS | HTTP Secure | Networking |
| IaC | Infrastructure as Code | Provisioning |
| IP | Internet Protocol | Networking |
| JSON | JavaScript Object Notation | Data Format |
| KMS | Key Management Service | Security |
| MTU | Maximum Transmission Unit | Networking |
| NTP | Network Time Protocol | Networking |
| OIDC | OpenID Connect | Authentication |
| OLM | Operator Lifecycle Manager | Kubernetes |
| OVN | Open Virtual Network | Networking |
| PGP | Pretty Good Privacy | Encryption |
| RBAC | Role-Based Access Control | Security |
| S3 | Simple Storage Service | Storage |
| SOPS | Secrets OPerationS | Security |
| SSH | Secure Shell | Security |
| SSL | Secure Sockets Layer | Security |
| TLS | Transport Layer Security | Security |
| UUID | Universally Unique Identifier | General |
| VLAN | Virtual Local Area Network | Networking |
| VPC | Virtual Private Cloud | Networking |
| VRRP | Virtual Router Redundancy Protocol | Networking |
| YAML | YAML Ain't Markup Language | Data Format |

---

## See Also

- [File Formats Reference](file-formats.md) - Detailed file format specifications
- [CLI Reference](../reference/cli.md) - Command-line interface documentation
- [Configuration Guide](../guides/configuration.md) - Configuration best practices
- [Architecture Overview](../dev/architecture.md) - System architecture documentation
