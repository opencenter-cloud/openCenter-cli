# AWS Provider


## Table of Contents

- [Purpose](#purpose)
- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Requirements](#requirements)
- [Configuration Options](#configuration-options)
- [Deployment Workflow](#deployment-workflow)
- [Limitations and Known Issues](#limitations-and-known-issues)
- [Development Status](#development-status)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: reference**

This document describes the AWS provider for openCenter. It covers architecture, features, requirements, configuration options, and deployment workflow for running Kubernetes clusters on Amazon Web Services.

## Purpose

The AWS provider enables deployment of Kubernetes clusters on AWS public cloud infrastructure. It uses Terraform/OpenTofu for infrastructure provisioning and supports both self-managed Kubernetes and AWS-native services integration.

## Overview

The AWS provider is currently in **alpha development**. It creates Kubernetes clusters on AWS infrastructure using VPC networking, EC2 instances, and AWS-native services. The provider handles infrastructure provisioning through Terraform/OpenTofu with plans for Kubernetes installation via Kubespray or EKS integration.

**Key characteristics:**
- Alpha development status (not production-ready)
- VPC-based networking with public and private subnets
- EC2 instance provisioning for cluster nodes
- IAM role and policy management
- Integration with AWS services (ELB, EBS, Route53)
- Multi-AZ deployment support (planned)

## Architecture

### Component Stack

The AWS provider uses a layered architecture:

```
┌─────────────────────────────────────────┐
│         openCenter CLI                  │
│  (Configuration & Orchestration)        │
└─────────────────┬───────────────────────┘
                  │
                  ├─── Generates main.tf
                  │
┌─────────────────▼───────────────────────┐
│      Terraform/OpenTofu                 │
│  (Infrastructure as Code)               │
└─────────────────┬───────────────────────┘
                  │
                  ├─── Provisions Infrastructure
                  │
┌─────────────────▼───────────────────────┐
│         AWS Services                    │
│  VPC │ EC2 │ ELB │ EBS │ Route53        │
└─────────────────┬───────────────────────┘
                  │
                  ├─── Creates Instances & Networks
                  │
┌─────────────────▼───────────────────────┐
│      Kubernetes Installation            │
│  (Kubespray or EKS - TBD)               │
└─────────────────┬───────────────────────┘
                  │
                  ├─── Installs Kubernetes
                  │
┌─────────────────▼───────────────────────┐
│      Kubernetes Cluster                 │
│  Control Plane │ Workers │ Services     │
└─────────────────────────────────────────┘
```

### Infrastructure Provisioning

Terraform/OpenTofu provisions these AWS resources:

**Networking:**
- VPC with configurable CIDR block
- Public subnets for external access
- Private subnets for cluster nodes
- Internet Gateway for public subnet access
- NAT Gateway for private subnet egress
- Route tables and associations
- Security groups for cluster communication

**Compute:**
- EC2 instances for master nodes
- EC2 instances for worker nodes
- Auto Scaling Groups (planned)
- Launch templates with user data

**Load Balancing:**
- Application Load Balancer for API server (planned)
- Network Load Balancer for services (planned)
- Target groups and health checks

**Storage:**
- EBS volumes for node storage
- EBS CSI driver integration (planned)
- Snapshot management (planned)

**IAM:**
- Instance profiles for nodes
- Roles for cluster components
- Policies for AWS service access

## Features

### VPC Networking

**Network Isolation:**
- Dedicated VPC per cluster
- Public subnets for load balancers and bastion
- Private subnets for cluster nodes
- Network ACLs for traffic control
- Security groups for fine-grained access

**Subnet Configuration:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: "vpc-0123456789abcdef0"  # Existing VPC
        private_subnets:
          - "10.0.1.0/24"
          - "10.0.2.0/24"
          - "10.0.3.0/24"
        public_subnets:
          - "10.0.101.0/24"
          - "10.0.102.0/24"
          - "10.0.103.0/24"
```

### Multi-AZ Deployment

**High Availability (Planned):**
- Master nodes distributed across availability zones
- Worker nodes spread across AZs
- EBS volumes with AZ-aware scheduling
- Cross-AZ load balancing

### IAM Integration

**Authentication Methods:**

*IAM Access Keys:*
- Programmatic access for Terraform
- Stored securely in SOPS-encrypted secrets
- Rotatable credentials

*IAM Profiles:*
- Instance profiles for EC2 nodes
- Service accounts for Kubernetes components
- Least-privilege access policies

*IAM Roles for Service Accounts (IRSA):*
- Fine-grained permissions for pods
- No static credentials in pods
- AWS STS token vending

### AWS Service Integration

**Planned Integrations:**
- **EBS CSI Driver**: Dynamic volume provisioning
- **ELB**: Load balancing for services
- **Route53**: DNS management
- **CloudWatch**: Logging and monitoring
- **Systems Manager**: Node management
- **Secrets Manager**: Secret storage (alternative to SOPS)

## Requirements

### AWS Account Prerequisites

**Account Access:**
- Active AWS account with billing enabled
- IAM user or role with sufficient permissions
- Access keys or IAM profile configured
- MFA enabled (recommended for production)

**Service Limits:**
- Sufficient EC2 instance limits for cluster size
- VPC limit (default: 5 per region)
- Elastic IP limit (default: 5 per region)
- EBS volume limits

### Required IAM Permissions

**Minimum IAM Policy:**

The IAM user or role must have permissions to:

**EC2:**
- Create, describe, modify, and terminate instances
- Create and manage security groups
- Allocate and associate Elastic IPs
- Create and attach EBS volumes
- Manage key pairs

**VPC:**
- Create and manage VPCs
- Create and manage subnets
- Create and manage route tables
- Create and manage internet gateways
- Create and manage NAT gateways

**ELB:**
- Create and manage load balancers
- Create and manage target groups
- Register and deregister targets

**IAM:**
- Create and manage instance profiles
- Create and manage roles
- Attach policies to roles

**Route53 (Optional):**
- Create and manage hosted zones
- Create and manage DNS records

See [IAM Configuration Guide](iam.md) for detailed policy examples.

### Network Requirements

**VPC Configuration:**
- VPC CIDR block (e.g., 10.0.0.0/16)
- At least 2 availability zones
- Public subnets for external access
- Private subnets for cluster nodes
- Internet Gateway for public subnets
- NAT Gateway for private subnet egress

**CIDR Planning:**
- VPC CIDR must not overlap with on-premises networks
- Subnet CIDRs must not overlap with pod/service networks
- Reserve IP space for future growth

**DNS and NTP:**
- Access to AWS DNS resolvers (VPC+2 address)
- NTP via Amazon Time Sync Service

### Quota Requirements

**Minimum Quotas for Small Cluster (3 masters, 2 workers):**

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| EC2 Instances | 5 | 10 |
| vCPUs | 20 | 40 |
| EBS Volumes | 5 | 10 |
| EBS Storage (GB) | 200 | 500 |
| Elastic IPs | 1 | 3 |
| Security Groups | 3 | 5 |
| Security Group Rules | 50 | 100 |
| VPCs | 1 | 2 |
| Subnets | 6 | 12 |
| NAT Gateways | 1 | 3 |

Check current quotas:
```bash
aws service-quotas list-service-quotas \
  --service-code ec2 \
  --query 'Quotas[?QuotaName==`Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances`]'
```

## Configuration Options

### Authentication Configuration

**IAM Access Keys:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        profile: ""  # Leave empty when using access keys

secrets:
  global:
    aws:
      infrastructure:
        access_key: "${AWS_ACCESS_KEY_ID}"
        secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
        region: "us-east-1"
```

**IAM Profile:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        profile: "opencenter-admin"  # AWS CLI profile name
```

### VPC and Subnet Configuration

**New VPC (Auto-created):**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        vpc_id: ""  # Empty = create new VPC
        private_subnets: []  # Auto-calculated
        public_subnets: []   # Auto-calculated
```

**Existing VPC:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        vpc_id: "vpc-0123456789abcdef0"
        private_subnets:
          - "10.0.1.0/24"
          - "10.0.2.0/24"
          - "10.0.3.0/24"
        public_subnets:
          - "10.0.101.0/24"
          - "10.0.102.0/24"
          - "10.0.103.0/24"
```

### Instance Configuration

**Node Sizing:**
```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_master: "t3.medium"    # 2 vCPU, 4 GB RAM
      flavor_worker: "t3.large"     # 2 vCPU, 8 GB RAM
      master_count: 3
      worker_count: 2
```

**Instance Type Recommendations:**

| Cluster Size | Master Type | Worker Type | Master Count | Worker Count |
|--------------|-------------|-------------|--------------|--------------|
| Dev/Test | t3.small | t3.medium | 1 | 1-2 |
| Small Prod | t3.medium | t3.large | 3 | 2-5 |
| Medium Prod | t3.large | t3.xlarge | 3 | 5-10 |
| Large Prod | t3.xlarge | t3.2xlarge | 3 | 10+ |

### Storage Configuration

**EBS Volume Settings:**
```yaml
opencenter:
  storage:
    worker_volume_size: 40  # GB
    worker_volume_type: "gp3"  # General Purpose SSD
    default_storage_class: "ebs-sc"
```

**EBS Volume Types:**
- `gp3`: General Purpose SSD (recommended, cost-effective)
- `gp2`: General Purpose SSD (legacy)
- `io2`: Provisioned IOPS SSD (high performance)
- `st1`: Throughput Optimized HDD (large sequential workloads)

## Deployment Workflow

### 1. Configuration

Create cluster configuration:

```bash
mise run build
./bin/openCenter cluster init my-aws-cluster \
  --opencenter.infrastructure.provider=aws \
  --opencenter.infrastructure.cloud.aws.region=us-east-1
```

Edit the generated configuration file:

```yaml
# ~/.config/openCenter/clusters/opencenter/.my-aws-cluster-config.yaml
schema_version: "1.0"
opencenter:
  meta:
    name: my-aws-cluster
    env: production
    region: us-east-1
  infrastructure:
    provider: aws
    cloud:
      aws:
        region: "us-east-1"
        vpc_id: ""  # Auto-create VPC
        private_subnets: []
        public_subnets: []
```

### 2. Validation

Validate configuration:

```bash
./bin/openCenter cluster validate my-aws-cluster
```

The validator checks:
- Schema compliance
- AWS region format
- VPC and subnet configuration
- CIDR overlap detection
- Credential format validation

### 3. Infrastructure Provisioning

Generate GitOps repository and Terraform configuration:

```bash
./bin/openCenter cluster setup my-aws-cluster
```

This creates:
- GitOps repository structure
- `main.tf` with AWS resources
- Terraform backend configuration
- Kubernetes manifests

### 4. Deployment

Provision infrastructure (manual for alpha):

```bash
cd <gitops-repo>/infrastructure/clusters/my-aws-cluster
terraform init
terraform plan
terraform apply
```

### 5. Kubernetes Installation

**Note**: Kubernetes installation method is TBD. Options under consideration:
- Kubespray (Ansible-based, consistent with OpenStack provider)
- EKS (AWS-managed control plane)
- Self-managed with kubeadm

## Limitations and Known Issues

### Current Limitations

**Alpha Status:**
- Not production-ready
- Limited testing and validation
- API may change without notice
- Incomplete feature set

**Missing Features:**
- Auto-scaling groups
- Multi-region support
- EKS integration
- AWS service integrations (EBS CSI, ELB, Route53)
- Spot instance support
- Windows worker nodes

**Networking:**
- Single VPC per cluster
- No VPC peering support
- Limited security group customization
- No PrivateLink integration

**Storage:**
- Basic EBS volume support only
- No EFS integration
- No S3 CSI driver
- Limited snapshot management

### Known Issues

**Issue: Terraform state management**
- **Symptom**: State conflicts in team environments
- **Cause**: Local state backend
- **Solution**: Configure S3 backend with DynamoDB locking

**Issue: Subnet CIDR conflicts**
- **Symptom**: Validation fails with overlapping subnets
- **Cause**: Incorrect CIDR planning
- **Solution**: Use non-overlapping CIDR blocks for all subnets

**Issue: IAM permission errors**
- **Symptom**: Terraform fails with access denied
- **Cause**: Insufficient IAM permissions
- **Solution**: Review and update IAM policy (see [IAM Guide](iam.md))

## Development Status

The AWS provider is under active development. Current priorities:

**Phase 1 (Current):**
- ✅ Basic VPC and subnet configuration
- ✅ IAM credential validation
- ✅ Configuration schema
- 🚧 EC2 instance provisioning
- 🚧 Security group configuration

**Phase 2 (Planned):**
- ⏳ Kubernetes installation (Kubespray)
- ⏳ EBS CSI driver integration
- ⏳ Load balancer configuration
- ⏳ Multi-AZ support

**Phase 3 (Future):**
- ⏳ EKS integration option
- ⏳ Auto-scaling groups
- ⏳ Spot instance support
- ⏳ Advanced networking features

**Legend**: ✅ Complete | 🚧 In Progress | ⏳ Planned

## Related Documentation

### Setup and Configuration
- [AWS Setup Guide](setup.md) - Detailed setup instructions
- [IAM Configuration](iam.md) - IAM roles and policies
- [VPC Design](vpc.md) - Network architecture and planning

### Operations
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions

### Reference
- [Configuration Reference](../../reference/configuration.md) - Complete configuration options
- [Provider Comparison](../README.md) - Compare AWS with other providers

## External Resources

- [AWS Documentation](https://docs.aws.amazon.com/)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [AWS CLI Reference](https://docs.aws.amazon.com/cli/latest/reference/)
- [AWS Well-Architected Framework](https://aws.amazon.com/architecture/well-architected/)

---

**Last Updated**: January 2025  
**Provider Status**: Alpha (In Development)  
**Maintained By**: openCenter Team
