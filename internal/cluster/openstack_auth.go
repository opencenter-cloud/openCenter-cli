// Copyright 2026 OpenCenter contributors
// Licensed under the Apache License, Version 2.0 (the "License");

package cluster

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"

	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// authenticateOpenStack builds a Keystone-authenticated provider client with a
// 30-second HTTP timeout. Shared by the Cinder, Octavia, and any other
// pre-destroy cleanup clients so the credential handling stays in one place.
// The explicit timeout is needed because gophercloud v1.14.1 does not honor
// context deadlines on its own.
func authenticateOpenStack(creds *credentials.OpenStackCredentials) (*gophercloud.ProviderClient, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint:            creds.AuthURL,
		ApplicationCredentialID:     creds.ApplicationCredentialID,
		ApplicationCredentialSecret: creds.ApplicationCredentialSecret,
		AllowReauth:                 true,
	}

	// Fall back to username/password when application credentials are absent.
	if creds.ApplicationCredentialID == "" {
		authOpts.Username = creds.Username
		authOpts.Password = creds.Password
		authOpts.DomainName = creds.Domain
		if authOpts.DomainName == "" {
			authOpts.DomainName = creds.UserDomainName
		}
		if creds.ProjectName != "" {
			authOpts.TenantName = creds.ProjectName
		} else {
			authOpts.TenantName = creds.TenantName
		}
	}

	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		return nil, fmt.Errorf("authenticate with OpenStack: %w", err)
	}
	provider.HTTPClient = http.Client{Timeout: 30 * time.Second}
	return provider, nil
}
