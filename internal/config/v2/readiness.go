package v2

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// ValidationSeverity identifies whether a readiness issue blocks deployment.
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
)

// ValidationCategory groups readiness issues by validation subsystem.
type ValidationCategory string

const (
	CategorySchema       ValidationCategory = "schema"
	CategoryProvider     ValidationCategory = "provider"
	CategoryGitOps       ValidationCategory = "gitops"
	CategoryServices     ValidationCategory = "services"
	CategoryConnectivity ValidationCategory = "connectivity"
)

// ValidationIssue is a structured validation finding.
type ValidationIssue struct {
	Severity   ValidationSeverity `json:"severity"`
	Category   ValidationCategory `json:"category"`
	Path       string             `json:"path,omitempty"`
	Message    string             `json:"message"`
	Suggestion string             `json:"suggestion,omitempty"`
}

// ReadinessReport contains deterministic, offline deployment-readiness findings.
type ReadinessReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidateReadiness validates cross-field deployment-readiness rules that are not
// covered by schema tags. It does not contact cloud providers or Git remotes.
func ValidateReadiness(cfg *Config) ReadinessReport {
	r := readinessBuilder{report: ReadinessReport{Valid: true}}
	if cfg == nil {
		r.addError(CategorySchema, "", "configuration is nil", "")
		return r.report
	}

	r.validateProvider(cfg)
	r.validateNetworkPlugin(cfg)
	r.validateGitOps(cfg)
	r.validateServiceSchedulingCapacity(cfg)
	r.validateStorageProfile(cfg)
	r.validateServiceSecrets(cfg)
	r.validateS3Buckets(cfg)

	return r.report
}

type readinessBuilder struct {
	report ReadinessReport
}

func (r *readinessBuilder) addError(category ValidationCategory, path, message, suggestion string) {
	r.addIssue(SeverityError, category, path, message, suggestion)
}

func (r *readinessBuilder) addWarning(category ValidationCategory, path, message, suggestion string) {
	r.addIssue(SeverityWarning, category, path, message, suggestion)
}

func (r *readinessBuilder) addIssue(severity ValidationSeverity, category ValidationCategory, path, message, suggestion string) {
	if severity == SeverityError {
		r.report.Valid = false
	}
	r.report.Issues = append(r.report.Issues, ValidationIssue{
		Severity:   severity,
		Category:   category,
		Path:       path,
		Message:    message,
		Suggestion: suggestion,
	})
}

func (r *readinessBuilder) validateProvider(cfg *Config) {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	switch provider {
	case "openstack":
		r.validateOpenStackProvider(cfg)
	case "baremetal":
		r.validateBaremetalProvider(cfg)
	case "vmware":
		r.validateVMwareProvider(cfg)
	case "kind":
		r.validateKindProvider(cfg)
	case "aws":
		r.validateAWSProvider(cfg)
	case "gcp":
		r.validateGCPProvider(cfg)
	case "azure":
		r.validateAzureProvider(cfg)
	case "magnum":
		r.validateMagnumProvider(cfg)
	case "vsphere":
		// vSphere is deprecated in favor of vmware; accept silently.
		return
	case "":
		r.addError(CategoryProvider, "opencenter.infrastructure.provider", "provider is required", "Set infrastructure.provider to a supported provider.")
	default:
		r.addError(CategoryProvider, "opencenter.infrastructure.provider", fmt.Sprintf("unsupported provider %q", provider), "Use one of: openstack, aws, gcp, azure, baremetal, vsphere, vmware, kind, magnum.")
	}
}

func (r *readinessBuilder) validateNetworkPlugin(cfg *Config) {
	plugins := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin
	type enabledPlugin struct {
		name   string
		path   string
		method string
	}

	var enabled []enabledPlugin
	if plugins.Calico != nil && plugins.Calico.Enabled {
		enabled = append(enabled, enabledPlugin{
			name:   "calico",
			path:   "opencenter.cluster.kubernetes.network_plugin.calico.install_method",
			method: plugins.Calico.InstallMethod,
		})
	}
	if plugins.Cilium != nil && plugins.Cilium.Enabled {
		enabled = append(enabled, enabledPlugin{
			name:   "cilium",
			path:   "opencenter.cluster.kubernetes.network_plugin.cilium.install_method",
			method: plugins.Cilium.InstallMethod,
		})
	}
	if plugins.KubeOVN != nil && plugins.KubeOVN.Enabled {
		enabled = append(enabled, enabledPlugin{
			name:   "kube-ovn",
			path:   "opencenter.cluster.kubernetes.network_plugin.kube-ovn.install_method",
			method: plugins.KubeOVN.InstallMethod,
		})
	}

	// Kind's built-in default CNI (kindnet) provides networking when the default
	// CNI is not disabled. In that mode kindnet is the CNI, so none of the
	// managed plugins (calico/cilium/kube-ovn) should be enabled. Only when the
	// operator opts into managed CNI (disable_default_cni=true) must exactly one
	// managed plugin be enabled.
	if strings.EqualFold(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider), "kind") {
		kindnetActive := cfg.OpenCenter.Infrastructure.Kind == nil || !cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI
		if kindnetActive {
			if len(enabled) > 0 {
				names := make([]string, 0, len(enabled))
				for _, plugin := range enabled {
					names = append(names, plugin.name)
				}
				r.addError(CategorySchema, "opencenter.cluster.kubernetes.network_plugin", fmt.Sprintf("kind uses its default CNI (kindnet) when disable_default_cni is false, so no managed network_plugin may be enabled; enabled: %s.", strings.Join(names, ", ")), "Disable calico/cilium/kube-ovn, or set opencenter.infrastructure.kind.disable_default_cni: true to manage the CNI.")
			}
			return
		}
	}

	switch len(enabled) {
	case 0:
		r.addError(CategorySchema, "opencenter.cluster.kubernetes.network_plugin", "exactly one network_plugin must be enabled.", "Enable exactly one of calico, cilium, or kube-ovn.")
		return
	case 1:
	default:
		names := make([]string, 0, len(enabled))
		for _, plugin := range enabled {
			names = append(names, plugin.name)
		}
		r.addError(CategorySchema, "opencenter.cluster.kubernetes.network_plugin", fmt.Sprintf("only one network_plugin may be enabled; enabled: %s.", strings.Join(names, ", ")), "Disable all but one of calico, cilium, or kube-ovn.")
		return
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))

	plugin := enabled[0]
	method := strings.ToLower(strings.TrimSpace(plugin.method))
	if method == "" {
		method = "helm"
	}

	if provider != "openstack" {
		return
	}

	switch method {
	case "helm", "kustomize-helm":
		return
	case "kubespray":
		r.addError(CategorySchema, plugin.path, fmt.Sprintf("OpenStack network_plugin %s cannot use install_method %q.", plugin.name, method), "Use install_method: helm or install_method: kustomize-helm.")
	default:
		r.addError(CategorySchema, plugin.path, fmt.Sprintf("unsupported OpenStack network_plugin install_method %q; accepted values are helm and kustomize-helm.", method), "Use install_method: helm or install_method: kustomize-helm.")
	}
}

