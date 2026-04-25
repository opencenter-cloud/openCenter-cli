---
id: integrate-ci-cd
title: "Integrate CI/CD"
sidebar_label: CI/CD Integration
description: How to integrate openCenter into CI/CD pipelines for automated cluster deployment and testing.
doc_type: how-to
audience: "devops engineers, developers"
tags: [ci-cd, github-actions, gitlab-ci, jenkins, automation]
---

# Integrate CI/CD

**Purpose:** For DevOps engineers, shows how to integrate openCenter into CI/CD pipelines for automated cluster deployment and testing.

This guide covers integrating openCenter with popular CI/CD platforms (GitHub Actions, GitLab CI, Jenkins) for automated cluster lifecycle management.

## Prerequisites

- CI/CD platform access (GitHub Actions, GitLab CI, or Jenkins)
- openCenter CLI installed on CI/CD runners
- Infrastructure provider credentials
- Git repository for cluster configuration

## Task Summary

Automate cluster deployment, validation, and testing using openCenter CLI in CI/CD pipelines, enabling infrastructure-as-code workflows with automated testing and deployment.

## Integration Patterns

### Pattern 1: Cluster Validation on PR

**Use case:** Validate cluster configuration changes before merge

**Workflow:**
1. Developer opens PR with configuration changes
2. CI runs `opencenter cluster validate`
3. CI reports validation results
4. PR can only merge if validation passes

### Pattern 2: Automated Cluster Deployment

**Use case:** Deploy cluster automatically on configuration changes

**Workflow:**
1. Configuration merged to main branch
2. CI runs `opencenter cluster generate`
3. CI commits generated files
4. CI runs `opencenter cluster deploy`
5. Cluster deployed automatically

### Pattern 3: Ephemeral Test Clusters

**Use case:** Create temporary clusters for testing

**Workflow:**
1. Test suite triggered
2. CI creates ephemeral cluster
3. CI runs tests against cluster
4. CI destroys cluster
5. Test results reported

### Pattern 4: Multi-Environment Promotion

**Use case:** Promote changes from dev → staging → production

**Workflow:**
1. Changes deployed to dev automatically
2. Tests run on dev cluster
3. If tests pass, promote to staging
4. Tests run on staging cluster
5. If tests pass, manual approval for production
6. Deploy to production

## GitHub Actions Integration

### Setup

Create `.github/workflows/opencenter.yaml`:

```yaml
name: openCenter CI/CD

on:
  pull_request:
    paths:
      - 'clusters/**'
  push:
    branches:
      - main
    paths:
      - 'clusters/**'

env:
  OPENCENTER_VERSION: "v1.0.0"

jobs:
  validate:
    name: Validate Configuration
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/${OPENCENTER_VERSION}/opencenter-linux-amd64 -o /usr/local/bin/opencenter
          chmod +x /usr/local/bin/opencenter
          opencenter version
      
      - name: Validate cluster configuration
        run: |
          opencenter cluster validate dev-cluster
        env:
          OPENCENTER_CONFIG_DIR: ${{ github.workspace }}/clusters
      
      - name: Comment validation results
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '✅ Cluster configuration validation passed!'
            })

  deploy-dev:
    name: Deploy to Dev
    runs-on: ubuntu-latest
    needs: validate
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/${OPENCENTER_VERSION}/opencenter-linux-amd64 -o /usr/local/bin/opencenter
          chmod +x /usr/local/bin/opencenter
      
      - name: Setup infrastructure credentials
        run: |
          echo "${{ secrets.OPENSTACK_CLOUDS_YAML }}" > ~/.config/openstack/clouds.yaml
      
      - name: Deploy cluster
        run: |
          opencenter cluster generate dev-cluster
          opencenter cluster deploy dev-cluster
        env:
          OPENCENTER_CONFIG_DIR: ${{ github.workspace }}/clusters
          OS_CLOUD: openstack
      
      - name: Run smoke tests
        run: |
          export KUBECONFIG=~/dev-cluster-gitops/infrastructure/clusters/dev-cluster/kubeconfig.yaml
          kubectl get nodes
          kubectl get pods -A
          ./tests/smoke-tests.sh

  deploy-staging:
    name: Deploy to Staging
    runs-on: ubuntu-latest
    needs: deploy-dev
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/${OPENCENTER_VERSION}/opencenter-linux-amd64 -o /usr/local/bin/opencenter
          chmod +x /usr/local/bin/opencenter
      
      - name: Deploy to staging
        run: |
          opencenter cluster generate staging-cluster
          opencenter cluster deploy staging-cluster
        env:
          OPENCENTER_CONFIG_DIR: ${{ github.workspace }}/clusters
      
      - name: Run integration tests
        run: |
          export KUBECONFIG=~/staging-cluster-gitops/infrastructure/clusters/staging-cluster/kubeconfig.yaml
          ./tests/integration-tests.sh

  deploy-production:
    name: Deploy to Production
    runs-on: ubuntu-latest
    needs: deploy-staging
    if: github.ref == 'refs/heads/main'
    environment:
      name: production
      url: https://my-app.example.com
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/${OPENCENTER_VERSION}/opencenter-linux-amd64 -o /usr/local/bin/opencenter
          chmod +x /usr/local/bin/opencenter
      
      - name: Deploy to production
        run: |
          opencenter cluster generate prod-cluster
          opencenter cluster deploy prod-cluster
        env:
          OPENCENTER_CONFIG_DIR: ${{ github.workspace }}/clusters
      
      - name: Verify production deployment
        run: |
          export KUBECONFIG=~/prod-cluster-gitops/infrastructure/clusters/prod-cluster/kubeconfig.yaml
          kubectl get nodes
          kubectl get pods -A
          ./tests/production-checks.sh
```

