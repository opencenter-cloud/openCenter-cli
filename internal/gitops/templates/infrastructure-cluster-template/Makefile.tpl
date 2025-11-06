.PHONY: clean lint rke terraform kubectl helm velero sops age kustomize flux gitops kubelogin egctl
.PHONY: secrets-encrypt secrets-decrypt secrets-list secrets-check secrets-status

BIN := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))/.bin
TERRAFORM_VERSION := 1.12.2
KUBECTL_VERSION := 1.28.0
HELM_VERSION := 3.13.0
VELERO_VERSION := 1.12.1
SOPS_VERSION := 3.8.1
KUSTOMIZE_VERSION := 5.2.1
FLUX_VERSION := 2.2.2
GITOPS_VERSION := 0.38.0
EGCTL_VERSION := 1.5.4
HELM_VERSION := 3.13.0

export PATH := $(BIN):$(PATH)
export TF_CLI_CONFIG_FILE=config.tfrc

export ANSIBLE_INVENTORY = {{- if .OpenCenter.GitOps.GitDir }}{{ .OpenCenter.GitOps.GitDir }}/inventory/inventory.yaml{{- else }}/tmp/inventory/inventory.yaml{{- end }}

# SOPS Configuration
SOPS_SCRIPT := ./sops_manager.sh
SOPS_CONFIG := .sops.yaml
SOPS_AGE_KEY_FILE ?= $(HOME)/.age/key.txt
export SOPS_AGE_KEY_FILE

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S),Linux)
	OS = linux
	GITOPS_OS = Linux
	ifeq ($(UNAME_M),x86_64)
		ARCH = amd64
	endif
	ifeq ($(UNAME_M),aarch64)
		ARCH = arm64
	endif
endif
ifeq ($(UNAME_S),Darwin)
	OS = darwin
	GITOPS_OS = Darwin
	ifeq ($(UNAME_M),x86_64)
		ARCH = amd64
	endif
	ifeq ($(UNAME_M),arm64)
		ARCH = arm64
	endif
endif

clean:
	rm cluster.rkestate kube_config_cluster.yml terraform.tfstate*

rke:

terraform:
	@if ! terraform --version | head -n 1 | grep $(TERRAFORM_VERSION); then \
		mkdir -p $(BIN); \
		curl -L https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_$(OS)_$(ARCH).zip > $(BIN)/terraform.zip; \
		unzip $(BIN)/terraform.zip -d $(BIN); \
		rm $(BIN)/terraform.zip; \
	fi;

kubectl:
	@if ! kubectl version --client --output=yaml 2>/dev/null | grep -q "gitVersion: v$(KUBECTL_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://dl.k8s.io/release/v$(KUBECTL_VERSION)/bin/$(OS)/$(ARCH)/kubectl" -o $(BIN)/kubectl; \
		chmod +x $(BIN)/kubectl; \
	fi;

helm:
	@if ! helm version --template="{{.Version}}" 2>/dev/null | grep -q "v$(HELM_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://get.helm.sh/helm-v$(HELM_VERSION)-$(OS)-$(ARCH).tar.gz" | tar xz -C $(BIN) --strip-components=1 $(OS)-$(ARCH)/helm; \
	fi;

velero:
	@if ! velero version --client-only 2>/dev/null | grep -q "v$(VELERO_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/vmware-tanzu/velero/releases/download/v$(VELERO_VERSION)/velero-v$(VELERO_VERSION)-$(OS)-$(ARCH).tar.gz" | tar xz -C $(BIN) --strip-components=1 velero-v$(VELERO_VERSION)-$(OS)-$(ARCH)/velero; \
	fi;

sops:
	@if ! sops --version 2>/dev/null | grep -q "$(SOPS_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/getsops/sops/releases/download/v$(SOPS_VERSION)/sops-v$(SOPS_VERSION).$(OS).$(ARCH)" -o $(BIN)/sops; \
		chmod +x $(BIN)/sops; \
	fi;

age:
	@if ! age --version 2>/dev/null | grep -q "v"; then \
		mkdir -p $(BIN); \
		curl -L "https://dl.filippo.io/age/latest?for=$(OS)/$(ARCH)" | tar xz -C $(BIN) --strip-components=1 age/age age/age-keygen; \
	fi;