func (r *readinessBuilder) validateOpenStackProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud
	os := cloud.OpenStack
	if os == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack", "openstack provider requires openstack cloud configuration", "Add opencenter.infrastructure.cloud.openstack.")
		return
	}

	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url", os.AuthURL, "OpenStack auth URL is required.", "Set the Keystone auth_url endpoint.")
	if parsed, err := url.Parse(strings.TrimSpace(os.AuthURL)); strings.TrimSpace(os.AuthURL) != "" && (err != nil || parsed.Scheme == "" || parsed.Host == "") {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url", "OpenStack auth URL must be a valid absolute URL.", "Use the full Keystone URL, for example https://keystone.example.com/v3.")
	} else if parsed != nil && parsed.Scheme == "http" {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url", "OpenStack auth URL is using plain HTTP.", "Use HTTPS for Keystone endpoints when possible.")
	}
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.region", os.Region, "OpenStack region is required.", "Set the OpenStack region.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.project_id", os.ProjectID, "OpenStack project ID is required.", "Set the OpenStack project ID.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.image_id", os.ImageID, "OpenStack image ID is required.", "Set an image ID that exists in Glance.")

	hasAppCredID := valueSet(os.ApplicationCredentialID)
	hasAppCredSecret := valueSet(os.ApplicationCredentialSecret)
	if hasAppCredID != hasAppCredSecret {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.application_credential_id", "OpenStack application credential ID and secret must be set together.", "Set both application_credential_id and application_credential_secret, or use username/password credentials.")
	}
	if isMissingSecret(os.ApplicationCredentialID) {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.application_credential_id", "OpenStack application credential ID is required for readiness validation.", "Create an OpenStack application credential and set its ID.")
	}
	if isMissingSecret(os.ApplicationCredentialSecret) {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.application_credential_secret", "OpenStack application credential secret is required for readiness validation.", "Set the OpenStack application credential secret.")
	}

	compute := cfg.OpenCenter.Infrastructure.Compute
	if compute.MasterCount > 0 {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_master", compute.FlavorMaster, "OpenStack master flavor is required when master_count is greater than zero.", "Set compute.flavor_master.")
	}
	if compute.WorkerCount > 0 {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_worker", compute.FlavorWorker, "OpenStack worker flavor is required when worker_count is greater than zero.", "Set compute.flavor_worker.")
	}
	if compute.WorkerCountWindows > 0 {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_worker_windows", compute.FlavorWorkerWindows, "OpenStack Windows worker flavor is required when worker_count_windows is greater than zero.", "Set compute.flavor_worker_windows.")
	}
	if cfg.OpenCenter.Infrastructure.Bastion.Enabled {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_bastion", compute.FlavorBastion, "OpenStack bastion flavor is required when bastion is enabled.", "Set compute.flavor_bastion.")
	}
	for i, pool := range compute.AdditionalServerPoolsWorker {
		if pool.Count > 0 {
			r.requireNonPlaceholder(CategoryProvider, fmt.Sprintf("opencenter.infrastructure.compute.additional_server_pools_worker[%d].flavor", i), pool.Flavor, "OpenStack worker pool flavor is required when pool count is greater than zero.", "Set each worker pool flavor.")
		}
	}

	if cloud.AWS != nil || cloud.GCP != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud", "inactive provider cloud sections are configured alongside openstack.", "Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateBaremetalProvider(cfg *Config) {
	infra := cfg.OpenCenter.Infrastructure
	compute := infra.Compute
	cloud := infra.Cloud

	// Baremetal requires pre-provisioned node definitions, not cloud flavors.
	if compute.MasterCount > 0 && len(compute.MasterNodes) == 0 {
		r.addError(CategoryProvider, "opencenter.infrastructure.compute.master_nodes",
			fmt.Sprintf("baremetal provider requires master_nodes when master_count is %d.", compute.MasterCount),
			"Define master_nodes with name and access_ip_v4 for each control plane node.")
	}
	if compute.WorkerCount > 0 && len(compute.WorkerNodes) == 0 {
		r.addError(CategoryProvider, "opencenter.infrastructure.compute.worker_nodes",
			fmt.Sprintf("baremetal provider requires worker_nodes when worker_count is %d.", compute.WorkerCount),
			"Define worker_nodes with name and access_ip_v4 for each worker node.")
	}
	if compute.WorkerCountWindows > 0 && len(compute.WindowsNodes) == 0 {
		r.addError(CategoryProvider, "opencenter.infrastructure.compute.windows_nodes",
			fmt.Sprintf("baremetal provider requires windows_nodes when worker_count_windows is %d.", compute.WorkerCountWindows),
			"Define windows_nodes with name and access_ip_v4 for each Windows worker node.")
	}

	// Validate each static node has required fields.
	for i, node := range compute.MasterNodes {
		r.validateStaticNode("master_nodes", i, node)
	}
	for i, node := range compute.WorkerNodes {
		r.validateStaticNode("worker_nodes", i, node)
	}
	for i, node := range compute.WindowsNodes {
		r.validateStaticNode("windows_nodes", i, node)
	}

	// Count mismatch warnings.
	if compute.MasterCount > 0 && len(compute.MasterNodes) > 0 && compute.MasterCount != len(compute.MasterNodes) {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.compute.master_count",
			fmt.Sprintf("master_count (%d) does not match the number of master_nodes defined (%d).", compute.MasterCount, len(compute.MasterNodes)),
			"Set master_count to match the number of master_nodes entries.")
	}
	if compute.WorkerCount > 0 && len(compute.WorkerNodes) > 0 && compute.WorkerCount != len(compute.WorkerNodes) {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.compute.worker_count",
			fmt.Sprintf("worker_count (%d) does not match the number of worker_nodes defined (%d).", compute.WorkerCount, len(compute.WorkerNodes)),
			"Set worker_count to match the number of worker_nodes entries.")
	}

	// Bastion should be disabled for baremetal (nodes are pre-provisioned).
	if infra.Bastion.Enabled {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.bastion.enabled",
			"bastion is enabled but baremetal nodes are pre-provisioned — bastion provisioning will be skipped.",
			"Set bastion.enabled to false for baremetal deployments.")
	}

	// VIP interface warning when kube-vip is used.
	if cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled && infra.Networking.VIPInterface == "" {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.networking.vip_interface",
			"kube_vip_enabled is true but vip_interface is not set — kube-vip may bind to the wrong interface.",
			"Set networking.vip_interface to the interface where the VIP should be advertised (e.g., eth0, bond0).")
	}

	// No cloud sections should be active for baremetal.
	if cloud.OpenStack != nil || cloud.AWS != nil || cloud.GCP != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud",
			"baremetal provider should not have cloud provider sections configured.",
			"Remove all cloud.* sections — baremetal uses pre-provisioned nodes defined in compute.master_nodes/worker_nodes.")
	}
}