### Secrets Configuration

Configure GitHub secrets:

```bash
# Navigate to repository settings
# Settings → Secrets and variables → Actions → New repository secret

# Add secrets:
OPENSTACK_CLOUDS_YAML: |
  clouds:
    openstack:
      auth:
        auth_url: https://identity.api.rackspacecloud.com/v3
        username: your-username
        password: your-password
        project_name: your-project
        user_domain_name: rackspace_cloud_domain
        project_domain_name: rackspace_cloud_domain
      region_name: sjc3

SOPS_AGE_KEY: age1... (SOPS Age private key)
```

## GitLab CI Integration

### Setup

Create `.gitlab-ci.yml`:

```yaml
stages:
  - validate
  - deploy-dev
  - test-dev
  - deploy-staging
  - test-staging
  - deploy-production

variables:
  OPENCENTER_VERSION: "v1.0.0"

.install_opencenter: &install_opencenter
  - curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/${OPENCENTER_VERSION}/opencenter-linux-amd64 -o /usr/local/bin/opencenter
  - chmod +x /usr/local/bin/opencenter
  - opencenter version

validate:
  stage: validate
  image: ubuntu:24.04
  before_script:
    - *install_opencenter
  script:
    - opencenter cluster validate dev-cluster
    - opencenter cluster validate staging-cluster
    - opencenter cluster validate prod-cluster
  only:
    changes:
      - clusters/**

deploy-dev:
  stage: deploy-dev
  image: ubuntu:24.04
  before_script:
    - *install_opencenter
    - echo "$OPENSTACK_CLOUDS_YAML" > ~/.config/openstack/clouds.yaml
  script:
    - opencenter cluster generate dev-cluster
    - opencenter cluster deploy dev-cluster
  environment:
    name: development
  only:
    - main
  except:
    - tags

test-dev:
  stage: test-dev
  image: ubuntu:24.04
  script:
    - export KUBECONFIG=~/dev-cluster-gitops/infrastructure/clusters/dev-cluster/kubeconfig.yaml
    - kubectl get nodes
    - kubectl get pods -A
    - ./tests/smoke-tests.sh
  dependencies:
    - deploy-dev
  only:
    - main

deploy-staging:
  stage: deploy-staging
  image: ubuntu:24.04
  before_script:
    - *install_opencenter
    - echo "$OPENSTACK_CLOUDS_YAML" > ~/.config/openstack/clouds.yaml
  script:
    - opencenter cluster generate staging-cluster
    - opencenter cluster deploy staging-cluster
  environment:
    name: staging
  only:
    - main
  when: on_success

test-staging:
  stage: test-staging
  image: ubuntu:24.04
  script:
    - export KUBECONFIG=~/staging-cluster-gitops/infrastructure/clusters/staging-cluster/kubeconfig.yaml
    - ./tests/integration-tests.sh
  dependencies:
    - deploy-staging
  only:
    - main

deploy-production:
  stage: deploy-production
  image: ubuntu:24.04
  before_script:
    - *install_opencenter
    - echo "$OPENSTACK_CLOUDS_YAML" > ~/.config/openstack/clouds.yaml
  script:
    - opencenter cluster generate prod-cluster
    - opencenter cluster deploy prod-cluster
  environment:
    name: production
    url: https://my-app.example.com
  only:
    - main
  when: manual  # Require manual approval for production
```

### Variables Configuration

Configure GitLab CI/CD variables:

