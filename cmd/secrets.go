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
	"text/tabwriter"

	"github.com/opencenter-cloud/opencenter-cli/internal/barbican"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
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
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster use' to set an active cluster")
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

			password, err := readSecretsLoginPassword(cmd, passwordIn)
			if err != nil {
				return err
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
	cmd.Flags().BoolVar(&passwordIn, "password-stdin", false, "Read password from stdin (required for non-interactive use)")

	return cmd
}

func newSecretsListCmd() *cobra.Command {
	var (
		labels []string
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
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster use' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}
			format := string(getGlobalOptions(cmd).Output)
			if format == string(OutputText) {
				format = "table"
			}
			out := cmd.OutOrStdout()

			switch backend {
			case "barbican":
				return listBarbicanSecrets(cmd.Context(), out, cfg, labels, format)
			case "sops":
				return listSOPSSecrets(cmd.Context(), out, cfg, format)
			case "file":
				return listFileSecrets(out, cfg, format)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().StringArrayVar(&labels, "label", []string{}, "Filter secrets by labels in key=value form")
	return cmd
}

func listBarbicanSecrets(ctx context.Context, out io.Writer, cfg *v2.Config, labels []string, format string) error {
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
		return json.NewEncoder(out).Encode(secrets)
	case "yaml":
		return yaml.NewEncoder(out).Encode(secrets)
	default:
		w := tabwriter.NewWriter(out, 0, 0, 3, ' ', tabwriter.AlignRight)
		fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tCREATED")
		for _, secret := range secrets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", secret.Name, secret.SecretType, secret.Status, secret.Created)
		}
		return w.Flush()
	}
}

func listSOPSSecrets(ctx context.Context, out io.Writer, cfg *v2.Config, format string) error {
	return listConfigMappedSecrets(out, cfg, format)
}

func listFileSecrets(out io.Writer, cfg *v2.Config, format string) error {
	return listConfigMappedSecrets(out, cfg, format)
}

func newSecretsDescribeCmd() *cobra.Command {
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
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster use' to set an active cluster")
				}
				clusterName = activeCluster
			}

			// Use resolveBackend helper
			backend, cfg, err := resolveBackend(cmd.Context(), clusterName)
			if err != nil {
				return err
			}
			format := string(getGlobalOptions(cmd).Output)
			if format == string(OutputText) {
				format = "table"
			}
			out := cmd.OutOrStdout()

			switch backend {
			case "barbican":
				return describeBarbicanSecret(cmd.Context(), out, cfg, name, format)
			case "sops":
				return describeSOPSSecret(cmd.Context(), cfg, name, format)
			case "file":
				return describeFileSecret(out, cfg, name, format)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	return cmd
}

func describeBarbicanSecret(ctx context.Context, out io.Writer, cfg *v2.Config, name string, format string) error {
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
		return json.NewEncoder(out).Encode(secret)
	case "yaml":
		return yaml.NewEncoder(out).Encode(secret)
	default:
		if _, err := fmt.Fprintf(out, "Name: %s\n", secret.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Type: %s\n", secret.SecretType); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Status: %s\n", secret.Status); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Created: %s\n", secret.Created); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "Content Types: %v\n", secret.ContentTypes)
		return err
	}
}

func describeSOPSSecret(ctx context.Context, cfg *v2.Config, name string, format string) error {
	return fmt.Errorf("SOPS backend does not support individual secret describe operations.\n\n" +
		"SOPS manages secrets as encrypted YAML files, not individual key-value pairs.\n" +
		"To inspect SOPS-encrypted secrets:\n" +
		"  opencenter secrets decrypt    Decrypt YAML files to view secret metadata\n" +
		"  opencenter secrets encrypt    Re-encrypt YAML files after inspection\n" +
		"  opencenter secrets status     Show encryption status of secret files\n\n" +
		"See: https://docs.opencenter.cloud/secrets/sops-encryption")
}

