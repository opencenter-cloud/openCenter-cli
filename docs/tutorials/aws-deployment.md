# Deploy Production Kubernetes on AWS


## Table of Contents

- [What You'll Build](#what-youll-build)
- [Prerequisites](#prerequisites)
- [Step 1: Gather AWS Information](#step-1-gather-aws-information)
- [Step 2: Initialize Cluster Configuration](#step-2-initialize-cluster-configuration)
- [Step 3: Configure AWS Credentials](#step-3-configure-aws-credentials)
- [Step 4: Configure Cluster Networking](#step-4-configure-cluster-networking)
- [Step 5: Configure Storage](#step-5-configure-storage)
- [Step 6: Enable SOPS Encryption](#step-6-enable-sops-encryption)
- [Step 7: Configure Terraform Backend](#step-7-configure-terraform-backend)
- [Step 8: Validate Configuration](#step-8-validate-configuration)
- [Step 9: Generate GitOps Repository](#step-9-generate-gitops-repository)
- [Step 10: Bootstrap Infrastructure](#step-10-bootstrap-infrastructure)
- [Step 11: Verify Cluster Deployment](#step-11-verify-cluster-deployment)
- [Step 12: Access Cluster Nodes](#step-12-access-cluster-nodes)
- [What You Learned](#what-you-learned)
- [Next Steps](#next-steps)
- [Common Issues](#common-issues)
- [Production Considerations](#production-considerations)
- [Related Documentation](#related-documentation)
**doc_type: tutorial**

Deploy a production-ready Kubernetes cluster on AWS in 60 minutes. You'll configure AWS credentials, set up VPC networking, generate a GitOps repository, and bootstrap a highly available cluster using Kubespray.

## What You'll Build

By the end of this tutorial, you'll have:
- A 3-master, 3-worker Kubernetes cluster running on AWS EC2
- Encrypted secrets managed with SOPS
- A complete GitOps repository structure
- AWS Cloud Controller Manager for load balancer integration
- EBS CSI driver for persistent storage
- Production-ready security configuration

## Prerequisites

Before starting, you need:
- **AWS account** with administrative access
- **AWS CLI** installed and configured (`aws configure`)
- **openCenter installed** (see [Getting Started](getting-started.md))
- **Terraform or OpenTofu** installed (v1.6+)
- **Git** installed and configured
- **60 minutes** of time

### AWS Requirements

Your AWS account needs:
- EC2 instance quota (6 instances minimum)
- VPC with public and private subnets
- IAM permissions for EC2, VPC, EBS, and ELB
- SSH key pair registered in your target region

### Verify AWS Access

Test your AWS connection:

```bash
aws sts get-caller-identity
aws ec2 describe-regions
aws ec2 describe-vpcs
```

If these commands work, you're ready to proceed.


## Step 1: Gather AWS Information

You need specific AWS details before initializing your cluster. Collect these values:

### AWS Credentials

Get your AWS access credentials:

```bash
# View your current credentials
aws configure list

# Or create new IAM user credentials
aws iam create-access-key --user-name opencenter-deploy
```

Record these values:
- **Access Key ID**: Your AWS access key (20 characters, alphanumeric)
- **Secret Access Key**: Your AWS secret key (40 characters, base64-like)
- **Region**: Target AWS region (e.g., `us-east-1`, `us-west-2`)

### VPC and Network Information

Find or create your VPC:

```bash
# List existing VPCs
aws ec2 describe-vpcs --query 'Vpcs[*].[VpcId,CidrBlock,Tags[?Key==`Name`].Value|[0]]' --output table

# Create new VPC (if needed)
aws ec2 create-vpc --cidr-block 10.0.0.0/16 --tag-specifications 'ResourceType=vpc,Tags=[{Key=Name,Value=prod-k8s-vpc}]'
```

Record:
- **VPC ID**: Your VPC identifier (e.g., `vpc-0123456789abcdef0`)
- **Private Subnets**: CIDR blocks for private subnets (e.g., `10.0.1.0/24`, `10.0.2.0/24`)
- **Public Subnets**: CIDR blocks for public subnets (e.g., `10.0.101.0/24`, `10.0.102.0/24`)

### EC2 Instance Information

Choose instance types for your cluster:

```bash
# List available instance types
aws ec2 describe-instance-types --filters "Name=instance-type,Values=t3.*,m5.*" --query 'InstanceTypes[*].[InstanceType,VCpuInfo.DefaultVCpus,MemoryInfo.SizeInMiB]' --output table
```

Recommended instance types:
- **Master nodes**: `t3.medium` (2 vCPU, 4GB RAM) or larger
- **Worker nodes**: `t3.large` (2 vCPU, 8GB RAM) or larger

### AMI Information

Find Ubuntu AMI for your region:

```bash
# Find latest Ubuntu 24.04 AMI
aws ec2 describe-images \
  --owners 099720109477 \
  --filters "Name=name,Values=ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*" \
  --query 'Images | sort_by(@, &CreationDate) | [-1].[ImageId,Name]' \
  --output table
```

Record the **AMI ID** (e.g., `ami-0abcdef1234567890`).


## Step 2: Initialize Cluster Configuration

Create your cluster configuration with AWS-specific settings:

```bash
openCenter cluster init prod-aws-k8s \
  --opencenter.meta.env=production \
  --opencenter.meta.region=us-east-1 \
  --opencenter.infrastructure.provider=aws \
  --opencenter.cluster.kubernetes.version=1.33.5 \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=3 \
  --opencenter.gitops.git_dir=/home/user/gitops/prod-aws-k8s
```

Replace these values with your AWS details:
- `region`: Your AWS region
- `git_dir`: Where to create the GitOps repository

You'll see output like:

```
Generated ed25519 SSH key pair at ~/.config/openCenter/clusters/opencenter/secrets/ssh/prod-aws-k8s-production-us-east-1
Created cluster configuration in organization 'opencenter' at '~/.config/openCenter/clusters/opencenter/infrastructure/clusters/prod-aws-k8s'
GitOps repository root: /home/user/gitops/prod-aws-k8s
SOPS key location: ~/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-aws-k8s-key.txt
```

The configuration file is at:
```
~/.config/openCenter/clusters/opencenter/.prod-aws-k8s-config.yaml
```

## Step 3: Configure AWS Credentials

Edit your cluster configuration to add AWS credentials:

```bash
# Open the configuration file
vim ~/.config/openCenter/clusters/opencenter/.prod-aws-k8s-config.yaml
```

Add your AWS credentials:

```yaml
opencenter:
  cluster:
    aws_access_key: "AKIAIOSFODNN7EXAMPLE"
    aws_secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  infrastructure:
    provider: aws
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

Replace with your actual AWS credentials and network configuration from Step 1.

**Security Note**: These credentials will be encrypted with SOPS before committing to Git. Never commit unencrypted credentials.


## Step 4: Configure Cluster Networking

Set up Kubernetes networking for AWS:

```yaml
opencenter:
  cluster:
    networking:
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      ntp_servers:
        - "time.aws.com"
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # Pod network CIDR
      subnet_services: "10.43.0.0/16"  # Service network CIDR
      network_plugin:
        calico:
          enabled: true
          encapsulation_type: "VXLAN"
          nat_outgoing: true
```

### Network Configuration Notes

**Subnet Planning:**
- Ensure pod and service CIDRs don't overlap with VPC CIDR
- Pod CIDR should be large enough for cluster growth
- Service CIDR typically smaller than pod CIDR

**Calico Settings:**
- `encapsulation_type`: Use `VXLAN` for overlay networking
- `nat_outgoing`: Enable NAT for pod traffic to internet

## Step 5: Configure Storage

Set up persistent storage with AWS EBS:

```yaml
opencenter:
  storage:
    default_storage_class: "gp3"
    worker_volume_size: 100  # GB
```

### Storage Options

**EBS Volume Types:**
- `gp3`: General Purpose SSD (recommended, best price/performance)
- `gp2`: General Purpose SSD (previous generation)
- `io2`: Provisioned IOPS SSD (high performance)

The AWS EBS CSI driver will be automatically configured during bootstrap.

## Step 6: Enable SOPS Encryption

Configure SOPS to encrypt secrets in your GitOps repository:

```yaml
secrets:
  sops_age_key_file: "/home/user/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-aws-k8s-key.txt"
```

Verify the key exists:

```bash
cat ~/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-aws-k8s-key.txt
```

You should see an Age key starting with `AGE-SECRET-KEY-`.

### Encrypt Sensitive Configuration

Before committing to Git, encrypt the configuration file:

```bash
# Install SOPS if not already installed
mise install sops

# Encrypt the configuration file
sops -e -i ~/.config/openCenter/clusters/opencenter/.prod-aws-k8s-config.yaml
```

The file will now contain encrypted values for sensitive fields like AWS credentials.


## Step 7: Configure Terraform Backend

Set up state storage for Terraform:

### Option A: S3 Backend (Recommended for Production)

```yaml
opentofu:
  enabled: true
  path: "opentofu"
  backend:
    type: "s3"
    s3:
      bucket: "prod-aws-k8s-terraform-state"
      key: "prod-aws-k8s/terraform.tfstate"
      region: "us-east-1"
      dynamodb_table: "terraform-state-lock"
```

Create the S3 bucket and DynamoDB table:

```bash
# Create S3 bucket for state
aws s3 mb s3://prod-aws-k8s-terraform-state --region us-east-1

# Enable versioning
aws s3api put-bucket-versioning \
  --bucket prod-aws-k8s-terraform-state \
  --versioning-configuration Status=Enabled

# Create DynamoDB table for state locking
aws dynamodb create-table \
  --table-name terraform-state-lock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

### Option B: Local Backend (Development Only)

```yaml
opentofu:
  enabled: true
  path: "opentofu"
  backend:
    type: "local"
    local:
      path: "/home/user/gitops/prod-aws-k8s/terraform.tfstate"
```

**Warning**: Local backend is not suitable for production or team environments.

## Step 8: Validate Configuration

Check that your configuration is valid before deployment:

```bash
mise run build
./bin/openCenter cluster validate prod-aws-k8s
```

You should see:

```
Validation successful.
```

### Common Validation Errors

**"AWS access key is required"**
- Add `opencenter.cluster.aws_access_key` to your config

**"invalid AWS region format"**
- Use standard AWS region format (e.g., `us-east-1`, `eu-west-1`)

**"invalid CIDR range"**
- Check `subnet_pods` and `subnet_services` don't overlap
- Ensure CIDRs are valid (e.g., `10.42.0.0/16`)

**"VPC ID is required"**
- Add `opencenter.infrastructure.cloud.aws.vpc_id`

If validation fails, read the error messages carefully. They indicate which fields need correction.


## Step 9: Generate GitOps Repository

Create the GitOps repository structure with all manifests:

```bash
./bin/openCenter cluster setup prod-aws-k8s
```

This command:
- Renders base GitOps structure
- Generates cluster-specific manifests
- Creates Terraform/OpenTofu configuration
- Sets up directory structure for applications

You'll see output like:

```
Created GitOps repo at: /home/user/gitops/prod-aws-k8s
```

### Explore the GitOps Repository

Check what was created:

```bash
cd /home/user/gitops/prod-aws-k8s
tree -L 3
```

You'll see:

```
.
├── applications/
│   └── overlays/
│       └── prod-aws-k8s/          # Application manifests
├── infrastructure/
│   └── clusters/
│       └── prod-aws-k8s/          # Infrastructure configs
│           ├── main.tf            # Terraform main config
│           ├── provider.tf        # AWS provider
│           ├── Makefile           # Build automation
│           └── logs/              # Bootstrap logs
└── .git/                          # Git repository
```

### Initialize Git Repository

Commit the initial configuration:

```bash
cd /home/user/gitops/prod-aws-k8s
git add .
git commit -m "Initial cluster configuration for prod-aws-k8s"
```

If you have a remote Git repository:

```bash
git remote add origin git@github.com:yourorg/prod-aws-k8s-gitops.git
git push -u origin main
```

## Step 10: Bootstrap Infrastructure

Deploy the cluster infrastructure to AWS:

```bash
./bin/openCenter cluster bootstrap prod-aws-k8s
```

This command runs these steps automatically:
1. **make terraform**: Generates Terraform configuration from templates
2. **terraform init**: Initializes Terraform providers and modules
3. **terraform apply**: Provisions AWS resources (EC2, VPC, security groups)

The bootstrap process takes 20-40 minutes. You'll see progress output:

```
Step "make-terraform": Run make terraform
$ make terraform
Running make terraform in /home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s

Step "terraform-init": Initialize Terraform
$ terraform init
Initializing Terraform in /home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s

Step "terraform-apply": Apply Terraform configuration
$ terraform apply -auto-approve
Applying Terraform configuration (this may take several minutes)...
Still running... (elapsed: 30s)
Still running... (elapsed: 1m0s)
...
Command completed successfully after 22m15s

Bootstrap complete.
Log written to /home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s/logs/bootstrap-2025-01-19-1737320400.log
```

### What Gets Created

AWS resources provisioned:
- **6 EC2 instances**: 3 masters, 3 workers
- **Security groups**: API access, node communication, pod networking
- **Elastic IPs**: For master nodes and load balancer
- **EBS volumes**: Root volumes for each node
- **Network Load Balancer**: For Kubernetes API server HA

### Monitor Progress

Watch AWS resources being created:

```bash
# In another terminal
watch -n 5 'aws ec2 describe-instances --filters "Name=tag:Name,Values=prod-aws-k8s-*" --query "Reservations[*].Instances[*].[InstanceId,State.Name,PrivateIpAddress]" --output table'
```

You'll see instances transition from `pending` to `running` state.


### Bootstrap Troubleshooting

**"terraform init failed"**
- Check internet connectivity for provider downloads
- Verify AWS credentials are correct
- Check `opentofu.backend` configuration

**"terraform apply failed: insufficient capacity"**
- Try different availability zones
- Use different instance types
- Check AWS service quotas: `aws service-quotas list-service-quotas --service-code ec2`

**"authentication failed"**
- Verify `aws_access_key` and `aws_secret_access_key`
- Check credentials haven't expired
- Ensure IAM user has required permissions

**"VPC not found"**
- Verify `vpc_id` exists: `aws ec2 describe-vpcs --vpc-ids <vpc-id>`
- Check you're in the correct region
- Confirm VPC is in the correct AWS account

### Resume Failed Bootstrap

If bootstrap fails partway through, fix the issue and resume:

```bash
# Resume from where it failed
./bin/openCenter cluster bootstrap prod-aws-k8s

# Or restart from a specific step
./bin/openCenter cluster bootstrap prod-aws-k8s --from-step terraform-apply

# Or restart completely
./bin/openCenter cluster bootstrap prod-aws-k8s --restart
```

Bootstrap state is saved in:
```
/home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s/logs/bootstrap-state.json
```

## Step 11: Verify Cluster Deployment

After bootstrap completes, verify your cluster is running.

### Check AWS Resources

Verify instances are running:

```bash
aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=prod-aws-k8s-*" \
  --query 'Reservations[*].Instances[*].[InstanceId,State.Name,PrivateIpAddress,PublicIpAddress,Tags[?Key==`Name`].Value|[0]]' \
  --output table
```

You should see:

```
-----------------------------------------------------------------
|                      DescribeInstances                        |
+----------------------+----------+-------------+----------------+
|  i-0abc123def456789  | running  | 10.0.1.10   | 54.123.45.67  |
|  i-0def456ghi789012  | running  | 10.0.1.11   | 54.123.45.68  |
|  i-0ghi789jkl012345  | running  | 10.0.1.12   | 54.123.45.69  |
|  i-0jkl012mno345678  | running  | 10.0.2.20   | None          |
|  i-0mno345pqr678901  | running  | 10.0.2.21   | None          |
|  i-0pqr678stu901234  | running  | 10.0.2.22   | None          |
+----------------------+----------+-------------+----------------+
```

All instances should show `running` state.

### Get Kubeconfig

The kubeconfig file is generated during bootstrap:

```bash
export KUBECONFIG=/home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s/kubeconfig.yaml
```

Or copy it to your default location:

```bash
mkdir -p ~/.kube
cp /home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s/kubeconfig.yaml ~/.kube/config
```

### Check Cluster Nodes

Verify all nodes are ready:

```bash
kubectl get nodes
```

Expected output:

```
NAME                STATUS   ROLES           AGE   VERSION
prod-aws-k8s-cp-1   Ready    control-plane   18m   v1.33.5
prod-aws-k8s-cp-2   Ready    control-plane   17m   v1.33.5
prod-aws-k8s-cp-3   Ready    control-plane   16m   v1.33.5
prod-aws-k8s-wn-1   Ready    <none>          15m   v1.33.5
prod-aws-k8s-wn-2   Ready    <none>          14m   v1.33.5
prod-aws-k8s-wn-3   Ready    <none>          13m   v1.33.5
```

All nodes should show `Ready` status. If nodes show `NotReady`, wait a few minutes for initialization to complete.


### Check System Pods

Verify core system components are running:

```bash
kubectl get pods -n kube-system
```

You should see pods for:
- `calico-node-*`: Calico networking (one per node)
- `calico-kube-controllers-*`: Calico controller
- `coredns-*`: DNS service
- `kube-apiserver-*`: API servers (one per master)
- `kube-controller-manager-*`: Controller managers
- `kube-scheduler-*`: Schedulers
- `kube-proxy-*`: Network proxy (one per node)

All pods should show `Running` status.

### Check AWS Integration

Verify AWS Cloud Controller Manager:

```bash
kubectl get pods -n kube-system -l app=aws-cloud-controller-manager
```

Verify AWS EBS CSI driver:

```bash
kubectl get pods -n kube-system -l app=ebs-csi-controller
kubectl get pods -n kube-system -l app=ebs-csi-node
```

### Test Storage

Create a test PVC to verify EBS CSI:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: gp3
EOF
```

Check PVC status:

```bash
kubectl get pvc test-pvc
```

Should show `Bound` status:

```
NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
test-pvc   Bound    pvc-abc123-def4-5678-90ab-cdef12345678    10Gi       RWO            gp3
```

Verify volume was created in AWS:

```bash
aws ec2 describe-volumes --filters "Name=tag:kubernetes.io/created-for/pvc/name,Values=test-pvc"
```

Clean up test PVC:

```bash
kubectl delete pvc test-pvc
```

### Test Networking

Deploy a test application to verify networking:

```bash
kubectl create deployment nginx --image=nginx:latest
kubectl expose deployment nginx --port=80 --type=LoadBalancer
```

Wait for external IP assignment:

```bash
kubectl get svc nginx -w
```

You should see an AWS ELB hostname assigned (this may take 2-3 minutes):

```
NAME    TYPE           CLUSTER-IP      EXTERNAL-IP                                                              PORT(S)        AGE
nginx   LoadBalancer   10.43.100.50    a1b2c3d4e5f6g7h8-123456789.us-east-1.elb.amazonaws.com                 80:30080/TCP   3m
```

Test connectivity:

```bash
curl http://a1b2c3d4e5f6g7h8-123456789.us-east-1.elb.amazonaws.com
```

You should see the nginx welcome page.

Clean up test deployment:

```bash
kubectl delete svc nginx
kubectl delete deployment nginx
```


## Step 12: Access Cluster Nodes

SSH into cluster nodes for troubleshooting or maintenance.

### SSH to Master Nodes

Master nodes have public IPs. SSH directly:

```bash
# Get master node public IP
aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=prod-aws-k8s-cp-1" \
  --query 'Reservations[0].Instances[0].PublicIpAddress' \
  --output text

# SSH to master
ssh -i ~/.config/openCenter/clusters/opencenter/secrets/ssh/prod-aws-k8s-production-us-east-1 ubuntu@54.123.45.67
```

### SSH to Worker Nodes

Worker nodes are in private subnets. SSH through a master node:

```bash
# From master node
ssh 10.0.2.20  # Worker node private IP
```

Or use SSH proxy jump:

```bash
ssh -i ~/.config/openCenter/clusters/opencenter/secrets/ssh/prod-aws-k8s-production-us-east-1 \
  -J ubuntu@54.123.45.67 \
  ubuntu@10.0.2.20
```

### Check Node Status

Once connected to a node:

```bash
# Check kubelet status
sudo systemctl status kubelet

# View kubelet logs
sudo journalctl -u kubelet -f

# Check container runtime
sudo crictl ps

# View node resources
top
df -h
```

## What You Learned

You now understand how to:

1. **Gather AWS prerequisites**: Credentials, VPC, subnets, and instance types
2. **Initialize cluster configuration**: Set AWS-specific parameters
3. **Configure credentials**: AWS access keys and secret keys
4. **Set up networking**: VPC, subnets, and Calico CNI
5. **Configure storage**: EBS volumes and storage classes
6. **Enable secrets encryption**: SOPS with Age keys
7. **Generate GitOps repository**: Complete infrastructure-as-code structure
8. **Bootstrap infrastructure**: Automated Terraform provisioning
9. **Verify deployment**: Check nodes, pods, storage, and networking
10. **Access cluster nodes**: SSH for troubleshooting and maintenance

### Key Concepts

**AWS Integration:**
- Cloud Controller Manager handles load balancers and node lifecycle
- EBS CSI driver provides persistent storage
- VPC networking integrates with Calico CNI
- Security groups control network access

**High Availability:**
- 3 master nodes provide control plane redundancy
- Network Load Balancer distributes API traffic
- etcd runs on all master nodes for data redundancy
- Multi-AZ deployment for fault tolerance

**GitOps Workflow:**
- Single YAML configuration as source of truth
- Generated manifests stored in Git
- Infrastructure changes tracked in version control
- Declarative updates through configuration changes

**Security:**
- SOPS encrypts secrets at rest
- SSH keys for node access
- Network policies via Calico
- IAM roles for AWS service integration


## Next Steps

Now that you have a running cluster, you can:

### Deploy Applications

1. **Add applications to GitOps repository**:
   ```bash
   cd /home/user/gitops/prod-aws-k8s/applications/overlays/prod-aws-k8s
   # Add your application manifests here
   git add .
   git commit -m "Add application manifests"
   git push
   ```

2. **Enable FluxCD for automated deployment**:
   ```bash
   kubectl apply -f /home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s/flux/
   ```

### Enable Additional Services

Edit your cluster configuration to enable services:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true  # Automated TLS certificates
    kube-prometheus-stack:
      enabled: true  # Monitoring with Prometheus and Grafana
    loki:
      enabled: true  # Log aggregation
```

Then regenerate and apply:

```bash
./bin/openCenter cluster setup prod-aws-k8s --force
cd /home/user/gitops/prod-aws-k8s
git add .
git commit -m "Enable monitoring and logging services"
kubectl apply -k applications/overlays/prod-aws-k8s/
```

### Scale the Cluster

Add more worker nodes:

```yaml
opencenter:
  cluster:
    kubernetes:
      worker_count: 6  # Increase from 3 to 6
```

Apply changes:

```bash
./bin/openCenter cluster validate prod-aws-k8s
./bin/openCenter cluster setup prod-aws-k8s --force
cd /home/user/gitops/prod-aws-k8s/infrastructure/clusters/prod-aws-k8s
terraform apply
```

### Configure Monitoring

Set up Prometheus and Grafana:

1. Enable kube-prometheus-stack in configuration
2. Add Grafana admin password to secrets
3. Access Grafana dashboard via LoadBalancer

See [Monitoring Setup](../how-to/monitoring.md) for details.

### Set Up Backups

Configure Velero for cluster backups:

1. Create S3 bucket for backups
2. Add AWS credentials to secrets
3. Enable Velero service
4. Configure backup schedules

See [Backup and Recovery](../how-to/backup-recovery.md) for details.

### Multi-Cluster Management

Deploy additional clusters:

```bash
./bin/openCenter cluster init prod-aws-k8s-west --opencenter.meta.region=us-west-2
./bin/openCenter cluster init staging-aws-k8s --opencenter.meta.env=staging
```

See [Multi-Cluster Management](multi-cluster.md) for managing multiple clusters.


## Common Issues

### Nodes Not Ready

**Symptom**: `kubectl get nodes` shows `NotReady` status

**Causes**:
- Kubelet not started
- Network plugin not initialized
- Node can't reach API server

**Solutions**:
```bash
# SSH to node and check kubelet
ssh ubuntu@<node-ip>
sudo systemctl status kubelet
sudo journalctl -u kubelet -n 100

# Check Calico pods
kubectl get pods -n kube-system -l k8s-app=calico-node

# Restart kubelet if needed
sudo systemctl restart kubelet
```

### LoadBalancer Service Not Getting External IP

**Symptom**: Services with `type: LoadBalancer` stuck without external IP

**Causes**:
- Cloud controller manager not running
- AWS credentials invalid
- IAM permissions insufficient

**Solutions**:
```bash
# Check cloud controller manager
kubectl get pods -n kube-system -l app=aws-cloud-controller-manager
kubectl logs -n kube-system -l app=aws-cloud-controller-manager

# Verify IAM permissions
aws iam get-user
aws iam list-attached-user-policies --user-name opencenter-deploy

# Check ELB creation
aws elb describe-load-balancers
```

### Storage Not Working

**Symptom**: PVCs stuck in `Pending` state

**Causes**:
- EBS CSI driver not running
- AWS credentials invalid
- Volume type not available in region

**Solutions**:
```bash
# Check CSI driver pods
kubectl get pods -n kube-system -l app=ebs-csi-controller

# Check CSI driver logs
kubectl logs -n kube-system -l app=ebs-csi-controller

# Verify EBS volume types
aws ec2 describe-volume-types --region us-east-1
```

### API Server Unreachable

**Symptom**: `kubectl` commands fail with connection errors

**Causes**:
- API server not running
- Security group blocking access
- Kubeconfig incorrect

**Solutions**:
```bash
# Check API server pods on master nodes
ssh ubuntu@<master-ip>
sudo crictl ps | grep kube-apiserver

# Verify security group rules
aws ec2 describe-security-groups --filters "Name=tag:Name,Values=prod-aws-k8s-*"

# Test API connectivity
curl -k https://<api-lb-hostname>:6443/healthz
```

## Production Considerations

Before using this cluster in production, address these items:

### Security Hardening

**Restrict API Access:**
```yaml
# Add to security group rules
aws ec2 authorize-security-group-ingress \
  --group-id sg-0123456789abcdef0 \
  --protocol tcp \
  --port 6443 \
  --cidr 203.0.113.0/24  # Your office network
```

**Enable Pod Security Standards:**
```bash
kubectl label namespace default pod-security.kubernetes.io/enforce=restricted
```

**Configure IAM Roles for Service Accounts (IRSA):**
```bash
# Create OIDC provider for cluster
eksctl utils associate-iam-oidc-provider --cluster prod-aws-k8s --approve
```

### High Availability

**Verify Multi-AZ Deployment:**
```bash
# Check instances are in different AZs
aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=prod-aws-k8s-*" \
  --query 'Reservations[*].Instances[*].[InstanceId,Placement.AvailabilityZone]' \
  --output table
```

**Test Failover:**
```bash
# Simulate master node failure
aws ec2 stop-instances --instance-ids i-0abc123def456789

# Verify cluster still responds
kubectl get nodes

# Restart node
aws ec2 start-instances --instance-ids i-0abc123def456789
```

### Backup Strategy

**Enable EBS Snapshots:**
```bash
# Create snapshot lifecycle policy
aws dlm create-lifecycle-policy \
  --execution-role-arn arn:aws:iam::123456789012:role/AWSDataLifecycleManagerDefaultRole \
  --description "Daily EBS snapshots" \
  --state ENABLED \
  --policy-details file://snapshot-policy.json
```

**Configure Velero:**
```yaml
opencenter:
  services:
    velero:
      enabled: true
      backup_bucket: "prod-aws-k8s-backups"
      region: "us-east-1"
```

### Cost Optimization

**Use Spot Instances for Workers:**
- Consider using EC2 Spot Instances for non-critical workloads
- Mix on-demand and spot instances for cost savings

**Right-Size Instances:**
- Monitor resource usage with Prometheus
- Adjust instance types based on actual usage
- Use AWS Compute Optimizer recommendations

**Enable EBS gp3 Volumes:**
- gp3 volumes offer better price/performance than gp2
- Already configured in this tutorial

## Related Documentation

- [Getting Started](getting-started.md) - Basic openCenter concepts
- [OpenStack Deployment](openstack-deployment.md) - Deploy on OpenStack
- [GitOps Workflow](gitops-workflow.md) - Understanding GitOps with openCenter
- [Configuration Reference](../reference/configuration.md) - Complete configuration options
- [Troubleshooting Guide](../how-to/troubleshooting.md) - Common issues and solutions
- [Backup and Recovery](../how-to/backup-recovery.md) - Backup strategies
- [Upgrading Clusters](../how-to/upgrading-clusters.md) - Cluster upgrade procedures