```bash
# Navigate to project settings
# Settings → CI/CD → Variables → Add variable

# Add variables:
OPENSTACK_CLOUDS_YAML: (OpenStack credentials YAML)
SOPS_AGE_KEY: (SOPS Age private key)

# Mark as protected and masked
```

## Jenkins Integration

### Setup

Create `Jenkinsfile`:

```groovy
pipeline {
    agent any
    
    environment {
        OPENCENTER_VERSION = 'v1.0.0'
        OPENCENTER_BIN = '/usr/local/bin/opencenter'
    }
    
    stages {
        stage('Install openCenter') {
            steps {
                sh '''
                    curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/${OPENCENTER_VERSION}/opencenter-linux-amd64 -o ${OPENCENTER_BIN}
                    chmod +x ${OPENCENTER_BIN}
                    ${OPENCENTER_BIN} version
                '''
            }
        }
        
        stage('Validate Configuration') {
            steps {
                sh '''
                    ${OPENCENTER_BIN} cluster validate dev-cluster
                    ${OPENCENTER_BIN} cluster validate staging-cluster
                    ${OPENCENTER_BIN} cluster validate prod-cluster
                '''
            }
        }
        
        stage('Deploy to Dev') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([file(credentialsId: 'openstack-clouds-yaml', variable: 'CLOUDS_YAML')]) {
                    sh '''
                        mkdir -p ~/.config/openstack
                        cp $CLOUDS_YAML ~/.config/openstack/clouds.yaml
                        ${OPENCENTER_BIN} cluster generate dev-cluster
                        ${OPENCENTER_BIN} cluster deploy dev-cluster
                    '''
                }
            }
        }
        
        stage('Test Dev') {
            when {
                branch 'main'
            }
            steps {
                sh '''
                    export KUBECONFIG=~/dev-cluster-gitops/infrastructure/clusters/dev-cluster/kubeconfig.yaml
                    kubectl get nodes
                    kubectl get pods -A
                    ./tests/smoke-tests.sh
                '''
            }
        }
        
        stage('Deploy to Staging') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([file(credentialsId: 'openstack-clouds-yaml', variable: 'CLOUDS_YAML')]) {
                    sh '''
                        cp $CLOUDS_YAML ~/.config/openstack/clouds.yaml
                        ${OPENCENTER_BIN} cluster generate staging-cluster
                        ${OPENCENTER_BIN} cluster deploy staging-cluster
                    '''
                }
            }
        }
        
        stage('Test Staging') {
            when {
                branch 'main'
            }
            steps {
                sh '''
                    export KUBECONFIG=~/staging-cluster-gitops/infrastructure/clusters/staging-cluster/kubeconfig.yaml
                    ./tests/integration-tests.sh
                '''
            }
        }
        
        stage('Deploy to Production') {
            when {
                branch 'main'
            }
            input {
                message "Deploy to production?"
                ok "Deploy"
            }
            steps {
                withCredentials([file(credentialsId: 'openstack-clouds-yaml', variable: 'CLOUDS_YAML')]) {
                    sh '''
                        cp $CLOUDS_YAML ~/.config/openstack/clouds.yaml
                        ${OPENCENTER_BIN} cluster generate prod-cluster
                        ${OPENCENTER_BIN} cluster deploy prod-cluster
                    '''
                }
            }
        }
        
        stage('Verify Production') {
            when {
                branch 'main'
            }
            steps {
                sh '''
                    export KUBECONFIG=~/prod-cluster-gitops/infrastructure/clusters/prod-cluster/kubeconfig.yaml
                    kubectl get nodes
                    kubectl get pods -A
                    ./tests/production-checks.sh
                '''
            }
        }
    }
    
    post {
        success {
            echo 'Pipeline succeeded!'
        }
        failure {
            echo 'Pipeline failed!'
        }
    }
}
```

### Credentials Configuration

Configure Jenkins credentials:

```bash
# Navigate to Jenkins
# Manage Jenkins → Credentials → Add Credentials

# Add credentials:
# Type: Secret file
# ID: openstack-clouds-yaml
# File: clouds.yaml (OpenStack credentials)

# Type: Secret text
# ID: sops-age-key
# Secret: age1... (SOPS Age private key)
```

## Ephemeral Test Clusters

### GitHub Actions Example

