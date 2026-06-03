---
id: flux-bootstrap-methods
title: "Configure Flux Bootstrap Authentication"
sidebar_label: Configure Flux Bootstrap
description: Set up SSH keys or personal access tokens for FluxCD bootstrap with GitHub, GitLab, or Gitea.
doc_type: how-to
audience: "operators, platform engineers"
tags: [flux, bootstrap, authentication, ssh, token, github, gitlab, gitea]
---
# Configure Flux Bootstrap Authentication

**Purpose:** For operators, shows how to configure SSH key or token-based authentication for FluxCD bootstrap, covering GitHub, GitLab, and Gitea providers.

FluxCD bootstrap requires Git repository access. openCenter supports two authentication methods:

* **SSH Key Authentication:** Recommended for production. Uses SSH key pairs for secure, passwordless access.
* **Token Authentication:** Useful for HTTPS-only environments. Uses personal access tokens (PAT) for authentication.

## Prerequisites

* openCenter CLI installed
* Cluster configuration created (`opencenter cluster init`)
* Git repository created (GitHub, GitLab, or Gitea)
* Admin access to create tokens or deploy keys

## Choose Your Authentication Method

| Method | Use Case | Security | Setup Complexity |
| --- | --- | --- | --- |
| SSH Key | Production, CI/CD | High (key-based) | Medium |
| Token | HTTPS-only, quick setup | Medium (token-based) | Low |

**Recommendation:** Use SSH keys for production clusters. Use tokens for local development or environments where SSH is blocked.

## SSH Key Authentication

### Step 1: Generate SSH Key Pair

openCenter generates SSH keys during cluster initialization:

```bash
opencenter cluster init my-cluster --org my-org
```

Keys are created at:

* Private key: `~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key`
* Public key: `~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key.pub`

To generate keys manually:

```bash
ssh-keygen -t ed25519 -C "flux-my-cluster" -f ~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key -N ""
```

### Step 2: Add Deploy Key to Repository

#### GitHub

1. Navigate to your repository on GitHub
2. Go to **Settings** → **Deploy keys**
3. Click **Add deploy key**
4. Enter a title (e.g., `flux-my-cluster`)
5. Paste the public key content:

   ```bash
   cat ~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key.pub
   ```
6. Check **Allow write access** (required for Flux to push status updates)
7. Click **Add key**

#### GitLab

1. Navigate to your project on GitLab
2. Go to **Settings** → **Repository** → **Deploy keys**
3. Click **Add new key**
4. Enter a title (e.g., `flux-my-cluster`)
5. Paste the public key content
6. Check **Grant write permissions to this key**
7. Click **Add key**

#### Gitea

1. Navigate to your repository on Gitea
2. Go to **Settings** → **Deploy Keys**
3. Click **Add Deploy Key**
4. Enter a title (e.g., `flux-my-cluster`)
5. Paste the public key content
6. Check **Enable Write Access**
7. Click **Add Deploy Key**

### Step 3: Configure openCenter

Update your cluster configuration to use SSH:

```yaml
opencenter:
  gitops:
    repository:
      url: "ssh://git@github.com/my-org/my-cluster-gitops.git"
      branch: "main"
    auth:
      ssh:
        private_key: "~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key"
        public_key: "~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key.pub"
```

To open the guided configuration workflow:

```bash
opencenter cluster configure my-org/my-cluster --guided
```

### Step 4: Bootstrap with SSH

```bash
opencenter cluster deploy my-cluster
```

Flux uses the SSH key to clone and push to the repository.

## Token Authentication

### Step 1: Create Personal Access Token

#### GitHub

1. Go to **GitHub** → **Settings** → **Developer settings** → **Personal access tokens** → **Tokens (classic)**
2. Click **Generate new token** → **Generate new token (classic)**
3. Enter a note (e.g., `flux-my-cluster`)
4. Set expiration (recommend 90 days for rotation)
5. Select scopes:
   * `repo` (Full control of private repositories)
6. Click **Generate token**
7. Copy the token immediately (it won’t be shown again)

**Fine-grained tokens (recommended for GitHub):**

1. Go to **Settings** → **Developer settings** → **Personal access tokens** → **Fine-grained tokens**
2. Click **Generate new token**
3. Enter a token name (e.g., `flux-my-cluster`)
4. Set expiration
5. Select **Repository access** → **Only select repositories** → choose your GitOps repo
6. Under **Permissions** → **Repository permissions**:
   * **Contents**: Read and write
   * **Metadata**: Read-only (automatically selected)
7. Click **Generate token**
8. Copy the token immediately

#### GitLab

1. Go to **GitLab** → **User Settings** → **Access Tokens**
2. Click **Add new token**
3. Enter a name (e.g., `flux-my-cluster`)
4. Set expiration date
5. Select scopes:
   * `api` (for full API access), or
   * `read_repository` + `write_repository` (minimal)
6. Click **Create personal access token**
7. Copy the token immediately

**Project access tokens (recommended for GitLab):**

1. Navigate to your project
2. Go to **Settings** → **Access Tokens**
3. Click **Add new token**
4. Enter a name and expiration
5. Select role: **Maintainer**
6. Select scopes: `read_repository`, `write_repository`
7. Click **Create project access token**
8. Copy the token

#### Gitea

1. Go to **Gitea** → **Settings** → **Applications**
2. Under **Manage Access Tokens**, enter a token name
3. Select scopes:
   * `repo` (repository access)
4. Click **Generate Token**
5. Copy the token immediately

### Step 2: Store Token Securely

Create a token file (never commit this to Git):

