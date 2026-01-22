# GitOps Workflow with openCenter


## Table of Contents

- [What You'll Learn](#what-youll-learn)
- [Prerequisites](#prerequisites)
- [Step 1: Understand the GitOps Repository Structure](#step-1-understand-the-gitops-repository-structure)
- [Step 2: Make Your First Configuration Change](#step-2-make-your-first-configuration-change)
- [Step 3: Deploy an Application via GitOps](#step-3-deploy-an-application-via-gitops)
- [Step 4: Manage Secrets with SOPS](#step-4-manage-secrets-with-sops)
- [Step 5: Enable a Service Through Configuration](#step-5-enable-a-service-through-configuration)
- [Step 6: Implement a Rollback](#step-6-implement-a-rollback)
- [Step 7: Multi-Environment Workflow](#step-7-multi-environment-workflow)
- [Step 8: Automate with FluxCD (Optional)](#step-8-automate-with-fluxcd-optional)
- [What You Learned](#what-you-learned)
- [Next Steps](#next-steps)
- [Common Patterns](#common-patterns)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
**doc_type: tutorial**

Learn GitOps principles by managing a Kubernetes cluster through Git. You'll make configuration changes, deploy applications, and understand how openCenter implements GitOps workflows in 45 minutes.

## What You'll Learn

By the end of this tutorial, you'll understand:
- GitOps principles and how openCenter implements them
- How to make configuration changes through Git
- Application deployment via GitOps
- How to manage secrets securely in Git
- Rollback and recovery procedures
- Multi-environment workflows

## Prerequisites

Before starting, you need:
- **openCenter installed** (see [Getting Started](getting-started.md))
- **A running cluster** (use [Kind Local Development](kind-local-dev.md) for practice)
- **Git** installed and configured
- **kubectl** installed
- **SOPS** installed for secrets management
- **45 minutes** of time

### Verify Prerequisites

Check you have a working cluster:

```bash
# Build openCenter
mise run build

# Check active cluster
./bin/openCenter cluster status

# Verify kubectl access
kubectl get nodes
```

If you don't have a cluster, create one with Kind:

```bash
./bin/openCenter cluster init gitops-demo --type kind
./bin/openCenter cluster setup gitops-demo
./bin/openCenter cluster bootstrap gitops-demo
```


## Step 1: Understand the GitOps Repository Structure

openCenter generates a GitOps repository with a specific structure. Let's explore it.

### Navigate to Your GitOps Repository

```bash
# Get the GitOps directory path
GITOPS_DIR=$(./bin/openCenter cluster status --paths | grep "GitOps directory" | awk '{print $3}')
cd $GITOPS_DIR
```

### Explore the Directory Structure

```bash
tree -L 3
```

You'll see:

```
.
├── applications/
│   ├── base/                      # Base application definitions
│   └── overlays/
│       └── gitops-demo/           # Cluster-specific overlays
│           ├── services/          # Enabled services
│           └── managed-services/  # Managed services
├── infrastructure/
│   └── clusters/
│       └── gitops-demo/           # Cluster infrastructure
│           ├── main.tf            # Terraform configuration
│           ├── provider.tf        # Provider configuration
│           ├── Makefile           # Build automation
│           └── flux/              # FluxCD manifests
└── .git/                          # Git repository
```

### Key Directories

**applications/overlays/[cluster-name]/**
- Contains cluster-specific application manifests
- Kustomize overlays for customization
- Service configurations (Calico, cert-manager, etc.)

**infrastructure/clusters/[cluster-name]/**
- Terraform/OpenTofu infrastructure code
- Provider-specific configurations
- FluxCD GitOps automation

**Base vs Overlays:**
- `base/`: Shared, reusable configurations
- `overlays/`: Cluster-specific customizations

This structure follows Kustomize patterns and supports multi-cluster deployments.

## Step 2: Make Your First Configuration Change

Let's change the Kubernetes version through Git.

### View Current Configuration

```bash
# Get cluster name
CLUSTER_NAME=$(./bin/openCenter cluster status --quiet)

# View current Kubernetes version
./bin/openCenter cluster validate $CLUSTER_NAME | grep "kubernetes.version"
```

### Update Configuration File

Edit the cluster configuration:

```bash
# Get config file path
CONFIG_FILE=~/.config/openCenter/clusters/opencenter/.${CLUSTER_NAME}-config.yaml

# Edit the file
vim $CONFIG_FILE
```

Change the Kubernetes version:

```yaml
opencenter:
  cluster:
    kubernetes:
      version: "1.33.5"  # Update to newer version
```

### Validate the Change

```bash
./bin/openCenter cluster validate $CLUSTER_NAME
```

You should see:

```
Validation successful.
```

### Regenerate GitOps Manifests

```bash
./bin/openCenter cluster setup $CLUSTER_NAME --force
```

This regenerates all manifests with the new configuration.


### Commit the Changes to Git

```bash
cd $GITOPS_DIR
git status
```

You'll see modified files. Review the changes:

```bash
git diff
```

Commit the changes:

```bash
git add .
git commit -m "feat: update Kubernetes version to 1.33.5"
```

### Push to Remote Repository

If you have a remote repository configured:

```bash
git push origin main
```

**Key Concept**: In GitOps, Git is the single source of truth. All changes flow through Git commits.

## Step 3: Deploy an Application via GitOps

Let's deploy a sample application using the GitOps workflow.

### Create Application Manifests

Create a new application directory:

```bash
cd $GITOPS_DIR/applications/overlays/$CLUSTER_NAME
mkdir -p apps/hello-world
```

Create a deployment manifest:

```bash
cat > apps/hello-world/deployment.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
    spec:
      containers:
      - name: hello-world
        image: nginxdemos/hello:latest
        ports:
        - containerPort: 80
EOF
```

Create a service manifest:

```bash
cat > apps/hello-world/service.yaml <<EOF
apiVersion: v1
kind: Service
metadata:
  name: hello-world
  namespace: default
spec:
  type: LoadBalancer
  selector:
    app: hello-world
  ports:
  - port: 80
    targetPort: 80
EOF
```

Create a kustomization file:

```bash
cat > apps/hello-world/kustomization.yaml <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - service.yaml
EOF
```

### Commit and Push

```bash
git add apps/hello-world/
git commit -m "feat: add hello-world application"
git push origin main
```

### Apply the Application

```bash
kubectl apply -k apps/hello-world/
```

Verify deployment:

```bash
kubectl get deployments hello-world
kubectl get pods -l app=hello-world
kubectl get svc hello-world
```

**GitOps Principle**: The Git repository contains the desired state. Applying manifests from Git ensures consistency.


## Step 4: Manage Secrets with SOPS

Secrets should never be committed to Git in plaintext. Let's use SOPS to encrypt them.

### Create a Secret File

Create a secret for your application:

```bash
cd $GITOPS_DIR/applications/overlays/$CLUSTER_NAME/apps/hello-world

cat > secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: hello-world-secret
  namespace: default
type: Opaque
stringData:
  database-password: "super-secret-password"
  api-key: "my-api-key-12345"
EOF
```

### Encrypt with SOPS

Get your SOPS Age key path:

```bash
SOPS_KEY=$(./bin/openCenter cluster status --paths | grep "SOPS key" | awk '{print $3}')
echo $SOPS_KEY
```

Encrypt the secret:

```bash
export SOPS_AGE_KEY_FILE=$SOPS_KEY
sops -e -i secret.yaml
```

View the encrypted file:

```bash
cat secret.yaml
```

You'll see encrypted values:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hello-world-secret
  namespace: default
type: Opaque
stringData:
  database-password: ENC[AES256_GCM,data:encrypted_data_here,iv:...,tag:...,type:str]
  api-key: ENC[AES256_GCM,data:encrypted_data_here,iv:...,tag:...,type:str]
sops:
  age:
    - recipient: age1234567890abcdef...
      enc: ENC[AES256_GCM,data:...,type:str]
  lastmodified: "2025-01-19T12:00:00Z"
  mac: ENC[AES256_GCM,data:...,type:str]
```

### Update Kustomization

Add the secret to kustomization:

```bash
cat >> kustomization.yaml <<EOF
  - secret.yaml
EOF
```

### Commit Encrypted Secret

```bash
git add secret.yaml kustomization.yaml
git commit -m "feat: add encrypted secrets for hello-world"
git push origin main
```

**Security Principle**: Encrypted secrets can be safely committed to Git. Only those with the Age key can decrypt them.

### Decrypt and Apply

To apply the secret to your cluster:

```bash
# Decrypt and apply in one command
sops -d secret.yaml | kubectl apply -f -

# Or use kustomize with SOPS
kubectl apply -k .
```

Verify the secret:

```bash
kubectl get secret hello-world-secret
kubectl get secret hello-world-secret -o jsonpath='{.data.database-password}' | base64 -d
```


## Step 5: Enable a Service Through Configuration

Let's enable cert-manager through the openCenter configuration.

### Update Cluster Configuration

Edit your cluster configuration:

```bash
CONFIG_FILE=~/.config/openCenter/clusters/opencenter/.${CLUSTER_NAME}-config.yaml
vim $CONFIG_FILE
```

Enable cert-manager:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      version: "v1.13.3"
```

### Validate and Regenerate

```bash
./bin/openCenter cluster validate $CLUSTER_NAME
./bin/openCenter cluster setup $CLUSTER_NAME --force
```

### Review Generated Manifests

Check what was generated:

```bash
cd $GITOPS_DIR/applications/overlays/$CLUSTER_NAME
ls -la services/cert-manager/
```

You'll see:
- FluxCD HelmRelease manifest
- Namespace configuration
- Service-specific settings

### Commit and Apply

```bash
git add .
git commit -m "feat: enable cert-manager service"
git push origin main

# Apply to cluster
kubectl apply -k services/cert-manager/
```

Verify cert-manager is running:

```bash
kubectl get pods -n cert-manager
```

**Configuration-Driven**: Services are enabled through configuration, not manual kubectl commands.

## Step 6: Implement a Rollback

Let's practice rolling back a change using Git.

### Make a Breaking Change

Edit the hello-world deployment to use a non-existent image:

```bash
cd $GITOPS_DIR/applications/overlays/$CLUSTER_NAME/apps/hello-world
vim deployment.yaml
```

Change the image to something invalid:

```yaml
      containers:
      - name: hello-world
        image: nginx:nonexistent-tag  # This will fail
```

Commit and apply:

```bash
git add deployment.yaml
git commit -m "test: intentionally break deployment"
kubectl apply -k .
```

Watch the deployment fail:

```bash
kubectl get pods -l app=hello-world -w
```

You'll see pods in `ImagePullBackOff` state.

### Rollback Using Git

Find the last good commit:

```bash
git log --oneline -5
```

Revert to the previous commit:

```bash
git revert HEAD
```

Or reset to a specific commit:

```bash
# Get the commit hash of the last good state
GOOD_COMMIT=$(git log --oneline -2 | tail -1 | awk '{print $1}')
git reset --hard $GOOD_COMMIT
```

Apply the rolled-back configuration:

```bash
kubectl apply -k .
```

Verify pods are healthy again:

```bash
kubectl get pods -l app=hello-world
```

**GitOps Recovery**: Git history provides a complete audit trail and easy rollback mechanism.


## Step 7: Multi-Environment Workflow

Let's set up a staging environment to demonstrate multi-environment GitOps.

### Create Staging Cluster Configuration

```bash
./bin/openCenter cluster init gitops-staging \
  --opencenter.meta.env=staging \
  --opencenter.meta.region=local \
  --type kind
```

### Setup Staging Cluster

```bash
./bin/openCenter cluster setup gitops-staging
./bin/openCenter cluster bootstrap gitops-staging
```

### Explore Multi-Cluster Structure

```bash
# Get staging GitOps directory
STAGING_GITOPS=$(./bin/openCenter cluster status gitops-staging --paths | grep "GitOps directory" | awk '{print $3}')

# Compare structures
tree -L 3 $GITOPS_DIR
tree -L 3 $STAGING_GITOPS
```

Both clusters have separate overlay directories:
- `applications/overlays/gitops-demo/` (production)
- `applications/overlays/gitops-staging/` (staging)

### Deploy to Staging First

Copy your application to staging:

```bash
cp -r $GITOPS_DIR/applications/overlays/$CLUSTER_NAME/apps/hello-world \
      $STAGING_GITOPS/applications/overlays/gitops-staging/apps/
```

Modify for staging (e.g., fewer replicas):

```bash
cd $STAGING_GITOPS/applications/overlays/gitops-staging/apps/hello-world
vim deployment.yaml
```

Change replicas:

```yaml
spec:
  replicas: 1  # Staging uses fewer resources
```

Commit and apply to staging:

```bash
cd $STAGING_GITOPS
git add .
git commit -m "feat: deploy hello-world to staging"

# Switch to staging cluster
kubectl config use-context kind-gitops-staging
kubectl apply -k applications/overlays/gitops-staging/apps/hello-world/
```

### Promote to Production

After testing in staging, promote to production:

```bash
# Copy tested configuration to production
cp -r $STAGING_GITOPS/applications/overlays/gitops-staging/apps/hello-world \
      $GITOPS_DIR/applications/overlays/$CLUSTER_NAME/apps/

# Adjust for production (more replicas)
cd $GITOPS_DIR/applications/overlays/$CLUSTER_NAME/apps/hello-world
vim deployment.yaml
# Change replicas back to 3

# Commit and apply to production
git add .
git commit -m "feat: promote hello-world to production"

# Switch to production cluster
kubectl config use-context kind-gitops-demo
kubectl apply -k .
```

**Multi-Environment Pattern**: Test in staging, promote to production through Git.


## Step 8: Automate with FluxCD (Optional)

For true GitOps automation, enable FluxCD to watch your Git repository.

### Install FluxCD

```bash
# Install Flux CLI
mise install flux

# Or use the binary from your package manager
# brew install fluxcd/tap/flux
```

### Bootstrap FluxCD

```bash
# Export GitHub token (or GitLab, etc.)
export GITHUB_TOKEN=<your-token>

# Bootstrap Flux
flux bootstrap github \
  --owner=<your-github-username> \
  --repository=gitops-demo \
  --branch=main \
  --path=applications/overlays/$CLUSTER_NAME \
  --personal
```

### Create GitRepository Source

```bash
flux create source git gitops-demo \
  --url=https://github.com/<your-username>/gitops-demo \
  --branch=main \
  --interval=1m
```

### Create Kustomization

```bash
flux create kustomization apps \
  --source=gitops-demo \
  --path="./applications/overlays/$CLUSTER_NAME/apps" \
  --prune=true \
  --interval=5m
```

### Verify Automation

Make a change and push to Git:

```bash
cd $GITOPS_DIR/applications/overlays/$CLUSTER_NAME/apps/hello-world
vim deployment.yaml
# Change replicas to 5

git add .
git commit -m "feat: scale hello-world to 5 replicas"
git push origin main
```

Watch Flux automatically apply the change:

```bash
flux get kustomizations --watch
kubectl get deployments hello-world -w
```

Within 5 minutes, Flux will detect the change and apply it automatically.

**Full GitOps**: With FluxCD, your cluster automatically syncs with Git. No manual kubectl apply needed.

## What You Learned

You now understand:

1. **GitOps Repository Structure**: How openCenter organizes manifests
2. **Configuration Changes**: Making changes through the configuration file
3. **Application Deployment**: Deploying apps via Git
4. **Secrets Management**: Using SOPS to encrypt secrets safely
5. **Service Enablement**: Enabling services through configuration
6. **Rollback Procedures**: Using Git history for recovery
7. **Multi-Environment Workflows**: Managing staging and production
8. **Automation with FluxCD**: Continuous deployment from Git

### Key GitOps Principles

**Git as Single Source of Truth:**
- All configuration lives in Git
- Git history provides audit trail
- Changes are traceable and reversible

**Declarative Configuration:**
- Describe desired state, not imperative steps
- Kubernetes reconciles actual state to desired state
- Configuration files are self-documenting

**Automated Synchronization:**
- Tools like FluxCD watch Git for changes
- Automatic application of changes
- Drift detection and correction

**Security:**
- Secrets encrypted with SOPS
- Access control through Git permissions
- Audit trail of all changes


## Next Steps

Now that you understand GitOps workflows, you can:

### Implement CI/CD Pipelines

Integrate GitOps with your CI/CD system:

```yaml
# Example GitHub Actions workflow
name: Deploy to Staging
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Validate manifests
        run: kubectl apply --dry-run=client -k applications/overlays/staging/
      - name: Deploy to staging
        run: |
          kubectl config use-context staging
          kubectl apply -k applications/overlays/staging/
```

### Set Up Pull Request Workflows

Use pull requests for change review:

1. Create feature branch for changes
2. Open PR with manifest changes
3. Run validation in CI
4. Review and approve
5. Merge to main triggers deployment

### Implement Progressive Delivery

Use Flagger for canary deployments:

```bash
# Install Flagger
kubectl apply -k github.com/fluxcd/flagger//kustomize/linkerd

# Create canary deployment
flux create canary hello-world \
  --target-kind=Deployment \
  --target-name=hello-world \
  --service-name=hello-world \
  --interval=1m
```

### Monitor GitOps Operations

Set up monitoring for GitOps:

```bash
# View Flux events
flux events

# Check reconciliation status
flux get all

# View logs
flux logs --all-namespaces
```

### Advanced Secrets Management

Integrate with external secret stores:

- **External Secrets Operator**: Sync from AWS Secrets Manager, Vault, etc.
- **Sealed Secrets**: Alternative to SOPS for Kubernetes-native encryption
- **Vault**: Enterprise secret management

See [Secrets Management](../how-to/secrets-management.md) for details.

### Multi-Cluster GitOps

Manage multiple clusters from one repository:

```
gitops-repo/
├── clusters/
│   ├── production/
│   │   ├── us-east-1/
│   │   └── us-west-2/
│   └── staging/
│       └── us-east-1/
└── apps/
    ├── base/
    └── overlays/
```

See [Multi-Cluster Management](multi-cluster.md) for patterns.

## Common Patterns

### Environment-Specific Configuration

Use Kustomize overlays for environment differences:

```yaml
# base/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  replicas: 1  # Default

# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patches:
  - patch: |-
      - op: replace
        path: /spec/replicas
        value: 5
    target:
      kind: Deployment
      name: app
```

### Shared Services

Deploy shared services once, used by all applications:

```
applications/
├── infrastructure/
│   ├── cert-manager/
│   ├── ingress-nginx/
│   └── monitoring/
└── apps/
    ├── app1/
    └── app2/
```

### Configuration Drift Detection

Detect when cluster state differs from Git:

```bash
# With FluxCD
flux diff kustomization apps

# Manual check
kubectl diff -k applications/overlays/$CLUSTER_NAME/apps/
```


## Troubleshooting

### Git Conflicts

**Symptom**: Merge conflicts when pulling changes

**Solution**:
```bash
# Fetch latest changes
git fetch origin main

# Rebase your changes
git rebase origin/main

# Resolve conflicts
git status
# Edit conflicting files
git add .
git rebase --continue
```

### SOPS Decryption Fails

**Symptom**: `failed to get the data key` error

**Solution**:
```bash
# Verify Age key is set
echo $SOPS_AGE_KEY_FILE

# Set the correct key
export SOPS_AGE_KEY_FILE=~/.config/openCenter/clusters/opencenter/secrets/age/keys/$CLUSTER_NAME-key.txt

# Test decryption
sops -d secret.yaml
```

### FluxCD Not Syncing

**Symptom**: Changes in Git not applied to cluster

**Solution**:
```bash
# Check Flux status
flux get all

# Force reconciliation
flux reconcile kustomization apps --with-source

# Check logs
flux logs --all-namespaces --follow

# Verify GitRepository source
flux get sources git
```

### Kustomize Build Fails

**Symptom**: `Error: accumulating resources` when applying

**Solution**:
```bash
# Test kustomize build locally
kubectl kustomize applications/overlays/$CLUSTER_NAME/apps/hello-world/

# Check for syntax errors
yamllint applications/overlays/$CLUSTER_NAME/apps/hello-world/*.yaml

# Verify resource references
kubectl apply --dry-run=client -k applications/overlays/$CLUSTER_NAME/apps/hello-world/
```

### Secrets Not Decrypting in Cluster

**Symptom**: Pods can't access secret values

**Solution**:
```bash
# Verify secret exists
kubectl get secret hello-world-secret

# Check secret data
kubectl get secret hello-world-secret -o yaml

# Ensure SOPS decrypted before applying
sops -d secret.yaml | kubectl apply -f -

# Or use SOPS kubectl plugin
kubectl apply -f <(sops -d secret.yaml)
```

## Best Practices

### Commit Messages

Use conventional commits for clarity:

```bash
feat: add new application deployment
fix: correct service port configuration
docs: update README with deployment instructions
chore: update dependencies
```

### Branch Strategy

**Trunk-Based Development:**
- Main branch is always deployable
- Short-lived feature branches
- Frequent integration

**GitFlow:**
- `main`: production
- `develop`: integration
- `feature/*`: new features
- `hotfix/*`: urgent fixes

### Code Review

Review all changes before merging:

1. Validate manifests in CI
2. Check for security issues
3. Verify resource limits
4. Test in staging first
5. Document breaking changes

### Directory Organization

Keep manifests organized:

```
apps/
├── app-name/
│   ├── base/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   └── kustomization.yaml
│   └── overlays/
│       ├── staging/
│       └── production/
```

### Documentation

Document your GitOps setup:

- README in repository root
- Architecture diagrams
- Deployment procedures
- Rollback instructions
- Contact information

## Related Documentation

- [Getting Started](getting-started.md) - Basic openCenter concepts
- [OpenStack Deployment](openstack-deployment.md) - Deploy on OpenStack
- [AWS Deployment](aws-deployment.md) - Deploy on AWS
- [GitOps Workflow Explanation](../explanation/gitops-workflow.md) - Deep dive into GitOps concepts
- [Configuration Reference](../reference/configuration.md) - Complete configuration options
- [Secrets Management](../how-to/secrets-management.md) - Advanced secrets handling
- [Deploying Changes](../how-to/deploying-changes.md) - Deployment workflows
- [Troubleshooting Guide](../how-to/troubleshooting.md) - Common issues and solutions