kustomize:
	@if ! kustomize version 2>/dev/null | grep -q "v$(KUSTOMIZE_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_$(OS)_$(ARCH).tar.gz" | tar xz -C $(BIN) kustomize; \
	fi;

flux:
	@if ! flux --version 2>/dev/null | grep -q "$(FLUX_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/fluxcd/flux2/releases/download/v$(FLUX_VERSION)/flux_$(FLUX_VERSION)_$(OS)_$(ARCH).tar.gz" | tar xz -C $(BIN) flux; \
	fi;

gitops:
	@if ! gitops version 2>/dev/null | grep -q "$(GITOPS_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/weaveworks/weave-gitops/releases/download/v$(GITOPS_VERSION)/gitops-$(GITOPS_OS)-$(UNAME_M).tar.gz" | tar xz -C $(BIN) gitops; \
	fi;

kubelogin:
	@if ! $(BIN)/kubectl-oidc_login --version 2>/dev/null; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/int128/kubelogin/releases/latest/download/kubelogin_$(OS)_$(ARCH).zip" -o $(BIN)/kubelogin.zip; \
		unzip -o $(BIN)/kubelogin.zip -d $(BIN); \
		mv $(BIN)/kubelogin $(BIN)/kubectl-oidc_login; \
		chmod +x $(BIN)/kubectl-oidc_login; \
		rm $(BIN)/kubelogin.zip; \
	fi;

egctl:
	@if ! egctl version 2>/dev/null | grep -q "v$(EGCTL_VERSION)"; then \
		mkdir -p $(BIN); \
		curl -L "https://github.com/envoyproxy/gateway/releases/download/v$(EGCTL_VERSION)/egctl_v$(EGCTL_VERSION)_$(OS)_$(ARCH).tar.gz" | tar xz -C $(BIN) bin/$(OS)/$(ARCH)/egctl --strip-components=3; \
	fi;

###############################################################################
# SOPS Secrets Management Targets
###############################################################################

# Check if SOPS script exists
.check-sops-script:
	@if [ ! -f "$(SOPS_SCRIPT)" ]; then \
		echo "Error: $(SOPS_SCRIPT) not found!"; \
		echo "Please ensure sops_manager.sh is in the current directory."; \
		exit 1; \
	fi
	@chmod +x $(SOPS_SCRIPT)

# Check if Age key file exists
.check-age-key:
	@if [ -z "$(SOPS_AGE_KEY_FILE)" ]; then \
		echo "Error: SOPS_AGE_KEY_FILE not set!"; \
		echo "Set it with: export SOPS_AGE_KEY_FILE=~/.age/key.txt"; \
		exit 1; \
	fi
	@if [ ! -f "$(SOPS_AGE_KEY_FILE)" ]; then \
		echo "Error: Age key file not found at $(SOPS_AGE_KEY_FILE)"; \
		echo "Generate one with: make age-keygen"; \
		exit 1; \
	fi

# Generate Age key pair
age-keygen: age
	@if [ -f "$(SOPS_AGE_KEY_FILE)" ]; then \
		echo "Age key already exists at $(SOPS_AGE_KEY_FILE)"; \
		echo "Backup your existing key before generating a new one!"; \
		exit 1; \
	fi
	@mkdir -p $(dirname $(SOPS_AGE_KEY_FILE))
	@age-keygen -o $(SOPS_AGE_KEY_FILE)
	@echo ""
	@echo "✓ Age key generated at: $(SOPS_AGE_KEY_FILE)"
	@echo ""
	@echo "IMPORTANT: Save your public key from above and add it to $(SOPS_CONFIG)"
	@echo "Example .sops.yaml entry:"
	@echo "  creation_rules:"
	@echo "    - path_regex: '^.*\.ya?ml$'"
	@echo "      age: age1..."

# List secrets that will be processed
secrets-list: .check-sops-script sops
	@echo "Listing secrets to be encrypted/decrypted..."
	@$(SOPS_SCRIPT) encrypt --list

# Show status of secrets (encrypted or not)
secrets-status: secrets-list

# Check secrets configuration
secrets-check: .check-sops-script .check-age-key sops age
	@echo "✓ SOPS script found: $(SOPS_SCRIPT)"
	@echo "✓ SOPS installed: $(sops --version)"
	@echo "✓ Age installed: $(age --version)"
	@echo "✓ Age key file: $(SOPS_AGE_KEY_FILE)"
	@if [ -f "$(SOPS_CONFIG)" ]; then \
		echo "✓ SOPS config found: $(SOPS_CONFIG)"; \
	else \
		echo "⚠ Warning: $(SOPS_CONFIG) not found"; \
		echo "  SOPS will use default behavior or environment variables"; \
	fi
	@echo ""
	@echo "Configuration OK!"