func (r *readinessBuilder) validateStaticNode(nodeType string, index int, node StaticNode) {
	path := fmt.Sprintf("opencenter.infrastructure.compute.%s[%d]", nodeType, index)
	if strings.TrimSpace(node.Name) == "" {
		r.addError(CategoryProvider, path+".name",
			fmt.Sprintf("%s[%d].name is required.", nodeType, index),
			"Each baremetal node must have a unique name.")
	}
	if strings.TrimSpace(node.AccessIPv4) == "" {
		r.addError(CategoryProvider, path+".access_ip_v4",
			fmt.Sprintf("%s[%d].access_ip_v4 is required.", nodeType, index),
			"Each baremetal node must have an IP address for SSH access.")
	}
}

func (r *readinessBuilder) validateVMwareProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud

	if cloud.VMware == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.vmware",
			"vmware provider requires cloud.vmware configuration section.",
			"Add infrastructure.cloud.vmware with vcenter_server, datacenter, datastore, network, and template.")
		return
	}

	vmw := cloud.VMware
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.vmware.vcenter_server", vmw.VCenterServer, "VMware vCenter server address is required.", "Set cloud.vmware.vcenter_server to the vCenter hostname or IP.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.vmware.datacenter", vmw.Datacenter, "VMware datacenter is required.", "Set cloud.vmware.datacenter.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.vmware.datastore", vmw.Datastore, "VMware datastore is required.", "Set cloud.vmware.datastore.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.vmware.network", vmw.Network, "VMware network is required.", "Set cloud.vmware.network.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.vmware.template", vmw.Template, "VMware VM template is required.", "Set cloud.vmware.template to the base VM template name.")

	// Warn if vSphere CSI is not enabled.
	if !vsphereCSIEnabled(cfg) {
		r.addWarning(CategoryProvider, "opencenter.cluster.kubernetes.storage_plugin.vsphere_csi",
			"vmware provider typically requires vSphere CSI for persistent storage.",
			"Enable storage_plugin.vsphere_csi or configure an alternative CSI driver.")
	}

	// No other cloud sections should be active.
	if cloud.OpenStack != nil || cloud.AWS != nil || cloud.GCP != nil || cloud.Azure != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud",
			"inactive provider cloud sections are configured alongside vmware.",
			"Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateKindProvider(cfg *Config) {
	infra := cfg.OpenCenter.Infrastructure

	// Kind should not have bastion enabled.
	if infra.Bastion.Enabled {
		r.addError(CategoryProvider, "opencenter.infrastructure.bastion.enabled",
			"bastion must be disabled for kind provider — kind runs locally.",
			"Set bastion.enabled to false.")
	}

	// Kind should not use kube-vip.
	if cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled {
		r.addError(CategoryProvider, "opencenter.cluster.kubernetes.kube_vip_enabled",
			"kube_vip_enabled must be false for kind provider.",
			"Set cluster.kubernetes.kube_vip_enabled to false.")
	}

	// Kind should not use VRRP.
	if infra.Networking.VRRPEnabled {
		r.addError(CategoryProvider, "opencenter.infrastructure.networking.vrrp_enabled",
			"vrrp_enabled must be false for kind provider.",
			"Set networking.vrrp_enabled to false.")
	}

	// No cloud sections should be active for kind.
	cloud := infra.Cloud
	if cloud.OpenStack != nil || cloud.AWS != nil || cloud.GCP != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.cloud",
			"kind provider does not use cloud sections — they will be ignored.",
			"Remove cloud.* sections for kind deployments.")
	}
}

