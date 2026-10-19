package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	provideropenstack "github.com/opencenter-cloud/opencenter-cli/internal/cluster/provider/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	utilerrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/spf13/cobra"
)

type providerOpenStackFlags struct {
	cloudName             string
	cloudsYAML            string
	imageName             string
	windowsImageName      string
	networkID             string
	externalNetworkID     string
	subnetID              string
	availabilityZone      string
	createInternalNetwork bool
	replace               bool
	importAuth            bool
	importTLS             bool
}

func newClusterProviderOpenStackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openstack",
		Short: "Manage OpenStack provider configuration",
		Long: `Bind an existing cluster (infrastructure.provider: openstack) to a specific
OpenStack cloud. Reads a clouds.yaml profile (--os-cloud), discovers images,
networks, subnets, and availability zones read-only, then plans or applies the
cluster's OpenStack config values from them: image IDs, internal/external
network and subnet IDs, availability zone, and optionally application credentials and TLS settings.

This command does not deploy the cluster or create remote resources; it only
writes the cluster's own configuration file. Use 'cluster deploy' for that.
With --create-internal-network it switches internal networking to
OpenTofu-managed mode instead of selecting an existing network.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newClusterProviderOpenStackOperationCmd("plan", false))
	cmd.AddCommand(newClusterProviderOpenStackOperationCmd("apply", true))
	return cmd
}

func newClusterProviderOpenStackOperationCmd(operation string, mutating bool) *cobra.Command {
	var flags providerOpenStackFlags
	var shortText, longText string
	if mutating {
		shortText = "apply OpenStack provider metadata and selections"
		longText = `Apply the planned OpenStack selections to the cluster's configuration file.

Reads the profile named by --os-cloud from clouds.yaml (override its location with
--clouds-yaml or OS_CLIENT_CONFIG_FILE), discovers live images, networks, subnets, and
availability zones read-only, computes the changes, then writes them to the cluster config after an
interactive confirmation prompt in text mode (use --yes to skip it). No remote resources are created
or modified; use 'cluster deploy' for that.

Selector flags (--image-name, --windows-image-name, --network-id, --subnet-id,
--external-network-id, --availability-zone) accept a resource name or ID; a name
that matches several discovered resources is rejected so nothing is picked
silently. With no --image-name, the name Ubuntu 24.04 is resolved against the
project inventory and reported as a warning; if the project does not have it, the
candidate list is printed instead. --windows-image-name is only required when the
cluster runs Windows workers.

Plan first with 'plan' and pass identical flags so the same change set is applied.`
	} else {
		shortText = "plan OpenStack provider metadata and selections"
		longText = `Preview the changes that would bind this cluster to the given OpenStack cloud.

Reads the profile named by --os-cloud from clouds.yaml (override its location with
--clouds-yaml or OS_CLIENT_CONFIG_FILE), discovers live images, networks, subnets, and
availability zones read-only, and prints the changes without writing anything: image IDs,
internal/external network and subnet IDs, availability zone, plus application credential and TLS
settings when requested. No remote resources are created or modified.

Selector flags (--image-name, --windows-image-name, --network-id, --subnet-id,
--external-network-id, --availability-zone) accept a resource name or ID; a name
that matches several discovered resources is rejected so nothing is picked
silently. With no --image-name, the name Ubuntu 24.04 is resolved against the
project inventory and reported as a warning; if the project does not have it, the
candidate list is printed instead. --windows-image-name is only required when the
cluster runs Windows workers.

