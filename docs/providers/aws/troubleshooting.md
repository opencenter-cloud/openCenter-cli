# AWS Troubleshooting Guide


## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Authentication and Credentials](#authentication-and-credentials)
- [VPC and Networking Issues](#vpc-and-networking-issues)
- [EC2 Instance Issues](#ec2-instance-issues)
- [Terraform/OpenTofu Issues](#terraformopentofu-issues)
- [Configuration Validation Errors](#configuration-validation-errors)
- [Cost and Billing Issues](#cost-and-billing-issues)
- [Getting Help](#getting-help)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: how-to**

This guide provides solutions to common AWS-specific issues encountered when deploying and managing Kubernetes clusters with openCenter.

## Quick Diagnostics

### Run Validation Checks

Before troubleshooting, validate your configuration:

```bash
# Build openCenter
mise run build

# Validate cluster configuration
./bin/openCenter cluster validate my-aws-cluster
```

**Validation checks:**
- ✅ Schema compliance
- ✅ AWS region format
- ✅ VPC and subnet configuration
- ✅ CIDR overlap detection
- ✅ Credential format validation
- ✅ Required fields

### Check AWS Connectivity

```bash
# Verify AWS CLI is installed
aws --version

# Test AWS credentials
aws sts get-caller-identity

# Check region configuration
aws configure get region

# List available regions
aws ec2 describe-regions --query 'Regions[*].RegionName' --output table
```

## Authentication and Credentials

### Problem: "Unable to locate credentials"

**Symptom:**
```
Error: NoCredentialProviders: no valid providers in chain
Unable to locate credentials
```

**Cause:** AWS credentials not configured or not accessible.

**Solution:**

1. **Check environment variables:**

```bash
# Verify AWS environment variables are set
env | grep AWS

# Should show:
# AWS_ACCESS_KEY_ID=AKIA...
# AWS_SECRET_ACCESS_KEY=...
# AWS_DEFAULT_REGION=us-east-1
```

2. **Check AWS CLI configuration:**

```bash
# View current configuration
aws configure list

# Should show:
#       Name                    Value             Type    Location
#       ----                    -----             ----    --------
#    profile                <not set>             None    None
# access_key     ****************AMPLE shared-credentials-file
# secret_key     ****************AMPLE shared-credentials-file
#     region                us-east-1      config-file    ~/.aws/config
```

3. **Configure credentials:**

```bash
# Option A: Set environment variables
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_DEFAULT_REGION="us-east-1"

# Option B: Configure AWS CLI
aws configure

# Option C: Use AWS profile
export AWS_PROFILE=opencenter-admin
```

4. **Test credentials:**

```bash
aws sts get-caller-identity
```

### Problem: "AccessDenied" or "UnauthorizedOperation"

**Symptom:**
```
Error: UnauthorizedOperation: You are not authorized to perform this operation
Status Code: 403
```

**Cause:** Insufficient IAM permissions.

**Solution:**

1. **Check current user/role:**

```bash
# Get current identity
aws sts get-caller-identity

# Output shows:
# {
#     "UserId": "AIDAI...",
#     "Account": "123456789012",
#     "Arn": "arn:aws:iam::123456789012:user/opencenter-provisioner"
# }
```

2. **List attached policies:**

```bash
# For IAM user
aws iam list-attached-user-policies --user-name opencenter-provisioner

# For IAM role
aws iam list-attached-role-policies --role-name opencenter-provisioner-role
```

3. **Review required permissions:**

See [IAM Configuration Guide](iam.md) for complete policy requirements.

4. **Attach missing permissions:**

```bash
# Attach AWS managed policy (for testing only)
aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::aws:policy/PowerUserAccess

# Or attach custom policy
aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::123456789012:policy/OpenCenterProvisioning
```

### Problem: "InvalidClientTokenId"

**Symptom:**
```
Error: InvalidClientTokenId: The security token included in the request is invalid
Status Code: 403
```

**Cause:** Access key is invalid, deleted, or belongs to different account.

**Solution:**

1. **Verify access key exists:**

```bash
# List access keys for user
aws iam list-access-keys --user-name opencenter-provisioner
```

2. **Create new access key:**

```bash
# Create new access key
aws iam create-access-key --user-name opencenter-provisioner

# Update configuration with new key
export AWS_ACCESS_KEY_ID="<new-access-key-id>"
export AWS_SECRET_ACCESS_KEY="<new-secret-access-key>"
```

3. **Delete old access key:**

```bash
# Delete old key
aws iam delete-access-key \
  --user-name opencenter-provisioner \
  --access-key-id AKIAIOSFODNN7EXAMPLE
```

### Problem: "SignatureDoesNotMatch"

**Symptom:**
```
Error: SignatureDoesNotMatch: The request signature we calculated does not match the signature you provided
```

**Cause:** Secret access key is incorrect or system clock is out of sync.

**Solution:**

1. **Check system time:**

```bash
# Check current time
date

# Sync with NTP (Linux)
sudo ntpdate -s time.nist.gov

# Or use systemd-timesyncd
sudo timedatectl set-ntp true
```

2. **Verify secret access key:**

```bash
# Create new access key pair
aws iam create-access-key --user-name opencenter-provisioner

# Update configuration
```

## VPC and Networking Issues

### Problem: "VPC not found"

**Symptom:**
```
Error: InvalidVpcID.NotFound: The vpc ID 'vpc-xxxxx' does not exist
```

**Solution:**

1. **List available VPCs:**

```bash
# List all VPCs
aws ec2 describe-vpcs \
  --query 'Vpcs[*].[VpcId,CidrBlock,Tags[?Key==`Name`].Value|[0]]' \
  --output table
```

2. **Verify VPC ID in configuration:**

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: "vpc-0123456789abcdef0"  # Use correct VPC ID
```

3. **Check region:**

```bash
# VPC might be in different region
aws ec2 describe-vpcs --region us-west-2
```

4. **Create new VPC (if needed):**

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: ""  # Empty = auto-create VPC
```

### Problem: "Subnet CIDR overlap"

**Symptom:**
```
Error: subnets overlap: 10.0.1.0/24 and 10.0.1.0/25
```

**Cause:** Subnet CIDRs overlap with each other or with pod/service networks.

**Solution:**

1. **Review CIDR allocation:**

```bash
# Check existing subnets in VPC
aws ec2 describe-subnets \
  --filters "Name=vpc-id,Values=vpc-xxxxx" \
  --query 'Subnets[*].[SubnetId,CidrBlock,AvailabilityZone]' \
  --output table
```

2. **Use non-overlapping CIDRs:**

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        private_subnets:
          - "10.0.1.0/24"   # 10.0.1.0 - 10.0.1.255
          - "10.0.2.0/24"   # 10.0.2.0 - 10.0.2.255
          - "10.0.3.0/24"   # 10.0.3.0 - 10.0.3.255
        public_subnets:
          - "10.0.101.0/24" # 10.0.101.0 - 10.0.101.255
          - "10.0.102.0/24" # 10.0.102.0 - 10.0.102.255
          - "10.0.103.0/24" # 10.0.103.0 - 10.0.103.255
```

3. **Verify pod/service networks don't overlap:**

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # Must not overlap with VPC
      subnet_services: "10.43.0.0/16"  # Must not overlap with VPC
```

### Problem: "Cannot reach internet from private subnet"

**Symptom:**
- Instances in private subnet cannot access internet
- `curl` commands timeout
- Package installation fails

**Diagnosis:**

```bash
# SSH to instance in private subnet (via bastion)
ssh -J ubuntu@<bastion-ip> ubuntu@<private-instance-ip>

# Test internet connectivity
curl -I https://www.google.com
# Should succeed but times out
```

**Solution:**

1. **Check NAT Gateway exists:**

```bash
# List NAT Gateways
aws ec2 describe-nat-gateways \
  --filter "Name=vpc-id,Values=vpc-xxxxx" \
  --query 'NatGateways[*].[NatGatewayId,State,SubnetId]' \
  --output table
```

2. **Verify NAT Gateway is available:**

```bash
# Check NAT Gateway state
aws ec2 describe-nat-gateways \
  --nat-gateway-ids nat-xxxxx \
  --query 'NatGateways[0].State'

# Should return: "available"
```

3. **Check route table:**

```bash
# Get route table for private subnet
aws ec2 describe-route-tables \
  --filters "Name=association.subnet-id,Values=subnet-xxxxx" \
  --query 'RouteTables[0].Routes' \
  --output table

# Should have route: 0.0.0.0/0 → nat-xxxxx
```

4. **Verify Elastic IP attached to NAT Gateway:**

```bash
# Check NAT Gateway has Elastic IP
aws ec2 describe-nat-gateways \
  --nat-gateway-ids nat-xxxxx \
  --query 'NatGateways[0].NatGatewayAddresses[0].PublicIp'
```

5. **Check security group allows outbound:**

```bash
# Check security group egress rules
aws ec2 describe-security-groups \
  --group-ids sg-xxxxx \
  --query 'SecurityGroups[0].IpPermissionsEgress'

# Should allow 0.0.0.0/0
```

### Problem: "Security group rule limit exceeded"

**Symptom:**
```
Error: RulesPerSecurityGroupLimitExceeded: The maximum number of rules per security group has been reached
```

**Cause:** Security group has more than 60 inbound or 60 outbound rules.

**Solution:**

1. **Check current rule count:**

```bash
# Count inbound rules
aws ec2 describe-security-groups \
  --group-ids sg-xxxxx \
  --query 'length(SecurityGroups[0].IpPermissions)'

# Count outbound rules
aws ec2 describe-security-groups \
  --group-ids sg-xxxxx \
  --query 'length(SecurityGroups[0].IpPermissionsEgress)'
```

2. **Consolidate rules:**

```bash
# Instead of multiple rules for individual IPs:
# 203.0.113.1/32, 203.0.113.2/32, 203.0.113.3/32
# Use CIDR block:
# 203.0.113.0/24

# Remove individual IP rules
aws ec2 revoke-security-group-ingress \
  --group-id sg-xxxxx \
  --ip-permissions IpProtocol=tcp,FromPort=22,ToPort=22,IpRanges='[{CidrIp=203.0.113.1/32}]'

# Add consolidated rule
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxxxx \
  --ip-permissions IpProtocol=tcp,FromPort=22,ToPort=22,IpRanges='[{CidrIp=203.0.113.0/24}]'
```

3. **Use security group references:**

```bash
# Instead of CIDR blocks, reference other security groups
aws ec2 authorize-security-group-ingress \
  --group-id sg-worker \
  --ip-permissions IpProtocol=tcp,FromPort=10250,ToPort=10250,UserIdGroupPairs='[{GroupId=sg-master}]'
```

## EC2 Instance Issues

### Problem: "InsufficientInstanceCapacity"

**Symptom:**
```
Error: InsufficientInstanceCapacity: We currently do not have sufficient capacity in the Availability Zone you requested
```

**Cause:** AWS temporarily out of capacity for requested instance type in specific AZ.

**Solution:**

1. **Try different availability zone:**

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        availability_zones:
          - "us-east-1b"  # Try different AZ
          - "us-east-1c"
          - "us-east-1d"
```

2. **Try different instance type:**

```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_master: "t3.large"   # Instead of t3.medium
      flavor_worker: "t3.xlarge"  # Instead of t3.large
```

3. **Wait and retry:**

```bash
# Capacity issues are usually temporary
# Wait 15-30 minutes and retry
sleep 1800
terraform apply
```

4. **Request capacity reservation (for production):**

```bash
# Create capacity reservation
aws ec2 create-capacity-reservation \
  --instance-type t3.medium \
  --instance-platform Linux/UNIX \
  --availability-zone us-east-1a \
  --instance-count 5
```

### Problem: "InstanceLimitExceeded"

**Symptom:**
```
Error: InstanceLimitExceeded: You have requested more instances than your current instance limit allows
```

**Cause:** Account has reached EC2 instance limit for instance type.

**Solution:**

1. **Check current limits:**

```bash
# Check vCPU limits
aws service-quotas get-service-quota \
  --service-code ec2 \
  --quota-code L-1216C47A \
  --region us-east-1

# Check instance limits
aws ec2 describe-account-attributes \
  --attribute-names max-instances
```

2. **Request limit increase:**

```bash
# Request quota increase via AWS Console:
# Service Quotas → AWS services → Amazon Elastic Compute Cloud (Amazon EC2)
# → Running On-Demand Standard instances → Request quota increase

# Or use CLI
aws service-quotas request-service-quota-increase \
  --service-code ec2 \
  --quota-code L-1216C47A \
  --desired-value 100 \
  --region us-east-1
```

3. **Use different instance types:**

```yaml
# Different instance families have separate limits
opencenter:
  cluster:
    kubernetes:
      flavor_master: "m5.large"   # M family instead of T family
      flavor_worker: "m5.xlarge"
```

### Problem: "Instance fails to start"

**Symptom:**
- Instance stuck in "pending" state
- Instance starts then immediately stops
- Status checks fail

**Diagnosis:**

```bash
# Check instance status
aws ec2 describe-instances \
  --instance-ids i-xxxxx \
  --query 'Reservations[0].Instances[0].State'

# Check status checks
aws ec2 describe-instance-status \
  --instance-ids i-xxxxx

# Get system log
aws ec2 get-console-output \
  --instance-id i-xxxxx \
  --output text
```

**Common Causes and Solutions:**

1. **Invalid AMI:**

```bash
# Verify AMI exists and is available
aws ec2 describe-images --image-ids ami-xxxxx

# Use different AMI
opencenter:
  infrastructure:
    cloud:
      aws:
        ami_id: "ami-0c55b159cbfafe1f0"  # Ubuntu 24.04
```

2. **Insufficient EBS volume size:**

```yaml
opencenter:
  storage:
    worker_volume_size: 40  # Increase from 20
```

3. **User data script errors:**

```bash
# Check cloud-init logs (after SSH access)
ssh ubuntu@<instance-ip>
sudo cat /var/log/cloud-init-output.log
```

## Terraform/OpenTofu Issues

### Problem: "Error locking state"

**Symptom:**
```
Error: Error acquiring the state lock
Lock Info:
  ID:        xxxxx
  Path:      terraform.tfstate
  Operation: OperationTypeApply
  Who:       user@hostname
  Version:   1.5.0
  Created:   2025-01-15 10:30:00
```

**Cause:** Previous Terraform operation was interrupted or another user is running Terraform.

**Solution:**

1. **Check if Terraform is actually running:**

```bash
# Check for terraform processes
ps aux | grep terraform

# If no processes, state is stale
```

2. **Force unlock (if safe):**

```bash
# Get lock ID from error message
terraform force-unlock xxxxx

# Confirm unlock
```

3. **Use DynamoDB locking (recommended):**

```yaml
opentofu:
  backend:
    type: "s3"
    s3:
      bucket: "my-org-terraform-state"
      key: "clusters/my-cluster/terraform.tfstate"
      region: "us-east-1"
      dynamodb_table: "terraform-state-lock"  # Prevents concurrent access
      encrypt: true
```

### Problem: "Resource already exists"

**Symptom:**
```
Error: Error creating VPC: VpcLimitExceeded: The maximum number of VPCs has been reached
```

**Solution:**

1. **Use existing VPC:**

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: "vpc-existing123"  # Use existing VPC
```

2. **Delete unused VPCs:**

```bash
# List VPCs
aws ec2 describe-vpcs \
  --query 'Vpcs[*].[VpcId,CidrBlock,Tags[?Key==`Name`].Value|[0]]' \
  --output table

# Delete unused VPC
aws ec2 delete-vpc --vpc-id vpc-xxxxx
```

3. **Import existing resource:**

```bash
# Import VPC into Terraform state
terraform import aws_vpc.main vpc-xxxxx
```

### Problem: "State file corruption"

**Symptom:**
```
Error: Failed to load state: state snapshot was created by Terraform v1.6.0, which is newer than current v1.5.0
```

**Solution:**

1. **Upgrade Terraform/OpenTofu:**

```bash
# Check current version
terraform version

# Upgrade to required version
mise install terraform@1.6.0
```

2. **Restore from backup:**

```bash
# S3 backend with versioning enabled
aws s3api list-object-versions \
  --bucket my-org-terraform-state \
  --prefix clusters/my-cluster/terraform.tfstate

# Restore previous version
aws s3api get-object \
  --bucket my-org-terraform-state \
  --key clusters/my-cluster/terraform.tfstate \
  --version-id <version-id> \
  terraform.tfstate
```

## Configuration Validation Errors

### Problem: "Invalid AWS region format"

**Symptom:**
```
Error: invalid AWS region format: us-east1
```

**Solution:**

Use correct region format with hyphen:

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        region: "us-east-1"  # Not "us-east1"
```

**Valid AWS regions:**
- us-east-1, us-east-2, us-west-1, us-west-2
- eu-west-1, eu-west-2, eu-west-3, eu-central-1
- ap-southeast-1, ap-southeast-2, ap-northeast-1

### Problem: "Invalid VPC ID format"

**Symptom:**
```
Error: invalid VPC ID format: vpc-123
```

**Solution:**

Use full VPC ID (17 characters after "vpc-"):

```yaml
opencenter:
  infrastructure:
    cloud:
      aws:
        vpc_id: "vpc-0123456789abcdef0"  # 17 hex characters
```

### Problem: "Required field missing"

**Symptom:**
```
Error: opencenter.infrastructure.cloud.aws.region is required
```

**Solution:**

Add required fields to configuration:

```yaml
opencenter:
  infrastructure:
    provider: aws
    cloud:
      aws:
        region: "us-east-1"  # Required

secrets:
  global:
    aws:
      infrastructure:
        access_key: "${AWS_ACCESS_KEY_ID}"        # Required
        secret_access_key: "${AWS_SECRET_ACCESS_KEY}"  # Required
```

## Cost and Billing Issues

### Problem: "Unexpected high costs"

**Diagnosis:**

```bash
# Check current month costs
aws ce get-cost-and-usage \
  --time-period Start=2025-01-01,End=2025-01-31 \
  --granularity MONTHLY \
  --metrics BlendedCost \
  --group-by Type=SERVICE

# Check data transfer costs
aws ce get-cost-and-usage \
  --time-period Start=2025-01-01,End=2025-01-31 \
  --granularity MONTHLY \
  --metrics BlendedCost \
  --filter file://filter.json

# filter.json:
# {
#   "Dimensions": {
#     "Key": "USAGE_TYPE_GROUP",
#     "Values": ["EC2: Data Transfer"]
#   }
# }
```

**Common Cost Drivers:**

1. **NAT Gateway costs:**
   - $0.045/hour per NAT Gateway
   - $0.045/GB data processed
   - 3 NAT Gateways = ~$100/month + data

2. **Cross-AZ data transfer:**
   - $0.01/GB each direction
   - Can add up quickly for chatty applications

3. **EBS volumes:**
   - gp3: $0.08/GB-month
   - io2: $0.125/GB-month + IOPS charges

**Cost Reduction:**

```yaml
# Use single NAT Gateway (non-production only)
opencenter:
  infrastructure:
    cloud:
      aws:
        single_nat_gateway: true  # Planned feature

# Use smaller volumes
opencenter:
  storage:
    worker_volume_size: 20  # Instead of 100
    worker_volume_type: "gp3"  # Instead of io2

# Use smaller instances
opencenter:
  cluster:
    kubernetes:
      flavor_master: "t3.small"
      flavor_worker: "t3.medium"
```

## Getting Help

### Collect Diagnostic Information

When reporting issues, collect:

1. **Validation output:**

```bash
./bin/openCenter cluster validate my-aws-cluster > validation.log 2>&1
```

2. **AWS environment:**

```bash
aws --version > aws-info.log
aws sts get-caller-identity >> aws-info.log 2>&1
aws ec2 describe-regions --region us-east-1 >> aws-info.log 2>&1
```

3. **Cluster configuration (sanitized):**

```bash
# Remove sensitive data before sharing
cat ~/.config/openCenter/clusters/opencenter/.my-aws-cluster-config.yaml | \
  sed 's/access_key:.*/access_key: REDACTED/' | \
  sed 's/secret_access_key:.*/secret_access_key: REDACTED/'
```

4. **Terraform state (if applicable):**

```bash
cd <gitops-repo>/infrastructure/clusters/my-aws-cluster
terraform show > terraform-state.log
```

### Enable Debug Logging

```bash
# Enable AWS CLI debug output
export AWS_DEBUG=1

# Enable Terraform debug logging
export TF_LOG=DEBUG
export TF_LOG_PATH=./terraform-debug.log

# Run commands with verbose output
./bin/openCenter cluster setup my-aws-cluster --verbose
```

### Common Log Locations

- **openCenter logs**: `./openCenter.log` (if enabled)
- **Terraform logs**: `<gitops-repo>/infrastructure/clusters/<cluster>/terraform-debug.log`
- **Cloud-init logs**: `/var/log/cloud-init.log` (on instances)
- **AWS CloudTrail**: AWS Console → CloudTrail → Event history

## Related Documentation

- [AWS Setup Guide](setup.md) - Complete setup instructions
- [IAM Configuration](iam.md) - IAM roles and policies
- [VPC Design Guide](vpc.md) - Network architecture
- [AWS Provider Overview](README.md) - Provider features
- [Configuration Reference](../../reference/configuration.md) - Complete configuration options

## External Resources

- [AWS Documentation](https://docs.aws.amazon.com/)
- [AWS CLI Troubleshooting](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-troubleshooting.html)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [AWS Support](https://console.aws.amazon.com/support/)

---

**Last Updated**: January 2025  
**Maintained By**: openCenter Team
