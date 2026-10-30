// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	kindDisableDefaultCNIFlagName = "kind-disable-default-cni"
	kindDisableDefaultCNIPath     = "opencenter.infrastructure.kind.disable_default_cni"
	baseDomainFlagName            = "base-domain"
	baseDomainConfigPath          = "opencenter.cluster.base_domain"
)

// initDomainPattern mirrors the validator's domain pattern
// (internal/core/validation/validators): labels of alphanumerics and hyphens,
// dots between them. Used to fail --base-domain before any cluster state is
// written, instead of surfacing a generic validate-stage error later.
var initDomainPattern = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// normalizeInitBaseDomain lowercases and trims a --base-domain value and
// rejects anything the fqdn validator would reject, so 'cluster init
// --base-domain' fails with no directories, keys, or config written.
func normalizeInitBaseDomain(value string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(value))
	if domain == "" {
		return "", fmt.Errorf("--base-domain cannot be empty")
	}
	if len(domain) > 253 {
		return "", fmt.Errorf("--base-domain is too long (maximum 253 characters): %s", domain)
	}
	if strings.Contains(domain, "..") {
		return "", fmt.Errorf("--base-domain cannot contain consecutive dots: %s", domain)
	}
	if !initDomainPattern.MatchString(domain) {
		return "", fmt.Errorf("invalid --base-domain %q: must contain only alphanumeric characters, hyphens, and dots (e.g. k8s.example.com)", domain)
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) > 63 {
			return "", fmt.Errorf("--base-domain label %q is too long (maximum 63 characters)", label)
		}
	}
	return domain, nil
}

func newClusterInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new cluster configuration (non-interactive)",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		Long: `Initialize a new cluster configuration with default values.

This command creates a new cluster configuration file with sensible defaults
based on the JSON schema. You can override any configuration value using
command-line flags with dot notation.

This command generates schema version "2.0" configuration only.

The configuration is created in an organization-based directory structure:
  ~/.config/opencenter/clusters/<organization>/<cluster>/

By default, the organization is set to "opencenter". Use --org to specify a different organization.

SOPS Age encryption keys and SSH key pairs are automatically generated if they
don't already exist, unless --no-keygen is specified.

With --os-cloud, initialization is followed immediately by the OpenStack
provider binding: the same discovery-and-apply pass as
'cluster provider openstack apply', run against the freshly created cluster
configuration. --os-cloud implies --type openstack. If the selected profile
has no application credential ID and secret, one is created through Keystone,
saved back into clouds.yaml, and written into the cluster configuration, so
the cluster is ready without a manual credential step. Internal networking
defaults to OpenTofu-managed (--create-internal-network is the default; pass
--create-internal-network=false to select an existing network). Provider
selector flags (--clouds-yaml, --image-name, --windows-image-name,
--network-id, --external-network-id, --subnet-id, --availability-zone,
--create-internal-network, --replace, --import-auth, --import-tls,
--create-application-credential) are only valid together with --os-cloud and
are forwarded to the provider apply pass.

