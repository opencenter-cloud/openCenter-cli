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
	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// Client is a wrapper around the Barbican client from gophercloud.
type Client struct {
	client *gophercloud.ServiceClient
	config *config.BarbicanConfig
}

// NewClient creates a new Barbican client.
func NewClient(cfg *config.BarbicanConfig) (*Client, error) {
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

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// Login authenticates with Keystone and returns a token.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	// Implementation to be added
	return "", fmt.Errorf("Login not implemented")
}

// GetSecret retrieves a secret from Barbican.
func (c *Client) GetSecret(ctx context.Context, name string) ([]byte, error) {
	listOpts := secrets.ListOpts{
		Name: name,
	}
	allPages, err := secrets.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	allSecrets, err := secrets.ExtractSecrets(allPages)
	if err != nil {
		return nil, err
	}
	if len(allSecrets) == 0 {
		return nil, fmt.Errorf("secret '%s' not found", name)
	}

	payload, err := secrets.GetPayload(c.client, allSecrets[0].SecretRef, nil).Extract()
	if err != nil {
		return nil, err
	}
	return []byte(strings.Trim(string(payload), `"`)), nil
}

// PutSecret creates or updates a secret in Barbican.
func (c *Client) PutSecret(ctx context.Context, name string, payload []byte, labels map[string]string) error {
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

	type secretRequest struct {
		Name                   string   `json:"name"`
		Payload                string   `json:"payload"`
		SecretType             string   `json:"secret_type"`
		PayloadContentEncoding string   `json:"payload_content_encoding"`
		Tags                   []string `json:"tags,omitempty"`
	}

	reqBody := secretRequest{
		Name:                   name,
		Payload:                string(payload),
		SecretType:             "opaque",
		PayloadContentEncoding: "base64",
		Tags:                   tags,
	}

	url := c.client.ServiceURL("secrets")
	var res gophercloud.Result
	_, err = c.client.Post(url, reqBody, &res.Body, &gophercloud.RequestOpts{
		OkCodes: []int{201},
	})
	return err
}

// ListSecrets lists secrets in Barbican.
func (c *Client) ListSecrets(ctx context.Context, labels map[string]string) ([]secrets.Secret, error) {
	listURL := c.client.ServiceURL("secrets")

	if len(labels) > 0 {
		query := url.Values{}
		for k, v := range labels {
			query.Add("tag", fmt.Sprintf("%s=%s", k, v))
		}
		listURL += "?" + query.Encode()
	}

	var allSecrets []secrets.Secret
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
		return nil, err
	}

	return allSecrets, nil
}

// DescribeSecret describes a secret in Barbican.
func (c *Client) DescribeSecret(ctx context.Context, name string) (*secrets.Secret, error) {
	listOpts := secrets.ListOpts{
		Name: name,
	}
	allPages, err := secrets.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	allSecrets, err := secrets.ExtractSecrets(allPages)
	if err != nil {
		return nil, err
	}
	if len(allSecrets) == 0 {
		return nil, fmt.Errorf("secret '%s' not found", name)
	}
	detailedSecret, err := secrets.Get(c.client, allSecrets[0].SecretRef).Extract()
	if err != nil {
		return nil, err
	}
	return detailedSecret, nil
}

// DeleteSecret deletes a secret from Barbican.
func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	listOpts := secrets.ListOpts{
		Name: name,
	}
	allPages, err := secrets.List(c.client, listOpts).AllPages()
	if err != nil {
		return err
	}
	allSecrets, err := secrets.ExtractSecrets(allPages)
	if err != nil {
		return err
	}
	if len(allSecrets) == 0 {
		return fmt.Errorf("secret '%s' not found", name)
	}

	return secrets.Delete(c.client, allSecrets[0].SecretRef).ExtractErr()
}
