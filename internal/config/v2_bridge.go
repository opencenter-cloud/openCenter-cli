package config

import (
	"fmt"
	"strings"
	"time"

	registrydefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

func isNativeV2ConfigData(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}

	if lookupNestedMap(raw, "opencenter", "infrastructure", "ssh") != nil {
		return true
	}
	if lookupNestedMap(raw, "opencenter", "infrastructure", "compute") != nil {
		return true
	}

	return false
}

func convertNativeV2ToLegacyConfig(cfg *v2.Config) (Config, error) {
	if cfg == nil {
		return Config{}, fmt.Errorf("v2 config cannot be nil")
	}

	clusterName := strings.TrimSpace(cfg.OpenCenter.Cluster.ClusterName)
	if clusterName == "" {
		clusterName = strings.TrimSpace(cfg.OpenCenter.Meta.Name)
	}
	if clusterName == "" {
		return Config{}, fmt.Errorf("cluster name cannot be empty")
	}

	provider := canonicalLegacyProvider(cfg.OpenCenter.Infrastructure.Provider)
	legacy, err := NewProviderDefault(clusterName, provider)
	if err != nil {
		legacy = NewDefault(clusterName)
		legacy.OpenCenter.Infrastructure.Provider = provider
	}

	legacy.SchemaVersion = cfg.SchemaVersion
	overlayLegacyMetadata(&legacy.Metadata, cfg.Metadata)
	overlayLegacyMeta(&legacy, cfg)
	overlayLegacyInfrastructure(&legacy, cfg)
	overlayLegacyCluster(&legacy, cfg)
	overlayLegacyGitOps(&legacy, cfg)
	overlayLegacyOpenTofu(&legacy, cfg)
	overlayLegacySecrets(&legacy, cfg)
	overlayLegacyServices(&legacy, cfg)
	overlayLegacyDeployment(&legacy, cfg)

	return legacy, nil
}

func overlayLegacyMetadata(dst *ConfigMetadata, src v2.ConfigMetadata) {
	if dst == nil {
		return
	}
	if ts, ok := parseTimestamp(src.CreatedAt); ok {
		dst.CreatedAt = ts
	}
	if ts, ok := parseTimestamp(src.UpdatedAt); ok {
		dst.UpdatedAt = ts
	}
	dst.CreatedBy = src.CreatedBy
	dst.Tags = cloneStringMap(src.Labels)
	dst.Annotations = cloneStringMap(src.Annotations)
}

func overlayLegacyMeta(dst *Config, src *v2.Config) {
	dst.OpenCenter.Meta.Name = src.OpenCenter.Meta.Name
	dst.OpenCenter.Meta.Organization = src.OpenCenter.Meta.Organization
	dst.OpenCenter.Meta.Env = src.OpenCenter.Meta.Env
	dst.OpenCenter.Meta.Region = src.OpenCenter.Meta.Region
	dst.OpenCenter.Meta.Stage = src.OpenCenter.Meta.Stage
	dst.OpenCenter.Meta.Status = src.OpenCenter.Meta.Status
	dst.OpenCenter.Cluster.ClusterName = firstNonEmpty(src.OpenCenter.Cluster.ClusterName, src.OpenCenter.Meta.Name)
}

