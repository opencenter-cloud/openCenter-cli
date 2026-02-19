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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rackerlabs/opencenter-cli/internal/barbican"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secrets across backends",
		Long:  `Manage secrets across different backends (Barbican, SOPS, file) based on cluster configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSecretsLoginCmd())
	cmd.AddCommand(newSecretsListCmd())
	cmd.AddCommand(newSecretsDescribeCmd())
	cmd.AddCommand(newSecretsGetCmd())
	cmd.AddCommand(newSecretsSetCmd())
	cmd.AddCommand(newSecretsDeleteCmd())
	cmd.AddCommand(newSecretsSyncCmd())
	cmd.AddCommand(newSecretsValidateCmd())
	cmd.AddCommand(newSecretsEncryptCmd())
	cmd.AddCommand(newSecretsDecryptCmd())
	cmd.AddCommand(newSecretsStatusCmd())
	cmd.AddCommand(NewSecretsKeysCmd())

	return cmd
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
			if clusterName == "" {
				activeCluster, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("no cluster specified and failed to get active cluster: %w", err)
				}
				if activeCluster == "" {
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster select' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Check backend before authenticating
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}

			if backend != "barbican" {
				return fmt.Errorf("login is only supported for the barbican backend")
			}

			barbicanCfg := &cfg.OpenCenter.Secrets.Barbican
			if projectID != "" {
				barbicanCfg.ProjectID = projectID
			}

			client, err := barbican.NewClient(barbicanCfg)
			if err != nil {
				return err
			}

			var password string
			if passwordIn {
				// Read password from stdin
				bytePassword, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("could not read password from stdin: %w", err)
				}
				password = strings.TrimSpace(string(bytePassword))
			}

			token, err := client.Login(cmd.Context(), username, password)
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
			if clusterName == "" {
				activeCluster, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("no cluster specified and failed to get active cluster: %w", err)
				}
				if activeCluster == "" {
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster select' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}

			switch backend {
			case "barbican":
				return listBarbicanSecrets(cmd.Context(), cfg, labels, format)
			case "sops":
				return listSOPSSecrets(cmd.Context(), cfg, format)
			case "file":
				return listFileSecrets(cfg, format)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Filter secrets by labels in key=value form")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, or yaml")
	return cmd
}

func listBarbicanSecrets(ctx context.Context, cfg *config.Config, labels []string, format string) error {
	client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
	if err != nil {
		return err
	}
	labelMap, err := barbican.ParseLabels(labels)
	if err != nil {
		return err
	}
	secrets, err := client.ListSecrets(ctx, labelMap)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(secrets)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(secrets)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)
		fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tCREATED")
		for _, secret := range secrets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", secret.Name, secret.SecretType, secret.Status, secret.Created)
		}
		w.Flush()
		return nil
	}
}

func listSOPSSecrets(ctx context.Context, cfg *config.Config, format string) error {
	// List secrets from the cluster configuration
	type SecretInfo struct {
		Name     string `json:"name" yaml:"name"`
		Type     string `json:"type" yaml:"type"`
		Location string `json:"location" yaml:"location"`
	}

	var secretsList []SecretInfo

	// Extract secrets from config
	if cfg.Secrets.CertManager.AWSAccessKey != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "cert-manager-aws-credentials",
			Type:     "aws-credentials",
			Location: "config: secrets.cert_manager",
		})
	}
	if cfg.Secrets.Keycloak.AdminPassword != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "keycloak-admin-password",
			Type:     "password",
			Location: "config: secrets.keycloak.admin_password",
		})
	}
	if cfg.Secrets.Grafana.AdminPassword != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "grafana-admin-password",
			Type:     "password",
			Location: "config: secrets.grafana.admin_password",
		})
	}
	if cfg.Secrets.Headlamp.OIDCClientSecret != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "headlamp-oidc-client-secret",
			Type:     "oidc-secret",
			Location: "config: secrets.headlamp.oidc_client_secret",
		})
	}
	if cfg.Secrets.WeaveGitOps.Password != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "weave-gitops-password",
			Type:     "password",
			Location: "config: secrets.weave_gitops.password",
		})
	}
	if cfg.Secrets.VSphereCsi.Username != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "vsphere-csi-credentials",
			Type:     "credentials",
			Location: "config: secrets.vsphere_csi",
		})
	}
	if cfg.Secrets.AlertProxy.CoreAccountNumber != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "alert-proxy-credentials",
			Type:     "credentials",
			Location: "config: secrets.alert_proxy",
		})
	}
	if cfg.Secrets.Loki.S3AccessKeyID != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "loki-s3-credentials",
			Type:     "s3-credentials",
			Location: "config: secrets.loki",
		})
	}
	if cfg.Secrets.Tempo.AccessKey != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "tempo-s3-credentials",
			Type:     "s3-credentials",
			Location: "config: secrets.tempo",
		})
	}

	// Add SSH key if present
	if cfg.Secrets.SSHKey.Private != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "ssh-key",
			Type:     "ssh-key",
			Location: "config: secrets.ssh_key",
		})
	}

	// Add SOPS age key
	if cfg.Secrets.SopsAgeKeyFile != "" {
		secretsList = append(secretsList, SecretInfo{
			Name:     "sops-age-key",
			Type:     "age-key",
			Location: cfg.Secrets.SopsAgeKeyFile,
		})
	}

	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(secretsList)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(secretsList)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tLOCATION")
		for _, secret := range secretsList {
			fmt.Fprintf(w, "%s\t%s\t%s\n", secret.Name, secret.Type, secret.Location)
		}
		w.Flush()
		return nil
	}
}

func listFileSecrets(cfg *config.Config, format string) error {
	// For file backend, secrets are stored directly in the config
	return listSOPSSecrets(nil, cfg, format)
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
			if clusterName == "" {
				activeCluster, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("no cluster specified and failed to get active cluster: %w", err)
				}
				if activeCluster == "" {
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster select' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}

			switch backend {
			case "barbican":
				return describeBarbicanSecret(cmd.Context(), cfg, name, format)
			case "sops":
				return describeSOPSSecret(cmd.Context(), cfg, name, format)
			case "file":
				return describeFileSecret(cfg, name, format)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, or yaml")
	return cmd
}

func describeBarbicanSecret(ctx context.Context, cfg *config.Config, name string, format string) error {
	client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
	if err != nil {
		return err
	}
	secret, err := client.DescribeSecret(ctx, name)
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
}

func describeSOPSSecret(ctx context.Context, cfg *config.Config, name string, format string) error {
	// For SOPS backend, describe shows metadata from config
	return fmt.Errorf("describe not yet implemented for SOPS backend")
}

func describeFileSecret(cfg *config.Config, name string, format string) error {
	// For file backend, describe shows metadata from config
	return fmt.Errorf("describe not yet implemented for file backend")
}

func newSecretsGetCmd() *cobra.Command {
	var (
		outputFile string
		show       bool
	)
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Download and decrypt a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			clusterName, _ := cmd.Flags().GetString("cluster")
			if clusterName == "" {
				activeCluster, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("no cluster specified and failed to get active cluster: %w", err)
				}
				if activeCluster == "" {
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster select' to set an active cluster")
				}
				clusterName = activeCluster
			}

			if outputFile == "" && !show {
				return fmt.Errorf("use --output-file to save the secret or --show to print it to stdout (warning: printing to stdout is insecure)")
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}

			switch backend {
			case "barbican":
				return getBarbicanSecret(cmd.Context(), cfg, name, outputFile, show)
			case "sops":
				return getSOPSSecret(cmd.Context(), cfg, name, outputFile, show)
			case "file":
				return getFileSecret(cfg, name, outputFile, show)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Path to save the secret")
	cmd.Flags().BoolVar(&show, "show", false, "Print secret to stdout (insecure)")
	return cmd
}

func getBarbicanSecret(ctx context.Context, cfg *config.Config, name string, outputFile string, show bool) error {
	client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
	if err != nil {
		return err
	}

	payload, err := client.GetSecret(ctx, name)
	if err != nil {
		return err
	}
	if outputFile != "" {
		err := os.WriteFile(outputFile, payload, 0600)
		if err != nil {
			return err
		}
		fmt.Printf("Secret '%s' saved to %s\n", name, outputFile)
	}
	if show {
		if outputFile != "" {
			fmt.Println("--- Secret Content ---")
		} else {
			fmt.Fprintln(os.Stderr, "Warning: Printing secret to stdout is insecure.")
		}
		fmt.Println(string(payload))
	}
	return nil
}

func getSOPSSecret(ctx context.Context, cfg *config.Config, name string, outputFile string, show bool) error {
	// For SOPS backend, get retrieves from config
	return fmt.Errorf("get not yet implemented for SOPS backend")
}

func getFileSecret(cfg *config.Config, name string, outputFile string, show bool) error {
	// For file backend, get retrieves from config
	return fmt.Errorf("get not yet implemented for file backend")
}

func newSecretsSetCmd() *cobra.Command {
	var (
		fromFile               string
		labels                 []string
		secretType             string
		payloadContentEncoding string
	)
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Create or update a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			var payload []byte
			var err error
			if fromFile != "" {
				payload, err = os.ReadFile(fromFile)
				if err != nil {
					return err
				}
			} else {
				payload, err = io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				if len(payload) == 0 {
					return fmt.Errorf("secret payload must be provided via --from-file or stdin")
				}
			}

			clusterName, _ := cmd.Flags().GetString("cluster")
			if clusterName == "" {
				activeCluster, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("no cluster specified and failed to get active cluster: %w", err)
				}
				if activeCluster == "" {
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster select' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}

			switch backend {
			case "barbican":
				return setBarbicanSecret(cmd.Context(), cfg, name, payload, labels, secretType, payloadContentEncoding)
			case "sops":
				return setSOPSSecret(cmd.Context(), cfg, name, payload)
			case "file":
				return setFileSecret(cfg, name, payload)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to a file containing the secret")
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Additional Barbican labels in key=value form")
	cmd.Flags().StringVar(&secretType, "secret-type", "opaque", "Type of the secret (e.g. opaque, passphrase)")
	cmd.Flags().StringVar(&payloadContentEncoding, "payload-encoding", "base64", "Encoding of the payload (e.g. base64)")
	return cmd
}

func setBarbicanSecret(ctx context.Context, cfg *config.Config, name string, payload []byte, labels []string, secretType string, payloadContentEncoding string) error {
	client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
	if err != nil {
		return err
	}

	labelMap, err := barbican.ParseLabels(labels)
	if err != nil {
		return err
	}

	encodedPayload := payload
	if payloadContentEncoding == "base64" {
		// If the user specified base64, we assume they are providing raw bytes that need encoding,
		// OR they are providing a string that is already encoded?
		// The Barbican API expects the payload to be consistent with the encoding specified.
		// If we look at the previous implementation:
		// encodedPayload := base64.StdEncoding.EncodeToString(payload)
		// It was always base64 encoding the input.
		// If we allow 'text/plain' type, we might not want to base64 encode.
		// For now, let's keep the behavior that if encoding is base64, we encode it.
		b64 := base64.StdEncoding.EncodeToString(payload)
		encodedPayload = []byte(b64)
	}

	err = client.PutSecret(ctx, name, encodedPayload, labelMap, secretType, payloadContentEncoding)
	if err != nil {
		return err
	}
	fmt.Printf("Secret '%s' created/updated successfully\n", name)
	return nil
}

func setSOPSSecret(ctx context.Context, cfg *config.Config, name string, payload []byte) error {
	// For SOPS backend, set updates config
	return fmt.Errorf("set not yet implemented for SOPS backend")
}

func setFileSecret(cfg *config.Config, name string, payload []byte) error {
	// For file backend, set updates config
	return fmt.Errorf("set not yet implemented for file backend")
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
			if clusterName == "" {
				activeCluster, err := getActiveCluster()
				if err != nil {
					return fmt.Errorf("no cluster specified and failed to get active cluster: %w", err)
				}
				if activeCluster == "" {
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster select' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}

			switch backend {
			case "barbican":
				return deleteBarbicanSecret(cmd.Context(), cfg, name)
			case "sops":
				return deleteSOPSSecret(cmd.Context(), cfg, name)
			case "file":
				return deleteFileSecret(cfg, name)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion of a secret")
	return cmd
}

func deleteBarbicanSecret(ctx context.Context, cfg *config.Config, name string) error {
	client, err := barbican.NewClient(&cfg.OpenCenter.Secrets.Barbican)
	if err != nil {
		return err
	}

	err = client.DeleteSecret(ctx, name)
	if err != nil {
		return err
	}
	fmt.Printf("Secret '%s' deleted successfully\n", name)
	return nil
}

func deleteSOPSSecret(ctx context.Context, cfg *config.Config, name string) error {
	// For SOPS backend, delete removes from config
	return fmt.Errorf("delete not yet implemented for SOPS backend")
}

func deleteFileSecret(cfg *config.Config, name string) error {
	// For file backend, delete removes from config
	return fmt.Errorf("delete not yet implemented for file backend")
}


