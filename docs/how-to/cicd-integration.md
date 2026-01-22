# Integrating openCenter with CI/CD Pipelines


## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Understanding CI/CD Integration](#understanding-cicd-integration)
- [Task 1: Set Up GitHub Actions Pipeline](#task-1-set-up-github-actions-pipeline)
- [Task 2: Set Up GitLab CI Pipeline](#task-2-set-up-gitlab-ci-pipeline)
- [Task 3: Set Up Jenkins Pipeline](#task-3-set-up-jenkins-pipeline)
- [Task 4: Implement Automated Testing](#task-4-implement-automated-testing)
- [Task 5: Set Up Monitoring and Alerting](#task-5-set-up-monitoring-and-alerting)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)
**doc_type**: how-to  
**priority**: 3  
**audience**: DevOps engineers automating cluster deployments  
**related_docs**:
- [GitOps Workflows](./gitops-workflows.md)
- [Automation Guide](./automation.md)
- [Security Best Practices](../explanation/security-model.md)

## Overview

This guide shows you how to integrate openCenter into CI/CD pipelines for automated cluster provisioning, validation, and deployment. You'll learn how to set up pipelines in GitHub Actions, GitLab CI, and Jenkins.

## Prerequisites

- openCenter CLI installed
- CI/CD platform access (GitHub Actions, GitLab CI, or Jenkins)
- Git repository for cluster configurations
- Cloud provider credentials configured

## Understanding CI/CD Integration

openCenter supports automation through:

1. **Validation Pipelines**: Validate configuration changes before merge
2. **Deployment Pipelines**: Automate cluster provisioning and setup
3. **Drift Detection**: Monitor and alert on configuration drift
4. **Disaster Recovery**: Automated backup and restore procedures

## Task 1: Set Up GitHub Actions Pipeline

### Step 1: Create Workflow Directory

```bash
mkdir -p .github/workflows
```

### Step 2: Create Validation Workflow

Create `.github/workflows/validate-cluster-config.yaml`:

```yaml
name: Validate Cluster Configuration

on:
  pull_request:
    paths:
      - 'clusters/**/*.yaml'
      - '.github/workflows/validate-cluster-config.yaml'
  push:
    branches:
      - main
    paths:
      - 'clusters/**/*.yaml'

jobs:
  validate:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install mise
        uses: jdx/mise-action@v2
        with:
          version: latest
      
      - name: Install openCenter CLI
        run: |
          # Download latest release
          curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
            -o /usr/local/bin/openCenter
          chmod +x /usr/local/bin/openCenter
          openCenter version
      
      - name: Validate cluster configurations
        run: |
          for config in clusters/**/*.yaml; do
            echo "Validating $config..."
            openCenter config validate --config "$config"
          done
      
      - name: Run schema validation
        run: |
          for config in clusters/**/*.yaml; do
            echo "Schema validation for $config..."
            openCenter config validate --config "$config" --schema-only
          done
      
      - name: Check for secrets in plaintext
        run: |
          # Ensure no plaintext secrets are committed
          if grep -r "password:\|secret:\|token:" clusters/ --include="*.yaml" | grep -v "sops"; then
            echo "ERROR: Plaintext secrets detected!"
            exit 1
          fi
          echo "✓ No plaintext secrets found"
      
      - name: Generate validation report
        if: always()
        run: |
          echo "## Validation Report" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "- Configurations validated: $(find clusters -name '*.yaml' | wc -l)" >> $GITHUB_STEP_SUMMARY
          echo "- Status: ${{ job.status }}" >> $GITHUB_STEP_SUMMARY
```

### Step 3: Create Deployment Workflow

Create `.github/workflows/deploy-cluster.yaml`:

```yaml
name: Deploy Cluster

on:
  workflow_dispatch:
    inputs:
      cluster_name:
        description: 'Cluster name to deploy'
        required: true
        type: string
      environment:
        description: 'Target environment'
        required: true
        type: choice
        options:
          - development
          - staging
          - production
      dry_run:
        description: 'Perform dry run only'
        required: false
        type: boolean
        default: false

env:
  OPENCENTER_CONFIG_DIR: ${{ github.workspace }}/clusters

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: ${{ inputs.environment }}
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install mise
        uses: jdx/mise-action@v2
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
            -o /usr/local/bin/openCenter
          chmod +x /usr/local/bin/openCenter
      
      - name: Configure cloud credentials
        env:
          OS_AUTH_URL: ${{ secrets.OS_AUTH_URL }}
          OS_USERNAME: ${{ secrets.OS_USERNAME }}
          OS_PASSWORD: ${{ secrets.OS_PASSWORD }}
          OS_PROJECT_NAME: ${{ secrets.OS_PROJECT_NAME }}
          OS_USER_DOMAIN_NAME: ${{ secrets.OS_USER_DOMAIN_NAME }}
          OS_PROJECT_DOMAIN_NAME: ${{ secrets.OS_PROJECT_DOMAIN_NAME }}
        run: |
          # Credentials are set via environment variables
          echo "✓ Cloud credentials configured"
      
      - name: Decrypt SOPS secrets
        env:
          SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
        run: |
          # Install SOPS
          curl -L https://github.com/getsops/sops/releases/latest/download/sops-v3.8.1.linux.amd64 \
            -o /usr/local/bin/sops
          chmod +x /usr/local/bin/sops
          
          # Verify SOPS can decrypt
          echo "$SOPS_AGE_KEY" > /tmp/age-key.txt
          export SOPS_AGE_KEY_FILE=/tmp/age-key.txt
          
          echo "✓ SOPS configured"
      
      - name: Validate cluster configuration
        run: |
          openCenter config validate \
            --config clusters/${{ inputs.cluster_name }}.yaml
      
      - name: Run preflight checks
        run: |
          openCenter cluster preflight ${{ inputs.cluster_name }} \
            --config clusters/${{ inputs.cluster_name }}.yaml
      
      - name: Generate GitOps repository
        if: ${{ !inputs.dry_run }}
        run: |
          openCenter cluster setup ${{ inputs.cluster_name }} \
            --config clusters/${{ inputs.cluster_name }}.yaml \
            --render
      
      - name: Provision infrastructure
        if: ${{ !inputs.dry_run }}
        run: |
          openCenter cluster bootstrap ${{ inputs.cluster_name }} \
            --config clusters/${{ inputs.cluster_name }}.yaml \
            --skip-gitops
      
      - name: Deploy cluster services
        if: ${{ !inputs.dry_run }}
        run: |
          # Apply GitOps manifests
          cd ~/gitops/${{ inputs.cluster_name }}
          
          # Bootstrap FluxCD
          flux bootstrap git \
            --url=https://github.com/${{ github.repository }} \
            --branch=main \
            --path=clusters/${{ inputs.cluster_name }}
      
      - name: Verify deployment
        if: ${{ !inputs.dry_run }}
        run: |
          # Wait for cluster to be ready
          timeout 600 bash -c 'until kubectl get nodes; do sleep 10; done'
          
          # Check critical services
          kubectl get pods -A
          kubectl get nodes
      
      - name: Generate deployment report
        if: always()
        run: |
          echo "## Deployment Report" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "- Cluster: ${{ inputs.cluster_name }}" >> $GITHUB_STEP_SUMMARY
          echo "- Environment: ${{ inputs.environment }}" >> $GITHUB_STEP_SUMMARY
          echo "- Dry Run: ${{ inputs.dry_run }}" >> $GITHUB_STEP_SUMMARY
          echo "- Status: ${{ job.status }}" >> $GITHUB_STEP_SUMMARY
      
      - name: Notify on failure
        if: failure()
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.create({
              owner: context.repo.owner,
              repo: context.repo.repo,
              title: `Deployment failed: ${{ inputs.cluster_name }}`,
              body: `Deployment of cluster ${{ inputs.cluster_name }} to ${{ inputs.environment }} failed.\n\nSee workflow run: ${context.serverUrl}/${context.repo.owner}/${context.repo.repo}/actions/runs/${context.runId}`,
              labels: ['deployment-failure', 'automated']
            })
```

### Step 4: Create Drift Detection Workflow

Create `.github/workflows/drift-detection.yaml`:

```yaml
name: Drift Detection

on:
  schedule:
    # Run every 6 hours
    - cron: '0 */6 * * *'
  workflow_dispatch:

jobs:
  detect-drift:
    runs-on: ubuntu-latest
    
    strategy:
      matrix:
        cluster:
          - production-cluster-1
          - production-cluster-2
          - staging-cluster
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
            -o /usr/local/bin/openCenter
          chmod +x /usr/local/bin/openCenter
      
      - name: Configure kubeconfig
        env:
          KUBECONFIG_DATA: ${{ secrets[format('KUBECONFIG_{0}', matrix.cluster)] }}
        run: |
          mkdir -p ~/.kube
          echo "$KUBECONFIG_DATA" | base64 -d > ~/.kube/config
      
      - name: Detect configuration drift
        id: drift
        run: |
          # Compare deployed state with desired state
          openCenter cluster validate ${{ matrix.cluster }} \
            --config clusters/${{ matrix.cluster }}.yaml \
            --check-deployed > drift-report.txt || echo "drift_detected=true" >> $GITHUB_OUTPUT
      
      - name: Create drift issue
        if: steps.drift.outputs.drift_detected == 'true'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const driftReport = fs.readFileSync('drift-report.txt', 'utf8');
            
            github.rest.issues.create({
              owner: context.repo.owner,
              repo: context.repo.repo,
              title: `Configuration drift detected: ${{ matrix.cluster }}`,
              body: `Configuration drift detected in cluster ${{ matrix.cluster }}.\n\n\`\`\`\n${driftReport}\n\`\`\``,
              labels: ['drift-detection', 'automated', '${{ matrix.cluster }}']
            })
```

## Task 2: Set Up GitLab CI Pipeline

### Step 1: Create GitLab CI Configuration

Create `.gitlab-ci.yml`:

```yaml
stages:
  - validate
  - build
  - deploy
  - verify

variables:
  OPENCENTER_VERSION: "latest"
  OPENCENTER_CONFIG_DIR: "${CI_PROJECT_DIR}/clusters"

.install_opencenter: &install_opencenter
  - |
    curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
      -o /usr/local/bin/openCenter
    chmod +x /usr/local/bin/openCenter
    openCenter version

validate:config:
  stage: validate
  image: ubuntu:22.04
  before_script:
    - apt-get update && apt-get install -y curl
    - *install_opencenter
  script:
    - |
      for config in clusters/**/*.yaml; do
        echo "Validating $config..."
        openCenter config validate --config "$config"
      done
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
    - if: '$CI_COMMIT_BRANCH == "main"'

validate:secrets:
  stage: validate
  image: ubuntu:22.04
  script:
    - |
      if grep -r "password:\|secret:\|token:" clusters/ --include="*.yaml" | grep -v "sops"; then
        echo "ERROR: Plaintext secrets detected!"
        exit 1
      fi
      echo "✓ No plaintext secrets found"
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'

build:gitops:
  stage: build
  image: ubuntu:22.04
  before_script:
    - apt-get update && apt-get install -y curl
    - *install_opencenter
  script:
    - |
      openCenter cluster setup ${CLUSTER_NAME} \
        --config clusters/${CLUSTER_NAME}.yaml \
        --render
  artifacts:
    paths:
      - ~/gitops/${CLUSTER_NAME}
    expire_in: 1 week
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
      when: manual

deploy:cluster:
  stage: deploy
  image: ubuntu:22.04
  environment:
    name: $ENVIRONMENT
    action: start
  before_script:
    - apt-get update && apt-get install -y curl
    - *install_opencenter
  script:
    - |
      # Configure cloud credentials
      export OS_AUTH_URL="${OS_AUTH_URL}"
      export OS_USERNAME="${OS_USERNAME}"
      export OS_PASSWORD="${OS_PASSWORD}"
      export OS_PROJECT_NAME="${OS_PROJECT_NAME}"
      
      # Run preflight checks
      openCenter cluster preflight ${CLUSTER_NAME} \
        --config clusters/${CLUSTER_NAME}.yaml
      
      # Bootstrap cluster
      openCenter cluster bootstrap ${CLUSTER_NAME} \
        --config clusters/${CLUSTER_NAME}.yaml
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
      when: manual
  dependencies:
    - build:gitops

verify:deployment:
  stage: verify
  image: bitnami/kubectl:latest
  script:
    - kubectl get nodes
    - kubectl get pods -A
    - |
      # Wait for all pods to be ready
      kubectl wait --for=condition=ready pod --all -A --timeout=600s
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
      when: on_success
  dependencies:
    - deploy:cluster
```

### Step 2: Configure GitLab CI Variables

In GitLab project settings, add CI/CD variables:

```
OS_AUTH_URL: https://openstack.example.com:5000/v3
OS_USERNAME: admin
OS_PASSWORD: <encrypted>
OS_PROJECT_NAME: my-project
OS_USER_DOMAIN_NAME: Default
OS_PROJECT_DOMAIN_NAME: Default
SOPS_AGE_KEY: <encrypted>
CLUSTER_NAME: production-cluster
ENVIRONMENT: production
```

## Task 3: Set Up Jenkins Pipeline

### Step 1: Create Jenkinsfile

Create `Jenkinsfile`:

```groovy
pipeline {
    agent any
    
    parameters {
        choice(
            name: 'CLUSTER_NAME',
            choices: ['dev-cluster', 'staging-cluster', 'prod-cluster'],
            description: 'Cluster to deploy'
        )
        choice(
            name: 'ACTION',
            choices: ['validate', 'setup', 'bootstrap', 'destroy'],
            description: 'Action to perform'
        )
        booleanParam(
            name: 'DRY_RUN',
            defaultValue: false,
            description: 'Perform dry run only'
        )
    }
    
    environment {
        OPENCENTER_CONFIG_DIR = "${WORKSPACE}/clusters"
        OS_AUTH_URL = credentials('openstack-auth-url')
        OS_USERNAME = credentials('openstack-username')
        OS_PASSWORD = credentials('openstack-password')
        OS_PROJECT_NAME = credentials('openstack-project')
        SOPS_AGE_KEY = credentials('sops-age-key')
    }
    
    stages {
        stage('Setup') {
            steps {
                script {
                    sh '''
                        # Install openCenter CLI
                        curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
                            -o /usr/local/bin/openCenter
                        chmod +x /usr/local/bin/openCenter
                        openCenter version
                        
                        # Install SOPS
                        curl -L https://github.com/getsops/sops/releases/latest/download/sops-v3.8.1.linux.amd64 \
                            -o /usr/local/bin/sops
                        chmod +x /usr/local/bin/sops
                    '''
                }
            }
        }
        
        stage('Validate') {
            steps {
                script {
                    sh """
                        openCenter config validate \
                            --config clusters/${params.CLUSTER_NAME}.yaml
                    """
                }
            }
        }
        
        stage('Preflight Checks') {
            when {
                expression { params.ACTION in ['setup', 'bootstrap'] }
            }
            steps {
                script {
                    sh """
                        openCenter cluster preflight ${params.CLUSTER_NAME} \
                            --config clusters/${params.CLUSTER_NAME}.yaml
                    """
                }
            }
        }
        
        stage('Setup GitOps') {
            when {
                expression { params.ACTION == 'setup' }
            }
            steps {
                script {
                    sh """
                        openCenter cluster setup ${params.CLUSTER_NAME} \
                            --config clusters/${params.CLUSTER_NAME}.yaml \
                            --render
                    """
                }
            }
        }
        
        stage('Bootstrap Cluster') {
            when {
                expression { params.ACTION == 'bootstrap' && !params.DRY_RUN }
            }
            steps {
                script {
                    sh """
                        openCenter cluster bootstrap ${params.CLUSTER_NAME} \
                            --config clusters/${params.CLUSTER_NAME}.yaml
                    """
                }
            }
        }
        
        stage('Verify Deployment') {
            when {
                expression { params.ACTION == 'bootstrap' && !params.DRY_RUN }
            }
            steps {
                script {
                    sh '''
                        # Wait for cluster to be ready
                        timeout 600 bash -c 'until kubectl get nodes; do sleep 10; done'
                        
                        # Verify critical services
                        kubectl get nodes
                        kubectl get pods -A
                    '''
                }
            }
        }
        
        stage('Destroy Cluster') {
            when {
                expression { params.ACTION == 'destroy' }
            }
            steps {
                input message: 'Are you sure you want to destroy the cluster?', ok: 'Destroy'
                script {
                    sh """
                        openCenter cluster destroy ${params.CLUSTER_NAME} \
                            --config clusters/${params.CLUSTER_NAME}.yaml \
                            --force
                    """
                }
            }
        }
    }
    
    post {
        always {
            script {
                // Archive logs
                archiveArtifacts artifacts: '**/*.log', allowEmptyArchive: true
            }
        }
        success {
            echo "Pipeline completed successfully!"
        }
        failure {
            emailext (
                subject: "Pipeline Failed: ${params.CLUSTER_NAME}",
                body: "Pipeline failed for cluster ${params.CLUSTER_NAME}. Check ${env.BUILD_URL} for details.",
                to: "${env.CHANGE_AUTHOR_EMAIL}"
            )
        }
    }
}
```

### Step 2: Configure Jenkins Credentials

Add credentials in Jenkins:

1. Navigate to "Manage Jenkins" → "Manage Credentials"
2. Add credentials:
   - `openstack-auth-url`: Secret text
   - `openstack-username`: Secret text
   - `openstack-password`: Secret text
   - `openstack-project`: Secret text
   - `sops-age-key`: Secret file

## Task 4: Implement Automated Testing

### Step 1: Create Test Workflow

Create `.github/workflows/test-cluster-config.yaml`:

```yaml
name: Test Cluster Configuration

on:
  pull_request:
    paths:
      - 'clusters/**/*.yaml'

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install dependencies
        run: |
          # Install openCenter CLI
          curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
            -o /usr/local/bin/openCenter
          chmod +x /usr/local/bin/openCenter
          
          # Install kind for local testing
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
          chmod +x ./kind
          sudo mv ./kind /usr/local/bin/kind
      
      - name: Create test cluster
        run: |
          kind create cluster --name test-cluster
      
      - name: Test cluster setup
        run: |
          # Test GitOps generation
          openCenter cluster setup test-cluster \
            --config clusters/test-cluster.yaml \
            --render
      
      - name: Validate generated manifests
        run: |
          # Validate Kubernetes manifests
          kubectl --dry-run=server -f ~/gitops/test-cluster/applications/
      
      - name: Cleanup
        if: always()
        run: |
          kind delete cluster --name test-cluster
```

## Task 5: Set Up Monitoring and Alerting

### Step 1: Create Monitoring Workflow

Create `.github/workflows/monitor-clusters.yaml`:

```yaml
name: Monitor Clusters

on:
  schedule:
    - cron: '*/15 * * * *'  # Every 15 minutes
  workflow_dispatch:

jobs:
  monitor:
    runs-on: ubuntu-latest
    
    strategy:
      matrix:
        cluster:
          - production-cluster-1
          - production-cluster-2
    
    steps:
      - name: Check cluster health
        env:
          KUBECONFIG_DATA: ${{ secrets[format('KUBECONFIG_{0}', matrix.cluster)] }}
        run: |
          mkdir -p ~/.kube
          echo "$KUBECONFIG_DATA" | base64 -d > ~/.kube/config
          
          # Check node status
          kubectl get nodes -o json | jq '.items[] | select(.status.conditions[] | select(.type=="Ready" and .status!="True")) | .metadata.name' > unhealthy-nodes.txt
          
          if [ -s unhealthy-nodes.txt ]; then
            echo "unhealthy_nodes=true" >> $GITHUB_OUTPUT
          fi
      
      - name: Alert on unhealthy nodes
        if: steps.monitor.outputs.unhealthy_nodes == 'true'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const nodes = fs.readFileSync('unhealthy-nodes.txt', 'utf8');
            
            github.rest.issues.create({
              owner: context.repo.owner,
              repo: context.repo.repo,
              title: `Unhealthy nodes detected: ${{ matrix.cluster }}`,
              body: `The following nodes are unhealthy:\n\n\`\`\`\n${nodes}\n\`\`\``,
              labels: ['cluster-health', 'automated', '${{ matrix.cluster }}']
            })
```

## Best Practices

1. **Use Secrets Management**: Store credentials in CI/CD secrets, never in code
2. **Implement Validation**: Validate configurations before deployment
3. **Enable Dry Runs**: Test changes with dry runs before applying
4. **Monitor Deployments**: Set up health checks and alerting
5. **Automate Rollbacks**: Implement automatic rollback on failure
6. **Use Environments**: Separate dev, staging, and production pipelines
7. **Audit Logs**: Enable audit logging for all CI/CD operations
8. **Version Control**: Tag releases and maintain changelog
9. **Test Locally**: Test pipeline changes locally before committing
10. **Document Workflows**: Maintain clear documentation for all pipelines

## Troubleshooting

### Pipeline Fails on Validation

**Problem**: Configuration validation fails in CI/CD

**Solution**: Run validation locally first:
```bash
mise run build
./bin/openCenter config validate --config clusters/my-cluster.yaml
```

### Credentials Not Working

**Problem**: Cloud provider authentication fails

**Solution**: Verify credentials are correctly set:
```bash
# Test credentials locally
export OS_AUTH_URL="..."
export OS_USERNAME="..."
openCenter cluster preflight my-cluster
```

### SOPS Decryption Fails

**Problem**: Cannot decrypt secrets in pipeline

**Solution**: Ensure SOPS_AGE_KEY is correctly configured:
```bash
# Verify age key format
echo "$SOPS_AGE_KEY" | grep "AGE-SECRET-KEY"

# Test decryption locally
export SOPS_AGE_KEY_FILE=/tmp/age-key.txt
sops -d clusters/secrets.yaml
```

## Next Steps

- [GitOps Workflows](./gitops-workflows.md) - Learn GitOps best practices
- [Automation Guide](./automation.md) - Advanced automation patterns
- [Audit and Compliance](./audit-compliance.md) - Implement compliance workflows
- [Disaster Recovery](./disaster-recovery.md) - Set up backup and restore