func overlayLegacyInfrastructure(dst *Config, src *v2.Config) {
	dst.OpenCenter.Infrastructure.Provider = canonicalLegacyProvider(src.OpenCenter.Infrastructure.Provider)
	dst.OpenCenter.Infrastructure.SSHUser = firstNonEmpty(src.OpenCenter.Infrastructure.SSH.Username, src.OpenCenter.Infrastructure.SSH.User, dst.OpenCenter.Infrastructure.SSHUser)
	dst.OpenCenter.Infrastructure.SSHKeyPath = src.OpenCenter.Infrastructure.SSH.KeyPath
	dst.OpenCenter.Infrastructure.OSVersion = src.OpenCenter.Infrastructure.OSVersion
	dst.OpenCenter.Infrastructure.ServerGroupAffinity = append([]string(nil), src.OpenCenter.Infrastructure.ServerGroupAffinity...)
	dst.OpenCenter.Infrastructure.K8sAPIIP = src.OpenCenter.Infrastructure.K8sAPIIP

	if src.OpenCenter.Infrastructure.Networking.SubnetNodes != "" {
		dst.OpenCenter.Cluster.Networking.SubnetNodes = src.OpenCenter.Infrastructure.Networking.SubnetNodes
	}
	if src.OpenCenter.Infrastructure.Networking.AllocationPoolStart != "" {
		dst.OpenCenter.Cluster.Networking.AllocationPoolStart = src.OpenCenter.Infrastructure.Networking.AllocationPoolStart
	}
	if src.OpenCenter.Infrastructure.Networking.AllocationPoolEnd != "" {
		dst.OpenCenter.Cluster.Networking.AllocationPoolEnd = src.OpenCenter.Infrastructure.Networking.AllocationPoolEnd
	}
	dst.OpenCenter.Cluster.Networking.VRRPEnabled = src.OpenCenter.Infrastructure.Networking.VRRPEnabled
	dst.OpenCenter.Cluster.Networking.VRRPIP = src.OpenCenter.Infrastructure.Networking.VRRPIP
	dst.OpenCenter.Cluster.Networking.UseOctavia = src.OpenCenter.Infrastructure.Networking.UseOctavia
	dst.OpenCenter.Cluster.Networking.LoadbalancerProvider = src.OpenCenter.Infrastructure.Networking.LoadbalancerProvider
	dst.OpenCenter.Cluster.Networking.UseDesignate = src.OpenCenter.Infrastructure.Networking.UseDesignate
	dst.OpenCenter.Cluster.Networking.DNSZoneName = src.OpenCenter.Infrastructure.Networking.DNSZoneName
	dst.OpenCenter.Cluster.Networking.DNSNameservers = append([]string(nil), src.OpenCenter.Infrastructure.Networking.DNSNameservers...)
	dst.OpenCenter.Cluster.Networking.NTPServers = append([]string(nil), src.OpenCenter.Infrastructure.Networking.NTPServers...)

	dst.OpenCenter.Cluster.Kubernetes.FlavorBastion = src.OpenCenter.Infrastructure.Compute.FlavorBastion
	dst.OpenCenter.Cluster.Kubernetes.FlavorMaster = src.OpenCenter.Infrastructure.Compute.FlavorMaster
	dst.OpenCenter.Cluster.Kubernetes.FlavorWorker = src.OpenCenter.Infrastructure.Compute.FlavorWorker
	dst.OpenCenter.Cluster.Kubernetes.FlavorWorkerWindows = src.OpenCenter.Infrastructure.Compute.FlavorWorkerWindows
	dst.OpenCenter.Cluster.Kubernetes.MasterCount = src.OpenCenter.Infrastructure.Compute.MasterCount
	dst.OpenCenter.Cluster.Kubernetes.WorkerCount = src.OpenCenter.Infrastructure.Compute.WorkerCount
	dst.OpenCenter.Cluster.Kubernetes.WorkerCountWindows = src.OpenCenter.Infrastructure.Compute.WorkerCountWindows

	dst.OpenCenter.Storage.DefaultStorageClass = src.OpenCenter.Infrastructure.Storage.DefaultStorageClass
	dst.OpenCenter.Storage.WorkerVolumeSize = src.OpenCenter.Infrastructure.Storage.WorkerVolumeSize
	dst.OpenCenter.Storage.WorkerVolumeDestinationType = src.OpenCenter.Infrastructure.Storage.WorkerVolumeDestinationType
	dst.OpenCenter.Storage.WorkerVolumeSourceType = src.OpenCenter.Infrastructure.Storage.WorkerVolumeSourceType
	dst.OpenCenter.Storage.WorkerVolumeType = src.OpenCenter.Infrastructure.Storage.WorkerVolumeType

	switch canonicalLegacyProvider(src.OpenCenter.Infrastructure.Provider) {
	case "kind":
		if dst.OpenCenter.Infrastructure.Kind == nil {
			_ = ApplyProviderDefaults(dst, "kind")
		}
		if dst.OpenCenter.Infrastructure.Kind != nil {
			dst.OpenCenter.Infrastructure.Kind.ClusterNameOverride = dst.ClusterName()
			dst.OpenCenter.Infrastructure.Kind.KubernetesVersion = src.OpenCenter.Cluster.Kubernetes.Version
			dst.OpenCenter.Infrastructure.Kind.ControlPlaneCount = src.OpenCenter.Infrastructure.Compute.MasterCount
			dst.OpenCenter.Infrastructure.Kind.WorkerCount = src.OpenCenter.Infrastructure.Compute.WorkerCount
			dst.OpenCenter.Infrastructure.Kind.APIServerPort = src.OpenCenter.Cluster.Kubernetes.APIPort
			dst.OpenCenter.Infrastructure.Kind.PodSubnet = src.OpenCenter.Cluster.Kubernetes.SubnetPods
			dst.OpenCenter.Infrastructure.Kind.ServiceSubnet = src.OpenCenter.Cluster.Kubernetes.SubnetServices
		}
	default:
		dst.OpenCenter.Infrastructure.Kind = nil
	}

	if src.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		openstack := src.OpenCenter.Infrastructure.Cloud.OpenStack
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = openstack.AuthURL
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Insecure = openstack.Insecure
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Region = openstack.Region
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = firstNonEmpty(openstack.Domain, openstack.DomainName)
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = firstNonEmpty(openstack.TenantName, openstack.ProjectName)
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.AvailabilityZone = openstack.AvailabilityZone
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectDomainName = openstack.ProjectDomainName
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.UserDomainName = openstack.UserDomainName
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.CA = openstack.CA
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID = openstack.ImageID
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.ImageIDWindows = openstack.ImageIDWindows
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingIPPool = firstNonEmpty(openstack.FloatingIPPool, dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingIPPool)
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingNetworkId = openstack.FloatingNetworkID
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.NetworkID = openstack.NetworkID
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.RouterExternalNetworkID = openstack.RouterExternalNetworkID
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.SubnetId = openstack.SubnetID
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL = append([]string(nil), openstack.Networking.K8sAPIPortACL...)
		dst.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.Designate.DNSZoneName = firstNonEmpty(openstack.DNSZoneName, openstack.Networking.Designate.DNSZoneName)
	}

	if src.OpenCenter.Infrastructure.Cloud.VMware != nil {
		vmware := src.OpenCenter.Infrastructure.Cloud.VMware
		dst.OpenCenter.Infrastructure.Cloud.VMware.VCenterServer = vmware.VCenterServer
		dst.OpenCenter.Infrastructure.Cloud.VMware.Datacenter = vmware.Datacenter
		dst.OpenCenter.Infrastructure.Cloud.VMware.Cluster = vmware.Cluster
		dst.OpenCenter.Infrastructure.Cloud.VMware.Datastore = vmware.Datastore
		dst.OpenCenter.Infrastructure.Cloud.VMware.Network = vmware.Network
		dst.OpenCenter.Infrastructure.Cloud.VMware.Folder = vmware.Folder
	}
}

