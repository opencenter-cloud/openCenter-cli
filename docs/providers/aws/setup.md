# AWS Setup Guide


## Table of Contents

- [Task Summary](#task-summary)
- [Prerequisites](#prerequisites)
- [Step 1: Configure AWS Credentials](#step-1-configure-aws-credentials)
- [Step 2: Plan VPC and Network Configuration](#step-2-plan-vpc-and-network-configuration)
- [Step 3: Select Instance Types](#step-3-select-instance-types)
- [Step 4: Configure Storage](#step-4-configure-storage)
- [Step 5: Create Cluster Configuration](#step-5-create-cluster-configuration)
- [Step 6: Validate Configuration](#step-6-validate-configuration)
- [Step 7: Configure Terraform Backend (Optional)](#step-7-configure-terraform-backend-optional)
- [Common Configurations](#common-configurations)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)
- [Related Documentation](#related-documentation)
**doc_type: how-to**

Configure openCenter to deploy Kubernetes clusters on AWS infrastructure. This guide walks through AWS account setup, IAM configuration, VPC planning, and cluster configuration.

## Task Summary

Set up openCenter to provision Kubernetes clusters on AWS by configuring IAM credentials, planning VPC topology, selecting instance types, and validating the configuration. The result is a validated cluster configuration ready for deployment.

## Prerequisites

Before starting, you need:

- **AWS account** with administrative access or sufficient IAM permissions
- **AWS CLI** installed and configured (version 2.x recommended)
- **openCenter installed** (see [Getting Started](../../tutorials/getting-started.md))
- **Terraform or OpenTofu** installed (version 1.5+ or compatible)
- **Basic AWS knowledge**: VPCs, subnets, EC2, IAM

Verify AWS CLI installation:
```bash
aws --version
# Should show: aws-cli/2.x.x or higher
```

Check AWS credentials:
```bash
aws sts get-caller-identity
```

## Step 1: Configure AWS Credentials

openCenter supports two authentication methods: IAM access keys or AWS CLI profiles.

### Option A: IAM Access Keys (Recommended for CI/CD)

Create an IAM user with programmatic access:

```bash
# Create IAM user
aws iam create-user --user-name opencenter-provisioner

# Attach required policies (see IAM guide for details)
aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess

aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::aws:policy/AmazonVPCFullAccess

# Create access key
aws iam create-access-key --user-name opencenter-provisioner
```

Save the access key output. You'll need the `AccessKeyId` and `SecretAccessKey`.

Store credentials in environment variables:
```bash
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_DEFAULT_REGION="us-east-1"
```

Add to your cluster configuration:
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"

secrets:
  global:
    aws:
      infrastructure:
        access_key: "${AWS_ACCESS_KEY_ID}"
        secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
        region: "us-east-1"
```

### Option B: AWS CLI Profile (Recommended for Local Development)

Configure an AWS CLI profile:

```bash
# Configure profile interactively
aws configure --profile opencenter-admin

# Or create profile manually
cat >> ~/.aws/credentials <<EOF
[opencenter-admin]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
EOF

cat >> ~/.aws/config <<EOF
[profile opencenter-admin]
region = us-east-1
output = json
EOF
```

Add to your cluster configuration:
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        profile: "opencenter-admin"
```

### Verify Authentication

Test your credentials:

```bash
# Using environment variables
aws sts get-caller-identity

# Using profile
aws sts get-caller-identity --profile opencenter-admin
```

You should see your account ID, user ARN, and user ID.

## Step 2: Plan VPC and Network Configuration

Design your cluster's network topology. You can use an existing VPC or let openCenter create one.

### Option A: Auto-Create VPC (Simplest)

Let openCenter create a new VPC with default settings:

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        vpc_id: ""  # Empty = auto-create
        private_subnets: []  # Auto-calculated
        public_subnets: []   # Auto-calculated
```

openCenter will create:
- VPC with 10.0.0.0/16 CIDR
- 3 public subnets (10.0.1.0/24, 10.0.2.0/24, 10.0.3.0/24)
- 3 private subnets (10.0.11.0/24, 10.0.12.0/24, 10.0.13.0/24)
- Internet Gateway
- NAT Gateway in each public subnet
- Route tables

### Option B: Use Existing VPC

Use an existing VPC with custom subnets:

1. **Find your VPC ID:**

```bash
aws ec2 describe-vpcs --query 'Vpcs[*].[VpcId,CidrBlock,Tags[?Key==`Name`].Value|[0]]' --output table
```

2. **List available subnets:**

```bash
aws ec2 describe-subnets \
  --filters "Name=vpc-id,Values=vpc-0123456789abcdef0" \
  --query 'Subnets[*].[SubnetId,CidrBlock,AvailabilityZone,Tags[?Key==`Name`].Value|[0]]' \
  --output table
```

3. **Configure in cluster YAML:**

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        vpc_id: "vpc-0123456789abcdef0"
        private_subnets:
          - "10.0.1.0/24"   # Private subnet AZ-a
          - "10.0.2.0/24"   # Private subnet AZ-b
          - "10.0.3.0/24"   # Private subnet AZ-c
        public_subnets:
          - "10.0.101.0/24" # Public subnet AZ-a
          - "10.0.102.0/24" # Public subnet AZ-b
          - "10.0.103.0/24" # Public subnet AZ-c
```

### Network Planning Guidelines

**CIDR Block Sizing:**

| Cluster Size | Nodes | VPC CIDR | Private Subnet | Public Subnet |
|--------------|-------|----------|----------------|---------------|
| Small | 5-10 | /16 | /24 (251 IPs) | /24 (251 IPs) |
| Medium | 10-50 | /16 | /22 (1019 IPs) | /24 (251 IPs) |
| Large | 50-250 | /16 | /20 (4091 IPs) | /24 (251 IPs) |

**Avoid CIDR Conflicts:**
- VPC CIDR must not overlap with on-premises networks
- Subnet CIDRs must not overlap with Kubernetes pod network (10.42.0.0/16)
- Subnet CIDRs must not overlap with Kubernetes service network (10.43.0.0/16)

**Multi-AZ Recommendations:**
- Use at least 2 availability zones for HA
- Distribute subnets evenly across AZs
- Place NAT Gateway in each AZ for redundancy

### Kubernetes Network Configuration

Configure pod and service networks:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # Pod network
      subnet_services: "10.43.0.0/16"  # Service network
```

These must not overlap with VPC or subnet CIDRs.

## Step 3: Select Instance Types

Choose EC2 instance types for your cluster nodes.

### List Available Instance Types

```bash
# List instance types in your region
aws ec2 describe-instance-types \
  --filters "Name=instance-type,Values=t3.*" \
  --query 'InstanceTypes[*].[InstanceType,VCpuInfo.DefaultVCpus,MemoryInfo.SizeInMiB]' \
  --output table

# Check instance type availability in specific AZ
aws ec2 describe-instance-type-offerings \
  --location-type availability-zone \
  --filters "Name=instance-type,Values=t3.medium" \
  --region us-east-1 \
  --query 'InstanceTypeOfferings[*].Location' \
  --output table
```

### Configure Node Instance Types

```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_master: "t3.medium"  # 2 vCPU, 4 GB RAM
      flavor_worker: "t3.large"   # 2 vCPU, 8 GB RAM
      master_count: 3             # HA control plane
      worker_count: 2             # Minimum for workloads
```

### Instance Type Recommendations

**Development/Testing:**
```yaml
flavor_master: "t3.small"   # 2 vCPU, 2 GB RAM
flavor_worker: "t3.medium"  # 2 vCPU, 4 GB RAM
master_count: 1
worker_count: 1
```

**Production (Small):**
```yaml
flavor_master: "t3.medium"  # 2 vCPU, 4 GB RAM
flavor_worker: "t3.large"   # 2 vCPU, 8 GB RAM
master_count: 3
worker_count: 3
```

**Production (Large):**
```yaml
flavor_master: "t3.large"    # 2 vCPU, 8 GB RAM
flavor_worker: "t3.2xlarge"  # 8 vCPU, 32 GB RAM
master_count: 3
worker_count: 5
```

### Instance Family Selection

**T3 (Burstable):**
- Cost-effective for variable workloads
- CPU credits for burst performance
- Good for dev/test environments

**M5 (General Purpose):**
- Balanced compute, memory, and networking
- Consistent performance
- Good for production workloads

**C5 (Compute Optimized):**
- High CPU-to-memory ratio
- Best for compute-intensive workloads
- Good for control plane nodes

**R5 (Memory Optimized):**
- High memory-to-CPU ratio
- Best for memory-intensive workloads
- Good for data processing workers

## Step 4: Configure Storage

Set up EBS volumes for cluster nodes.

### EBS Volume Configuration

```yaml
opencenter:
  storage:
    worker_volume_size: 40           # GB per worker
    worker_volume_type: "gp3"        # General Purpose SSD v3
    default_storage_class: "ebs-sc"  # Kubernetes storage class
```

### EBS Volume Types

**gp3 (General Purpose SSD v3) - Recommended:**
- 3,000 IOPS baseline
- 125 MB/s throughput baseline
- Cost-effective
- Configurable IOPS and throughput

**gp2 (General Purpose SSD v2) - Legacy:**
- IOPS scales with volume size (3 IOPS/GB)
- Burstable to 3,000 IOPS
- Being replaced by gp3

**io2 (Provisioned IOPS SSD):**
- Up to 64,000 IOPS per volume
- 99.999% durability
- High performance, higher cost
- Use for databases and critical workloads

**st1 (Throughput Optimized HDD):**
- 500 MB/s max throughput
- Lower cost
- Use for large sequential workloads

### Check EBS Quotas

```bash
# Check EBS volume quota
aws service-quotas get-service-quota \
  --service-code ebs \
  --quota-code L-D18FCD1D \
  --region us-east-1

# Check EBS storage quota
aws service-quotas get-service-quota \
  --service-code ebs \
  --quota-code L-7A658B76 \
  --region us-east-1
```

## Step 5: Create Cluster Configuration

Initialize a cluster configuration with your settings:

```bash
mise run build
./bin/openCenter cluster init my-aws-cluster \
  --opencenter.infrastructure.provider=aws \
  --opencenter.infrastructure.cloud.aws.region=us-east-1 \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=2
```

This creates a configuration file at:
```
~/.config/openCenter/clusters/opencenter/.my-aws-cluster-config.yaml
```

### Edit Configuration File

Open the file and add your AWS-specific settings:

```yaml
schema_version: "1.0"
opencenter:
  meta:
    name: my-aws-cluster
    env: production
    region: us-east-1
    organization: opencenter
  
  infrastructure:
    provider: aws
    ssh_user: ubuntu
    os_version: "24"  # Ubuntu 24.04
    
    cloud:
      aws:
        region: "us-east-1"
        profile: "opencenter-admin"  # Or leave empty for access keys
        vpc_id: ""  # Empty = auto-create VPC
        private_subnets: []
        public_subnets: []
  
  cluster:
    cluster_name: my-aws-cluster
    base_domain: "k8s.example.com"
    cluster_fqdn: "my-aws-cluster.us-east-1.k8s.example.com"
    
    networking:
      k8s_api_port_acl:
        - "0.0.0.0/0"  # Restrict in production!
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
    
    kubernetes:
      version: "1.33.5"
      flavor_master: "t3.medium"
      flavor_worker: "t3.large"
      master_count: 3
      worker_count: 2
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"
  
  storage:
    worker_volume_size: 40
    worker_volume_type: "gp3"
    default_storage_class: "ebs-sc"

secrets:
  global:
    aws:
      infrastructure:
        access_key: "${AWS_ACCESS_KEY_ID}"
        secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
        region: "us-east-1"
```

## Step 6: Validate Configuration

Run validation checks:

```bash
./bin/openCenter cluster validate my-aws-cluster
```

The validator checks:
- Schema compliance
- AWS region format
- VPC ID format (if provided)
- Subnet CIDR validity
- Subnet overlap detection
- Credential format
- Required fields

### Fix Common Validation Errors

**Error: Invalid AWS region format**
```
Error: invalid AWS region format: us-east1
```

Fix: Use correct region format with hyphen:
```yaml
region: "us-east-1"  # Not "us-east1"
```

**Error: Subnets overlap**
```
Error: subnets overlap: 10.0.1.0/24 and 10.0.1.0/25
```

Fix: Use non-overlapping CIDR blocks:
```yaml
private_subnets:
  - "10.0.1.0/24"
  - "10.0.2.0/24"  # Not 10.0.1.0/25
```

**Error: Invalid VPC ID format**
```
Error: invalid VPC ID format: vpc-123
```

Fix: Use full VPC ID:
```yaml
vpc_id: "vpc-0123456789abcdef0"  # 17 characters after vpc-
```

## Step 7: Configure Terraform Backend (Optional)

For team environments, configure S3 backend for Terraform state:

### Create S3 Bucket and DynamoDB Table

```bash
# Create S3 bucket for state
aws s3api create-bucket \
  --bucket my-org-terraform-state \
  --region us-east-1

# Enable versioning
aws s3api put-bucket-versioning \
  --bucket my-org-terraform-state \
  --versioning-configuration Status=Enabled

# Create DynamoDB table for locking
aws dynamodb create-table \
  --table-name terraform-state-lock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

### Configure Backend in Cluster Config

```yaml
opentofu:
  enabled: true
  backend:
    type: "s3"
    s3:
      bucket: "my-org-terraform-state"
      key: "clusters/my-aws-cluster/terraform.tfstate"
      region: "us-east-1"
      dynamodb_table: "terraform-state-lock"
      encrypt: true
```

## Common Configurations

### Development Cluster (Minimal Cost)

Single master, minimal workers for testing:

```yaml
opencenter:
  cluster:
    kubernetes:
      master_count: 1
      worker_count: 1
      flavor_master: "t3.small"
      flavor_worker: "t3.medium"
  
  storage:
    worker_volume_size: 20
    worker_volume_type: "gp3"
```

**Estimated cost**: ~$50-70/month

### Production Cluster (High Availability)

Three masters, multiple workers, multi-AZ:

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: "vpc-0123456789abcdef0"
        private_subnets:
          - "10.0.1.0/24"   # us-east-1a
          - "10.0.2.0/24"   # us-east-1b
          - "10.0.3.0/24"   # us-east-1c
        public_subnets:
          - "10.0.101.0/24" # us-east-1a
          - "10.0.102.0/24" # us-east-1b
          - "10.0.103.0/24" # us-east-1c
  
  cluster:
    kubernetes:
      master_count: 3
      worker_count: 5
      flavor_master: "t3.large"
      flavor_worker: "t3.xlarge"
  
  storage:
    worker_volume_size: 100
    worker_volume_type: "gp3"
```

**Estimated cost**: ~$400-600/month

### High-Security Cluster (Restricted Access)

Private cluster with bastion host and restricted API access:

```yaml
opencenter:
  cluster:
    networking:
      k8s_api_port_acl:
        - "10.0.0.0/8"      # Internal network only
        - "203.0.113.0/24"  # Office network
      security:
        os_hardening: true
    
    kubernetes:
      security:
        k8s_hardening: true
        pod_security_exemptions:
          - "kube-system"
```

## Troubleshooting

### Authentication Failures

**Symptom**: `Unable to locate credentials`

**Solution**: Verify AWS credentials are configured:

```bash
# Check environment variables
env | grep AWS

# Check AWS CLI configuration
aws configure list

# Test credentials
aws sts get-caller-identity
```

### Insufficient IAM Permissions

**Symptom**: `AccessDenied` errors during validation or deployment

**Solution**: Review IAM permissions. See [IAM Configuration Guide](iam.md) for required policies.

```bash
# Check current user permissions
aws iam get-user-policy \
  --user-name opencenter-provisioner \
  --policy-name OpenCenterProvisioning
```

### VPC Not Found

**Symptom**: `VPC vpc-xxx not found`

**Solution**: Verify VPC exists and you have access:

```bash
aws ec2 describe-vpcs --vpc-ids vpc-0123456789abcdef0
```

### Subnet CIDR Conflicts

**Symptom**: Validation fails with overlapping subnets

**Solution**: Use a subnet calculator to plan non-overlapping CIDRs:

```bash
# Check existing subnets in VPC
aws ec2 describe-subnets \
  --filters "Name=vpc-id,Values=vpc-0123456789abcdef0" \
  --query 'Subnets[*].[SubnetId,CidrBlock]' \
  --output table
```

### Region Not Available

**Symptom**: `Region us-east-1 not available`

**Solution**: Check AWS service health and your account's enabled regions:

```bash
# List enabled regions
aws ec2 describe-regions --query 'Regions[*].RegionName' --output table

# Check service availability
aws ec2 describe-availability-zones --region us-east-1
```

## Next Steps

After validating your configuration:

1. **Generate GitOps repository**: Run `openCenter cluster setup` to create infrastructure manifests
2. **Review Terraform configuration**: Check generated `main.tf` in your GitOps repository
3. **Deploy infrastructure**: Run `terraform apply` to provision AWS resources
4. **Install Kubernetes**: Follow Kubernetes installation guide (TBD)

## Related Documentation

- [IAM Configuration Guide](iam.md) - Detailed IAM setup and policies
- [VPC Design Guide](vpc.md) - Network architecture and planning
- [Troubleshooting Guide](troubleshooting.md) - Common AWS issues
- [AWS Provider Overview](README.md) - Provider features and architecture
- [Configuration Reference](../../reference/configuration.md) - Complete configuration options

---

**Last Updated**: January 2025  
**Maintained By**: openCenter Team
