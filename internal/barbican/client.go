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

package barbican

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/keymanager/v1/secrets"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
)

// Client is a wrapper around the Barbican client from gophercloud.
type Client struct {
	client         *gophercloud.ServiceClient
	config         *config.BarbicanConfig
	retryHandler   resilience.RetryHandler
	circuitBreaker resilience.CircuitBreaker
}

// NewClient creates a new Barbican client.
func NewClient(cfg *config.BarbicanConfig) (*Client, error) {
	if cfg.AuthURL == "" {
		return nil, fmt.Errorf("barbican backend requires auth_url to be configured\n\n" +
			"The 'opencenter secrets' commands are designed for Barbican (OpenStack Key Manager).\n" +
			"Your cluster is likely configured to use a different secrets backend (sops or file).\n\n" +
			"To fix this issue, choose one of the following options:\n\n" +
			"1. If you need Barbican, configure authentication:\n" +
			"   opencenter cluster edit <cluster-name>\n" +
			"   # Set opencenter.secrets.backend to 'barbican'\n" +
			"   # Set opencenter.secrets.barbican.auth_url to your OpenStack Keystone endpoint\n" +
			"   # Example: https://keystone.example.com:5000/v3\n\n" +
			"2. If using SOPS backend (recommended for GitOps):\n" +
			"   # SOPS secrets are managed through GitOps manifests, not via CLI commands\n" +
			"   # Use: opencenter cluster sync-secrets <cluster-name>\n" +
			"   # Use: opencenter cluster validate-secrets <cluster-name>\n\n" +
			"3. If using file-based secrets:\n" +
			"   # File secrets are stored in the cluster configuration\n" +
			"   # Edit directly: opencenter cluster edit <cluster-name>")
	}

	provider, err := openstack.NewClient(cfg.AuthURL)
	if err != nil {
		return nil, fmt.Errorf("could not create OpenStack client: %w", err)
	}

	token, err := LoadToken()
	if err == nil && token != "" {
		provider.TokenID = token
	} else {
		if cfg.UserDomainName == "" {
			cfg.UserDomainName = "Default"
		}
		err = openstack.Authenticate(provider, gophercloud.AuthOptions{
			IdentityEndpoint: cfg.AuthURL,
			Username:         os.Getenv("OS_USERNAME"),
			Password:         os.Getenv("OS_PASSWORD"),
			TenantID:         cfg.ProjectID,
			DomainName:       cfg.UserDomainName,
			Scope: &gophercloud.AuthScope{
				ProjectID: cfg.ProjectID,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("could not authenticate: %w", err)
		}
	}

	client, err := openstack.NewKeyManagerV1(provider, gophercloud.EndpointOpts{
		Region: cfg.Region,
	})
	client.Endpoint = strings.TrimSuffix(client.Endpoint, "/") + "/v1/"
	if err != nil {
		return nil, fmt.Errorf("could not create Barbican client: %w", err)
	}

	// Create retry handler with API call configuration
	retryHandler := resilience.NewRetryHandler(resilience.DefaultConfig)

	// Create circuit breaker for OpenStack API calls
	circuitBreaker := resilience.NewCircuitBreaker(resilience.DefaultCircuitBreakerConfig)

	return &Client{
		client:         client,
		config:         cfg,
		retryHandler:   retryHandler,
		circuitBreaker: circuitBreaker,
	}, nil
}

// Login authenticates with Keystone and returns a token.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	return Authenticate(ctx, c.config, username, password)
}

// GetSecret retrieves a secret from Barbican.
func (c *Client) GetSecret(ctx context.Context, name string) ([]byte, error) {
	var result []byte
	// Wrap with circuit breaker
	err := c.circuitBreaker.Call(ctx, func() error {
		// Then wrap with retry handler
		return c.retryHandler.Do(ctx, func() error {
			listOpts := secrets.ListOpts{
				Name: name,
			}
			allPages, err := secrets.List(c.client, listOpts).AllPages()
			if err != nil {
				return fmt.Errorf("failed to list secrets for retrieval: %w", err)
			}
			allSecrets, err := secrets.ExtractSecrets(allPages)
			if err != nil {
				return fmt.Errorf("failed to extract secrets: %w", err)
			}
			if len(allSecrets) == 0 {
				return fmt.Errorf("secret '%s' not found", name)
			}

			payload, err := secrets.GetPayload(c.client, allSecrets[0].SecretRef, nil).Extract()
			if err != nil {
				return fmt.Errorf("failed to get secret payload: %w", err)
			}
			result = []byte(strings.Trim(string(payload), `"`))
			return nil
		})
	})
	return result, err
}

// PutSecret creates or updates a secret in Barbican.
func (c *Client) PutSecret(ctx context.Context, name string, payload []byte, labels map[string]string, secretType, payloadContentEncoding string) error {
	return c.circuitBreaker.Call(ctx, func() error {
		return c.retryHandler.Do(ctx, func() error {
			existingSecret, err := c.DescribeSecret(ctx, name)
			if err == nil && existingSecret != nil {
				err = c.DeleteSecret(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to delete existing secret %s for update: %w", name, err)
				}
			}

			var tags []string
			for k, v := range labels {
				tags = append(tags, fmt.Sprintf("%s=%s", k, v))
			}

			createOpts := createOptsWithTags{
				CreateOpts: secrets.CreateOpts{
					Name:                   name,
					Payload:                string(payload),
					SecretType:             secrets.SecretType(secretType),
					PayloadContentEncoding: payloadContentEncoding,
				},
				Tags: tags,
			}

			_, err = secrets.Create(c.client, createOpts).Extract()
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}
			return nil
		})
	})
}