--base-domain sets opencenter.cluster.base_domain (and the derived
cluster_fqdn) on the new cluster and tags it managed.base_domain: default, so
the cluster keeps following cluster_defaults.base_domain from the CLI settings
file: later 'opencenter settings set cluster_defaults.base_domain ...' changes
propagate through generate/validate/deploy at every config load. To freeze the
domain against settings changes later, use 'cluster set
<cluster> opencenter.cluster.base_domain=...'.`,
		Example: `  # Initialize with defaults
  opencenter cluster init my-cluster

  # Initialize from existing config file
  opencenter cluster init --config-file my-cluster-config.yaml

  # Initialize with organization
  opencenter cluster init my-cluster --org myorg

  # Initialize a VMware cluster
  opencenter cluster init my-cluster --org production --type vmware

  # Initialize with a specific region
  opencenter cluster init my-cluster --type openstack --region sjc3

  # Initialize a Kind cluster with the built-in CNI disabled
  opencenter cluster init my-cluster --org local --type kind --kind-disable-default-cni

  # Initialize without key generation
  opencenter cluster init my-cluster --no-keygen

  # Overwrite existing config
  opencenter cluster init my-cluster --force

  # Initialize and bind to an OpenStack cloud in one step (Tofu-managed
  # internal network and an auto-minted application credential included)
  opencenter cluster init my-cluster --org production --os-cloud prod-profile --yes

  # Initialize with an explicit base domain (still follows settings changes)
  opencenter cluster init my-cluster --base-domain k8s.example.com`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterInit,
	}

	cmd.Flags().String("org", "", "organization name (defaults to 'opencenter')")
	cmd.Flags().String("type", "openstack", "cluster type: openstack, baremetal, kind, vmware, magnum")
	cmd.Flags().String("region", "", "cluster region (e.g. dfw3, sjc3, iad3, ord1)")
	cmd.Flags().String(baseDomainFlagName, "", "base DNS domain for the cluster (sets opencenter.cluster.base_domain; the cluster keeps following cluster_defaults.base_domain from settings)")
	cmd.Flags().String("config-file", "", "load configuration from file")
	cmd.Flags().String("config", "", "load configuration from file")
	_ = cmd.Flags().MarkHidden("config")
	cmd.Flags().Bool("strict", false, "fail if required values are missing")
	cmd.Flags().Bool("force", false, "overwrite existing config file")
	cmd.Flags().Bool("no-keygen", false, "do not auto-generate keys")
	cmd.Flags().Bool("no-sops-keygen", false, "do not auto-generate SOPS keys")
	cmd.Flags().Bool("regenerate-keys", false, "regenerate keys even if they exist")
	cmd.Flags().Bool("full-schema", false, "generate configuration with all available fields")
	cmd.Flags().Bool(kindDisableDefaultCNIFlagName, false, "disable Kind's default CNI so cluster networking is managed by openCenter")
	cmd.Flags().StringArray("server-pool", []string{}, "additional server pool configuration")
	cmd.Flags().String("os-cloud", "", "after init, bind the cluster to this clouds.yaml profile (implies --type openstack)")
	registerProviderOpenStackFlags(cmd, &providerOpenStackFlags{createInternalNetwork: true})
	_ = cmd.Flags().MarkHidden("clouds-yaml")
	_ = cmd.Flags().MarkHidden("image-name")
	_ = cmd.Flags().MarkHidden("windows-image-name")
	_ = cmd.Flags().MarkHidden("network-id")
	_ = cmd.Flags().MarkHidden("external-network-id")
	_ = cmd.Flags().MarkHidden("subnet-id")
	_ = cmd.Flags().MarkHidden("availability-zone")
	_ = cmd.Flags().MarkHidden("create-internal-network")
	_ = cmd.Flags().MarkHidden("replace")
	_ = cmd.Flags().MarkHidden("import-auth")
	_ = cmd.Flags().MarkHidden("create-application-credential")
	_ = cmd.Flags().MarkHidden("import-tls")

	return cmd
}

func runClusterInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	app, err := di.NewApp(config.ResolveClustersDir())
	if err != nil {
		return fmt.Errorf("initialize application graph: %w", err)
	}
	initService := app.InitService

	// Parse command-line options
	opts, flags, err := parseInitOptions(cmd, args)
	if err != nil {
		return err
	}

	// Reject planned providers that are not yet available
	if err := checkProviderAvailability(opts.Provider); err != nil {
		return err
	}
	// Fail before writing anything if the requested cloud profile cannot be
	// used: a missing/unreadable clouds.yaml or an unknown profile must not
	// leave a half-created cluster behind. A profile without an application
	// credential is fine — the apply pass mints one.
	if flags != nil {
		if err := preflightInitProviderBinding(flags); err != nil {
			return err
		}
	}

	// Execute initialization. --dry-run is honoured by the service: it resolves
	// and validates everything, then writes no directories, no config, no keys,
	// no git repo, and mints no application credential.
	opts.DryRun = getGlobalOptions(cmd).DryRun
	result, err := initService.Initialize(ctx, opts)
	if err != nil {
		return err
	}

	// Display results
	fmt.Fprint(cmd.OutOrStdout(), result.Message)

	// Post-init OpenStack provider binding (--os-cloud). Under --dry-run this
	// runs the plan pass, not apply: apply would mint a real application
	// credential through Keystone, persist it into clouds.yaml, and patch the
	// cluster config - none of which a preview may do. The plan still performs
	// its read-only discovery, so the preview shows exactly the provider values
	// (auth_url, region, NTP, flavors, images, networks) the real run would set.
	if flags == nil {
		return nil
	}
	if opts.DryRun {
		return withInMemoryProviderConfig(result.Config, func() error {
			return runInitProviderOpenStackApply(cmd, "plan", false, opts.Organization+"/"+opts.ClusterName, *flags)
		})
	}
	return runInitProviderOpenStackApply(cmd, "apply", true, opts.Organization+"/"+opts.ClusterName, *flags)
}