# Encrypt secrets with backups (safe)
secrets-encrypt: .check-sops-script .check-age-key sops
	@if [ ! -f "$(SOPS_CONFIG)" ]; then \
		echo "Warning: $(SOPS_CONFIG) not found!"; \
		echo "SOPS will use default configuration."; \
		echo ""; \
	fi
	@echo "Encrypting secrets..."
	@$(SOPS_SCRIPT) encrypt -v

# Encrypt secrets without backups (fast)
secrets-encrypt-fast: .check-sops-script .check-age-key sops
	@if [ ! -f "$(SOPS_CONFIG)" ]; then \
		echo "Warning: $(SOPS_CONFIG) not found!"; \
		echo "SOPS will use default configuration."; \
		echo ""; \
	fi
	@echo "Encrypting secrets (no backups)..."
	@$(SOPS_SCRIPT) encrypt -v --no-backup

# Decrypt secrets with backups (safe)
secrets-decrypt: .check-sops-script .check-age-key sops
	@echo "Decrypting secrets..."
	@$(SOPS_SCRIPT) decrypt -v

# Decrypt secrets without backups (fast)
secrets-decrypt-fast: .check-sops-script .check-age-key sops
	@echo "Decrypting secrets (no backups)..."
	@$(SOPS_SCRIPT) decrypt -v --no-backup

# Dry run - show what would be encrypted
secrets-encrypt-dry: .check-sops-script sops
	@echo "Dry run - showing what would be encrypted..."
	@$(SOPS_SCRIPT) encrypt --dry-run

# Dry run - show what would be decrypted
secrets-decrypt-dry: .check-sops-script sops
	@echo "Dry run - showing what would be decrypted..."
	@$(SOPS_SCRIPT) decrypt --dry-run

# Clean backup files created by SOPS operations
secrets-clean-backups:
	@echo "Cleaning SOPS backup files..."
	@find . -type f \( -name "*.backup.*" -o -name "*.encrypted.*" \) -print
	@echo ""
	@read -p "Delete these files? [y/N] " confirm; \
	if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then \
		find . -type f \( -name "*.backup.*" -o -name "*.encrypted.*" \) -delete; \
		echo "✓ Backup files cleaned"; \
	else \
		echo "Cancelled"; \
	fi

# Help target for secrets management
secrets-help:
	@echo "SOPS Secrets Management Commands:"
	@echo ""
	@echo "Setup:"
	@echo "  make age-keygen          - Generate Age encryption key pair"
	@echo "  make secrets-check       - Verify SOPS configuration"
	@echo ""
	@echo "Information:"
	@echo "  make secrets-list        - List files that will be processed"
	@echo "  make secrets-status      - Show status of secrets (alias for secrets-list)"
	@echo ""
	@echo "Encryption:"
	@echo "  make secrets-encrypt     - Encrypt secrets (with backups)"
	@echo "  make secrets-encrypt-fast - Encrypt secrets (no backups, faster)"
	@echo "  make secrets-encrypt-dry - Dry run encryption"
	@echo ""
	@echo "Decryption:"
	@echo "  make secrets-decrypt     - Decrypt secrets (with backups)"
	@echo "  make secrets-decrypt-fast - Decrypt secrets (no backups, faster)"
	@echo "  make secrets-decrypt-dry - Dry run decryption"
	@echo ""
	@echo "Maintenance:"
	@echo "  make secrets-clean-backups - Remove backup files"
	@echo ""
	@echo "Configuration:"
	@echo "  SOPS_AGE_KEY_FILE        - Path to Age private key (default: ~/.age/key.txt)"
	@echo "  $(SOPS_CONFIG)     - SOPS configuration file (defines encryption rules)"
	@echo ""
	@echo "Examples:"
	@echo "  # Setup"
	@echo "  make age-keygen"
	@echo "  export SOPS_AGE_KEY_FILE=~/.age/key.txt"
	@echo "  # Add public key to $(SOPS_CONFIG)"
	@echo ""
	@echo "  # Usage"
	@echo "  make secrets-encrypt"
	@echo "  make secrets-decrypt"