Use 'apply' with identical flags to persist the same change set.`
	}
	cmd := &cobra.Command{
		Use:   operation + " <cluster>",
		Short: shortText,
		Long:  longText,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClusterProviderOpenStack(cmd, operation, mutating, args[0], flags)
		},
	}
	cmd.Flags().StringVar(&flags.cloudName, "os-cloud", "", "profile name inside clouds.yaml to use for discovery")
	cmd.Flags().StringVar(&flags.cloudsYAML, "clouds-yaml", "", "path to clouds.yaml")
	cmd.Flags().StringVar(&flags.imageName, "image-name", "", "Linux image name or ID")
	cmd.Flags().StringVar(&flags.windowsImageName, "windows-image-name", "", "Windows image name or ID")
	cmd.Flags().StringVar(&flags.networkID, "network-id", "", "internal network name or ID")
	cmd.Flags().StringVar(&flags.externalNetworkID, "external-network-id", "", "external network name or ID")
	cmd.Flags().StringVar(&flags.subnetID, "subnet-id", "", "internal subnet name or ID")
	cmd.Flags().StringVar(&flags.availabilityZone, "availability-zone", "", "availability zone name or ID")
	cmd.Flags().BoolVar(&flags.createInternalNetwork, "create-internal-network", false, "let OpenTofu create and manage the internal network and subnet")
	cmd.Flags().BoolVar(&flags.replace, "replace", false, "allow replacing populated provider selections")
	cmd.Flags().BoolVar(&flags.importAuth, "import-auth", false, "write application credential fields from the selected profile")
	cmd.Flags().BoolVar(&flags.importTLS, "import-tls", false, "persist selected profile TLS settings")
	return cmd
}

func runClusterProviderOpenStack(cmd *cobra.Command, operation string, mutating bool, identifier string, flags providerOpenStackFlags) error {
	if strings.TrimSpace(flags.cloudName) == "" {
		return NewExitError(2, "--os-cloud is required", nil)
	}
	clusterPaths, err := resolveProviderClusterPaths(cmd.Context(), identifier)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid") {
			return NewExitError(2, "invalid cluster identifier", err)
		}
		return v2.NewConfigNotFoundError(identifier, fmt.Errorf("resolve cluster configuration: %w", err))
	}
	rawData, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return v2.NewConfigNotFoundError(identifier, err)
		}
		return fmt.Errorf("read cluster configuration: %w", err)
	}
	cfg, err := v2.DecodePublicConfig(rawData)
	if err != nil {
		return NewExitError(2, "decode cluster configuration", err)
	}
	if strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) != "openstack" {
		return NewExitError(2, fmt.Sprintf("cluster provider is %q; OpenStack provider operation requires provider openstack", cfg.OpenCenter.Infrastructure.Provider), nil)
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		return NewExitError(2, "cluster OpenStack provider configuration is missing", nil)
	}
	if flags.createInternalNetwork {
		if strings.TrimSpace(flags.networkID) != "" || strings.TrimSpace(flags.subnetID) != "" {
			return NewExitError(2, "--create-internal-network cannot be combined with --network-id or --subnet-id", nil)
		}
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking != nil && strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.VLAN.ID) != "" {
			return NewExitError(2, "--create-internal-network cannot be used when networking.vlan.id is set", nil)
		}
	}
	cloudsPath := strings.TrimSpace(flags.cloudsYAML)
	if cloudsPath == "" {
		cloudsPath = cloudopenstack.DefaultCloudsYAMLPath()
	}
	profile, err := cloudopenstack.LoadProfile(cloudsPath, flags.cloudName)
	if err != nil {
		return err
	}
	if flags.importAuth && (strings.TrimSpace(profile.AppCredID) == "" || strings.TrimSpace(profile.AppCredSecret) == "") {
		return NewExitError(2, "--import-auth requires an application credential ID and secret; name-only application credentials cannot be persisted", nil)
	}
	snapshot, err := cloudopenstack.NewProfileDiscovery(profile).DiscoverWithOptions(cmd.Context(), cloudopenstack.DiscoveryOptions{SkipInternalNetworkAndSubnet: flags.createInternalNetwork})
	if err != nil {
		return fmt.Errorf("discover OpenStack provider inventory: %w", err)
	}
	var authImport *provideropenstack.AuthImport
	if flags.importAuth {
		authImport = &provideropenstack.AuthImport{ApplicationCredentialID: profile.AppCredID, ApplicationCredentialSecret: profile.AppCredSecret}
	}
	result, prospective, err := provideropenstack.Plan(cmd.Context(), cfg, snapshot, provideropenstack.Options{
		ImageName: flags.imageName, WindowsImageName: flags.windowsImageName, NetworkID: flags.networkID,
		ExternalNetworkID: flags.externalNetworkID, SubnetID: flags.subnetID, AvailabilityZone: flags.availabilityZone,
		CreateInternalNetwork: flags.createInternalNetwork, Replace: flags.replace, ImportAuth: authImport, ImportTLS: flags.importTLS,
	})
	if err != nil {
		return NewExitError(2, "invalid OpenStack provider selection", err)
	}
	result.Operation = "cluster.provider.openstack." + operation
	result.CloudProfile = flags.cloudName
	result.Warnings = append(result.Warnings, snapshot.Warnings...)
	result.Warnings = uniqueSortedStrings(result.Warnings)

	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	ioHandler := v2.NewConfigIOHandler(fileSystem)
	validate := func(ctx context.Context, candidate *v2.Config) error { return ioHandler.ValidateConfig(ctx, candidate) }
	if result.Status != provideropenstack.StatusBlocked {
		if err := validate(cmd.Context(), prospective); err != nil {
			return NewExitError(2, "prospective configuration validation failed", err)
		}
	}
	if result.Status == provideropenstack.StatusBlocked {
		if err := writeProviderOpenStackOutput(cmd, getGlobalOptions(cmd).Output, result); err != nil {
			return err
		}
		return NewExitError(2, "provider OpenStack plan is blocked", nil)
	}

	if mutating && result.Status == provideropenstack.StatusPlanned && len(result.Changes) > 0 && !getGlobalOptions(cmd).DryRun {
		if err := validateProviderApplyInteraction(cmd); err != nil {
			return err
		}
		renderProviderApplyReview(cmd, result)
		confirmed, err := confirmProviderApply(cmd)
		if err != nil {
			return err
		}
		if !confirmed {
			result.Status = provideropenstack.StatusDeclined
			result.Warnings = append(result.Warnings, "apply declined; configuration was not changed")
			return writeProviderOpenStackOutput(cmd, getGlobalOptions(cmd).Output, result)
		}
		result, err = (provideropenstack.ApplyPersistence{FileSystem: fileSystem, Validate: validate}).Apply(cmd.Context(), clusterPaths.ConfigPath, cfg, prospective, rawData, result)
		if err != nil {
			return err
		}
	}
	return writeProviderOpenStackOutput(cmd, getGlobalOptions(cmd).Output, result)
}

func validateProviderApplyInteraction(cmd *cobra.Command) error {
	if getGlobalOptions(cmd).Output != OutputText && !getGlobalOptions(cmd).Yes {
		return NewExitError(2, "structured apply requires --yes; refusing interactive confirmation", nil)
	}
	return nil
}

func renderProviderApplyReview(cmd *cobra.Command, result provideropenstack.Result) {
	stderr := cmd.ErrOrStderr()
	fmt.Fprintln(stderr, "Provider-only OpenStack apply review:")
	for _, change := range result.Changes {
		fmt.Fprintf(stderr, "  %s: %s -> %s\n", change.Path, change.Old, change.New)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "  warning: %s\n", warning)
	}
}

func confirmProviderApply(cmd *cobra.Command) (bool, error) {
	if getGlobalOptions(cmd).Yes {
		return true, nil
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Apply the provider-only OpenStack patch? [y/N] ")
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && len(answer) == 0 {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	return strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes"), nil
}

func writeProviderOpenStackOutput(cmd *cobra.Command, format OutputFormat, result provideropenstack.Result) error {
	if format != OutputText {
		return writeStructuredOutput(cmd, format, result)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "OpenStack provider %s for %s: %s\n", result.Operation, result.Cluster, result.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "Cloud profile: %s\n", result.CloudProfile)
	if result.InternalNetworkMode != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Internal network mode: %s\n", result.InternalNetworkMode)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Changes:")
	for _, change := range result.Changes {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s -> %s\n", change.Path, change.Old, change.New)
	}
	for _, selection := range result.Selections {
		fmt.Fprintf(cmd.OutOrStdout(), "Selection required: %s\n", selection.Field)
		for _, candidate := range selection.Candidates {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", candidate.Name, candidate.ID)
		}
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Remote actions: none")
	return nil
}

func resolveProviderClusterPaths(ctx context.Context, identifier string) (*paths.ClusterPaths, error) {
	resolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return nil, err
	}
	parts := strings.Split(identifier, "/")
	if len(parts) == 2 {
		return resolver.Resolve(ctx, parts[1], parts[0])
	}
	if len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
		return nil, fmt.Errorf("invalid cluster identifier %q", identifier)
	}
	return resolver.ResolveWithFallback(ctx, parts[0])
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok || strings.TrimSpace(value) == "" {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