func (r *readinessBuilder) validateAWSProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud

	if cloud.AWS == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.aws",
			"aws provider requires cloud.aws configuration section.",
			"Add infrastructure.cloud.aws with region, vpc_id, subnet_ids, and ami_id.")
		return
	}

	aws := cloud.AWS
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.aws.region", aws.Region, "AWS region is required.", "Set cloud.aws.region (e.g., us-east-1).")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.aws.vpc_id", aws.VPCID, "AWS VPC ID is required.", "Set cloud.aws.vpc_id.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.aws.ami_id", aws.AMIID, "AWS AMI ID is required.", "Set cloud.aws.ami_id to the base image for cluster nodes.")
	if len(aws.SubnetIDs) == 0 {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.aws.subnet_ids",
			"AWS subnet_ids is required.",
			"Set cloud.aws.subnet_ids to at least one subnet.")
	}

	// No other cloud sections should be active.
	if cloud.OpenStack != nil || cloud.GCP != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud",
			"inactive provider cloud sections are configured alongside aws.",
			"Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateGCPProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud

	if cloud.GCP == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.gcp",
			"gcp provider requires cloud.gcp configuration section.",
			"Add infrastructure.cloud.gcp with project, region, network, subnetwork, and image_family.")
		return
	}

	gcp := cloud.GCP
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.gcp.project", gcp.Project, "GCP project is required.", "Set cloud.gcp.project.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.gcp.region", gcp.Region, "GCP region is required.", "Set cloud.gcp.region (e.g., us-central1).")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.gcp.network", gcp.Network, "GCP network is required.", "Set cloud.gcp.network.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.gcp.subnetwork", gcp.Subnetwork, "GCP subnetwork is required.", "Set cloud.gcp.subnetwork.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.gcp.image_family", gcp.ImageFamily, "GCP image family is required.", "Set cloud.gcp.image_family.")

	// No other cloud sections should be active.
	if cloud.OpenStack != nil || cloud.AWS != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud",
			"inactive provider cloud sections are configured alongside gcp.",
			"Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateAzureProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud

	if cloud.Azure == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.azure",
			"azure provider requires cloud.azure configuration section.",
			"Add infrastructure.cloud.azure with subscription_id, resource_group, location, vnet_name, subnet_name, and image_reference.")
		return
	}

	az := cloud.Azure
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.azure.subscription_id", az.SubscriptionID, "Azure subscription ID is required.", "Set cloud.azure.subscription_id.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.azure.resource_group", az.ResourceGroup, "Azure resource group is required.", "Set cloud.azure.resource_group.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.azure.location", az.Location, "Azure location is required.", "Set cloud.azure.location (e.g., eastus).")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.azure.vnet_name", az.VNetName, "Azure VNet name is required.", "Set cloud.azure.vnet_name.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.azure.subnet_name", az.SubnetName, "Azure subnet name is required.", "Set cloud.azure.subnet_name.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.azure.image_reference", az.ImageReference, "Azure image reference is required.", "Set cloud.azure.image_reference.")

	// No other cloud sections should be active.
	if cloud.OpenStack != nil || cloud.AWS != nil || cloud.GCP != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud",
			"inactive provider cloud sections are configured alongside azure.",
			"Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateMagnumProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud
	if cloud.Magnum == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.magnum",
			"magnum provider requires cloud.magnum configuration.",
			"Add cloud.magnum with the Keystone endpoint, project_id, application credentials, and cluster_template.")
		return
	}

	magnum := cloud.Magnum
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.magnum.auth_url", magnum.AuthURL,
		"Magnum Keystone auth_url is required.", "Set cloud.magnum.auth_url to the Keystone endpoint.")
	if parsed, err := url.Parse(strings.TrimSpace(magnum.AuthURL)); strings.TrimSpace(magnum.AuthURL) != "" && (err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https")) {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.magnum.auth_url",
			"Magnum auth_url must be a valid absolute HTTP(S) Keystone URL.",
			"Use the full Keystone URL, for example https://keystone.example.com/v3.")
	} else if parsed != nil && parsed.Scheme == "http" {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.cloud.magnum.auth_url",
			"Magnum auth_url is using plain HTTP.", "Use HTTPS for Keystone endpoints when possible.")
	}
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.magnum.region", magnum.Region,
		"Magnum region is required.", "Set cloud.magnum.region.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.magnum.project_id", magnum.ProjectID,
		"Magnum project_id is required.", "Set the Keystone project ID used by Magnum.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.magnum.cluster_template", magnum.ClusterTemplate,
		"Magnum cluster_template is required.", "Set the existing Magnum cluster template name or ID.")

	hasAppCredID := valueSet(magnum.ApplicationCredentialID)
	hasAppCredSecret := valueSet(magnum.ApplicationCredentialSecret)
	if hasAppCredID != hasAppCredSecret {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.magnum.application_credential_id",
			"Magnum application credential ID and secret must be set together.",
			"Set both application_credential_id and application_credential_secret.")
	}
	if !hasAppCredID {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.magnum.application_credential_id",
			"Magnum application credential ID is required for readiness validation.",
			"Create a Keystone application credential and set its ID.")
	}
	if !hasAppCredSecret {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.magnum.application_credential_secret",
			"Magnum application credential secret is required for readiness validation.",
			"Set the Keystone application credential secret.")
	}

	if cloud.OpenStack != nil || cloud.AWS != nil || cloud.GCP != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud",
			"inactive provider cloud sections are configured alongside magnum.",
			"Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateGitOps(cfg *Config) {
	repoURL := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL)
	if repoURL == "" {
		r.addError(CategoryGitOps, "opencenter.gitops.repository.url", "GitOps repository URL is required.", "Set gitops.repository.url.")
		return
	}

	auth := cfg.OpenCenter.GitOps.Auth
	if auth.SSH != nil && auth.Token != nil {
		r.addError(CategoryGitOps, "opencenter.gitops.auth", "GitOps SSH and token auth are both configured.", "Configure exactly one GitOps auth method.")
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		r.addError(CategoryGitOps, "opencenter.gitops.repository.url", "GitOps repository URL is not valid.", "Use an https:// or ssh:// Git repository URL.")
		return
	}

	switch strings.ToLower(parsed.Scheme) {
	case "https":
		r.validateGitOpsHTTPS(parsed, auth)
	case "ssh":
		r.validateGitOpsSSH(auth)
	default:
		r.addError(CategoryGitOps, "opencenter.gitops.repository.url", "GitOps repository URL must use https or ssh.", "Use https:// for token auth or ssh:// for deploy-key auth.")
	}
}

