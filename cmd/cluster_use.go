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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/spf13/cobra"
)

var (
	docStyle          = lipgloss.NewStyle().Margin(1, 2)
	titleStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
)

// ClusterMetadata represents cluster metadata for display in cluster use.
type ClusterMetadata struct {
	Name         string `yaml:"name"`
	Environment  string `yaml:"env"`
	Region       string `yaml:"region"`
	Status       string `yaml:"status"`
	Organization string `yaml:"organization"`
}

// ClusterSelectOutput represents the complete output for cluster use command.
type ClusterSelectOutput struct {
	Metadata       ClusterMetadata
	Paths          *paths.ClusterPaths
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
func loadClusterMetadata(ctx context.Context, clusterName string) (ClusterMetadata, error) {
	// Parse the cluster identifier to extract organization and cluster name
	organization, actualClusterName, err := config.ParseClusterIdentifier(clusterName)
	if err != nil {
		return ClusterMetadata{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	// Load cluster configuration (Load function now handles organization/cluster format)
	cfg, err := loadConfig(ctx, clusterName)
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

// generateClusterSelectOutput generates the complete output for cluster use command.
// The clusterName parameter can be in "cluster" or "organization/cluster" format.
// The shellOverride parameter allows overriding automatic shell detection (empty string for auto-detect).
func generateClusterSelectOutput(clusterName string, shellOverride string) (ClusterSelectOutput, error) {
	// Parse the cluster identifier to extract organization and cluster name
	organization, actualClusterName, err := config.ParseClusterIdentifier(clusterName)
	if err != nil {
		return ClusterSelectOutput{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	// Create path resolver
	pathResolver := config.NewPathResolverFromConfig()

	// Validate that cluster exists first
	ctx := context.Background()
	var clusterPaths *paths.ClusterPaths

	// If organization was explicitly provided in the identifier, use Resolve
	// Otherwise use ResolveWithFallback to search all organizations
	if strings.Contains(clusterName, "/") {
		clusterPaths, err = pathResolver.Resolve(ctx, actualClusterName, organization)
		if err != nil {
			return ClusterSelectOutput{}, fmt.Errorf("cluster not found: %w\n\nCheck available clusters with: opencenter cluster list", err)
		}
	} else {
		clusterPaths, err = pathResolver.ResolveWithFallback(ctx, actualClusterName)
		if err != nil {
			return ClusterSelectOutput{}, fmt.Errorf("cluster not found: %w\n\nCheck available clusters with: opencenter cluster list", err)
		}
	}

	// Load cluster metadata
	metadata, err := loadClusterMetadata(ctx, clusterName)
	if err != nil {
		return ClusterSelectOutput{}, err
	}

	// Use organization from metadata if available, otherwise use parsed organization
	if metadata.Organization == "" {
		metadata.Organization = organization
	}

	// Create GitOps info
	gitOpsInfo := GitOpsInfo{
		GitDir:            clusterPaths.GitOpsDir,
		ApplicationsDir:   clusterPaths.ApplicationsDir,
		InfrastructureDir: clusterPaths.ClusterDir,
		SecretsDir:        clusterPaths.SecretsDir,
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

	// Generate export commands if cluster is deployed or successful
	var exportCommands []string
	status := strings.ToLower(metadata.Status)
	if status == "deployed" || status == "success" {
		exportCommands = generateExportCommands(clusterPaths, shell)
	}

	return ClusterSelectOutput{
		Metadata:       metadata,
		Paths:          clusterPaths,
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
func generateExportCommands(clusterPaths *paths.ClusterPaths, shell string) []string {
	var commands []string

	switch shell {
	case "fish":
		// Fish shell syntax
		if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx KUBECONFIG %s", clusterPaths.KubeconfigPath))
		}
		if _, err := os.Stat(clusterPaths.InventoryPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx ANSIBLE_INVENTORY %s", clusterPaths.InventoryPath))
		}
		if _, err := os.Stat(clusterPaths.SOPSKeyPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx SOPS_AGE_KEY_FILE %s", clusterPaths.SOPSKeyPath))
		}
		if _, err := os.Stat(clusterPaths.VenvPath); err == nil {
			activateScript := filepath.Join(clusterPaths.VenvPath, "bin", "activate.fish")
			if _, err := os.Stat(activateScript); err == nil {
				commands = append(commands, fmt.Sprintf("source %s", activateScript))
			}
		}
		if _, err := os.Stat(clusterPaths.BinPath); err == nil {
			commands = append(commands, fmt.Sprintf("set -gx PATH %s $PATH", clusterPaths.BinPath))
		}

	case "powershell":
		// PowerShell syntax
		if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:KUBECONFIG = '%s'", clusterPaths.KubeconfigPath))
		}
		if _, err := os.Stat(clusterPaths.InventoryPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:ANSIBLE_INVENTORY = '%s'", clusterPaths.InventoryPath))
		}
		if _, err := os.Stat(clusterPaths.SOPSKeyPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:SOPS_AGE_KEY_FILE = '%s'", clusterPaths.SOPSKeyPath))
		}
		if _, err := os.Stat(clusterPaths.VenvPath); err == nil {
			activateScript := filepath.Join(clusterPaths.VenvPath, "Scripts", "Activate.ps1")
			if _, err := os.Stat(activateScript); err == nil {
				commands = append(commands, fmt.Sprintf(". %s", activateScript))
			}
		}
		if _, err := os.Stat(clusterPaths.BinPath); err == nil {
			commands = append(commands, fmt.Sprintf("$env:PATH = '%s;' + $env:PATH", clusterPaths.BinPath))
		}

	default:
		// Bash/Zsh syntax (POSIX-compatible)
		if _, err := os.Stat(clusterPaths.KubeconfigPath); err == nil {
			commands = append(commands, fmt.Sprintf("export KUBECONFIG=%s", clusterPaths.KubeconfigPath))
		}
		if _, err := os.Stat(clusterPaths.InventoryPath); err == nil {
			commands = append(commands, fmt.Sprintf("export ANSIBLE_INVENTORY=%s", clusterPaths.InventoryPath))
		}
		if _, err := os.Stat(clusterPaths.SOPSKeyPath); err == nil {
			commands = append(commands, fmt.Sprintf("export SOPS_AGE_KEY_FILE=%s", clusterPaths.SOPSKeyPath))
		}
		if _, err := os.Stat(clusterPaths.VenvPath); err == nil {
			activateScript := filepath.Join(clusterPaths.VenvPath, "bin", "activate")
			if _, err := os.Stat(activateScript); err == nil {
				commands = append(commands, fmt.Sprintf("source %s", activateScript))
			}
		}
		if _, err := os.Stat(clusterPaths.BinPath); err == nil {
			commands = append(commands, fmt.Sprintf("export PATH=%s:$PATH", clusterPaths.BinPath))
		}
	}

	return commands
}

// displayClusterSelectOutput displays the enhanced cluster use output.
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
		fmt.Fprintf(cmd.OutOrStdout(), "Available Environment Variables (%s):\n", output.Shell)
		fmt.Fprintf(cmd.OutOrStdout(), "  • Cloud credentials (AWS/OpenStack)\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  • KUBECONFIG, ANSIBLE_INVENTORY, SOPS_AGE_KEY_FILE\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  • Virtual environment and PATH\n")
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Environment Setup:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  No environment setup commands available (cluster status: %s)\n", output.Metadata.Status)
	}
}

// newClusterUseCmd creates the enhanced command for setting the active cluster.
//
// This command allows the user to set the active cluster and displays comprehensive
// information about the cluster including metadata, GitOps paths, and environment
// setup commands. If a cluster name is provided as an argument, it is set as active
// directly and displays the enhanced information. If no argument is given, it launches
// an interactive terminal UI where the user can select from a list of available clusters.
//
// By default, the active cluster is session-scoped (current terminal only) when shell
// integration is enabled. Use --persistent to set a global active cluster that
// affects all terminals.
//
// The enhanced output includes:
// - Cluster metadata (name, environment, region, status, organization)
// - GitOps repository information and paths
// - Cluster-specific paths (SOPS keys, configuration, etc.)
// - Environment setup commands for deployed clusters
//
// The --clear flag can be used to deactivate the current cluster without selecting a new one.
// The --clear-persistent flag removes the persistent active cluster.
// The --activate flag can be used to automatically activate the cluster environment.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `use` command.
func newClusterUseCmd() *cobra.Command {
	var shellOverride string
	var clearActive bool
	var clearPersistent bool
	var persistentSelection bool

	cmd := &cobra.Command{
		Use:   "use [name]",
		Short: "Set the active cluster",
		Long: `Set the active cluster and display comprehensive information including:
- Cluster metadata (name, environment, region, status, organization)
- GitOps repository paths and structure
- Cluster-specific paths (SOPS keys, configuration files)
- Environment setup commands for shell configuration

By default, the active cluster is session-scoped (current terminal only) when shell
integration is enabled via: eval "$(opencenter shell-init)"

Use --persistent to set a global active cluster that affects all terminals.

If no cluster name is provided, an interactive selection menu is displayed.
For deployed clusters, environment setup commands are generated to configure
KUBECONFIG, ANSIBLE_INVENTORY, virtual environment, and PATH variables.

Use --clear to deactivate the current session cluster.
Use --clear-persistent to remove the persistent active cluster.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Handle --clear-persistent flag to remove persistent active cluster
			if clearPersistent {
				if err := setActiveCluster(""); err != nil {
					return fmt.Errorf("failed to clear persistent cluster: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Persistent active cluster cleared\n")
				return nil
			}

			// Handle --clear flag to deactivate current cluster
			if clearActive {
				if sessionFile := os.Getenv("OPENCENTER_SESSION_FILE"); sessionFile != "" {
					if err := os.Remove(sessionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("failed to clear session cluster: %w", err)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "unset OPENCENTER_CLUSTER")
				} else {
					if err := setActiveCluster(""); err != nil {
						return fmt.Errorf("failed to clear active cluster: %w", err)
					}
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Active cluster cleared\n")
				return nil
			}

			var name string
			if len(args) > 0 {
				name = args[0]
			}

			// If name not provided, prompt with interactive selection
			if name == "" {
				names, err := listClusters(ctx)
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

			// Generate enhanced cluster use output
			output, err := generateClusterSelectOutput(name, shellOverride)
			if err != nil {
				return err
			}

			// Handle session-scoped vs persistent selection
			if persistentSelection {
				// Set persistent cluster (affects all terminals)
				if err := setActiveCluster(name); err != nil {
					return fmt.Errorf("failed to set active cluster: %w", err)
				}
			} else {
				// Session-scoped selection (default)
				sessionFile := os.Getenv("OPENCENTER_SESSION_FILE")
				if sessionFile == "" {
					// No shell integration - inform user and fall back to persistent
					fmt.Fprintf(os.Stderr, "⚠️  Shell integration not detected. Setting persistent active cluster.\n")
					fmt.Fprintf(os.Stderr, "💡 To enable session-scoped selection, run: eval \"$(opencenter shell-init)\"\n")
					fmt.Fprintf(os.Stderr, "💡 Or use --persistent flag to suppress this warning.\n\n")

					if err := setActiveCluster(name); err != nil {
						return fmt.Errorf("failed to set active cluster: %w", err)
					}
				} else {
					// Shell integration active - use session file
					if err := os.WriteFile(sessionFile, []byte(name), 0600); err != nil {
						return fmt.Errorf("failed to update session: %w", err)
					}

					fmt.Fprintf(cmd.OutOrStdout(), "export OPENCENTER_CLUSTER=%s\n", name)
				}
			}

			// Show full enhanced output
			scope := "session"
			if persistentSelection {
				scope = "persistent"
			} else if os.Getenv("OPENCENTER_SESSION_FILE") == "" {
				scope = "persistent (no shell integration)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active cluster set to %s (%s)\n\n", name, scope)
			displayClusterSelectOutput(output, cmd)

			// Inform user about exporting environment (only if commands are available)
			if len(output.ExportCommands) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nTo export ALL cluster environment (credentials + paths), run:\n")
				switch output.Shell {
				case "fish":
					fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster env %s --shell fish | source\n", name)
				case "powershell":
					fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster env %s --shell powershell | Invoke-Expression\n", name)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "  eval $(opencenter cluster env %s)\n", name)
				}
			}

			return nil
		},
	}

	// Add flag to override shell detection
	cmd.Flags().StringVar(&shellOverride, "shell", "", "Override shell detection (bash, zsh, fish, powershell)")

	// Add flag to clear active cluster (deactivate)
	cmd.Flags().BoolVar(&clearActive, "clear", false, "Clear the active cluster (deactivate session)")

	// Add flag to clear persistent active cluster
	cmd.Flags().BoolVar(&clearPersistent, "clear-persistent", false, "Clear the persistent active cluster")

	// Add flag for persistent selection (affects all terminals)
	cmd.Flags().BoolVar(&persistentSelection, "persistent", false, "Set persistent active cluster (affects all terminals)")

	return cmd
}
