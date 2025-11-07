# How-To: Generate a SOPS (age) Key

Use the built-in helper to create a SOPS age secret key file for local development and testing.

Prerequisites
- `openCenter` binary built (see README quickstart)

Steps
- Generate the key:
  ```bash
  ./openCenter sops generate-key --key-file ~/.config/sops/age/keys.txt
  ```
- Point your cluster config at the key:
  ```yaml
  secrets:
    sops_age_key_file: ~/.config/sops/age/keys.txt
  ```
- Encrypt a file with SOPS (optional example):
  ```bash
  # Ensure sops is installed; use your package manager
  export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
  sops --encrypt --in-place secrets.yaml
  ```

Notes
- The command generates a proper Age key pair and sets file mode `0600` for the private key.
- The generated key is compatible with standard Age encryption tools.
- If you run `openCenter cluster init` without specifying `secrets.sops_age_key_file`, the CLI will auto-generate a key under `~/.config/openCenter/sops/age/keys/<cluster-name>-key.txt` and record the path in your config.
 - To prevent auto-generation during init, pass `--no-sops-keygen`.
