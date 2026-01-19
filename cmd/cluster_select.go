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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/credentials"
	"github.com/spf13/cobra"
)

var (
	docStyle          = lipgloss.NewStyle().Margin(1, 2)
	titleStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
)

// ClusterMetadata represents cluster metadata for display in cluster select.
type ClusterMetadata struct {
	Name         string `yaml:"name"`
	Environment  string `yaml:"env"`
	Region       string `yaml:"region"`
	Status       string `yaml:"status"`
	Organization string `yaml:"organization"`
}

// ClusterSelectOutput represents the complete output for cluster select command.
type ClusterSelectOutput struct {
	Metadata       ClusterMetadata
	Paths          config.ClusterPaths
	ExportCommands []string
	GitOpsInfo     GitOpsInfo
	Shell          string
}

// GitOpsInfo represents GitOps repository information.
type GitOpsInfo struct {
	GitDir            string
	ApplicationsDir   string
	InfrastructureDir string
	SecretsDir        string
}

// item represents a single selectable entry in the interactive list.
// It implements the `list.Item` interface required by the `huh` library's list component.
type item struct {
	title        string
	description  string
	organization string
}

// Title returns the display text for the list item.
func (i item) Title() string { return i.title }

// Description provides additional details for the list item.
func (i item) Description() string { return i.description }

// FilterValue returns the string value used for filtering the list.
func (i item) FilterValue() string { return i.title }

// model encapsulates the state for the interactive cluster selection list.
// It holds the list component, the user's final choice, and a flag for quitting.
type model struct {
	list     list.Model
	choice   string
	quitting bool
}

// Init initializes the Bubble Tea model.
// It is part of the `tea.Model` interface and is called once at the start.
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the model's state.
// It processes key presses for navigation, selection, and quitting, as well as
// window resize events to ensure the list is rendered correctly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.title
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the UI for the current state of the model.
// It displays the interactive list unless the user has made a choice or is quitting.
func (m model) View() string {
	if m.choice != "" || m.quitting {
		return ""
	}
	return docStyle.Render(m.list.View())
}

