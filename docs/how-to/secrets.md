# Secrets Management with Barbican

`openCenter secrets` introduces a Barbican-backed control plane for handling the credentials, bootstrap bundles, and opaque data that the rest of the CLI requires. Instead of committing sensitive values into Git or passing them through ad-hoc scripts, the command talks directly to [OpenStack Barbican](https://docs.openstack.org/barbican/latest/) and stores every secret with cluster-aware metadata so it can be fetched on demand by GitOps automation, CI pipelines, or other teams.

## Barbican vs. SOPS

The `openCenter` CLI supports two methods for secret management, each with a distinct purpose:

1.  **SOPS (GitOps/Static Encryption):**
    *   **Usage:** Used for encrypting configuration files that must be stored in Git (e.g., Kubernetes manifests, Helm values).
    *   **Mechanism:** Encrypts values in-place using Age keys or KMS, allowing safe commitment to version control.
    *   **Command:** `openCenter sops`
    *   **Use Case:** GitOps workflows where configuration is driven by the repository state.

2.  **Barbican (Runtime/Cloud Secrets):**
    *   **Usage:** Used for storing high-sensitivity credentials (e.g., cloud API keys, bootstrap tokens, PKI assets) that should **never** be committed to Git, even in encrypted form.
    *   **Mechanism:** Stores secrets in a centralized OpenStack Barbican service, secured by Keystone authentication and RBAC.
    *   **Command:** `openCenter secrets`
    *   **Use Case:** Runtime secret retrieval, bootstrapping clusters, and handling credentials that exist outside the GitOps lifecycle.

## Why Barbican

- **Centralized** – Barbican provides a single API for all OpenStack projects, so OpenCenter can manage secrets without depending on a particular cloud provider or secret store implementation.
- **Auditable** – Access to `openCenter secrets` is gated by OpenStack Keystone, giving you fine-grained RBAC, logging, and rotation policies.
- **Versionable** – The CLI writes tags (`organization`, `cluster`, `scope`, `version`) that make it trivial to identify which revision of a secret is attached to a GitOps run.
- **Operational parity** – SOPS still encrypts files that must live in Git, while Barbican holds the high-sensitivity values (cloud API keys, bootstrap tokens, PKI assets) that should never be committed.

## Barbican-backed Workflow

1. Authenticate against Keystone once with `openCenter secrets login`.
2. Point the CLI at the desired organization/cluster (`--opencenter.meta.organization`, `--cluster`) or rely on the active cluster selection.
3. Use `openCenter secrets put` to upload opaque blobs or structured YAML/JSON documents. The CLI wraps them inside a Barbican secret (type `opaque`) and adds deterministic `metadata`.
4. Use `openCenter secrets get` to hydrate manifests before a deployment, or `openCenter secrets sync` to mirror the values onto disk for GitOps tooling that expects local files.
5. Track what is stored remotely with `openCenter secrets list` or `openCenter secrets describe`.

All operations are idempotent: pushing the same payload again safely updates the previous secret while preserving history in Barbican.

## Configuration and Authentication

`openCenter secrets` reads credentials from the active cluster configuration (`~/.config/openCenter/clusters/<org>/<cluster>/cluster.yaml`) under the `opencenter.secrets` block. A minimal configuration looks like this:

```yaml
opencenter:
  secrets:
    backend: barbican
    barbican:
      auth_url: https://identity.example.com/v3
      project_id: 5b3ff03f24bf4bfebd3b1cda2a0e3f74
      region: regionOne
      user_domain_name: Default
      project_domain_name: Default
      ca_cert: /etc/ssl/certs/ca-bundle.crt
```

Override any value at runtime with dotted flags, e.g.:

```bash
openCenter secrets list \
  --opencenter.secrets.barbican.auth-url=https://identity.example.com/v3 \
  --opencenter.secrets.barbican.region=phx
```

The command also understands the standard OpenStack environment variables (`OS_AUTH_URL`, `OS_USERNAME`, `OS_PASSWORD`, `OS_PROJECT_ID`, etc.) so you can reuse existing automation. Authentication artifacts are cached in `~/.config/openCenter/barbican/` and renewed automatically when they are close to expiring.

## Command Layout

General syntax:

```bash
openCenter secrets <subcommand> [flags]
```

Common flags:

- `--cluster string` – Cluster name (default: active cluster).
- `--organization string` – Organization override (default: active organization).
- `--format string` – Output format: `table` (default), `json`, or `yaml`.
- `--label stringArray` – Additional Barbican labels in `key=value` form.
- `--opencenter.secrets.*` – Overrides for any Barbican auth setting.

### Subcommands

#### `openCenter secrets login`

Create or refresh a Keystone token. By default the command prompts for an application credential or password; pass secrets on stdin for non-interactive environments.

```bash
openCenter secrets login \
  --opencenter.secrets.barbican.auth-url=https://identity.example.com/v3 \
  --username svc-opencenter \
  --project-id 5b3ff03f24bf4bfebd3b1cda2a0e3f74 \
  --password-stdin
```

Store an application credential ID/secret by exporting `OS_APPLICATION_CREDENTIAL_ID` and `OS_APPLICATION_CREDENTIAL_SECRET`; the CLI prioritizes those values when present.

#### `openCenter secrets list`

List secrets associated with the current cluster. Results include metadata tags, version, size, and the last rotation timestamp.

```bash
openCenter secrets list --format table
openCenter secrets list --label scope=bootstrap --format json
```

#### `openCenter secrets describe <name>`

Show metadata, full tag set, and audit information for a single secret without returning the payload. Useful for verifying replication, expiration, or custom labels.

```bash
openCenter secrets describe cluster-admin-kubeconfig
```

#### `openCenter secrets get <name>`

Download and decrypt a secret. Pipe the value to another command, save it to disk, or decode structured payloads automatically.

```bash
# Save bootstrap token locally
openCenter secrets get bootstrap-token \
  --output-file ./clusters/prod/secrets/bootstrap-token.txt

# Inject directly into a manifest template
openCenter secrets get gitops-private-key --format raw | kubectl create secret generic gitops-key --from-file=ssh-privatekey=/dev/stdin
```

#### `openCenter secrets put <name>`

Create or update a Barbican secret. Pass `--from-file`, `--value`, or `--json` depending on the data source. The CLI automatically labels the secret with organization/cluster/scope and keeps the payload encrypted in transit.

```bash
# Upload from a local file
openCenter secrets put gitops-private-key \
  --from-file=./clusters/prod/secrets/gitops-id_ed25519 \
  --label scope=gitops --label managed-by=opencenter

# Upload inline value
openCenter secrets put api-token --value "${CI_JOB_TOKEN}"
```

#### `openCenter secrets delete <name>`

Delete a secret after confirming it is no longer referenced by any cluster. By default the command refuses to delete secrets that are still tagged with `scope=bootstrap`; use `--force` to override.

```bash
openCenter secrets delete ephemeral-ci-token --force
```

#### `openCenter secrets sync`

Materialize a filtered subset of Barbican secrets onto disk so GitOps tooling can run offline. By default it only writes to `./clusters/<cluster>/secrets/remote/` and never overwrites existing files without `--force`.

```bash
openCenter secrets sync \
  --directory ./clusters/prod/secrets/remote \
  --label scope=bootstrap \
  --format yaml
```

## Example End-to-End Flow

```bash
# 1. Authenticate with application credentials
export OS_AUTH_URL=https://identity.example.com/v3
export OS_APPLICATION_CREDENTIAL_ID=3d9420af54544d95a8de0b1f9ec332b5
export OS_APPLICATION_CREDENTIAL_SECRET="$(op read op://platform/opencenter/app-cred)"
openCenter secrets login

# 2. Upload a new PKI bundle for the production cluster
openCenter secrets put prod-api-pki \
  --cluster prod-cluster \
  --label scope=bootstrap \
  --from-file=./clusters/prod/secrets/pki.tar.gz

# 3. Confirm it exists
openCenter secrets list --cluster prod-cluster

# 4. Pull it back down during bootstrap
openCenter secrets get prod-api-pki \
  --cluster prod-cluster \
  --output-file=/tmp/prod-api-pki.tar.gz
```

## Troubleshooting Tips

- Run with `--verbose` to surface Barbican API calls and Keystone token refreshes.
- Set `OPENCENTER_SECRETS_CACHE_TTL` (seconds) if you want to shorten the lifetime of cached tokens; the CLI purges tokens automatically after the TTL.
- Use `openstack secret list --name <name>` to double-check that the secret exists and carries the labels you expect.
- If authentication fails, validate that `--opencenter.secrets.barbican.ca-cert` points to a certificate bundle that trusts your Keystone endpoint, or export `REQUESTS_CA_BUNDLE`.
- To keep GitOps repositories self contained, pair Barbican secrets with the existing `openCenter sops` commands—SOPS encrypts templates in Git, while `openCenter secrets` stores the runtime values those templates reference.

The `openCenter secrets` command cements Barbican as the authoritative source of truth for secrets while keeping the rest of the CLI unchanged, giving teams a secure workflow without sacrificing the familiar GitOps experience.
