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
func generateClusterSelectOutput(clusterName string) (ClusterSelectOutput, error) {
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

	// Generate export commands if cluster is deployed
	var exportCommands []string
	if strings.ToLower(metadata.Status) == "deployed" {
		exportCommands = generateExportCommands(paths)
	}

	return ClusterSelectOutput{
		Metadata:       metadata,
		Paths:          paths,
		ExportCommands: exportCommands,
		GitOpsInfo:     gitOpsInfo,
	}, nil
}

// generateExportCommands generates shell export commands for cluster environment setup.
func generateExportCommands(paths config.ClusterPaths) []string {
	var commands []string

	// KUBECONFIG export
	if _, err := os.Stat(paths.KubeconfigPath); err == nil {
		commands = append(commands, fmt.Sprintf("export KUBECONFIG=%s", paths.KubeconfigPath))
	}

	// ANSIBLE_INVENTORY export
	if _, err := os.Stat(paths.InventoryPath); err == nil {
		commands = append(commands, fmt.Sprintf("export ANSIBLE_INVENTORY=%s", paths.InventoryPath))
	}

	// Virtual environment activation
	if _, err := os.Stat(paths.VenvPath); err == nil {
		activateScript := filepath.Join(paths.VenvPath, "bin", "activate")
		if _, err := os.Stat(activateScript); err == nil {
			commands = append(commands, fmt.Sprintf("source %s", activateScript))
		}
	}

	// PATH update for .bin directory
	if _, err := os.Stat(paths.BinPath); err == nil {
		commands = append(commands, fmt.Sprintf("export PATH=%s:$PATH", paths.BinPath))
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
		return fmt.Errorf("failed to get config path for cluster %s: %w", clusterName, err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("cluster configuration '%s' not found. Use 'openCenter cluster list' to see available clusters", clusterName)
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
		fmt.Fprintf(cmd.OutOrStdout(), "Environment Setup Commands:\n")
		for _, command := range output.ExportCommands {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", command)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
		fmt.Fprintf(cmd.OutOrStdout(), "To configure your shell environment, run:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  eval \"$(openCenter cluster select %s)\"\n", output.Metadata.Name)
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
// Returns:
//   - *cobra.Command: A pointer to the configured `select` command.
func newClusterSelectCmd() *cobra.Command {
	var showExportOnly bool

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
KUBECONFIG, ANSIBLE_INVENTORY, virtual environment, and PATH variables.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			output, err := generateClusterSelectOutput(name)
			if err != nil {
				return err
			}

			// Set active cluster
			if err := config.SetActive(name); err != nil {
				return fmt.Errorf("failed to set active cluster: %w", err)
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
			}

			return nil
		},
	}

	// Add flag for export-only mode (useful for shell evaluation)
	cmd.Flags().BoolVar(&showExportOnly, "export-only", false, "Only output export commands for shell evaluation")

	return cmd
}