func (r *readinessBuilder) validateGitOpsHTTPS(parsed *url.URL, auth GitOpsAuth) {
	if auth.Token == nil {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.token", "HTTPS GitOps repository requires token auth.", "Configure gitops.auth.token for HTTPS repositories.")
		return
	}
	if isMissingSecret(auth.Token.Token) && isMissingSecret(auth.Token.TokenFile) {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.token.token", "HTTPS GitOps repository requires a token value or token_file.", "Set gitops.auth.token.token or gitops.auth.token.token_file.")
	}
	expectedProvider := gitProviderForHost(parsed.Hostname())
	if expectedProvider != "" && strings.ToLower(strings.TrimSpace(auth.Token.Provider)) != expectedProvider {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.token.provider", fmt.Sprintf("GitOps token provider must be %q for host %q.", expectedProvider, parsed.Hostname()), "Set gitops.auth.token.provider to match the repository host.")
	}
}

func (r *readinessBuilder) validateGitOpsSSH(auth GitOpsAuth) {
	if auth.SSH == nil {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.ssh", "SSH GitOps repository requires SSH key auth.", "Configure gitops.auth.ssh for SSH repositories.")
		return
	}
	r.requireNonPlaceholder(CategoryGitOps, "opencenter.gitops.auth.ssh.private_key", auth.SSH.PrivateKey, "SSH GitOps repository requires a private key path.", "Set gitops.auth.ssh.private_key.")
	r.requireNonPlaceholder(CategoryGitOps, "opencenter.gitops.auth.ssh.public_key", auth.SSH.PublicKey, "SSH GitOps repository requires a public key path.", "Set gitops.auth.ssh.public_key.")
}

func gitProviderForHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		return "github"
	case host == "gitlab.com" || strings.Contains(host, "gitlab"):
		return "gitlab"
	case host != "":
		return "gitea"
	default:
		return ""
	}
}

func (r *readinessBuilder) validateServiceSchedulingCapacity(cfg *Config) {
	if !serviceEnabled(cfg, "keycloak") {
		return
	}
	keycloak, _ := configuredService(cfg, "keycloak").(*services.KeycloakConfig)
	if keycloak == nil || keycloak.Instances <= 0 {
		return
	}

	workers := configuredSchedulableLinuxWorkers(cfg)
	if keycloak.Instances > workers {
		r.addError(
			CategoryServices,
			"opencenter.services.keycloak.instances",
			fmt.Sprintf("Keycloak instances (%d) exceed configured schedulable Linux workers (%d); Keycloak uses hostname topology spread with DoNotSchedule.", keycloak.Instances, workers),
			"Increase Linux worker capacity, including additional worker pools, or reduce Keycloak instances.",
		)
	}
}

func configuredSchedulableLinuxWorkers(cfg *Config) int {
	if cfg == nil {
		return 0
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Deployment.Method), "kamaji") && cfg.Deployment.Kamaji != nil && len(cfg.Deployment.Kamaji.WorkerPools) > 0 {
		capacity := 0
		for _, pool := range cfg.Deployment.Kamaji.WorkerPools {
			if !strings.EqualFold(strings.TrimSpace(pool.OS), "windows") && pool.Count > 0 {
				capacity += pool.Count
			}
		}
		return capacity
	}

	compute := cfg.OpenCenter.Infrastructure.Compute
	capacity := compute.WorkerCount
	for _, pool := range compute.AdditionalServerPoolsWorker {
		if pool.Count > 0 {
			capacity += pool.Count
		}
	}
	return capacity
}

func (r *readinessBuilder) validateStorageProfile(cfg *Config) {
	for _, issue := range storagePolicyIssues(cfg) {
		r.addError(CategoryServices, issue.path, issue.message, "Use the supported external S3 or non-production Longhorn/RustFS storage profile.")
	}
}