// preflightInitProviderBinding validates everything runClusterProviderOpenStack
// checks before its first remote call, so 'cluster init --os-cloud' fails with
// no cluster files written when the profile is unusable: a missing/unreadable
// clouds.yaml or an unknown profile. A profile without a persistable
// application credential is NOT a preflight failure — the apply pass mints one
// through Keystone and persists it back to clouds.yaml — so the preflight
// deliberately performs no credential check and no credential write.
func preflightInitProviderBinding(flags *providerOpenStackFlags) error {
	cloudsPath := strings.TrimSpace(flags.cloudsYAML)
	if cloudsPath == "" {
		cloudsPath = cloudopenstack.DefaultCloudsYAMLPath()
	}
	if _, err := cloudopenstack.LoadProfile(cloudsPath, flags.cloudName); err != nil {
		return err
	}
	return nil
}

// runInitProviderOpenStackApply is the seam through which 'cluster init
// --os-cloud' invokes the provider apply pass; tests replace it to assert the
// binding without a live cloud.
var runInitProviderOpenStackApply = runClusterProviderOpenStack

// inMemoryProviderConfig carries the freshly built cluster configuration into
// the provider binding when the caller has it in memory and has not written it
// to disk - 'cluster init --os-cloud --dry-run'. It is set only for the duration
// of that call and cleared immediately after, so a normal provider operation
// always reads real state.
var inMemoryProviderConfig *v2.Config

// withInMemoryProviderConfig runs fn with cfg published to the provider
// binding, restoring the previous value afterwards.
func withInMemoryProviderConfig(cfg *v2.Config, fn func() error) error {
	previous := inMemoryProviderConfig
	inMemoryProviderConfig = cfg
	defer func() { inMemoryProviderConfig = previous }()
	return fn()
}

// setupContainer initializes the DI container with all required services
func setupContainer(container di.Container) error {
	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return err
	}
	if err := container.Singleton("path-resolver", func() (*paths.PathResolver, error) {
		return pathResolver, nil
	}); err != nil {
		return err
	}
	if err := container.Singleton("config-manager", di.ProvideConfigManager); err != nil {
		return err
	}
	if err := container.Singleton("validation-engine", di.ProvideValidationEngine); err != nil {
		return err
	}
	if err := container.Singleton("init-service", di.ProvideInitService); err != nil {
		return err
	}
	return container.Initialize()
}

