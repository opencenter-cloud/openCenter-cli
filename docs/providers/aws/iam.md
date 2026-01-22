# AWS IAM Configuration


## Table of Contents

- [Purpose](#purpose)
- [IAM Overview](#iam-overview)
- [Minimum Required Permissions](#minimum-required-permissions)
- [Optional Permissions](#optional-permissions)
- [IAM Roles for Cluster Components](#iam-roles-for-cluster-components)
- [IAM Roles for Service Accounts (IRSA)](#iam-roles-for-service-accounts-irsa)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting IAM Issues](#troubleshooting-iam-issues)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: reference**

This document provides IAM policies, roles, and permissions required for openCenter to provision and manage Kubernetes clusters on AWS.

## Purpose

IAM (Identity and Access Management) controls access to AWS resources. openCenter requires specific permissions to create VPCs, EC2 instances, load balancers, and other infrastructure components. This guide provides the minimum required permissions and recommended security practices.

## IAM Overview

### Authentication Methods

openCenter supports two IAM authentication methods:

**IAM User with Access Keys:**
- Programmatic access for automation
- Access key ID and secret access key
- Suitable for CI/CD pipelines
- Requires key rotation

**IAM Role with Instance Profile:**
- No static credentials
- Temporary security tokens via STS
- Suitable for EC2-based deployments
- Automatic credential rotation

### Permission Scope

openCenter requires permissions in these AWS service categories:

- **EC2**: Instance, volume, and network management
- **VPC**: Network infrastructure creation
- **IAM**: Role and policy management for cluster components
- **ELB**: Load balancer provisioning (planned)
- **Route53**: DNS management (optional)
- **S3**: Terraform state storage (optional)

## Minimum Required Permissions

### IAM Policy for Infrastructure Provisioning

This policy grants minimum permissions for openCenter to provision cluster infrastructure:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EC2InstanceManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:RunInstances",
        "ec2:TerminateInstances",
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceStatus",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeInstanceTypeOfferings",
        "ec2:StartInstances",
        "ec2:StopInstances",
        "ec2:RebootInstances",
        "ec2:ModifyInstanceAttribute",
        "ec2:DescribeInstanceAttribute"
      ],
      "Resource": "*"
    },
    {
      "Sid": "EC2VolumeManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateVolume",
        "ec2:DeleteVolume",
        "ec2:DescribeVolumes",
        "ec2:AttachVolume",
        "ec2:DetachVolume",
        "ec2:ModifyVolume",
        "ec2:DescribeVolumeStatus",
        "ec2:DescribeVolumeAttribute"
      ],
      "Resource": "*"
    },
    {
      "Sid": "VPCManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateVpc",
        "ec2:DeleteVpc",
        "ec2:DescribeVpcs",
        "ec2:ModifyVpcAttribute",
        "ec2:CreateSubnet",
        "ec2:DeleteSubnet",
        "ec2:DescribeSubnets",
        "ec2:ModifySubnetAttribute",
        "ec2:CreateRouteTable",
        "ec2:DeleteRouteTable",
        "ec2:DescribeRouteTables",
        "ec2:AssociateRouteTable",
        "ec2:DisassociateRouteTable",
        "ec2:CreateRoute",
        "ec2:DeleteRoute",
        "ec2:ReplaceRoute"
      ],
      "Resource": "*"
    },
    {
      "Sid": "InternetGatewayManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateInternetGateway",
        "ec2:DeleteInternetGateway",
        "ec2:DescribeInternetGateways",
        "ec2:AttachInternetGateway",
        "ec2:DetachInternetGateway"
      ],
      "Resource": "*"
    },
    {
      "Sid": "NATGatewayManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateNatGateway",
        "ec2:DeleteNatGateway",
        "ec2:DescribeNatGateways"
      ],
      "Resource": "*"
    },
    {
      "Sid": "ElasticIPManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:AllocateAddress",
        "ec2:ReleaseAddress",
        "ec2:DescribeAddresses",
        "ec2:AssociateAddress",
        "ec2:DisassociateAddress"
      ],
      "Resource": "*"
    },
    {
      "Sid": "SecurityGroupManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateSecurityGroup",
        "ec2:DeleteSecurityGroup",
        "ec2:DescribeSecurityGroups",
        "ec2:AuthorizeSecurityGroupIngress",
        "ec2:AuthorizeSecurityGroupEgress",
        "ec2:RevokeSecurityGroupIngress",
        "ec2:RevokeSecurityGroupEgress",
        "ec2:ModifySecurityGroupRules"
      ],
      "Resource": "*"
    },
    {
      "Sid": "KeyPairManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateKeyPair",
        "ec2:DeleteKeyPair",
        "ec2:DescribeKeyPairs",
        "ec2:ImportKeyPair"
      ],
      "Resource": "*"
    },
    {
      "Sid": "TagManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:CreateTags",
        "ec2:DeleteTags",
        "ec2:DescribeTags"
      ],
      "Resource": "*"
    },
    {
      "Sid": "IAMRoleManagement",
      "Effect": "Allow",
      "Action": [
        "iam:CreateRole",
        "iam:DeleteRole",
        "iam:GetRole",
        "iam:ListRoles",
        "iam:AttachRolePolicy",
        "iam:DetachRolePolicy",
        "iam:PutRolePolicy",
        "iam:DeleteRolePolicy",
        "iam:GetRolePolicy",
        "iam:ListRolePolicies",
        "iam:ListAttachedRolePolicies"
      ],
      "Resource": "arn:aws:iam::*:role/opencenter-*"
    },
    {
      "Sid": "IAMInstanceProfileManagement",
      "Effect": "Allow",
      "Action": [
        "iam:CreateInstanceProfile",
        "iam:DeleteInstanceProfile",
        "iam:GetInstanceProfile",
        "iam:ListInstanceProfiles",
        "iam:AddRoleToInstanceProfile",
        "iam:RemoveRoleFromInstanceProfile"
      ],
      "Resource": "arn:aws:iam::*:instance-profile/opencenter-*"
    },
    {
      "Sid": "IAMPassRole",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::*:role/opencenter-*",
      "Condition": {
        "StringEquals": {
          "iam:PassedToService": "ec2.amazonaws.com"
        }
      }
    }
  ]
}
```

### Create IAM Policy

Save the policy to a file and create it:

```bash
# Save policy to file
cat > opencenter-provisioning-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    ... (policy from above)
  ]
}
EOF