func (r *readinessBuilder) validateServiceSecrets(cfg *Config) {
	if serviceEnabled(cfg, "keycloak") {
		if !oidcClientSecretsProvidedInternally(cfg) {
			r.requireSecret("secrets.keycloak.client_secret", cfg.Secrets.Keycloak.ClientSecret, "Keycloak client secret is required when keycloak is enabled.")
		}
		r.requireSecret("secrets.keycloak.admin_password", cfg.Secrets.Keycloak.AdminPassword, "Keycloak admin password is required when keycloak is enabled.")
	}
	if serviceEnabled(cfg, "headlamp") && headlampUsesOIDC(cfg) && !oidcClientSecretsProvidedInternally(cfg) {
		r.requireSecret("secrets.headlamp.oidc_client_secret", cfg.Secrets.Headlamp.OIDCClientSecret, "Headlamp OIDC client secret is required when Headlamp OIDC is enabled.")
	}
	if serviceEnabled(cfg, "kube-prometheus-stack") {
		r.requireSecret("secrets.grafana.admin_password", cfg.Secrets.Grafana.AdminPassword, "Grafana admin password is required when kube-prometheus-stack is enabled.")
		r.validateKubePrometheusStackWebhookURL(cfg)
	}
	r.validateCertManagerSecrets(cfg)
	r.validateEtcdBackupSecrets(cfg)
	r.validateVeleroSecrets(cfg)
	r.validateLokiSecrets(cfg)
	r.validateTempoSecrets(cfg)
	r.validateMimirSecrets(cfg)
	r.validateHarborSecrets(cfg)
	if serviceEnabled(cfg, "weave-gitops") {
		if isMissingSecret(cfg.Secrets.WeaveGitOps.Password) && isMissingSecret(cfg.Secrets.WeaveGitOps.PasswordHash) {
			r.addError(CategoryServices, "secrets.weave_gitops.password", "Weave GitOps requires password or password_hash when enabled.", "Set secrets.weave_gitops.password_hash or secrets.weave_gitops.password.")
		}
	}
	if serviceEnabled(cfg, "alert-proxy") {
		r.requireSecret("secrets.alert_proxy.core_device_id", cfg.Secrets.AlertProxy.CoreDeviceId, "Alert proxy core device ID is required when alert-proxy is enabled.")
		r.requireSecret("secrets.alert_proxy.account_service_token", cfg.Secrets.AlertProxy.AccountServiceToken, "Alert proxy account service token is required when alert-proxy is enabled.")
		r.requireSecret("secrets.alert_proxy.core_account_number", cfg.Secrets.AlertProxy.CoreAccountNumber, "Alert proxy core account number is required when alert-proxy is enabled.")
	}
	if vsphereCSIEnabled(cfg) {
		r.requireSecret("secrets.vsphere_csi.vcenter_host", cfg.Secrets.VSphereCsi.VCenterHost, "vSphere CSI vCenter host is required when vSphere CSI is enabled.")
		r.requireSecret("secrets.vsphere_csi.username", cfg.Secrets.VSphereCsi.Username, "vSphere CSI username is required when vSphere CSI is enabled.")
		r.requireSecret("secrets.vsphere_csi.password", cfg.Secrets.VSphereCsi.Password, "vSphere CSI password is required when vSphere CSI is enabled.")
	}
}

func (r *readinessBuilder) validateKubePrometheusStackWebhookURL(cfg *Config) {
	stack, _ := configuredService(cfg, "kube-prometheus-stack").(*services.PrometheusStackConfig)
	if stack == nil {
		return
	}

	webhookURL := strings.TrimSpace(stack.WebhookURL)
	if webhookURL == "" {
		return
	}
	parsed, err := url.Parse(webhookURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		r.addError(
			CategoryServices,
			"opencenter.services.kube-prometheus-stack.webhook_url",
			"kube-prometheus-stack webhook_url must be an absolute HTTPS URL when configured.",
			"Set a complete https:// webhook URL or leave webhook_url empty to disable Microsoft Teams notifications.",
		)
	}
}

func (r *readinessBuilder) validateVeleroSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "velero") || ResolveVeleroStorageBackend(cfg) == "none" || UsesManagedObjectStorage(cfg) {
		return
	}
	service, ok := configuredService(cfg, "velero").(*services.VeleroConfig)
	if !ok || service == nil {
		r.addError(CategoryServices, "opencenter.services.velero", fmt.Sprintf("velero has unexpected configuration type %T.", configuredService(cfg, "velero")), "Use the canonical Velero service configuration.")
		return
	}
	if ResolveVeleroStorageBackend(cfg) != "s3" {
		return
	}
	r.requireS3Endpoint("opencenter.services.velero.s3_endpoint", service.S3Endpoint, "Velero S3 storage requires a configured endpoint.")
	if strings.TrimSpace(service.BackupBucket) == "" {
		r.addError(CategoryServices, "opencenter.services.velero.backup_bucket", "Velero S3 storage requires a bucket name.", "Set a non-empty Velero backup bucket.")
	}
	if strings.TrimSpace(service.Region) == "" && strings.TrimSpace(service.S3Region) == "" {
		r.addError(CategoryServices, "opencenter.services.velero.region", "Velero S3 storage requires a region.", "Set Velero region or s3_region.")
	}
	r.requireSecret("secrets.velero.access_key_id", cfg.Secrets.Velero.AccessKeyID, "Velero S3 storage requires an access key ID.")
	r.requireSecret("secrets.velero.secret_access_key", cfg.Secrets.Velero.SecretAccessKey, "Velero S3 storage requires a secret access key.")
}

func (r *readinessBuilder) validateCertManagerSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "cert-manager") {
		return
	}
	certManager, _ := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if certManager == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(certManager.DNSProvider)) {
	case "route53":
		r.requireSecret("secrets.cert_manager.aws_access_key", cfg.Secrets.CertManager.AWSAccessKey, "cert-manager Route53 DNS requires AWS access key.")
		r.requireSecret("secrets.cert_manager.aws_secret_access_key", cfg.Secrets.CertManager.AWSSecretAccessKey, "cert-manager Route53 DNS requires AWS secret access key.")
	case "cloudflare":
		r.requireSecret("secrets.cert_manager.cloudflare_api_token", cfg.Secrets.CertManager.CloudflareAPIToken, "cert-manager Cloudflare DNS requires API token.")
	}
}