```yaml
name: Ephemeral Test Cluster

on:
  pull_request:
    paths:
      - 'src/**'
      - 'tests/**'

jobs:
  test:
    name: Run Tests on Ephemeral Cluster
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/v1.0.0/opencenter-linux-amd64 -o /usr/local/bin/opencenter
          chmod +x /usr/local/bin/opencenter
      
      - name: Create ephemeral cluster
        run: |
          # Create unique cluster name
          CLUSTER_NAME="test-${{ github.run_id }}"
          
          # Initialize cluster
          opencenter cluster init $CLUSTER_NAME \
            --org ci-testing \
            --type kind
          
          # Deploy cluster
          opencenter cluster generate $CLUSTER_NAME
          opencenter cluster deploy $CLUSTER_NAME
        env:
          OPENCENTER_CONFIG_DIR: /tmp/opencenter
      
      - name: Run tests
        run: |
          export KUBECONFIG=/tmp/opencenter/clusters/ci-testing/$CLUSTER_NAME/kubeconfig.yaml
          
          # Deploy application
          kubectl apply -f k8s/
          
          # Wait for pods
          kubectl wait --for=condition=ready pod -l app=my-app --timeout=300s
          
          # Run tests
          ./tests/integration-tests.sh
      
      - name: Destroy ephemeral cluster
        if: always()
        run: |
          CLUSTER_NAME="test-${{ github.run_id }}"
          opencenter cluster destroy $CLUSTER_NAME
        env:
          OPENCENTER_CONFIG_DIR: /tmp/opencenter
```

## Verification

Verify CI/CD integration:

```bash
# 1. Trigger pipeline
git commit -m "Test CI/CD integration"
git push

# 2. Check pipeline status
# GitHub Actions: Actions tab
# GitLab CI: CI/CD → Pipelines
# Jenkins: Build history

# 3. Verify cluster deployment
opencenter cluster list

# 4. Verify cluster health
export KUBECONFIG=~/dev-cluster-gitops/infrastructure/clusters/dev-cluster/kubeconfig.yaml
kubectl get nodes
kubectl get pods -A

# 5. Check test results
# Review test logs in CI/CD platform
```

## Troubleshooting

### Pipeline Fails: openCenter CLI Not Found

**Symptom:** `opencenter: command not found`

**Solution:**

```yaml
# Ensure openCenter CLI is installed in pipeline
- name: Install openCenter CLI
  run: |
    curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/v1.0.0/opencenter-linux-amd64 -o /usr/local/bin/opencenter
    chmod +x /usr/local/bin/opencenter
    opencenter version
```

### Pipeline Fails: Authentication Error

**Symptom:** `Error: OpenStack authentication failed`

**Solution:**

```yaml
# Verify credentials are configured
- name: Setup credentials
  run: |
    echo "${{ secrets.OPENSTACK_CLOUDS_YAML }}" > ~/.config/openstack/clouds.yaml
  env:
    OPENSTACK_CLOUDS_YAML: ${{ secrets.OPENSTACK_CLOUDS_YAML }}
```

### Pipeline Timeout

**Symptom:** Pipeline times out during cluster deployment

**Solution:**

```yaml
# Increase timeout
jobs:
  deploy:
    timeout-minutes: 60  # Default is 360 (6 hours)
```

### Cluster Already Exists

**Symptom:** `Error: Cluster already exists`

**Solution:**

```bash
# Use unique cluster names for ephemeral clusters
CLUSTER_NAME="test-${CI_PIPELINE_ID}"  # GitLab
CLUSTER_NAME="test-${{ github.run_id }}"  # GitHub Actions
CLUSTER_NAME="test-${BUILD_NUMBER}"  # Jenkins

# Or destroy existing cluster first
opencenter cluster destroy $CLUSTER_NAME || true
```

## Best Practices

1. **Use ephemeral clusters for testing:** Create and destroy clusters per test run
2. **Validate before deploy:** Always validate configuration in pipeline
3. **Test in dev/staging first:** Never deploy directly to production
4. **Use manual approval for production:** Require human approval for production deployments
5. **Store credentials securely:** Use CI/CD platform's secret management
6. **Monitor pipeline duration:** Optimize for faster feedback
7. **Cache dependencies:** Cache openCenter CLI and other tools
8. **Fail fast:** Stop pipeline on first failure
9. **Notify on failures:** Send alerts for pipeline failures
10. **Document pipeline:** Add comments explaining pipeline steps

## Related Topics

- [Validate Configuration](validate-configuration.md) - Configuration validation
- [Multi-Cluster Management](../tutorials/multi-cluster-setup.md) - Manage multiple clusters
- [Configuration Lifecycle](../explanation/configuration-lifecycle.md) - Configuration management
- [CLI Commands](../reference/cli-commands.md) - Complete CLI reference

---

## Evidence

This guide is based on:

- CLI automation: `.kiro/steering/product.md:22` scriptable
- CI/CD patterns: Industry best practices
- openCenter CLI: `cmd/` directory structure
- Configuration management: `internal/config/`