```bash
# Create token file
echo "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" > ~/.config/opencenter/clusters/my-org/secrets/git-token.txt

# Secure permissions
chmod 600 ~/.config/opencenter/clusters/my-org/secrets/git-token.txt
```

### Step 3: Configure openCenter

Update your cluster configuration to use token authentication:

```yaml
opencenter:
  gitops:
    repository:
      url: "https://github.com/my-org/my-cluster-gitops.git"
      branch: "main"
    auth:
      token:
        provider: "github"  # or "gitlab", "gitea"
        token: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
        owner: "my-org"     # optional: overrides owner extracted from URL
```

Set either `token` or `token_file: "~/.config/opencenter/clusters/my-org/secrets/git-token.txt"`. The default `cluster init` template uses `token: "CHANGEME"` so generated configs are actionable without guessing the field name; replace it before deployment. The token `owner` field is optional. If not specified, openCenter extracts the owner from the repository URL. Use `owner` when:

* The URL owner differs from the token owner (e.g., organization vs personal account)
* Using a personal token with `--personal` flag for GitHub

To open the guided configuration workflow:

```bash
opencenter cluster configure my-org/my-cluster --guided
```

### Step 4: Bootstrap with Token

```bash
opencenter cluster deploy my-cluster
```

Flux uses the token for HTTPS authentication.

## Verification

After bootstrap, verify authentication is working:

```bash
# Check GitRepository status
kubectl get gitrepositories -n flux-system

# Expected output shows Ready status
NAME                    URL                                              READY   STATUS
flux-system             ssh://git@github.com/my-org/my-cluster-gitops    True    Fetched revision: main@sha1:abc123
```

If authentication fails:

```bash
# Check detailed status
kubectl describe gitrepository flux-system -n flux-system

# Check Flux logs
kubectl logs -n flux-system deployment/source-controller
```

## Rotate Credentials

### Rotate SSH Keys

```bash
# Generate new key pair
opencenter secrets keys rotate --cluster my-cluster --type ssh

# Update deploy key in Git provider (follow Step 2 above)

# Re-bootstrap to update cluster secret
opencenter cluster deploy my-cluster --restart
```

### Rotate Tokens

1. Generate new token in Git provider (follow token creation steps)
2. Update token file:

   ```bash
   echo "new_token_value" > ~/.config/opencenter/clusters/my-org/secrets/git-token.txt
   ```
3. Re-bootstrap:

   ```bash
   opencenter cluster deploy my-cluster --restart
   ```

## Troubleshooting

### SSH: Permission Denied

**Problem:** `Permission denied (publickey)` during bootstrap.

**Causes:**

* Deploy key not added to repository
* Wrong key file path in configuration
* Key doesn’t have write access

**Solution:**

```bash
# Verify key exists
ls -la ~/.config/opencenter/clusters/my-org/secrets/ssh/

# Test SSH connection (GitHub example)
ssh -T -i ~/.config/opencenter/clusters/my-org/secrets/ssh/my-cluster-key git@github.com

# Verify deploy key has write access in repository settings
```

### Token: Authentication Failed

**Problem:** `authentication required` or `401 Unauthorized` during bootstrap.

**Causes:**

* Token expired
* Insufficient token scopes
* Wrong token provider configured

**Solution:**

```bash
# Verify token file exists and has content
cat ~/.config/opencenter/clusters/my-org/secrets/git-token.txt

# Test token (GitHub example)
curl -H "Authorization: token $(cat ~/.config/opencenter/clusters/my-org/secrets/git-token.txt)" \
  https://api.github.com/user

# Regenerate token with correct scopes if needed
```

### GitRepository Stuck in "Fetching"

**Problem:** GitRepository shows `Fetching` status indefinitely.

**Causes:**

* Network connectivity issues
* Firewall blocking Git protocol
* Invalid repository URL

**Solution:**

```bash
# Check source-controller logs
kubectl logs -n flux-system deployment/source-controller | tail -50

# Verify URL is accessible from cluster
kubectl run -it --rm debug --image=alpine --restart=Never -- \
  sh -c "apk add git && git ls-remote https://github.com/my-org/my-cluster-gitops.git"
```

## Security Best Practices

### SSH Keys

* Use Ed25519 keys (more secure than RSA)
* Store private keys with `600` permissions
* Use deploy keys (repository-scoped) instead of user SSH keys
* Rotate keys every 180 days

### Tokens

* Use fine-grained tokens with minimal scopes
* Set short expiration (90 days recommended)
* Use project/repository tokens instead of personal tokens when possible
* Store tokens in files, not environment variables or configuration
* Rotate tokens before expiration

### General

* Never commit credentials to Git
* Use SOPS to encrypt credential paths in configuration
* Audit credential access regularly
* Revoke credentials immediately when compromised

## Next Steps

* [Manage Secrets](manage-secrets.md) - Encrypt secrets with SOPS
* [Troubleshoot Deployment](troubleshoot-deployment.md) - Fix bootstrap issues
* [GitOps Workflow](../concepts/gitops-workflow.md) - Understand reconciliation

---

## Evidence

This how-to guide is based on:

* GitOps configuration: `internal/config/types_gitops.go:17-23` (GitToken, GitTokenProvider, GitOwner fields)
* Bootstrap implementation: `internal/localdev/flux/service.go:51-160` (provider-specific bootstrap commands)
* URL parsing: `internal/localdev/flux/service.go:162-230` (parseGitHubURL, parseGitLabURL, parseGitURL)
* SSH key generation: `internal/sops/manager.go`
* Security model: `docs/concepts/security-model.md:104-108`
* FluxCD documentation: https://fluxcd.io/docs/installation/bootstrap/