# Create IAM policy
aws iam create-policy \
  --policy-name OpenCenterProvisioning \
  --policy-document file://opencenter-provisioning-policy.json \
  --description "Permissions for openCenter to provision Kubernetes clusters"

# Note the policy ARN from output
```

### Attach Policy to IAM User

```bash
# Attach to existing user
aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::123456789012:policy/OpenCenterProvisioning

# Or create new user and attach policy
aws iam create-user --user-name opencenter-provisioner

aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::123456789012:policy/OpenCenterProvisioning

# Create access key
aws iam create-access-key --user-name opencenter-provisioner
```

## Optional Permissions

### Load Balancer Management (Planned)

For ELB/ALB/NLB support:

```json
{
  "Sid": "LoadBalancerManagement",
  "Effect": "Allow",
  "Action": [
    "elasticloadbalancing:CreateLoadBalancer",
    "elasticloadbalancing:DeleteLoadBalancer",
    "elasticloadbalancing:DescribeLoadBalancers",
    "elasticloadbalancing:ModifyLoadBalancerAttributes",
    "elasticloadbalancing:CreateTargetGroup",
    "elasticloadbalancing:DeleteTargetGroup",
    "elasticloadbalancing:DescribeTargetGroups",
    "elasticloadbalancing:RegisterTargets",
    "elasticloadbalancing:DeregisterTargets",
    "elasticloadbalancing:DescribeTargetHealth",
    "elasticloadbalancing:CreateListener",
    "elasticloadbalancing:DeleteListener",
    "elasticloadbalancing:DescribeListeners",
    "elasticloadbalancing:AddTags",
    "elasticloadbalancing:RemoveTags"
  ],
  "Resource": "*"
}
```

### Route53 DNS Management (Optional)

For automatic DNS record management:

```json
{
  "Sid": "Route53Management",
  "Effect": "Allow",
  "Action": [
    "route53:CreateHostedZone",
    "route53:DeleteHostedZone",
    "route53:GetHostedZone",
    "route53:ListHostedZones",
    "route53:ChangeResourceRecordSets",
    "route53:ListResourceRecordSets",
    "route53:GetChange"
  ],
  "Resource": "*"
}
```

### S3 Backend for Terraform State (Recommended)

For remote state storage:

```json
{
  "Sid": "S3StateManagement",
  "Effect": "Allow",
  "Action": [
    "s3:ListBucket",
    "s3:GetObject",
    "s3:PutObject",
    "s3:DeleteObject"
  ],
  "Resource": [
    "arn:aws:s3:::my-org-terraform-state",
    "arn:aws:s3:::my-org-terraform-state/*"
  ]
}
```

```json
{
  "Sid": "DynamoDBLocking",
  "Effect": "Allow",
  "Action": [
    "dynamodb:GetItem",
    "dynamodb:PutItem",
    "dynamodb:DeleteItem"
  ],
  "Resource": "arn:aws:dynamodb:*:*:table/terraform-state-lock"
}
```

## IAM Roles for Cluster Components

### Master Node IAM Role

IAM role for Kubernetes control plane nodes:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EC2ReadOnly",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeRegions",
        "ec2:DescribeRouteTables",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeSubnets",
        "ec2:DescribeVolumes",
        "ec2:DescribeVpcs"
      ],
      "Resource": "*"
    },
    {
      "Sid": "ELBManagement",
      "Effect": "Allow",
      "Action": [
        "elasticloadbalancing:DescribeLoadBalancers",
        "elasticloadbalancing:DescribeTargetGroups",
        "elasticloadbalancing:DescribeTargetHealth"
      ],
      "Resource": "*"
    }
  ]
}
```