func (r *readinessBuilder) validateEtcdBackupSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "etcd-backup") || storageBackendDoesNotUseObjectStorage(cfg, "etcd-backup") || UsesManagedObjectStorage(cfg) {
		return
	}
	service := configuredService(cfg, "etcd-backup")
	etcdBackup, ok := service.(*services.EtcdBackupConfig)
	if !ok || etcdBackup == nil {
		r.addError(CategoryServices, "opencenter.services.etcd-backup", fmt.Sprintf("etcd-backup has unexpected configuration type %T.", service), "Use the canonical etcd-backup service configuration.")
		return
	}
	r.requireS3Endpoint("opencenter.services.etcd-backup.s3_endpoint", etcdBackup.S3Endpoint, "etcd-backup requires a configured S3 endpoint.")
	if strings.TrimSpace(etcdBackup.S3BucketName) == "" {
		r.addError(CategoryServices, "opencenter.services.etcd-backup.s3_bucket_name", "etcd-backup requires a bucket name.", "Set a non-empty S3 bucket name.")
	}
	if strings.TrimSpace(etcdBackup.S3Region) == "" {
		r.addError(CategoryServices, "opencenter.services.etcd-backup.s3_region", "etcd-backup requires an S3 region.", "Set a non-empty S3 region.")
	}
	r.requireSecret("secrets.etcd_backup.access_key_id", cfg.Secrets.EtcdBackup.AccessKeyID, "etcd-backup requires an access key ID.")
	r.requireSecret("secrets.etcd_backup.secret_access_key", cfg.Secrets.EtcdBackup.SecretAccessKey, "etcd-backup requires a secret access key.")
}

func (r *readinessBuilder) validateLokiSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "loki") || UsesManagedObjectStorage(cfg) {
		return
	}
	switch ResolveObjectStorageBackend(cfg, "loki") {
	case "swift":
		r.requireSecret("secrets.loki.swift_application_credential_secret", cfg.GetLokiSwiftApplicationCredentialSecret(), "Loki Swift storage requires an application credential secret.")
	case "s3":
		loki, _ := configuredService(cfg, "loki").(*services.LokiConfig)
		if loki != nil {
			r.requireS3Endpoint("opencenter.services.loki.s3_endpoint", loki.S3Endpoint, "Loki S3 storage requires a configured endpoint.")
		}
		accessKey, secretKey := cfg.GetLokiS3Credentials()
		r.requireSecret("secrets.loki.s3_access_key_id", accessKey, "Loki S3 storage requires an access key ID.")
		r.requireSecret("secrets.loki.s3_secret_access_key", secretKey, "Loki S3 storage requires a secret access key.")
	}
}

func (r *readinessBuilder) validateTempoSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "tempo") || UsesManagedObjectStorage(cfg) {
		return
	}
	switch ResolveObjectStorageBackend(cfg, "tempo") {
	case "swift":
		r.requireSecret("secrets.tempo.swift_application_credential_secret", cfg.GetTempoSwiftApplicationCredentialSecret(), "Tempo Swift storage requires an application credential secret.")
	case "s3":
		tempo, _ := configuredService(cfg, "tempo").(*services.TempoConfig)
		if tempo != nil {
			r.requireS3Endpoint("opencenter.services.tempo.s3_endpoint", tempo.S3Endpoint, "Tempo S3 storage requires a configured endpoint.")
		}
		accessKey, secretKey := cfg.GetTempoS3Credentials()
		r.requireSecret("secrets.tempo.access_key", accessKey, "Tempo S3 storage requires an access key.")
		r.requireSecret("secrets.tempo.secret_key", secretKey, "Tempo S3 storage requires a secret key.")
	}
}

func (r *readinessBuilder) validateMimirSecrets(cfg *Config) {
	// Mimir has no managed RustFS credential source in the pinned chart
	// contract. Always validate the effective Mimir backend, including when
	// the cluster selects the managed object-storage profile.
	if !serviceEnabled(cfg, "mimir") {
		return
	}
	backend := ResolveObjectStorageBackend(cfg, "mimir")
	if backend == "swift" && strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) != "openstack" {
		r.addError(CategoryProvider, "opencenter.services.mimir.storage_type", "Mimir's resolved Swift storage backend is only supported on OpenStack infrastructure.", "Set services.mimir.storage_type to s3 for non-OpenStack providers.")
	}
	switch backend {
	case "swift":
		credentialID, credentialSecret, pairErr := cfg.ResolveMimirSwiftCredentials()
		if pairErr != nil {
			r.addError(CategoryServices, "opencenter.services.mimir.swift_application_credential_id", pairErr.Error(), "Set both service-specific Mimir Swift credential fields, or clear both and configure the global OpenStack pair.")
			break
		}
		if valueSet(credentialID) != valueSet(credentialSecret) {
			r.addError(CategoryServices, "opencenter.services.mimir.swift_application_credential_id", "Mimir Swift application credential ID and secret must be set together.", "Set both the typed Mimir credential fields or both global OpenStack application credential values.")
		}
		if !valueSet(credentialID) {
			r.addError(CategoryServices, "opencenter.services.mimir.swift_application_credential_id", "Mimir Swift blocks storage requires an application credential ID.", "Set swift_application_credential_id or the global OpenStack application credential ID.")
		}
		r.requireSecret("secrets.mimir.swift_application_credential_secret", credentialSecret, "Mimir Swift blocks storage requires an application credential secret.")
	case "s3":
		mimir, _ := configuredService(cfg, "mimir").(*services.MimirConfig)
		if mimir == nil {
			r.addError(CategoryServices, "opencenter.services.mimir", fmt.Sprintf("mimir has unexpected configuration type %T.", configuredService(cfg, "mimir")), "Use the canonical Mimir service configuration.")
			return
		}
		r.requireMimirS3Endpoint("opencenter.services.mimir.s3_endpoint", mimir.S3Endpoint)
		accessKey, secretKey := cfg.GetMimirS3Credentials()
		r.requireSecret("secrets.mimir.s3_access_key_id", accessKey, "Mimir S3 storage requires an access key ID.")
		r.requireSecret("secrets.mimir.s3_secret_access_key", secretKey, "Mimir S3 storage requires a secret access key.")
	}
}