// loadClusterMetadata loads cluster metadata from the configuration file.
// The clusterName parameter can be in "cluster" or "organization/cluster" format.
func loadClusterMetadata(clusterName string) (ClusterMetadata, error) {
	// Parse the cluster identifier to extract organization and cluster name
	organization, actualClusterName, err := config.ParseClusterIdentifier(clusterName)
	if err != nil {
		return ClusterMetadata{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	// Load cluster configuration (Load function now handles organization/cluster format)
	cfg, err := config.Load(clusterName)
	if err != nil {
		return ClusterMetadata{}, fmt.Errorf("failed to load cluster configuration: %w", err)
	}

	// Extract metadata from configuration
	metadata := ClusterMetadata{
		Name:         cfg.OpenCenter.Meta.Name,
		Environment:  cfg.OpenCenter.Meta.Env,
		Region:       cfg.OpenCenter.Meta.Region,
		Status:       cfg.OpenCenter.Meta.Status,
		Organization: cfg.OpenCenter.Meta.Organization,
	}

	// Use cluster name as fallback if not set in config
	if metadata.Name == "" {
		metadata.Name = actualClusterName
	}

	// Use organization from parsed identifier if not set in config
	if metadata.Organization == "" {
		metadata.Organization = organization
	}

	return metadata, nil
}

// generateClusterSelectOutput generates the complete output for cluster select command.
// The clusterName parameter can be in "cluster" or "organization/cluster" format.
// The shellOverride parameter allows overriding automatic shell detection (empty string for auto-detect).
func generateClusterSelectOutput(clusterName string, shellOverride string) (ClusterSelectOutput, error) {
	// Parse the cluster identifier to extract organization and cluster name
	organization, actualClusterName, err := config.ParseClusterIdentifier(clusterName)
	if err != nil {
		return ClusterSelectOutput{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	// Get CLI configuration manager
	configManager, err := config.NewConfigManager("")
	if err != nil {
		return ClusterSelectOutput{}, fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create path resolver
	pathResolver := config.NewPathResolver(configManager)

	// Validate that cluster exists first
	if err := validateClusterExists(clusterName, pathResolver); err != nil {
		return ClusterSelectOutput{}, err
	}

	// Load cluster metadata
	metadata, err := loadClusterMetadata(clusterName)
	if err != nil {
		return ClusterSelectOutput{}, err
	}

	// Use organization from metadata if available, otherwise use parsed organization
	if metadata.Organization == "" {
		metadata.Organization = organization
	}

	// Resolve cluster paths using organization from metadata
	paths := pathResolver.ResolveClusterPaths(actualClusterName, metadata.Organization)

	// Create GitOps info
	gitOpsInfo := GitOpsInfo{
		GitDir:            paths.GitOpsDir,
		ApplicationsDir:   filepath.Join(paths.GitOpsDir, "applications", "overlays", actualClusterName),
		InfrastructureDir: filepath.Join(paths.GitOpsDir, "infrastructure", "clusters", actualClusterName),
		SecretsDir:        paths.SecretsDir,
	}

	// Detect shell (or use override)
	shell := shellOverride
	if shell == "" {
		shell = detectShell()
	}

	// Validate shell override if provided
	if shellOverride != "" {
		validShells := map[string]bool{"bash": true, "zsh": true, "fish": true, "powershell": true}
		if !validShells[shell] {
			return ClusterSelectOutput{}, fmt.Errorf("invalid shell: %s (valid options: bash, zsh, fish, powershell)", shell)
		}
	}

	// Generate export commands if cluster is deployed
	var exportCommands []string
	if strings.ToLower(metadata.Status) == "deployed" {
		exportCommands = generateExportCommands(paths, shell)
	}

	return ClusterSelectOutput{
		Metadata:       metadata,
		Paths:          paths,
		ExportCommands: exportCommands,
		GitOpsInfo:     gitOpsInfo,
		Shell:          shell,
	}, nil
}

// detectShell detects the current shell from environment variables.
// Returns one of: bash, zsh, fish, powershell, or unknown.
func detectShell() string {
	// Check SHELL environment variable (Unix-like systems)
	if shell := os.Getenv("SHELL"); shell != "" {
		if strings.Contains(shell, "fish") {
			return "fish"
		}
		if strings.Contains(shell, "zsh") {
			return "zsh"
		}
		if strings.Contains(shell, "bash") {
			return "bash"
		}
	}

	// Check for PowerShell on Windows
	if psModulePath := os.Getenv("PSModulePath"); psModulePath != "" {
		return "powershell"
	}

	// Default to bash for unknown shells
	return "bash"
}

// generateExportCommands generates shell-specific export commands for cluster environment setup.
func generateExportCommands(paths config.ClusterPaths, shell string) []string {
	var commands []string

	switch shell {
	case "fish":
		// Fish shell syntax
		if _, err := os.Stat(paths.KubeconfigPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx KUBECONFIG %s", paths.KubeconfigPath))
		}
		if _, err := os.Stat(paths.InventoryPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx ANSIBLE_INVENTORY %s", paths.InventoryPath))
		}
		if _, err := os.Stat(paths.VenvPath); err == nil {
			activateScript := filepath.Join(paths.VenvPath, "bin", "activate.fish")
			if _, err := os.Stat(activateScript); err == nil {
				commands = append(commands, fmt.Sprintf("source %s", activateScript))
			}
		}
		if _, err := os.Stat(paths.BinPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx PATH %s $PATH", paths.BinPath))
		}

	case "powershell":
		// PowerShell syntax
		if _, err := os.Stat(paths.KubeconfigPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:KUBECONFIG = '%s'", paths.KubeconfigPath))
		}
		if _, err := os.Stat(paths.InventoryPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:ANSIBLE_INVENTORY = '%s'", paths.InventoryPath))
		}
		if _, err := os.Stat(paths.VenvPath); err == nil {
			activateScript := filepath.Join(paths.VenvPath, "Scripts", "Activate.ps1")
			if _, err := os.Stat(activateScript); err == nil {
				commands = append(commands, fmt.Sprintf(". %s", activateScript))
			}
		}
		if _, err := os.Stat(paths.BinPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:PATH = '%s;' + $env:PATH", paths.BinPath))
		}

	default:
		// Bash/Zsh syntax (POSIX-compatible)
		if _, err := os.Stat(paths.KubeconfigPath); err == nil {
			commands = append(commands, fmt.Sprintf("export KUBECONFIG=%s", paths.KubeconfigPath))
		}
		if _, err := os.Stat(paths.InventoryPath); err == nil {
			commands = append(commands, fmt.Sprintf("export ANSIBLE_INVENTORY=%s", paths.InventoryPath))
		}
		if _, err := os.Stat(paths.VenvPath); err == nil {
			activateScript := filepath.Join(paths.VenvPath, "bin", "activate")
			if _, err := os.Stat(activateScript); err == nil {
				commands = append(commands, fmt.Sprintf("source %s", activateScript))
			}
		}
		if _, err := os.Stat(paths.BinPath); err == nil {
			commands = append(commands, fmt.Sprintf("export PATH=%s:$PATH", paths.BinPath))
		}
	}

	return commands
}

// validateClusterExists validates that the specified cluster exists in the organization structure.
// The clusterName parameter can be in "cluster" or "organization/cluster" format.
func validateClusterExists(clusterName string, pathResolver *config.PathResolver) error {
	// Check if cluster configuration exists
	// ConfigPath now handles organization/cluster format
	path, err := config.ConfigPath(clusterName)
	if err != nil {
		// Get list of available clusters for helpful error message
		availableClusters, listErr := config.List()

		// Build error message with organization-based structure reference
		var errMsg strings.Builder
		errMsg.WriteString(fmt.Sprintf("cluster configuration directory '%s' not found in clusters subdirectory", clusterName))

		// Add helpful suggestions
		if listErr == nil && len(availableClusters) > 0 {
			errMsg.WriteString("\n\nAvailable clusters:")
			for _, cluster := range availableClusters {
				errMsg.WriteString(fmt.Sprintf("\n  - %s", cluster))
			}
			errMsg.WriteString("\n\nUse 'openCenter cluster list' to see all available clusters")
		} else {
			errMsg.WriteString("\n\nUse 'openCenter cluster list' to see available clusters")
		}

		// Add hint about organization format
		if !strings.Contains(clusterName, "/") {
			errMsg.WriteString("\n\nNote: If your cluster is in an organization, use the format: organization/cluster")
		}

		return errors.New(errMsg.String())
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Get list of available clusters for helpful error message
		availableClusters, listErr := config.List()

		// Build error message with organization-based structure reference
		var errMsg strings.Builder
		errMsg.WriteString(fmt.Sprintf("cluster configuration directory '%s' not found in clusters subdirectory", clusterName))

		// Add helpful suggestions
		if listErr == nil && len(availableClusters) > 0 {
			errMsg.WriteString("\n\nAvailable clusters:")
			for _, cluster := range availableClusters {
				errMsg.WriteString(fmt.Sprintf("\n  - %s", cluster))
			}
			errMsg.WriteString("\n\nUse 'openCenter cluster list' to see all available clusters")
		} else {
			errMsg.WriteString("\n\nUse 'openCenter cluster list' to see available clusters")
		}

		// Add hint about organization format
		if !strings.Contains(clusterName, "/") {
			errMsg.WriteString("\n\nNote: If your cluster is in an organization, use the format: organization/cluster")
		}

		return errors.New(errMsg.String())
	}

	return nil
}

// displayClusterSelectOutput displays the enhanced cluster select output.
func displayClusterSelectOutput(output ClusterSelectOutput, cmd *cobra.Command) {
	// Display cluster metadata
	fmt.Fprintf(cmd.OutOrStdout(), "Cluster Information:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Name:         %s\n", output.Metadata.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Environment:  %s\n", output.Metadata.Environment)
	fmt.Fprintf(cmd.OutOrStdout(), "  Region:       %s\n", output.Metadata.Region)
	fmt.Fprintf(cmd.OutOrStdout(), "  Status:       %s\n", output.Metadata.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "  Organization: %s\n", output.Metadata.Organization)
	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	// Display GitOps information
	fmt.Fprintf(cmd.OutOrStdout(), "GitOps Repository:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Directory:      %s\n", output.GitOpsInfo.GitDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  Applications Directory: %s\n", output.GitOpsInfo.ApplicationsDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  Infrastructure Directory: %s\n", output.GitOpsInfo.InfrastructureDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  Secrets Directory:     %s\n", output.GitOpsInfo.SecretsDir)
	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	// Display cluster-specific paths
	fmt.Fprintf(cmd.OutOrStdout(), "Cluster Paths:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Cluster Directory:     %s\n", output.Paths.ClusterDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Path:         %s\n", output.Paths.SOPSKeyPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Config Path:      %s\n", output.Paths.SOPSConfigPath)
	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	// Display export commands if available
	if len(output.ExportCommands) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Environment Setup Commands (%s):\n", output.Shell)
		for _, command := range output.ExportCommands {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", command)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
		fmt.Fprintf(cmd.OutOrStdout(), "To configure your shell environment, run:\n")

		// Provide shell-specific instructions
		switch output.Shell {
		case "fish":
			fmt.Fprintf(cmd.OutOrStdout(), "  openCenter cluster select %s --export-only | source\n", output.Metadata.Name)
		case "powershell":
			fmt.Fprintf(cmd.OutOrStdout(), "  openCenter cluster select %s --export-only | Invoke-Expression\n", output.Metadata.Name)
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "  eval \"$(openCenter cluster select %s --export-only)\"\n", output.Metadata.Name)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Environment Setup:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  No environment setup commands available (cluster status: %s)\n", output.Metadata.Status)
	}
}

// newClusterSelectCmd creates the enhanced command for selecting the active cluster.
//
// This command allows the user to set the active cluster and displays comprehensive
// information about the cluster including metadata, GitOps paths, and environment
// setup commands. If a cluster name is provided as an argument, it is set as active
// directly and displays the enhanced information. If no argument is given, it launches
// an interactive terminal UI where the user can select from a list of available clusters.
//
// The enhanced output includes:
// - Cluster metadata (name, environment, region, status, organization)
// - GitOps repository information and paths
// - Cluster-specific paths (SOPS keys, configuration, etc.)
// - Environment setup commands for deployed clusters
//
// The --clear flag can be used to deactivate the current cluster without selecting a new one.
// The --activate flag can be used to automatically activate the cluster environment.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `select` command.
func newClusterSelectCmd() *cobra.Command {
	var showExportOnly bool
	var shellOverride string
	var clearActive bool
	var activateEnv bool

	cmd := &cobra.Command{
		Use:   "select [name]",
		Short: "Select the active cluster and display environment information",
		Long: `Select the active cluster and display comprehensive information including:
- Cluster metadata (name, environment, region, status, organization)
- GitOps repository paths and structure
- Cluster-specific paths (SOPS keys, configuration files)
- Environment setup commands for shell configuration

If no cluster name is provided, an interactive selection menu is displayed.
For deployed clusters, environment setup commands are generated to configure
KUBECONFIG, ANSIBLE_INVENTORY, virtual environment, and PATH variables.

Use --clear to deactivate the current cluster without selecting a new one.
Use --activate to automatically activate the cluster environment (sets environment variables).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --clear flag to deactivate current cluster
			if clearActive {
				// Unset AWS credentials
				var deactivateOutput strings.Builder
				awsVars := credentials.GetAWSEnvVars()
				for _, envVar := range awsVars {
					deactivateOutput.WriteString(fmt.Sprintf("unset %s\n", envVar))
				}

				// Unset OpenStack credentials
				osVars := credentials.GetOpenStackEnvVars()
				for _, envVar := range osVars {
					deactivateOutput.WriteString(fmt.Sprintf("unset %s\n", envVar))
				}

				// Unset cluster-specific environment variables
				deactivateOutput.WriteString("unset BIN\n")
				deactivateOutput.WriteString("unset KUBECONFIG\n")
				deactivateOutput.WriteString("unset OPENCENTER_ACTIVE_CLUSTER\n")

				// Clear active cluster
				if err := config.SetActive(""); err != nil {
					return fmt.Errorf("failed to clear active cluster: %w", err)
				}

				if showExportOnly {
					// Output deactivation commands for shell evaluation
					fmt.Fprint(cmd.OutOrStdout(), deactivateOutput.String())
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Active cluster cleared\n\n")
					fmt.Fprintf(cmd.OutOrStdout(), "To deactivate the cluster environment, run:\n")
					fmt.Fprintf(cmd.OutOrStdout(), "  eval $(openCenter cluster select --clear --export-only)\n")
				}
				return nil
			}

			var name string
			if len(args) > 0 {
				name = args[0]
			}

			// If name not provided, prompt with interactive selection
			if name == "" {
				names, err := config.List()
				if err != nil {
					return err
				}
				if len(names) == 0 {
					return errors.New("no clusters defined")
				}

				items := []list.Item{}
				for _, clusterName := range names {
					// Extract organization and cluster name
					var org string
					if strings.Contains(clusterName, "/") {
						parts := strings.SplitN(clusterName, "/", 2)
						org = parts[0]
					} else {
						org = ""
					}

					// Create description with organization info
					description := ""
					if org != "" {
						description = fmt.Sprintf("Organization: %s", org)
					}

					items = append(items, item{
						title:        clusterName,
						description:  description,
						organization: org,
					})
				}

				delegate := list.NewDefaultDelegate()
				delegate.Styles.SelectedTitle = selectedItemStyle
				delegate.Styles.NormalTitle = itemStyle
				delegate.ShowDescription = true

				l := list.New(items, delegate, 0, 0)
				l.Title = "Select a cluster"
				l.Styles.Title = titleStyle

				m := model{list: l}
				p := tea.NewProgram(m, tea.WithAltScreen())

				finalModel, err := p.Run()
				if err != nil {
					return err
				}

				m, ok := finalModel.(model)
				if !ok {
					return errors.New("could not cast model")
				}
				name = m.choice
			}

			if name == "" {
				return nil
			}

			// Generate enhanced cluster select output
			output, err := generateClusterSelectOutput(name, shellOverride)
			if err != nil {
				return err
			}

			// Set active cluster
			if err := config.SetActive(name); err != nil {
				return fmt.Errorf("failed to set active cluster: %w", err)
			}

			// Handle --activate flag to automatically activate environment
			if activateEnv {
				// Load cluster configuration
				cfg, err := config.Load(name)
				if err != nil {
					return fmt.Errorf("failed to load cluster configuration: %w", err)
				}

				// Get cluster paths
				configManager, err := config.NewConfigManager("")
				if err != nil {
					return fmt.Errorf("failed to create config manager: %w", err)
				}
				pathResolver := config.NewPathResolver(configManager)

				// Parse cluster identifier to get organization and cluster name
				organization, actualClusterName, err := config.ParseClusterIdentifier(name)
				if err != nil {
					return fmt.Errorf("invalid cluster identifier: %w", err)
				}

				// Use organization from config if available
				if cfg.OpenCenter.Meta.Organization != "" {
					organization = cfg.OpenCenter.Meta.Organization
				}

				// Resolve cluster paths
				paths := pathResolver.ResolveClusterPaths(actualClusterName, organization)

				// Create credentials extractor
				extractor := credentials.NewExtractor(cfg)

				var activateOutput strings.Builder

				// Export cloud provider credentials
				awsCreds, awsErr := extractor.ExtractAWS()
				osCreds, osErr := extractor.ExtractOpenStack()

				hasAWS := awsErr == nil && !awsCreds.IsEmpty()
				hasOS := osErr == nil && !osCreds.IsEmpty()

				if hasAWS {
					activateOutput.WriteString(awsCreds.ToEnvVars())
				}
				if hasOS {
					if hasAWS {
						activateOutput.WriteString("\n")
					}
					activateOutput.WriteString(osCreds.ToEnvVars())
				}

				// Add cluster-specific environment variables
				if hasAWS || hasOS {
					activateOutput.WriteString("\n")
				}

				// Set BIN directory to full cluster path
				activateOutput.WriteString(fmt.Sprintf("export BIN=%s\n", paths.BinPath))

				// Extend PATH to include BIN directory
				activateOutput.WriteString("export PATH=${BIN}:${PATH}\n")

				// Set KUBECONFIG to full cluster path
				activateOutput.WriteString(fmt.Sprintf("export KUBECONFIG=%s\n", paths.KubeconfigPath))

				// Set active cluster environment variable
				activateOutput.WriteString(fmt.Sprintf("export OPENCENTER_ACTIVE_CLUSTER=%s\n", name))

				if showExportOnly {
					// Only show activation commands for shell evaluation
					fmt.Fprint(cmd.OutOrStdout(), activateOutput.String())
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Active cluster set to %s\n\n", name)
					displayClusterSelectOutput(output, cmd)
					fmt.Fprintf(cmd.OutOrStdout(), "\nCluster environment activated\n")
					fmt.Fprintf(cmd.OutOrStdout(), "\nTo apply these settings to your shell, run:\n")
					fmt.Fprintf(cmd.OutOrStdout(), "  eval $(openCenter cluster select %s --activate --export-only)\n", name)
				}
				return nil
			}

			// Display output based on flags
			if showExportOnly {
				// Only show export commands for shell evaluation
				for _, command := range output.ExportCommands {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", command)
				}
			} else {
				// Show full enhanced output
				fmt.Fprintf(cmd.OutOrStdout(), "Active cluster set to %s\n\n", name)
				displayClusterSelectOutput(output, cmd)

				// Inform user about activation
				fmt.Fprintf(cmd.OutOrStdout(), "\nTo activate the cluster environment with credentials, run:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  eval $(openCenter cluster select %s --activate --export-only)\n", name)
			}

			return nil
		},
	}

	// Add flag for export-only mode (useful for shell evaluation)
	cmd.Flags().BoolVar(&showExportOnly, "export-only", false, "Only output export commands for shell evaluation")

	// Add flag to override shell detection
	cmd.Flags().StringVar(&shellOverride, "shell", "", "Override shell detection (bash, zsh, fish, powershell)")

	// Add flag to clear active cluster (deactivate)
	cmd.Flags().BoolVar(&clearActive, "clear", false, "Clear the active cluster (deactivate)")

	// Add flag to activate cluster environment
	cmd.Flags().BoolVar(&activateEnv, "activate", false, "Activate cluster environment (set credentials and paths)")

	return cmd
}