Create the role:

```bash
# Create trust policy
cat > master-trust-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

# Create role
aws iam create-role \
  --role-name opencenter-master-role \
  --assume-role-policy-document file://master-trust-policy.json

# Attach policy
aws iam put-role-policy \
  --role-name opencenter-master-role \
  --policy-name MasterNodePolicy \
  --policy-document file://master-node-policy.json

# Create instance profile
aws iam create-instance-profile \
  --instance-profile-name opencenter-master-profile

# Add role to instance profile
aws iam add-role-to-instance-profile \
  --instance-profile-name opencenter-master-profile \
  --role-name opencenter-master-role
```

### Worker Node IAM Role

IAM role for Kubernetes worker nodes:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EC2ReadOnly",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    },
    {
      "Sid": "EBSVolumeManagement",
      "Effect": "Allow",
      "Action": [
        "ec2:AttachVolume",
        "ec2:DetachVolume",
        "ec2:DescribeVolumes",
        "ec2:DescribeVolumeStatus"
      ],
      "Resource": "*"
    }
  ]
}
```

Create the role:

```bash
# Create role (using same trust policy as master)
aws iam create-role \
  --role-name opencenter-worker-role \
  --assume-role-policy-document file://master-trust-policy.json

# Attach policy
aws iam put-role-policy \
  --role-name opencenter-worker-role \
  --policy-name WorkerNodePolicy \
  --policy-document file://worker-node-policy.json

# Create instance profile
aws iam create-instance-profile \
  --instance-profile-name opencenter-worker-profile

# Add role to instance profile
aws iam add-role-to-instance-profile \
  --instance-profile-name opencenter-worker-profile \
  --role-name opencenter-worker-role
```

## IAM Roles for Service Accounts (IRSA)

### OIDC Provider Setup

For Kubernetes service accounts to assume IAM roles:

```bash
# Get cluster OIDC issuer URL (after cluster creation)
OIDC_ISSUER=$(kubectl get --raw /.well-known/openid-configuration | jq -r '.issuer')

# Create OIDC provider
aws iam create-open-id-connect-provider \
  --url $OIDC_ISSUER \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list <thumbprint>
```

### EBS CSI Driver IAM Role

For EBS CSI driver to manage volumes:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateSnapshot",
        "ec2:AttachVolume",
        "ec2:DetachVolume",
        "ec2:ModifyVolume",
        "ec2:DescribeAvailabilityZones",
        "ec2:DescribeInstances",
        "ec2:DescribeSnapshots",
        "ec2:DescribeTags",
        "ec2:DescribeVolumes",
        "ec2:DescribeVolumesModifications"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateTags"
      ],
      "Resource": [
        "arn:aws:ec2:*:*:volume/*",
        "arn:aws:ec2:*:*:snapshot/*"
      ],
      "Condition": {
        "StringEquals": {
          "ec2:CreateAction": [
            "CreateVolume",
            "CreateSnapshot"
          ]
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteTags"
      ],
      "Resource": [
        "arn:aws:ec2:*:*:volume/*",
        "arn:aws:ec2:*:*:snapshot/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateVolume"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "aws:RequestTag/ebs.csi.aws.com/cluster": "true"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateVolume"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "aws:RequestTag/CSIVolumeName": "*"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteVolume"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "ec2:ResourceTag/ebs.csi.aws.com/cluster": "true"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteVolume"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "ec2:ResourceTag/CSIVolumeName": "*"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteVolume"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "ec2:ResourceTag/kubernetes.io/created-for/pvc/name": "*"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteSnapshot"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "ec2:ResourceTag/CSIVolumeSnapshotName": "*"
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteSnapshot"
      ],
      "Resource": "*",
      "Condition": {
        "StringLike": {
          "ec2:ResourceTag/ebs.csi.aws.com/cluster": "true"
        }
      }
    }
  ]
}
```

## Security Best Practices

### Principle of Least Privilege

**Start with minimum permissions:**
- Use the minimum required policy initially
- Add permissions only when needed
- Remove unused permissions regularly
- Audit IAM policies quarterly

**Resource-level permissions:**
```json
{
  "Effect": "Allow",
  "Action": "ec2:TerminateInstances",
  "Resource": "arn:aws:ec2:*:*:instance/*",
  "Condition": {
    "StringEquals": {
      "ec2:ResourceTag/ManagedBy": "openCenter"
    }
  }
}
```