func overlayLegacyCluster(dst *Config, src *v2.Config) {
	dst.OpenCenter.Cluster.BaseDomain = src.OpenCenter.Cluster.BaseDomain
	dst.OpenCenter.Cluster.ClusterFQDN = src.OpenCenter.Cluster.ClusterFQDN
	dst.OpenCenter.Cluster.AdminEmail = src.OpenCenter.Cluster.AdminEmail
	dst.OpenCenter.Cluster.SSHAuthorizedKeys = append([]string(nil), src.OpenCenter.Infrastructure.SSH.AuthorizedKeys...)

	dst.OpenCenter.Cluster.Kubernetes.Version = src.OpenCenter.Cluster.Kubernetes.Version
	dst.OpenCenter.Cluster.Kubernetes.APIPort = src.OpenCenter.Cluster.Kubernetes.APIPort
	dst.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = src.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled
	dst.OpenCenter.Cluster.Kubernetes.SubnetPods = src.OpenCenter.Cluster.Kubernetes.SubnetPods
	dst.OpenCenter.Cluster.Kubernetes.SubnetServices = src.OpenCenter.Cluster.Kubernetes.SubnetServices
	dst.OpenCenter.Cluster.Kubernetes.LoadbalancerProvider = src.OpenCenter.Infrastructure.Networking.LoadbalancerProvider
	dst.OpenCenter.Cluster.Kubernetes.DNSZoneName = src.OpenCenter.Infrastructure.Networking.DNSZoneName

	if src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico != nil {
		dst.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled
	}
	if src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium != nil {
		dst.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled
		dst.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.OperatorEnabled = true
		dst.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.KubeProxyReplacement = strings.EqualFold(src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.TunnelMode, "disabled")
	}
	if src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN != nil {
		dst.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = src.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled
	}

	if src.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi != nil {
		dst.OpenCenter.Cluster.Kubernetes.StoragePlugin.Cinder.Enabled = src.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi.Enabled
	}
	if src.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi != nil {
		dst.OpenCenter.Cluster.Kubernetes.StoragePlugin.Vsphere.Enabled = src.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi.Enabled
	}

	dst.OpenCenter.Cluster.Kubernetes.OIDC.Enabled = src.OpenCenter.Cluster.Kubernetes.OIDC.Enabled
	dst.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL = src.OpenCenter.Cluster.Kubernetes.OIDC.IssuerURL
	dst.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID = src.OpenCenter.Cluster.Kubernetes.OIDC.ClientID
	dst.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernameClaim = src.OpenCenter.Cluster.Kubernetes.OIDC.UsernameClaim
	dst.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsClaim = src.OpenCenter.Cluster.Kubernetes.OIDC.GroupsClaim

	dst.OpenCenter.Cluster.Kubernetes.Security.K8sHardening = src.OpenCenter.Cluster.Kubernetes.Security.PodSecurityStandards != ""
	dst.OpenCenter.Cluster.Kubernetes.Security.PodSecurityExemptions = append([]string(nil), src.OpenCenter.Cluster.Kubernetes.Security.AdmissionControllers...)
}