// parseInitOptions parses command-line flags into InitOptions
func parseInitOptions(cmd *cobra.Command, args []string) (cluster.InitOptions, *providerOpenStackFlags, error) {
	opts := cluster.InitOptions{}
	var providerFlags *providerOpenStackFlags

	// Get cluster name from args, config file, or active cluster
	if len(args) > 0 {
		opts.ClusterName = args[0]
	} else if configFile := getInitConfigFileFlag(cmd); configFile != "" {
		name, err := extractClusterNameFromConfig(configFile)
		if err != nil {
			return opts, nil, err
		}
		opts.ClusterName = name
		opts.ConfigFile = configFile
	} else {
		activeName, err := getActiveCluster()
		if err != nil || activeName == "" {
			return opts, nil, fmt.Errorf("no cluster name provided and no active cluster set")
		}
		opts.ClusterName = activeName
	}

	// Parse flags
	opts.Organization, _ = cmd.Flags().GetString("org")
	opts.Provider, _ = cmd.Flags().GetString("type")
	opts.Provider = strings.ToLower(strings.TrimSpace(opts.Provider))
	opts.Region, _ = cmd.Flags().GetString("region")
	opts.Region = strings.TrimSpace(opts.Region)
	opts.RegionExplicit = cmd.Flags().Changed("region") && opts.Region != ""
	opts.DeploymentMethod = "kubespray"

	// Handle org/cluster identifier format: extract organization if embedded
	// in the cluster name and no explicit --org was provided
	if strings.Contains(opts.ClusterName, "/") {
		parts := strings.SplitN(opts.ClusterName, "/", 2)
		if opts.Organization == "" {
			opts.Organization = parts[0]
		}
		opts.ClusterName = parts[1]
	}
	opts.Force, _ = cmd.Flags().GetBool("force")
	opts.Strict, _ = cmd.Flags().GetBool("strict")
	opts.NoKeyGen, _ = cmd.Flags().GetBool("no-keygen")
	opts.NoSOPSKeyGen, _ = cmd.Flags().GetBool("no-sops-keygen")
	opts.RegenerateKeys, _ = cmd.Flags().GetBool("regenerate-keys")
	opts.FullSchema, _ = cmd.Flags().GetBool("full-schema")
	opts.ServerPools, _ = cmd.Flags().GetStringArray("server-pool")

	flagOverrides, deprecatedOrg, err := parseInitFlagOverrides(cmd)
	if err != nil {
		return opts, nil, err
	}
	opts.FlagOverrides = flagOverrides
	if opts.Organization == "" && deprecatedOrg != "" {
		opts.Organization = deprecatedOrg
	}

	// --base-domain: validate before anything is written, then funnel into the
	// same dotted-override path as --opencenter.cluster.base_domain=... so the
	// value lands in both the struct and the config map. The explicit flag is
	// the last word: it replaces a duplicate dotted override rather than
	// racing with it inside ProcessFlags (map iteration order).
	if cmd.Flags().Changed(baseDomainFlagName) {
		baseDomainRaw, _ := cmd.Flags().GetString(baseDomainFlagName)
		baseDomain, bdErr := normalizeInitBaseDomain(baseDomainRaw)
		if bdErr != nil {
			return opts, nil, bdErr
		}
		opts.BaseDomain = baseDomain
		deduped := make([]string, 0, len(opts.FlagOverrides)+1)
		prefix := "--" + baseDomainConfigPath + "="
		for _, override := range opts.FlagOverrides {
			if strings.HasPrefix(override, prefix) {
				continue
			}
			deduped = append(deduped, override)
		}
		opts.FlagOverrides = append(deduped, prefix+baseDomain)
	}

	if enabled, changed, err := kindDisableDefaultCNIValue(cmd, opts.Provider); err != nil {
		return opts, nil, err
	} else if changed {
		opts.KindDisableDefaultCNI = &enabled
	}

	providerFlags, err = parseInitProviderBindingFlags(cmd)
	if err != nil {
		return opts, nil, err
	}
	opts.SchemaVersion = "2.0"
	if providerFlags != nil {
		// --os-cloud implies an OpenStack cluster; an explicit conflicting
		// --type was rejected in parseInitProviderBindingFlags.
		opts.Provider = "openstack"
	}
	opts.OrganizationExplicit = opts.Organization != ""

	return opts, providerFlags, nil
}

func getInitConfigFileFlag(cmd *cobra.Command) string {
	if configFile, _ := cmd.Flags().GetString("config-file"); configFile != "" {
		return configFile
	}
	configFile, _ := cmd.Flags().GetString("config")
	return configFile
}

// providerOpenStackSelectorFlagNames are the hidden passthrough flags that
// only make sense with --os-cloud on 'cluster init'.
var providerOpenStackSelectorFlagNames = []string{
	"clouds-yaml", "image-name", "windows-image-name", "network-id",
	"external-network-id", "subnet-id", "availability-zone",
	"create-internal-network", "replace", "import-auth", "import-tls",
	"create-application-credential",
}

// parseInitProviderBindingFlags reads --os-cloud and the hidden provider
// selector flags into a providerOpenStackFlags struct, or returns nil when
// --os-cloud is absent. It rejects selector flags without --os-cloud and a
// conflicting explicit --type, so a typo cannot silently produce an unbound or
// wrong-provider cluster.
func parseInitProviderBindingFlags(cmd *cobra.Command) (*providerOpenStackFlags, error) {
	cloudName, err := cmd.Flags().GetString("os-cloud")
	if err != nil {
		return nil, err
	}
	cloudName = strings.TrimSpace(cloudName)
	if cloudName == "" {
		for _, name := range providerOpenStackSelectorFlagNames {
			if cmd.Flags().Changed(name) {
				return nil, fmt.Errorf("--%s is only valid together with --os-cloud", name)
			}
		}
		return nil, nil
	}
	if t, _ := cmd.Flags().GetString("type"); cmd.Flags().Changed("type") && !strings.EqualFold(strings.TrimSpace(t), "openstack") {
		return nil, fmt.Errorf("--os-cloud implies --type openstack; cannot combine with --type %s", strings.TrimSpace(t))
	}
	flags := &providerOpenStackFlags{cloudName: cloudName}
	get := func(name string) string { v, _ := cmd.Flags().GetString(name); return v }
	getBool := func(name string) bool { v, _ := cmd.Flags().GetBool(name); return v }
	flags.cloudsYAML = get("clouds-yaml")
	flags.imageName = get("image-name")
	flags.windowsImageName = get("windows-image-name")
	flags.networkID = get("network-id")
	flags.externalNetworkID = get("external-network-id")
	flags.subnetID = get("subnet-id")
	flags.availabilityZone = get("availability-zone")
	flags.createInternalNetwork = getBool("create-internal-network")
	flags.replace = getBool("replace")
	flags.importAuth = getBool("import-auth")
	flags.importTLS = getBool("import-tls")
	flags.createApplicationCredential = getBool("create-application-credential")
	return flags, nil
}

