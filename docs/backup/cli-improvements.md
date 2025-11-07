1. CLI Configuration (CLIConfig)
Purpose: Controls CLI behavior, logging, default paths, and user preferences
Location: ~/.config/openCenter/config.yaml
Environment Override: OPENCENTER_CONFIG_DIR can change the base directory

Configuration Sections:
Logging Configuration:

level: Log verbosity (debug, info, warn, error) - default: "warn"
format: Output format (text, json, yaml) - default: "text"
output: Destination (stdout, stderr, or file path) - default: "stderr"
file: Rotation settings for file logging (maxSize, maxBackups, maxAge, compress)
Paths Configuration:
  - configDir: Base directory for configurations - default: ~/.config/openCenter
  - clustersDir: Generated cluster manifests - default: ~/.config/openCenter/clusters
    - clustersDir: /etc/openCenter


1. cluster init
Creation Path:
  - if metadata.organization is suite:
      - cluster path: clustersDir/<organization>/<cluster name>
         - /etc/openCenter/rackspace/cluster01
      - sops key destination: clustersDir/<organization>/secrets/age/keys/<cluster name>-key.txt
         - /etc/openCenter/rackspace/secrets/age/keys/cluster01-key.txt
         - /etc/openCenter/rackspace/.sops.yaml
  - else:
      - cluster path: clusterDir/<cluster name> 
         - /etc/openCenter/cluster01
      - sops key destination: clusterDir/<cluster name>/secrets/age/keys/<cluster name>-key.txt 

2. cluster select < cluster name>
Prints Metadata information:
    -  Cluster metadata (name, env, region, status, organization
    -  Cluster Path
    -  Sops key path
    - if cluster status is deployed:
      -  export KUBECONFIG=<cluster path>/kubeconfig.yaml
      -  export ANSIBLE_INVENTORY=<cluster path>/inventory/inventory.yaml
      -  source <cluster path>/venv/bin/activate
      -  export BIN=<cluster path>/.bin
      -  export PATH=${BIN}:${PATH}



3. CLI Behavior
autoConfirm: Auto-confirm destructive operations - default: false
dryRun: Enable dry-run by default - default: false
verbose: Enable verbose output - default: false

4. CLI Configuration Commands
The CLI provides a complete config command suite:

# View current configuration
opencenter config view

# Set individual values using dot notation
openCenter config set logging.level debug
openCenter config set defaults.provider kind
openCenter config set behavior.verbose true

# Get specific values
openCenter config get logging.level
openCenter config get defaults.provider

# Reset to defaults
opencenter config reset [--force]

# Show config file path
opencenter config path


5. Configuration Loading & Precedence
CLI Config Precedence (highest to lowest):
  -  Environment variables (e.g., OPENCENTER_CONFIG_DIR)
  -  CLI configuration file (~/.config/openCenter/config.yaml)
  -  Hardcoded defaults

Cluster Config Precedence (for cluster operations):
  --set key=value flag overrides (dot notation)
  Command-line flags (--provider, --cluster, etc.)
  Configuration file (with os.ExpandEnv() variable expansion)
  CLI configuration defaults
  Hardcoded defaults

6. Smart Configuration Management
  Auto-creation: Config file and directories are created automatically when needed
  Merging: Partial configurations are merged with defaults to ensure completeness
  Validation: Configuration values are validated (log levels, formats, boolean values)
  Path expansion: Supports ~ expansion for home directory paths
  Environment expansion: Config files support ${VAR} environment variable expansion