func overlayLegacyGitOps(dst *Config, src *v2.Config) {
	dst.OpenCenter.GitOps.GitDir = src.OpenCenter.GitOps.GitDir
	dst.OpenCenter.GitOps.GitURL = src.OpenCenter.GitOps.GitURL
	dst.OpenCenter.GitOps.GitSSHKey = src.OpenCenter.GitOps.GitSSHKey
	dst.OpenCenter.GitOps.GitSSHPub = src.OpenCenter.GitOps.GitSSHPub
	dst.OpenCenter.GitOps.GitBranch = firstNonEmpty(src.OpenCenter.GitOps.GitBranch, src.OpenCenter.GitOps.Branch)
	dst.OpenCenter.GitOps.Release = src.OpenCenter.GitOps.Release
	dst.OpenCenter.GitOps.Branch = src.OpenCenter.GitOps.Branch
	dst.OpenCenter.GitOps.Uri = firstNonEmpty(src.OpenCenter.GitOps.URI, src.OpenCenter.GitOps.BaseRepoURL)
	dst.OpenCenter.GitOps.GitOpsBaseRepo = firstNonEmpty(src.OpenCenter.GitOps.GitOpsBaseRepo, src.OpenCenter.GitOps.BaseRepoURL)
	dst.OpenCenter.GitOps.GitOpsBaseRelease = firstNonEmpty(src.OpenCenter.GitOps.GitOpsBaseRelease, src.OpenCenter.GitOps.BaseRepoRelease)
	dst.OpenCenter.GitOps.GitOpsBranch = src.OpenCenter.GitOps.GitOpsBranch
	dst.OpenCenter.GitOps.Flux.Interval = firstNonEmpty(src.OpenCenter.GitOps.Flux.Interval, src.OpenCenter.GitOps.FluxInterval)
	dst.OpenCenter.GitOps.Flux.Prune = src.OpenCenter.GitOps.Flux.Prune || src.OpenCenter.GitOps.FluxPrune
	dst.OpenCenter.GitOps.OverlayUnits = src.OpenCenter.GitOps.OverlayUnits
}

func overlayLegacyOpenTofu(dst *Config, src *v2.Config) {
	dst.OpenTofu.Enabled = src.OpenTofu.Enabled
	dst.OpenTofu.Path = src.OpenTofu.Path
	dst.OpenTofu.Backend.Type = src.OpenTofu.Backend.Type
	if src.OpenTofu.Backend.Local != nil {
		dst.OpenTofu.Backend.Local.Path = src.OpenTofu.Backend.Local.Path
	}
	if src.OpenTofu.Backend.S3 != nil {
		dst.OpenTofu.Backend.S3.Bucket = src.OpenTofu.Backend.S3.Bucket
		dst.OpenTofu.Backend.S3.Key = src.OpenTofu.Backend.S3.Key
		dst.OpenTofu.Backend.S3.Region = src.OpenTofu.Backend.S3.Region
	}
}

