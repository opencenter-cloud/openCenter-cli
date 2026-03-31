package gitops

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
)

var dns1123NamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func validateOverlayUnitConfig(cfg config.Config) error {
	if err := validateCustomerManagedOverlay(cfg); err != nil {
		return err
	}
	if err := validateSOPSOverlay(cfg); err != nil {
		return err
	}
	return nil
}

func validateCustomerManagedOverlay(cfg config.Config) error {
	customer := cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged
	if !customer.Enabled {
		return nil
	}

	if strings.TrimSpace(customer.RepositoryName) == "" {
		return fmt.Errorf("customer-managed overlay requires repository_name")
	}
	if !dns1123NamePattern.MatchString(customer.RepositoryName) {
		return fmt.Errorf("customer-managed overlay repository_name %q must be DNS-1123 compatible", customer.RepositoryName)
	}
	if strings.TrimSpace(customer.RepositoryURL) == "" {
		return fmt.Errorf("customer-managed overlay requires repository_url")
	}
	if len(customer.Kustomizations) == 0 {
		return fmt.Errorf("customer-managed overlay requires at least one kustomization")
	}

	repositoryURL, err := url.Parse(customer.RepositoryURL)
	if err != nil {
		return fmt.Errorf("customer-managed overlay repository_url %q is invalid: %w", customer.RepositoryURL, err)
	}

	switch repositoryURL.Scheme {
	case "ssh":
		if strings.TrimSpace(repositoryURL.Host) == "" {
			return fmt.Errorf("customer-managed overlay ssh repository_url %q requires a host", customer.RepositoryURL)
		}
		if customer.EmitSecret {
			secrets := cfg.Secrets.OverlayUnits.CustomerManaged
			if strings.TrimSpace(secrets.Identity) == "" {
				return fmt.Errorf("customer-managed overlay emit_secret requires secrets.overlay_units.customer_managed.identity")
			}
			if strings.TrimSpace(secrets.IdentityPub) == "" {
				return fmt.Errorf("customer-managed overlay emit_secret requires secrets.overlay_units.customer_managed.identity_pub")
			}
			if strings.TrimSpace(secrets.KnownHosts) == "" {
				return fmt.Errorf("customer-managed overlay emit_secret requires secrets.overlay_units.customer_managed.known_hosts")
			}
		}
	case "https":
		if customer.EmitSecret {
			return fmt.Errorf("customer-managed overlay emit_secret is only supported for ssh repository URLs")
		}
	default:
		return fmt.Errorf("customer-managed overlay repository_url %q must use ssh or https", customer.RepositoryURL)
	}

	for idx, kustomization := range customer.Kustomizations {
		if strings.TrimSpace(kustomization.Name) == "" {
			return fmt.Errorf("customer-managed overlay kustomizations[%d] requires name", idx)
		}
		if strings.TrimSpace(kustomization.Path) == "" {
			return fmt.Errorf("customer-managed overlay kustomizations[%d] requires path", idx)
		}
		if !strings.HasPrefix(kustomization.Path, "/") {
			return fmt.Errorf("customer-managed overlay kustomizations[%d] path %q must start with /", idx, kustomization.Path)
		}
	}

	return nil
}

func validateSOPSOverlay(cfg config.Config) error {
	sopsConfig := cfg.OpenCenter.GitOps.OverlayUnits.SOPS
	if !sopsConfig.Enabled {
		return nil
	}
	if len(sopsConfig.Rules) == 0 {
		return fmt.Errorf("sops overlay generation requires at least one rule")
	}

	for idx, rule := range sopsConfig.Rules {
		if strings.TrimSpace(rule.PathRegex) == "" {
			return fmt.Errorf("sops overlay generation rules[%d] requires path_regex", idx)
		}
		if len(rule.AgeRecipients) == 0 {
			return fmt.Errorf("sops overlay generation rules[%d] requires at least one age_recipient", idx)
		}
		for recipientIndex, recipient := range rule.AgeRecipients {
			if strings.TrimSpace(recipient) == "" {
				return fmt.Errorf("sops overlay generation rules[%d] age_recipients[%d] cannot be empty", idx, recipientIndex)
			}
		}
	}

	return nil
}
