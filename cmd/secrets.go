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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rackerlabs/openCenter-cli/internal/barbican"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secrets with Barbican",
		Long:  `Provides a Barbican-backed control plane for handling credentials, bootstrap bundles, and opaque data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSecretsLoginCmd())
	cmd.AddCommand(newSecretsListCmd())
	cmd.AddCommand(newSecretsDescribeCmd())
	cmd.AddCommand(newSecretsGetCmd())
	cmd.AddCommand(newSecretsPutCmd())
	cmd.AddCommand(newSecretsDeleteCmd())
	cmd.AddCommand(newSecretsSyncCmd())

	return cmd
}

func parseLabels(labels []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, label := range labels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s", label)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

func newSecretsLoginCmd() *cobra.Command {
	var (
		username   string
		projectID  string
		passwordIn bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Create or refresh a Keystone token",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}

			barbicanCfg := &cfg.OpenCenter.Secrets.Barbican
			if projectID != "" {
				barbicanCfg.ProjectID = projectID
			}

			token, err := barbican.Authenticate(cmd.Context(), barbicanCfg, username, "", passwordIn)
			if err != nil {
				return err
			}

			err = barbican.StoreToken(token)
			if err != nil {
				return err
			}

			fmt.Println("Successfully authenticated and token stored.")
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "OpenStack username")
	cmd.Flags().StringVar(&projectID, "project-id", "", "OpenStack project ID")
	cmd.Flags().BoolVar(&passwordIn, "password-stdin", false, "Read password from stdin")

	return cmd
}

func newSecretsListCmd() *cobra.Command {
	var (
		labels []string
		format string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List secrets associated with the current cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}
			client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
			if err != nil {
				return err
			}
			labelMap, err := parseLabels(labels)
			if err != nil {
				return err
			}
			secrets, err := client.ListSecrets(cmd.Context(), labelMap)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				json.NewEncoder(os.Stdout).Encode(secrets)
			case "yaml":
				yaml.NewEncoder(os.Stdout).Encode(secrets)
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)
				fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tCREATED")
				for _, secret := range secrets {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", secret.Name, secret.SecretType, secret.Status, secret.Created)
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Filter secrets by labels in key=value form")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, or yaml")
	return cmd
}

func newSecretsDescribeCmd() *cobra.Command {
	var (
		format string
	)
	cmd := &cobra.Command{
		Use:   "describe <name>",
		Short: "Show metadata for a single secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}
			client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
			if err != nil {
				return err
			}
			secret, err := client.DescribeSecret(cmd.Context(), name)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				json.NewEncoder(os.Stdout).Encode(secret)
			case "yaml":
				yaml.NewEncoder(os.Stdout).Encode(secret)
			default:
				fmt.Printf("Name: %s\n", secret.Name)
				fmt.Printf("Type: %s\n", secret.SecretType)
				fmt.Printf("Status: %s\n", secret.Status)
				fmt.Printf("Created: %s\n", secret.Created)
				fmt.Printf("Content Types: %v\n", secret.ContentTypes)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, or yaml")
	return cmd
}

func newSecretsGetCmd() *cobra.Command {
	var (
		outputFile string
	)
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Download and decrypt a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}
			client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
			if err != nil {
				return err
			}
			payload, err := client.GetSecret(cmd.Context(), name)
			if err != nil {
				return err
			}
			if outputFile != "" {
				err := ioutil.WriteFile(outputFile, payload, 0600)
				if err != nil {
					return err
				}
				fmt.Printf("Secret '%s' saved to %s\n", name, outputFile)
			} else {
				fmt.Println(string(payload))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Path to save the secret")
	return cmd
}

func newSecretsPutCmd() *cobra.Command {
	var (
		fromFile string
		value    string
		labels   []string
	)
	cmd := &cobra.Command{
		Use:   "put <name>",
		Short: "Create or update a Barbican secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			var payload []byte
			var err error
			if fromFile != "" {
				payload, err = ioutil.ReadFile(fromFile)
				if err != nil {
					return err
				}
			} else if value != "" {
				payload = []byte(value)
			} else {
				return fmt.Errorf("either --from-file or --value must be specified")
			}

			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}

			client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
			if err != nil {
				return err
			}

			labelMap, err := parseLabels(labels)
			if err != nil {
				return err
			}

			encodedPayload := base64.StdEncoding.EncodeToString(payload)
			err = client.PutSecret(cmd.Context(), name, []byte(encodedPayload), labelMap)
			if err != nil {
				return err
			}
			fmt.Printf("Secret '%s' created/updated successfully\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to a file containing the secret")
	cmd.Flags().StringVar(&value, "value", "", "Value of the secret")
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Additional Barbican labels in key=value form")
	return cmd
}

func newSecretsDeleteCmd() *cobra.Command {
	var (
		force bool
	)
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}
			client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
			if err != nil {
				return err
			}

			secret, err := client.DescribeSecret(cmd.Context(), name)
			if err != nil {
				return err
			}

			isBootstrap := false
			for _, tag := range secret.Tags {
				if tag == "scope=bootstrap" {
					isBootstrap = true
					break
				}
			}

			if isBootstrap && !force {
				return fmt.Errorf("secret '%s' is a bootstrap secret. Use --force to delete", name)
			}

			err = client.DeleteSecret(cmd.Context(), name)
			if err != nil {
				return err
			}
			fmt.Printf("Secret '%s' deleted successfully\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion of a secret")
	return cmd
}

func newSecretsSyncCmd() *cobra.Command {
	var (
		directory string
		labels    []string
		format    string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Materialize a filtered subset of Barbican secrets onto disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, _ := cmd.Flags().GetString("cluster")
			cfg, err := config.Load(clusterName)
			if err != nil {
				return err
			}
			client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
			if err != nil {
				return err
			}

			labelMap, err := parseLabels(labels)
			if err != nil {
				return err
			}

			secrets, err := client.ListSecrets(cmd.Context(), labelMap)
			if err != nil {
				return err
			}

			if directory == "" {
				return fmt.Errorf("--directory is required")
			}
			err = os.MkdirAll(directory, 0755)
			if err != nil {
				return err
			}

			for _, secret := range secrets {
				payload, err := client.GetSecret(cmd.Context(), secret.Name)
				if err != nil {
					return err
				}

				var data []byte
				switch format {
				case "json":
					data, err = json.MarshalIndent(map[string]string{"name": secret.Name, "payload": string(payload)}, "", "  ")
				case "yaml":
					data, err = yaml.Marshal(map[string]string{"name": secret.Name, "payload": string(payload)})
				default:
					data = payload
				}
				if err != nil {
					return err
				}

				fileName := fmt.Sprintf("%s.%s", secret.Name, format)
				filePath := fmt.Sprintf("%s/%s", directory, fileName)
				err = ioutil.WriteFile(filePath, data, 0600)
				if err != nil {
					return err
				}
				fmt.Printf("Synced secret '%s' to %s\n", secret.Name, filePath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&directory, "directory", "", "Directory to sync secrets to")
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Filter secrets by labels in key=value form")
	cmd.Flags().StringVar(&format, "format", "yaml", "Output format: yaml or json")
	return cmd
}