func describeFileSecret(out io.Writer, cfg *v2.Config, name string, format string) error {
	return describeConfigSecret(out, cfg, name, format)
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
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster use' to set an active cluster")
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
				return getFileSecret(cmd.Context(), cfg, name, outputFile, show)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Path to save the secret")
	cmd.Flags().BoolVar(&show, "show", false, "Print secret to stdout (insecure)")
	return cmd
}

func getBarbicanSecret(ctx context.Context, cfg *v2.Config, name string, outputFile string, show bool) error {
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

func getSOPSSecret(ctx context.Context, cfg *v2.Config, name string, outputFile string, show bool) error {
	return fmt.Errorf("SOPS backend does not support individual secret get operations.\n\n" +
		"SOPS manages secrets as encrypted YAML files, not individual key-value pairs.\n" +
		"To work with SOPS secrets:\n" +
		"  opencenter secrets decrypt    Decrypt YAML files to view secret values\n" +
		"  opencenter secrets encrypt    Re-encrypt YAML files after editing\n" +
		"  opencenter secrets status     Show encryption status of secret files\n\n" +
		"See: https://docs.opencenter.cloud/secrets/sops-encryption")
}

func getFileSecret(ctx context.Context, cfg *v2.Config, name string, outputFile string, show bool) error {
	_ = ctx
	return getConfigSecret(cfg, name, outputFile, show)
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
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster use' to set an active cluster")
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
				return setFileSecret(cmd.Context(), cfg, name, payload)
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

func setBarbicanSecret(ctx context.Context, cfg *v2.Config, name string, payload []byte, labels []string, secretType string, payloadContentEncoding string) error {
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

func setSOPSSecret(ctx context.Context, cfg *v2.Config, name string, payload []byte) error {
	return fmt.Errorf("SOPS backend does not support individual secret set operations.\n\n" +
		"SOPS manages secrets as encrypted YAML files, not individual key-value pairs.\n" +
		"To create or update SOPS secrets:\n" +
		"  opencenter secrets decrypt    Decrypt YAML files for editing\n" +
		"  opencenter secrets encrypt    Re-encrypt YAML files after changes\n" +
		"  opencenter secrets status     Show encryption status of secret files\n\n" +
		"See: https://docs.opencenter.cloud/secrets/sops-encryption")
}

func setFileSecret(ctx context.Context, cfg *v2.Config, name string, payload []byte) error {
	if err := setConfigSecret(ctx, cfg, name, payload); err != nil {
		return err
	}
	fmt.Printf("Secret '%s' created/updated successfully\n", name)
	return nil
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
					return fmt.Errorf("no cluster specified and no active cluster set. Use --cluster flag or 'opencenter cluster use' to set an active cluster")
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
				return deleteFileSecret(cmd.Context(), cfg, name)
			default:
				return fmt.Errorf("unsupported secrets backend: %s (supported: barbican, sops, file)", backend)
			}
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion of a secret")
	return cmd
}

func deleteBarbicanSecret(ctx context.Context, cfg *v2.Config, name string) error {
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

func deleteSOPSSecret(ctx context.Context, cfg *v2.Config, name string) error {
	return fmt.Errorf("SOPS backend does not support individual secret delete operations.\n\n" +
		"SOPS manages secrets as encrypted YAML files, not individual key-value pairs.\n" +
		"To remove secrets from SOPS-encrypted files:\n" +
		"  opencenter secrets decrypt    Decrypt YAML files for editing\n" +
		"  opencenter secrets encrypt    Re-encrypt YAML files after removing entries\n" +
		"  opencenter secrets status     Show encryption status of secret files\n\n" +
		"See: https://docs.opencenter.cloud/secrets/sops-encryption")
}

func deleteFileSecret(ctx context.Context, cfg *v2.Config, name string) error {
	if err := deleteConfigSecret(ctx, cfg, name); err != nil {
		return err
	}
	fmt.Printf("Secret '%s' deleted successfully\n", name)
	return nil
}