func kindDisableDefaultCNIValue(cmd *cobra.Command, provider string) (bool, bool, error) {
	if cmd == nil || !cmd.Flags().Changed(kindDisableDefaultCNIFlagName) {
		return false, false, nil
	}

	if !strings.EqualFold(strings.TrimSpace(provider), "kind") {
		if strings.TrimSpace(provider) == "" {
			return false, false, fmt.Errorf("--%s is only valid for kind clusters", kindDisableDefaultCNIFlagName)
		}
		return false, false, fmt.Errorf("--%s is only valid for kind clusters (provider: %s)", kindDisableDefaultCNIFlagName, provider)
	}

	enabled, err := cmd.Flags().GetBool(kindDisableDefaultCNIFlagName)
	if err != nil {
		return false, false, fmt.Errorf("read --%s: %w", kindDisableDefaultCNIFlagName, err)
	}

	return enabled, true, nil
}

func parseInitFlagOverrides(cmd *cobra.Command) ([]string, string, error) {
	rawArgs := rawCommandArgs(cmd)
	overrides := make([]string, 0)
	deprecatedOrganization := ""

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			continue
		}

		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")

		if flag := lookupCommandFlag(cmd, name); flag != nil {
			if !hasValue && flag.NoOptDefVal == "" && i+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[i+1], "-") {
				i++
			}
			continue
		}

		if !strings.Contains(name, ".") {
			return nil, "", fmt.Errorf("unknown flag: --%s", name)
		}

		if !hasValue {
			if i+1 >= len(rawArgs) || strings.HasPrefix(rawArgs[i+1], "-") {
				return nil, "", fmt.Errorf("flag needs an argument: --%s", name)
			}
			value = rawArgs[i+1]
			i++
		}

		if name == "opencenter.meta.organization" {
			deprecatedOrganization = value
			continue
		}

		overrides = append(overrides, "--"+name+"="+value)
	}

	return overrides, deprecatedOrganization, nil
}

func lookupCommandFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if flag := cmd.Flags().Lookup(name); flag != nil {
		return flag
	}
	if flag := cmd.InheritedFlags().Lookup(name); flag != nil {
		return flag
	}
	return nil
}

func rawCommandArgs(cmd *cobra.Command) []string {
	if cmd != nil {
		value := reflect.ValueOf(cmd)
		if value.IsValid() && value.Kind() == reflect.Ptr {
			argsField := value.Elem().FieldByName("args")
			if argsField.IsValid() && argsField.Kind() == reflect.Slice {
				args := make([]string, 0, argsField.Len())
				for i := 0; i < argsField.Len(); i++ {
					args = append(args, argsField.Index(i).String())
				}
				if len(args) > 0 {
					return args
				}
			}
		}
	}

	if len(os.Args) > 1 {
		return append([]string(nil), os.Args[1:]...)
	}

	return nil
}

// extractClusterNameFromConfig extracts the cluster name from a config file
func extractClusterNameFromConfig(configFile string) (string, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return "", fmt.Errorf("reading config file: %w", err)
	}

	var tempCfg struct {
		OpenCenter struct {
			Cluster struct {
				ClusterName string `yaml:"cluster_name"`
			} `yaml:"cluster"`
		} `yaml:"opencenter"`
	}

	if err := yaml.Unmarshal(data, &tempCfg); err != nil {
		return "", fmt.Errorf("parsing config file: %w", err)
	}

	if tempCfg.OpenCenter.Cluster.ClusterName == "" {
		return "", fmt.Errorf("cluster name not found in config file")
	}

	return tempCfg.OpenCenter.Cluster.ClusterName, nil
}