### Credential Management

**Access key rotation:**
```bash
# Create new access key
aws iam create-access-key --user-name opencenter-provisioner

# Update configuration with new key
# Test new key works
# Delete old access key
aws iam delete-access-key \
  --user-name opencenter-provisioner \
  --access-key-id AKIAIOSFODNN7EXAMPLE
```

**Use AWS Secrets Manager:**
```bash
# Store access key in Secrets Manager
aws secretsmanager create-secret \
  --name opencenter/aws-credentials \
  --secret-string '{"access_key":"AKIA...","secret_key":"wJal..."}'

# Retrieve in automation
aws secretsmanager get-secret-value \
  --secret-id opencenter/aws-credentials \
  --query SecretString \
  --output text
```

### Multi-Factor Authentication

**Enable MFA for IAM users:**
```bash
# Enable MFA device
aws iam enable-mfa-device \
  --user-name opencenter-provisioner \
  --serial-number arn:aws:iam::123456789012:mfa/opencenter-provisioner \
  --authentication-code-1 123456 \
  --authentication-code-2 789012
```

**Require MFA for sensitive operations:**
```json
{
  "Effect": "Allow",
  "Action": "ec2:TerminateInstances",
  "Resource": "*",
  "Condition": {
    "Bool": {
      "aws:MultiFactorAuthPresent": "true"
    }
  }
}
```

### Service Control Policies (SCPs)

For AWS Organizations, use SCPs to enforce guardrails:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Deny",
      "Action": [
        "ec2:RunInstances"
      ],
      "Resource": "arn:aws:ec2:*:*:instance/*",
      "Condition": {
        "StringNotEquals": {
          "ec2:InstanceType": [
            "t3.small",
            "t3.medium",
            "t3.large",
            "t3.xlarge"
          ]
        }
      }
    }
  ]
}
```

### CloudTrail Logging

Enable CloudTrail to audit IAM actions:

```bash
# Create CloudTrail trail
aws cloudtrail create-trail \
  --name opencenter-audit \
  --s3-bucket-name my-org-cloudtrail-logs

# Start logging
aws cloudtrail start-logging --name opencenter-audit

# Query recent IAM actions
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=Username,AttributeValue=opencenter-provisioner \
  --max-results 50
```

## Troubleshooting IAM Issues

### Access Denied Errors

**Symptom**: `AccessDenied` or `UnauthorizedOperation` errors

**Diagnosis**:
```bash
# Check current user identity
aws sts get-caller-identity

# List attached policies
aws iam list-attached-user-policies --user-name opencenter-provisioner

# Get policy document
aws iam get-policy-version \
  --policy-arn arn:aws:iam::123456789012:policy/OpenCenterProvisioning \
  --version-id v1
```

**Solution**: Add missing permissions to IAM policy

### PassRole Errors

**Symptom**: `not authorized to perform: iam:PassRole`

**Cause**: Missing `iam:PassRole` permission

**Solution**: Add PassRole permission with condition:
```json
{
  "Effect": "Allow",
  "Action": "iam:PassRole",
  "Resource": "arn:aws:iam::*:role/opencenter-*",
  "Condition": {
    "StringEquals": {
      "iam:PassedToService": "ec2.amazonaws.com"
    }
  }
}
```

### Instance Profile Not Found

**Symptom**: `Invalid IAM Instance Profile name`

**Diagnosis**:
```bash
# List instance profiles
aws iam list-instance-profiles

# Get specific instance profile
aws iam get-instance-profile \
  --instance-profile-name opencenter-master-profile
```

**Solution**: Create instance profile and attach role

### Policy Size Limit

**Symptom**: `LimitExceeded: Cannot exceed quota for PoliciesPerUser`

**Solution**: Consolidate policies or use managed policies:
```bash
# Detach inline policies
aws iam delete-user-policy \
  --user-name opencenter-provisioner \
  --policy-name OldPolicy

# Attach managed policy instead
aws iam attach-user-policy \
  --user-name opencenter-provisioner \
  --policy-arn arn:aws:iam::aws:policy/PowerUserAccess
```

## Related Documentation

- [AWS Setup Guide](setup.md) - Complete setup instructions
- [VPC Design Guide](vpc.md) - Network architecture
- [Troubleshooting Guide](troubleshooting.md) - Common AWS issues
- [AWS Provider Overview](README.md) - Provider features

## External Resources

- [AWS IAM Documentation](https://docs.aws.amazon.com/iam/)
- [IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [IAM Policy Simulator](https://policysim.aws.amazon.com/)
- [AWS Security Best Practices](https://aws.amazon.com/architecture/security-identity-compliance/)

---

**Last Updated**: January 2025  
**Maintained By**: openCenter Team
