.PHONY: clean lint rke terraform kubectl helm velero sops age kustomize flux gitops kubelogin egctl

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