func (r *readinessBuilder) validateS3Buckets(cfg *Config) {
	for _, issue := range s3BucketIssues(cfg) {
		r.addError(CategoryServices, issue.path, issue.message, "Use a unique S3 bucket name that follows the S3 naming rules.")
	}
}

func (r *readinessBuilder) validateHarborSecrets(cfg *Config) {
	if !isServiceEnabled(cfg, "harbor") {
		return
	}
	harbor, _ := configuredService(cfg, "harbor").(*services.HarborConfig)
	if !storageBackendDoesNotUseObjectStorage(cfg, "harbor") && !UsesManagedObjectStorage(cfg) {
		if harbor != nil {
			r.requireS3Endpoint("opencenter.services.harbor.s3_endpoint", harbor.S3Endpoint, "Harbor S3 storage requires a configured endpoint.")
		}
		r.requireSecret("secrets.harbor.s3_access_key_id", cfg.GetHarborS3AccessKey(), "Harbor S3 access key is required when Harbor is enabled.")
		r.requireSecret("secrets.harbor.s3_secret_access_key", cfg.GetHarborS3SecretKey(), "Harbor S3 secret access key is required when Harbor is enabled.")
	}
	r.requireSecret("secrets.harbor.admin_password", cfg.Secrets.Harbor.AdminPassword, "Harbor admin password is required when Harbor is enabled.")
	r.requireSecret("secrets.harbor.registry_password", cfg.Secrets.Harbor.RegistryPassword, "Harbor registry password is required when Harbor is enabled.")
	r.requireSecret("secrets.harbor.database_password", cfg.Secrets.Harbor.DatabasePassword, "Harbor database password is required when Harbor is enabled.")
}

func (r *readinessBuilder) requireS3Endpoint(path, value, message string) {
	if err := ValidateS3Endpoint(value); err != nil {
		r.addError(CategoryServices, path, message, "Set a provider-specific absolute HTTP(S) S3 endpoint.")
	}
}

func (r *readinessBuilder) requireMimirS3Endpoint(path, value string) {
	if err := ValidateMimirS3Endpoint(value); err != nil {
		r.addError(CategoryServices, path, "Mimir S3 storage requires an absolute HTTP(S) root endpoint without a path.", "Set the S3 endpoint URL to its root (for example https://s3.example.com); Mimir uses bucket_lookup_type: path for path-style addressing.")
	}
}

func (r *readinessBuilder) requireSecret(path, value, message string) {
	r.requireNonPlaceholder(CategoryServices, path, value, message, "Set a non-placeholder secret value.")
}

func (r *readinessBuilder) requireNonPlaceholder(category ValidationCategory, path, value, message, suggestion string) {
	if isMissingSecret(value) {
		r.addError(category, path, message, suggestion)
	}
}

func serviceEnabled(cfg *Config, serviceName string) bool {
	return serviceEnabledInMap(cfg.OpenCenter.Services, serviceName) || serviceEnabledInMap(cfg.OpenCenter.ManagedServices, serviceName)
}

func serviceEnabledInMap(servicesMap ServiceMap, serviceName string) bool {
	if svc, ok := servicesMap[serviceName]; ok {
		if enabler, ok := svc.(interface{ IsEnabled() bool }); ok {
			return enabler.IsEnabled()
		}
	}
	return false
}

func headlampUsesOIDC(cfg *Config) bool {
	if cfg.OpenCenter.Identity.OIDC.Enabled {
		return true
	}
	if cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled {
		return true
	}
	headlamp, _ := cfg.OpenCenter.Services["headlamp"].(*services.HeadlampConfig)
	if headlamp == nil {
		return false
	}
	return strings.TrimSpace(headlamp.OIDCIssuerURL) != "" || strings.TrimSpace(headlamp.OIDCClientID) != "" || serviceEnabled(cfg, "keycloak")
}

func oidcClientSecretsProvidedInternally(cfg *Config) bool {
	oidc := cfg.OpenCenter.Identity.OIDC
	if !oidc.Enabled {
		return false
	}
	source := strings.ToLower(strings.TrimSpace(oidc.Source))
	if source == "" {
		source = OIDCSourceInternal
	}
	provider := strings.ToLower(strings.TrimSpace(oidc.Provider))
	if provider == "" {
		provider = OIDCProviderKeycloak
	}
	return source == OIDCSourceInternal && provider == OIDCProviderKeycloak && serviceEnabled(cfg, "keycloak")
}

func vsphereCSIEnabled(cfg *Config) bool {
	if serviceEnabled(cfg, "vsphere-csi") {
		return true
	}
	plugin := cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi
	return plugin != nil && plugin.Enabled
}

func isMissingSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, PlaceholderSecret)
}

func valueSet(value string) bool {
	return !isMissingSecret(value)
}