type createOptsWithTags struct {
	secrets.CreateOpts
	Tags []string `json:"tags,omitempty"`
}

func (opts createOptsWithTags) ToSecretCreateMap() (map[string]interface{}, error) {
	b, err := opts.CreateOpts.ToSecretCreateMap()
	if err != nil {
		return nil, err
	}
	if len(opts.Tags) > 0 {
		b["tags"] = opts.Tags
	}
	return b, nil
}

// ListSecrets lists secrets in Barbican.
func (c *Client) ListSecrets(ctx context.Context, labels map[string]string) ([]secrets.Secret, error) {
	var allSecrets []secrets.Secret
	err := c.circuitBreaker.Call(ctx, func() error {
		return c.retryHandler.Do(ctx, func() error {
			listURL := c.client.ServiceURL("secrets")

			if len(labels) > 0 {
				query := url.Values{}
				for k, v := range labels {
					query.Add("tag", fmt.Sprintf("%s=%s", k, v))
				}
				listURL += "?" + query.Encode()
			}

			allSecrets = nil // Reset on retry
			pager := pagination.NewPager(c.client, listURL, func(r pagination.PageResult) pagination.Page {
				return secrets.SecretPage{LinkedPageBase: pagination.LinkedPageBase{PageResult: r}}
			})

			err := pager.EachPage(func(page pagination.Page) (bool, error) {
				secretList, err := secrets.ExtractSecrets(page)
				if err != nil {
					return false, err
				}
				allSecrets = append(allSecrets, secretList...)
				return true, nil
			})
			if err != nil {
				return fmt.Errorf("failed to list secrets: %w", err)
			}
			return nil
		})
	})
	return allSecrets, err
}

// DescribeSecret describes a secret in Barbican.
func (c *Client) DescribeSecret(ctx context.Context, name string) (*secrets.Secret, error) {
	var detailedSecret *secrets.Secret
	err := c.circuitBreaker.Call(ctx, func() error {
		return c.retryHandler.Do(ctx, func() error {
			listOpts := secrets.ListOpts{
				Name: name,
			}
			allPages, err := secrets.List(c.client, listOpts).AllPages()
			if err != nil {
				return fmt.Errorf("failed to list secrets for description: %w", err)
			}
			allSecrets, err := secrets.ExtractSecrets(allPages)
			if err != nil {
				return fmt.Errorf("failed to extract secrets: %w", err)
			}
			if len(allSecrets) == 0 {
				return fmt.Errorf("secret '%s' not found", name)
			}
			detailedSecret, err = secrets.Get(c.client, allSecrets[0].SecretRef).Extract()
			if err != nil {
				return fmt.Errorf("failed to get secret details: %w", err)
			}
			return nil
		})
	})
	return detailedSecret, err
}

// DeleteSecret deletes a secret from Barbican.
func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	return c.circuitBreaker.Call(ctx, func() error {
		return c.retryHandler.Do(ctx, func() error {
			listOpts := secrets.ListOpts{
				Name: name,
			}
			allPages, err := secrets.List(c.client, listOpts).AllPages()
			if err != nil {
				return fmt.Errorf("failed to list secrets for deletion: %w", err)
			}
			allSecrets, err := secrets.ExtractSecrets(allPages)
			if err != nil {
				return fmt.Errorf("failed to extract secrets: %w", err)
			}
			if len(allSecrets) == 0 {
				return fmt.Errorf("secret '%s' not found", name)
			}

			err = secrets.Delete(c.client, allSecrets[0].SecretRef).ExtractErr()
			if err != nil {
				return fmt.Errorf("failed to delete secret: %w", err)
			}
			return nil
		})
	})
}
