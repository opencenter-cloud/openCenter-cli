# AWS VPC Design


## Table of Contents

- [Purpose](#purpose)
- [VPC Architecture Overview](#vpc-architecture-overview)
- [CIDR Block Planning](#cidr-block-planning)
- [Multi-AZ Deployment](#multi-az-deployment)
- [Routing and Gateways](#routing-and-gateways)
- [Security Groups](#security-groups)
- [Network Performance](#network-performance)
- [VPC Peering and Connectivity](#vpc-peering-and-connectivity)
- [Cost Optimization](#cost-optimization)
- [Troubleshooting Network Issues](#troubleshooting-network-issues)
- [Best Practices Summary](#best-practices-summary)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: explanation**

This document explains VPC (Virtual Private Cloud) networking concepts for Kubernetes clusters on AWS. It covers network architecture, subnet design, routing, security groups, and best practices for production deployments.

## Purpose

Understanding VPC design helps you plan network topology that balances security, performance, and cost. This guide explains how openCenter structures AWS networks and how to customize them for your requirements.

## VPC Architecture Overview

### Network Layers

A Kubernetes cluster on AWS uses a multi-tier network architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                      Internet                                │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ Internet Gateway
                         │
┌────────────────────────▼────────────────────────────────────┐
│                   Public Subnets                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ NAT Gateway  │  │ NAT Gateway  │  │ NAT Gateway  │      │
│  │   AZ-a       │  │   AZ-b       │  │   AZ-c       │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │              │
│  ┌──────▼───────┐  ┌──────▼───────┐  ┌──────▼───────┐      │
│  │ Load Balancer│  │ Load Balancer│  │ Load Balancer│      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ Route Tables
                         │
┌────────────────────────▼────────────────────────────────────┐
│                  Private Subnets                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Master 1    │  │  Master 2    │  │  Master 3    │      │
│  │   AZ-a       │  │   AZ-b       │  │   AZ-c       │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Worker 1    │  │  Worker 2    │  │  Worker 3    │      │
│  │   AZ-a       │  │   AZ-b       │  │   AZ-c       │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ Pod Network (CNI)
                         │
┌────────────────────────▼────────────────────────────────────┐
│                    Pod Network                               │
│  10.42.0.0/16 - Kubernetes Pods                             │
│  10.43.0.0/16 - Kubernetes Services                         │
└─────────────────────────────────────────────────────────────┘
```

### Network Components

**VPC (Virtual Private Cloud):**
- Isolated network environment
- Configurable CIDR block (e.g., 10.0.0.0/16)
- Spans all availability zones in a region
- Contains subnets, route tables, and gateways

**Public Subnets:**
- Direct route to Internet Gateway
- Hosts NAT Gateways for private subnet egress
- Hosts load balancers for external access
- Instances receive public IP addresses

**Private Subnets:**
- No direct internet access
- Routes through NAT Gateway for egress
- Hosts cluster nodes (masters and workers)
- Enhanced security through isolation

**Internet Gateway:**
- Provides internet connectivity for VPC
- Attached to VPC
- Used by public subnets

**NAT Gateway:**
- Enables private subnet instances to access internet
- Deployed in public subnets
- High availability with one per AZ
- Managed service (no maintenance required)

## CIDR Block Planning

### VPC CIDR Selection

Choose a VPC CIDR block that:
- Doesn't overlap with on-premises networks
- Doesn't overlap with other VPCs (if peering)
- Provides sufficient IP addresses for growth
- Follows RFC 1918 private address space

**Recommended VPC CIDRs:**

| Use Case | CIDR Block | Total IPs | Usable IPs |
|----------|------------|-----------|------------|
| Small | 10.0.0.0/16 | 65,536 | 65,531 |
| Medium | 10.0.0.0/14 | 262,144 | 262,139 |
| Large | 10.0.0.0/12 | 1,048,576 | 1,048,571 |

**Example VPC CIDR:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_cidr: "10.0.0.0/16"  # 65,536 IP addresses
```

### Subnet CIDR Allocation

Divide VPC CIDR into subnets across availability zones:

**Example for 10.0.0.0/16 VPC:**

| Subnet Type | AZ | CIDR | IPs | Purpose |
|-------------|----|----- |-----|---------|
| Public | us-east-1a | 10.0.1.0/24 | 256 | NAT, LB |
| Public | us-east-1b | 10.0.2.0/24 | 256 | NAT, LB |
| Public | us-east-1c | 10.0.3.0/24 | 256 | NAT, LB |
| Private | us-east-1a | 10.0.11.0/24 | 256 | Nodes |
| Private | us-east-1b | 10.0.12.0/24 | 256 | Nodes |
| Private | us-east-1c | 10.0.13.0/24 | 256 | Nodes |
| Reserved | - | 10.0.0.0/24 | 256 | Future use |
| Reserved | - | 10.0.4.0/22 | 1,024 | Future use |

**Configuration:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: ""  # Auto-create
        public_subnets:
          - "10.0.1.0/24"
          - "10.0.2.0/24"
          - "10.0.3.0/24"
        private_subnets:
          - "10.0.11.0/24"
          - "10.0.12.0/24"
          - "10.0.13.0/24"
```

### Kubernetes Network Planning

Kubernetes uses separate CIDR blocks for pods and services:

**Pod Network:**
- Default: 10.42.0.0/16 (65,536 IPs)
- Must not overlap with VPC CIDR
- Size based on: nodes × pods-per-node

**Service Network:**
- Default: 10.43.0.0/16 (65,536 IPs)
- Must not overlap with VPC or pod CIDR
- Size based on: expected number of services

**Example:**
```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # 65k pod IPs
      subnet_services: "10.43.0.0/16"  # 65k service IPs
```

### CIDR Overlap Detection

openCenter validates that CIDRs don't overlap:

```bash
# Validation checks for:
# - VPC CIDR vs pod network
# - VPC CIDR vs service network
# - Public subnet overlap
# - Private subnet overlap
# - Subnet vs pod/service networks

mise run build
./bin/openCenter cluster validate my-aws-cluster
```

## Multi-AZ Deployment

### Availability Zone Strategy

Distribute resources across multiple AZs for high availability:

**Benefits:**
- Survive AZ failures
- Reduce latency with local resources
- Meet compliance requirements
- Improve application resilience

**Recommendations:**
- Use at least 2 AZs (3 recommended)
- Distribute master nodes evenly
- Distribute worker nodes evenly
- Place NAT Gateway in each AZ

**Example 3-AZ Configuration:**
```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"
        availability_zones:
          - "us-east-1a"
          - "us-east-1b"
          - "us-east-1c"
        public_subnets:
          - "10.0.1.0/24"   # AZ-a
          - "10.0.2.0/24"   # AZ-b
          - "10.0.3.0/24"   # AZ-c
        private_subnets:
          - "10.0.11.0/24"  # AZ-a
          - "10.0.12.0/24"  # AZ-b
          - "10.0.13.0/24"  # AZ-c
  
  cluster:
    kubernetes:
      master_count: 3  # One per AZ
      worker_count: 6  # Two per AZ
```

### AZ Failure Scenarios

**Single AZ Failure:**
- 2/3 masters remain (quorum maintained)
- 2/3 workers remain (workloads continue)
- API server remains available
- etcd maintains quorum

**Two AZ Failure (rare):**
- 1/3 masters remain (no quorum)
- 1/3 workers remain (degraded capacity)
- API server unavailable
- Manual intervention required

## Routing and Gateways

### Route Tables

Each subnet has an associated route table:

**Public Subnet Route Table:**
```
Destination       Target
10.0.0.0/16       local
0.0.0.0/0         igw-xxxxx (Internet Gateway)
```

**Private Subnet Route Table (per AZ):**
```
Destination       Target
10.0.0.0/16       local
0.0.0.0/0         nat-xxxxx (NAT Gateway in same AZ)
```

### Internet Gateway

Single Internet Gateway per VPC:
- Provides internet connectivity
- Attached to VPC
- Used by public subnets
- No bandwidth limits
- No additional cost

### NAT Gateway

One NAT Gateway per availability zone:

**Benefits:**
- High availability (managed service)
- Automatic scaling (up to 45 Gbps)
- No maintenance required
- Elastic IP per NAT Gateway

**Cost Considerations:**
- Hourly charge per NAT Gateway
- Data processing charges
- Consider NAT instance for cost savings (not recommended for production)

**Configuration:**
```yaml
# openCenter creates NAT Gateway automatically
# One per public subnet (one per AZ)
opencenter:
  infrastructure:
    cloud:
      aws:
        public_subnets:
          - "10.0.1.0/24"  # NAT Gateway created here
          - "10.0.2.0/24"  # NAT Gateway created here
          - "10.0.3.0/24"  # NAT Gateway created here
```

## Security Groups

### Security Group Architecture

Security groups act as virtual firewalls:

**Cluster Security Groups:**
- Master security group (control plane)
- Worker security group (data plane)
- Load balancer security group (ingress)

**Default Rules:**

**Master Security Group:**
```
Inbound:
- TCP 6443 from 0.0.0.0/0 (API server)
- TCP 2379-2380 from master SG (etcd)
- TCP 10250 from master/worker SG (kubelet)
- All traffic from master SG (inter-master)

Outbound:
- All traffic to 0.0.0.0/0
```

**Worker Security Group:**
```
Inbound:
- TCP 10250 from master SG (kubelet)
- TCP 30000-32767 from LB SG (NodePort)
- All traffic from worker SG (inter-worker)

Outbound:
- All traffic to 0.0.0.0/0
```

**Load Balancer Security Group:**
```
Inbound:
- TCP 443 from 0.0.0.0/0 (HTTPS)
- TCP 80 from 0.0.0.0/0 (HTTP)

Outbound:
- TCP 30000-32767 to worker SG (NodePort)
```

### Security Group Best Practices

**Principle of Least Privilege:**
- Allow only required ports
- Restrict source IPs when possible
- Use security group references (not CIDRs)
- Regularly audit rules

**Example Restricted API Access:**
```yaml
opencenter:
  cluster:
    networking:
      k8s_api_port_acl:
        - "10.0.0.0/8"      # Corporate network
        - "203.0.113.0/24"  # Office network
        # Not 0.0.0.0/0
```

**Security Group Limits:**
- 60 inbound rules per security group
- 60 outbound rules per security group
- 5 security groups per network interface
- 2,500 security groups per VPC (default)

## Network Performance

### Bandwidth Considerations

**Instance Network Performance:**
- Varies by instance type
- Up to 100 Gbps for largest instances
- Enhanced networking recommended

**Network Bandwidth by Instance Type:**

| Instance Type | Network Performance | Use Case |
|---------------|---------------------|----------|
| t3.small | Up to 5 Gbps | Dev/test |
| t3.medium | Up to 5 Gbps | Small workloads |
| t3.large | Up to 5 Gbps | General purpose |
| m5.large | Up to 10 Gbps | Production |
| m5.xlarge | Up to 10 Gbps | Production |
| c5.2xlarge | Up to 10 Gbps | Compute intensive |
| c5.4xlarge | Up to 10 Gbps | Compute intensive |

**NAT Gateway Bandwidth:**
- Up to 45 Gbps per NAT Gateway
- Scales automatically
- No configuration required

### Latency Optimization

**Same-AZ Communication:**
- Lowest latency (< 1ms)
- No data transfer charges
- Preferred for latency-sensitive workloads

**Cross-AZ Communication:**
- Higher latency (1-2ms)
- Data transfer charges apply ($0.01/GB)
- Required for high availability

**Pod-to-Pod Communication:**
- Depends on CNI plugin
- AWS VPC CNI: native VPC routing (lowest latency)
- Overlay networks: additional encapsulation overhead

### MTU Configuration

**Maximum Transmission Unit:**
- Default: 1500 bytes (standard Ethernet)
- Jumbo frames: 9001 bytes (within VPC)
- Affects throughput for large transfers

**When to Use Jumbo Frames:**
- High-throughput workloads
- Large file transfers
- Database replication
- Storage traffic

**Configuration:**
```yaml
# Note: MTU configuration is CNI-specific
# AWS VPC CNI uses VPC MTU automatically
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        aws_vpc_cni:
          enabled: true
          # MTU inherited from VPC (9001 within VPC)
```

## VPC Peering and Connectivity

### VPC Peering (Planned)

Connect multiple VPCs:

**Use Cases:**
- Multi-cluster networking
- Shared services VPC
- Development/staging/production isolation
- Cross-region connectivity

**Limitations:**
- No transitive peering
- CIDR blocks must not overlap
- Route table updates required
- Cross-region peering has higher latency

### VPN Connectivity (Planned)

Connect VPC to on-premises networks:

**AWS Site-to-Site VPN:**
- IPsec VPN tunnels
- Up to 1.25 Gbps per tunnel
- Redundant tunnels for HA
- Lower cost than Direct Connect

**AWS Direct Connect:**
- Dedicated network connection
- 1 Gbps to 100 Gbps
- Lower latency than VPN
- Higher cost, longer setup time

### PrivateLink (Planned)

Access AWS services privately:

**Benefits:**
- No internet gateway required
- Traffic stays on AWS network
- Enhanced security
- Simplified network architecture

**Supported Services:**
- S3 (via Gateway Endpoint)
- DynamoDB (via Gateway Endpoint)
- ECR, ECS, EKS (via Interface Endpoint)
- Many other AWS services

## Cost Optimization

### Network Cost Factors

**Data Transfer Costs:**
- Internet egress: $0.09/GB (first 10 TB)
- Cross-AZ transfer: $0.01/GB each direction
- Same-AZ transfer: Free
- VPC peering: $0.01/GB

**NAT Gateway Costs:**
- Hourly charge: $0.045/hour per NAT Gateway
- Data processing: $0.045/GB
- 3 NAT Gateways (HA): ~$100/month + data

**Load Balancer Costs:**
- Application LB: $0.0225/hour + LCU charges
- Network LB: $0.0225/hour + NLCU charges

### Cost Reduction Strategies

**Single NAT Gateway (Non-Production):**
```yaml
# Use one NAT Gateway for all AZs
# Saves ~$65/month but reduces availability
# NOT recommended for production
opencenter:
  infrastructure:
    cloud:
      aws:
        single_nat_gateway: true  # Planned feature
```

**VPC Endpoints for AWS Services:**
```yaml
# Use VPC endpoints to avoid NAT Gateway data charges
# S3 and DynamoDB endpoints are free
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_endpoints:
          - "s3"
          - "dynamodb"
          - "ecr.api"
          - "ecr.dkr"
```

**Right-Size Subnets:**
- Don't over-allocate IP addresses
- Smaller subnets = more efficient use
- Plan for growth but avoid waste

## Troubleshooting Network Issues

### Connectivity Testing

**Test internet connectivity from private subnet:**
```bash
# SSH to instance in private subnet (via bastion)
curl -I https://www.google.com

# Should succeed via NAT Gateway
```

**Test cross-AZ connectivity:**
```bash
# From instance in AZ-a
ping <instance-ip-in-az-b>

# Should succeed (1-2ms latency)
```

**Test security group rules:**
```bash
# Check if port is accessible
nc -zv <instance-ip> 6443

# Check from specific source
aws ec2 describe-security-groups \
  --group-ids sg-xxxxx \
  --query 'SecurityGroups[0].IpPermissions'
```

### Common Issues

**Issue: Cannot reach internet from private subnet**
- Check NAT Gateway exists and is available
- Verify route table has 0.0.0.0/0 → NAT Gateway
- Check security group allows outbound traffic
- Verify NAT Gateway has Elastic IP

**Issue: Cross-AZ communication fails**
- Check security groups allow traffic between AZs
- Verify route tables have local routes
- Check network ACLs (if customized)

**Issue: High data transfer costs**
- Review cross-AZ traffic patterns
- Consider VPC endpoints for AWS services
- Optimize application architecture
- Use CloudWatch to identify sources

## Best Practices Summary

**Network Design:**
1. Use at least 3 availability zones for production
2. Plan CIDR blocks to avoid overlaps
3. Reserve IP space for future growth
4. Use private subnets for cluster nodes
5. Deploy NAT Gateway in each AZ

**Security:**
1. Restrict API server access with security groups
2. Use security group references (not CIDRs)
3. Enable VPC Flow Logs for audit
4. Regularly review security group rules
5. Use network ACLs for additional defense

**Performance:**
1. Choose appropriate instance types for network requirements
2. Use Enhanced Networking when available
3. Consider placement groups for low-latency workloads
4. Monitor network metrics in CloudWatch
5. Use VPC endpoints to reduce NAT Gateway traffic

**Cost:**
1. Use single NAT Gateway for non-production (with caution)
2. Implement VPC endpoints for AWS services
3. Monitor data transfer costs
4. Right-size subnets and instances
5. Use Reserved Instances for predictable workloads

## Related Documentation

- [AWS Setup Guide](setup.md) - Complete setup instructions
- [IAM Configuration](iam.md) - IAM roles and policies
- [Troubleshooting Guide](troubleshooting.md) - Common AWS issues
- [AWS Provider Overview](README.md) - Provider features

## External Resources

- [AWS VPC Documentation](https://docs.aws.amazon.com/vpc/)
- [VPC Design Best Practices](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-network-design.html)
- [AWS Network Performance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html)
- [VPC Pricing](https://aws.amazon.com/vpc/pricing/)

---

**Last Updated**: January 2025  
**Maintained By**: openCenter Team
