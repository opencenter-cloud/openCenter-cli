# How-To: Use a Local Gitea for Bootstrap Testing

This guide shows how to stand up a disposable local Gitea, create a test user with API tokens, upload an SSH key, and push a GitOps repo via `openCenter cluster bootstrap`.

Prerequisites
- Docker or Podman (defaults to Docker; set `CONTAINER_RUNTIME=podman` to use Podman)
- `mise` installed

Start Gitea and configure user
- Start and configure in one step:
  ```bash
  mise run gitea-up
  ```
  This runs, in order:
  - `gitea-setup`: launches the Gitea container with HTTPS on `https://localhost:3001` and SSH on `localhost:2222`.
  - `gitea-configure`: creates an admin token, creates `newuser`, generates an SSH keypair, uploads the public key, and promotes `newuser` to admin (global read/write).

Artifacts created
- Admin token: `.gitea_admin_token`
- User token: `.gitea_newuser_token`
- SSH keypair for `newuser`:
  - Private: `.gitea_newuser_id_ed25519`
  - Public: `.gitea_newuser_id_ed25519.pub` (uploaded to the user account)

Configure SSH for pushes (recommended)
- Option A: One-off command environment
  ```bash
  export GIT_SSH_COMMAND="ssh -i ./.gitea_newuser_id_ed25519 -o StrictHostKeyChecking=no -p 2222"
  ```
- Option B: Add to `~/.ssh/config`
  ```
  Host gitea-local
    HostName localhost
    Port 2222
    User git
    IdentityFile /full/path/to/.gitea_newuser_id_ed25519
    StrictHostKeyChecking no
  ```

Set git_url for openCenter
- Use SSH with explicit port (since Gitea listens on host port 2222):
  ```
  ssh://git@localhost:2222/newuser/test-repo.git
  ```
  or, if you added a host entry in SSH config as above:
  ```
  ssh://git@gitea-local/newuser/test-repo.git
  ```

Example workflow
1) Initialize and select a cluster (if not already):
```bash
./bin/openCenter cluster init dev --force
./bin/openCenter cluster select dev
```

2) Set the GitOps directory and remote URL in your config (edit `~/.config/openCenter/dev.yaml` or `$OPENCENTER_CONFIG_DIR/dev.yaml`):
```yaml
cluster_name: dev
gitops:
  git_dir: ./gitops-dev
  git_url: ssh://git@localhost:2222/newuser/test-repo.git
```

3) Render/setup and bootstrap:
```bash
./bin/openCenter cluster setup --render
# Ensure SSH env or config is in place (see above)
./bin/openCenter cluster bootstrap
```

Tear down
- Stop and remove the container and local data (DANGEROUS: deletes local Gitea data):
  ```bash
  mise run gitea-cleanup
  ```
  This will also remove the generated tokens and SSH keypair from the working directory.

Notes
- Self-signed HTTPS is enabled; the script uses `-k` where necessary.
- The helper promotes `newuser` to admin to guarantee read/write across repositories for testing only. Do not use this pattern in production.