func overlayLegacySecrets(dst *Config, src *v2.Config) {
	dst.Secrets.SopsAgeKeyFile = firstNonEmpty(src.Secrets.SopsAgeKeyFile, src.Secrets.SOPSConfig.AgeKeyFile)
	dst.Secrets.SSHKey.Private = src.Secrets.SSHKey.Private
	dst.Secrets.SSHKey.Public = src.Secrets.SSHKey.Public
	dst.Secrets.SSHKey.Cypher = src.Secrets.SSHKey.Cypher
	dst.Secrets.Global.AWS.Infrastructure.AccessKey = src.Secrets.Global.AWS.Infrastructure.AccessKey
	dst.Secrets.Global.AWS.Infrastructure.SecretAccessKey = src.Secrets.Global.AWS.Infrastructure.SecretAccessKey
	dst.Secrets.Global.AWS.Infrastructure.Region = src.Secrets.Global.AWS.Infrastructure.Region
	dst.Secrets.Global.AWS.Application.AccessKey = src.Secrets.Global.AWS.Application.AccessKey
	dst.Secrets.Global.AWS.Application.SecretAccessKey = src.Secrets.Global.AWS.Application.SecretAccessKey
	dst.Secrets.Global.AWS.Application.Region = src.Secrets.Global.AWS.Application.Region
	dst.OpenCenter.Secrets.Backend = src.OpenCenter.Secrets.Backend
	dst.OpenCenter.Secrets.Barbican.AuthURL = src.OpenCenter.Secrets.Barbican.AuthURL
	dst.OpenCenter.Secrets.Barbican.ProjectID = src.OpenCenter.Secrets.Barbican.ProjectID
	dst.OpenCenter.Secrets.Barbican.Region = src.OpenCenter.Secrets.Barbican.Region
	dst.OpenCenter.Secrets.Barbican.UserDomainName = src.OpenCenter.Secrets.Barbican.UserDomainName
	dst.OpenCenter.Secrets.Barbican.ProjectDomainName = src.OpenCenter.Secrets.Barbican.ProjectDomainName
	dst.OpenCenter.Secrets.Barbican.CACert = src.OpenCenter.Secrets.Barbican.CACert
	dst.Secrets.OverlayUnits = src.Secrets.OverlayUnits
}

func overlayLegacyServices(dst *Config, src *v2.Config) {
	if len(src.OpenCenter.Services) > 0 {
		var mapped ServiceMap
		if remapServiceMap(src.OpenCenter.Services, &mapped) == nil {
			dst.OpenCenter.Services = mapped
		}
	}

	if len(src.OpenCenter.ManagedServices) > 0 {
		var mapped ServiceMap
		if remapServiceMap(src.OpenCenter.ManagedServices, &mapped) == nil {
			dst.OpenCenter.ManagedService = mapped
		}
	}
}

func overlayLegacyDeployment(dst *Config, src *v2.Config) {
	dst.Deployment.AutoDeploy = src.Deployment.AutoDeploy
	dst.Deployment.Method = src.Deployment.Method
	if src.Deployment.Kubespray != nil && src.Deployment.Kubespray.Version != "" {
		dst.OpenCenter.Cluster.Kubernetes.KubesprayVersion = "v" + strings.TrimPrefix(src.Deployment.Kubespray.Version, "v")
	}
}

func remapServiceMap(src v2.ServiceMap, dst *ServiceMap) error {
	data, err := yaml.Marshal(src)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, dst)
}

func lookupNestedMap(data map[string]any, parts ...string) map[string]any {
	current := any(data)
	for _, part := range parts {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		value, ok := next[part]
		if !ok {
			return nil
		}
		current = value
	}
	result, ok := current.(map[string]any)
	if !ok {
		return nil
	}
	return result
}

func parseTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, true
		}
	}

	return time.Time{}, false
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func canonicalLegacyProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "vsphere":
		return "vmware"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func defaultLegacyV2Loader() *v2.ConfigLoader {
	return v2.NewConfigLoader(registrydefaults.NewRegistry())
}
