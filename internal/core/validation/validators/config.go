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

package validators

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// ConfigValidator validates configuration values.
type ConfigValidator struct {
	clusterNameValidator *ClusterNameValidator
	emailPattern         *regexp.Regexp
	domainPattern        *regexp.Regexp
}

// NewConfigValidator creates a new configuration validator.
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		clusterNameValidator: NewClusterNameValidator(),
		emailPattern:         regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		domainPattern:        regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`),
	}
}

// Name returns the validator name.
func (v *ConfigValidator) Name() string {
	return "config"
}

// Validate validates a configuration value based on the context.
// The value should be a map with "type" and "value" keys.
func (v *ConfigValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := &validation.ValidationResult{
		Valid:    true,
		Errors:   []*validation.ValidationIssue{},
		Warnings: []*validation.ValidationIssue{},
		Info:     []*validation.ValidationIssue{},
	}

	configMap, ok := value.(map[string]interface{})
	if !ok {
		result.AddError("config", "value must be a map with 'type' and 'value' keys")
		return result, nil
	}

	configType, ok := configMap["type"].(string)
	if !ok {
		result.AddError("config", "missing or invalid 'type' field")
		return result, nil
	}

	configValue := configMap["value"]

	switch configType {
	case "email":
		v.validateEmail(result, configValue)
	case "domain":
		v.validateDomain(result, configValue)
	case "fqdn":
		v.validateFQDN(result, configValue)
	case "url":
		v.validateURL(result, configValue)
	case "ip":
		v.validateIP(result, configValue)
	case "cidr":
		v.validateCIDR(result, configValue)
	case "port":
		v.validatePort(result, configValue)
	case "cluster-name":
		// Delegate to cluster name validator
		clusterResult, err := v.clusterNameValidator.Validate(ctx, configValue)
		if err != nil {
			return nil, err
		}
		result.Merge(clusterResult)
	default:
		result.AddWarning("config", fmt.Sprintf("unknown config type '%s', skipping validation", configType))
	}

	return result, nil
}

// validateEmail validates an email address.
func (v *ConfigValidator) validateEmail(result *validation.ValidationResult, value interface{}) {
	email, ok := value.(string)
	if !ok {
		result.AddError("email", "email must be a string")
		return
	}

	if email == "" {
		result.AddError("email", "email cannot be empty")
		return
	}

	if !v.emailPattern.MatchString(email) {
		result.AddError("email", "invalid email format",
			"Email must be in format: user@domain.com",
			"Example: admin@example.com")
		return
	}

	// Additional checks
	if len(email) > 254 {
		result.AddError("email", "email is too long (maximum 254 characters)")
		return
	}

	parts := strings.Split(email, "@")
	if len(parts[0]) > 64 {
		result.AddError("email", "email local part is too long (maximum 64 characters)")
		return
	}
}

// validateDomain validates a domain name.
func (v *ConfigValidator) validateDomain(result *validation.ValidationResult, value interface{}) {
	domain, ok := value.(string)
	if !ok {
		result.AddError("domain", "domain must be a string")
		return
	}

	if domain == "" {
		result.AddError("domain", "domain cannot be empty")
		return
	}

	if len(domain) > 253 {
		result.AddError("domain", "domain is too long (maximum 253 characters)")
		return
	}

	if !v.domainPattern.MatchString(domain) {
		result.AddError("domain", "invalid domain format",
			"Domain must contain only alphanumeric characters, hyphens, and dots",
			"Example: example.com or sub.example.com")
		return
	}

	// Check for consecutive dots
	if strings.Contains(domain, "..") {
		result.AddError("domain", "domain cannot contain consecutive dots")
		return
	}

	// Check label lengths
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) > 63 {
			result.AddError("domain", fmt.Sprintf("domain label '%s' is too long (maximum 63 characters)", label))
			return
		}
	}
}

// validateFQDN validates a fully qualified domain name.
func (v *ConfigValidator) validateFQDN(result *validation.ValidationResult, value interface{}) {
	fqdn, ok := value.(string)
	if !ok {
		result.AddError("fqdn", "FQDN must be a string")
		return
	}

	// FQDN is essentially a domain name
	v.validateDomain(result, value)

	// Update field name
	for _, issue := range result.Errors {
		if issue.Field == "domain" {
			issue.Field = "fqdn"
		}
	}

	// Additional FQDN-specific checks
	if !strings.Contains(fqdn, ".") {
		result.AddWarning("fqdn", "FQDN should contain at least one dot (e.g., host.domain.com)")
	}
}

// validateURL validates a URL.
func (v *ConfigValidator) validateURL(result *validation.ValidationResult, value interface{}) {
	urlStr, ok := value.(string)
	if !ok {
		result.AddError("url", "URL must be a string")
		return
	}

	if urlStr == "" {
		result.AddError("url", "URL cannot be empty")
		return
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		result.AddError("url", fmt.Sprintf("invalid URL format: %v", err),
			"Ensure URL is properly formatted",
			"Example: https://example.com/path")
		return
	}

	// Check scheme
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		if scheme == "" {
			result.AddError("url", "URL must include a scheme (http:// or https://)")
		} else {
			result.AddError("url", fmt.Sprintf("unsupported URL scheme: %s (must be http or https)", scheme))
		}
		return
	}

	// Check host
	if parsedURL.Host == "" {
		result.AddError("url", "URL must include a host")
		return
	}

	// Warn about HTTP for external URLs
	host := strings.ToLower(parsedURL.Hostname())
	isLocal := host == "localhost" ||
		host == "127.0.0.1" ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "172.16.") ||
		strings.HasPrefix(host, "172.17.") ||
		strings.HasPrefix(host, "172.18.") ||
		strings.HasPrefix(host, "172.19.") ||
		strings.HasPrefix(host, "172.20.") ||
		strings.HasPrefix(host, "172.21.") ||
		strings.HasPrefix(host, "172.22.") ||
		strings.HasPrefix(host, "172.23.") ||
		strings.HasPrefix(host, "172.24.") ||
		strings.HasPrefix(host, "172.25.") ||
		strings.HasPrefix(host, "172.26.") ||
		strings.HasPrefix(host, "172.27.") ||
		strings.HasPrefix(host, "172.28.") ||
		strings.HasPrefix(host, "172.29.") ||
		strings.HasPrefix(host, "172.30.") ||
		strings.HasPrefix(host, "172.31.")

	if !isLocal && scheme == "http" {
		result.AddWarning("url", "external URLs should use HTTPS for security",
			"Consider using https:// instead of http://")
	}
}

// validateIP validates an IP address.
func (v *ConfigValidator) validateIP(result *validation.ValidationResult, value interface{}) {
	ipStr, ok := value.(string)
	if !ok {
		result.AddError("ip", "IP address must be a string")
		return
	}

	if ipStr == "" {
		result.AddError("ip", "IP address cannot be empty")
		return
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		result.AddError("ip", "invalid IP address format",
			"IP must be in IPv4 (e.g., 192.168.1.1) or IPv6 format",
			"Example IPv4: 10.0.0.1",
			"Example IPv6: 2001:db8::1")
		return
	}

	// Add info about IP version
	if ip.To4() != nil {
		result.AddInfo("ip", "valid IPv4 address")
	} else {
		result.AddInfo("ip", "valid IPv6 address")
	}
}

// validateCIDR validates a CIDR notation.
func (v *ConfigValidator) validateCIDR(result *validation.ValidationResult, value interface{}) {
	cidrStr, ok := value.(string)
	if !ok {
		result.AddError("cidr", "CIDR must be a string")
		return
	}

	if cidrStr == "" {
		result.AddError("cidr", "CIDR cannot be empty")
		return
	}

	ip, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		result.AddError("cidr", fmt.Sprintf("invalid CIDR format: %v", err),
			"CIDR must be in format: IP/prefix",
			"Example IPv4: 192.168.1.0/24",
			"Example IPv6: 2001:db8::/32")
		return
	}

	// Check if IP is the network address
	if !ip.Equal(ipNet.IP) {
		result.AddWarning("cidr",
			fmt.Sprintf("IP address %s is not the network address (should be %s)", ip, ipNet.IP),
			fmt.Sprintf("Consider using %s instead", ipNet.String()))
	}

	// Add info about CIDR
	ones, bits := ipNet.Mask.Size()
	result.AddInfo("cidr", fmt.Sprintf("valid CIDR with /%d prefix (%d-bit address space)", ones, bits))
}

// validatePort validates a port number.
func (v *ConfigValidator) validatePort(result *validation.ValidationResult, value interface{}) {
	var port int

	switch v := value.(type) {
	case int:
		port = v
	case int64:
		port = int(v)
	case float64:
		port = int(v)
	case string:
		// Try to parse string as int
		_, err := fmt.Sscanf(v, "%d", &port)
		if err != nil {
			result.AddError("port", "port must be a number")
			return
		}
	default:
		result.AddError("port", "port must be a number")
		return
	}

	if port < 1 || port > 65535 {
		result.AddError("port", fmt.Sprintf("port %d is out of valid range (1-65535)", port),
			"Use a port number between 1 and 65535")
		return
	}

	// Warn about privileged ports
	if port < 1024 {
		result.AddWarning("port",
			fmt.Sprintf("port %d is a privileged port (< 1024) and may require elevated permissions", port),
			"Consider using a port >= 1024 for non-system services")
	}

	// Warn about commonly used ports
	commonPorts := map[int]string{
		22:   "SSH",
		80:   "HTTP",
		443:  "HTTPS",
		3306: "MySQL",
		5432: "PostgreSQL",
		6379: "Redis",
		8080: "HTTP alternate",
		9090: "Prometheus",
	}

	if service, exists := commonPorts[port]; exists {
		result.AddInfo("port", fmt.Sprintf("port %d is commonly used for %s", port, service))
	}
}
