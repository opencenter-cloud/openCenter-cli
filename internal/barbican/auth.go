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
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
)

// Authenticate handles the authentication against Keystone and returns a token.
func Authenticate(ctx context.Context, cfg *config.BarbicanConfig, username, password string) (string, error) {
	provider, err := openstack.NewClient(cfg.AuthURL)
	if err != nil {
		return "", fmt.Errorf("could not create OpenStack client: %w", err)
	}

	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: cfg.AuthURL,
		Username:         username,
		Password:         password,
		TenantID:         cfg.ProjectID,
		DomainName:       cfg.UserDomainName,
		Scope: &gophercloud.AuthScope{
			ProjectID: cfg.ProjectID,
		},
	}

	err = openstack.Authenticate(provider, authOpts)
	if err != nil {
		return "", fmt.Errorf("could not authenticate: %w", err)
	}

	return provider.Token(), nil
}
