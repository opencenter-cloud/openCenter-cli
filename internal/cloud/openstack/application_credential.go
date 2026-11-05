package openstack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gophercloud/gophercloud"
	gopenstack "github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"gopkg.in/yaml.v3"
)

// AppCredentialProvisioner creates an OpenStack application credential for the
// authenticated profile user. The real implementation talks to Keystone; tests
// replace it to exercise the persist-and-return flow without a live cloud.
type AppCredentialProvisioner interface {
	CreateApplicationCredential(ctx context.Context, profile Profile, name, description string) (AppCredential, error)
}

// AppCredential (the minted ID/Secret pair) is declared in storage.go and
// reused here.

// DefaultApplicationCredentialName is the credential name used when the
// caller does not choose one.
const DefaultApplicationCredentialName = "opencenter"

// gophercloudAppCredentialProvisioner authenticates with the profile's own
// credentials and creates an unrestricted application credential owned by the
// authenticated user.
type gophercloudAppCredentialProvisioner struct{}

// NewAppCredentialProvisioner returns the live Keystone provisioner.
func NewAppCredentialProvisioner() AppCredentialProvisioner {
	return gophercloudAppCredentialProvisioner{}
}

func (gophercloudAppCredentialProvisioner) CreateApplicationCredential(ctx context.Context, profile Profile, name, description string) (AppCredential, error) {
	provider, err := profile.provider(ctx)
	if err != nil {
		return AppCredential{}, err
	}
	userID, err := authenticatedUserID(provider)
	if err != nil {
		return AppCredential{}, err
	}
	identity, err := gopenstack.NewIdentityV3(provider, profile.endpointOpts())
	if err != nil {
		return AppCredential{}, fmt.Errorf("create identity client: %w", err)
	}
	credential, err := applicationcredentials.Create(identity, userID, applicationcredentials.CreateOpts{
		Name:        name,
		Description: description,
		// No access rules: the credential is the cluster's
		// infrastructure-admin identity. OpenTofu, CCM, Designate and
		// the readiness re-checks all act through it, so restricting
		// it to a hand-curated rule list would break provisioning in
		// ways the operator cannot see from the config file.
		Unrestricted: true,
	}).Extract()
	if err != nil {
		return AppCredential{}, fmt.Errorf("create application credential: %s", maskProfileSecrets(err.Error(), profile))
	}
	if strings.TrimSpace(credential.ID) == "" || strings.TrimSpace(credential.Secret) == "" {
		return AppCredential{}, fmt.Errorf("keystone returned an incomplete application credential")
	}
	return AppCredential{ID: credential.ID, Secret: credential.Secret}, nil
}

// EnsureApplicationCredential returns a persistable application credential ID
// and secret for the profile, minting one when the profile has none.
//
// The three profile credential shapes behave differently:
//
//   - ID + secret: already persistable; returned unchanged, nothing created.
//   - name + secret: cannot be persisted. The cluster config stores an
//     application-credential ID (OpenTofu and the rendered services consume
//     the ID), and a name is only resolvable against the issuing user, so a
//     name-only credential is replaced by a freshly minted ID/secret pair.
//   - no credential at all (username/password profile): a credential is
//     minted so the cluster never depends on a raw password.
//
// A minted pair is written back into the profile's clouds.yaml before being
// returned: the secret is only available in the create response, so a failure
// to persist is fatal rather than a silent loss. The write preserves the rest
// of the file and is performed atomically with 0600 permissions.
func EnsureApplicationCredential(ctx context.Context, profile Profile, provisioner AppCredentialProvisioner, name string) (AppCredential, Profile, error) {
	if err := ctx.Err(); err != nil {
		return AppCredential{}, profile, err
	}
	if strings.TrimSpace(profile.AppCredID) != "" && strings.TrimSpace(profile.AppCredSecret) != "" {
		return AppCredential{ID: strings.TrimSpace(profile.AppCredID), Secret: profile.AppCredSecret}, profile, nil
	}
	if provisioner == nil {
		return AppCredential{}, profile, fmt.Errorf("application credential provisioner is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultApplicationCredentialName
	}
	// Validate the persistence target BEFORE minting: the secret is only
	// ever returned by the create call, so minting into a profile with no
	// writable clouds.yaml would create a credential that is immediately
	// unrecoverable.
	if strings.TrimSpace(profile.SourcePath) == "" {
		return AppCredential{}, profile, fmt.Errorf("cloud profile %q has no clouds.yaml path; cannot persist the application credential", profile.Name)
	}
	description := fmt.Sprintf("openCenter cluster credential for profile %s", profile.Name)
	created, err := provisioner.CreateApplicationCredential(ctx, profile, name, description)
	if err != nil {
		return AppCredential{}, profile, err
	}
	updated, err := persistApplicationCredential(profile, created, name)
	if err != nil {
		return AppCredential{}, profile, err
	}
	return created, updated, nil
}

// persistApplicationCredential writes a minted credential pair into the
// profile's clouds.yaml and returns the profile reloaded from it, so callers
// observe exactly what is on disk.
func persistApplicationCredential(profile Profile, credential AppCredential, name string) (Profile, error) {
	if strings.TrimSpace(profile.SourcePath) == "" {
		return Profile{}, fmt.Errorf("cloud profile %q has no clouds.yaml path; cannot persist the application credential", profile.Name)
	}
	data, err := os.ReadFile(profile.SourcePath)
	if err != nil {
		return Profile{}, fmt.Errorf("read clouds.yaml for credential persistence: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Profile{}, fmt.Errorf("parse clouds.yaml for credential persistence: %w", err)
	}
	if doc.Kind == 0 {
		return Profile{}, fmt.Errorf("clouds.yaml %s is empty", profile.SourcePath)
	}
	clouds := yamlMapValue(&doc, "clouds")
	if clouds == nil {
		return Profile{}, fmt.Errorf("clouds.yaml %s has no clouds mapping", profile.SourcePath)
	}
	profileNode := yamlMapValue(clouds, profile.Name)
	if profileNode == nil {
		return Profile{}, fmt.Errorf("cloud profile %q not found in %s", profile.Name, profile.SourcePath)
	}
	auth := yamlMapValue(profileNode, "auth")
	if auth == nil {
		auth = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		yamlMapSet(profileNode, "auth", auth)
	}
	yamlMapSet(auth, "application_credential_id", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: credential.ID})
	yamlMapSet(auth, "application_credential_secret", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: credential.Secret})
	// The name is written alongside the ID so openstacksdk/OSC users of the
	// same file see which credential the pair belongs to. gophercloud's
	// AuthOptions prefers the ID over the name whenever both are set, so a
	// stale name cannot hijack authentication here.
	yamlMapSet(auth, "application_credential_name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name})

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return Profile{}, fmt.Errorf("marshal clouds.yaml: %w", err)
	}
	if err := writeFileAtomic0600(profile.SourcePath, out); err != nil {
		return Profile{}, fmt.Errorf("persist application credential to clouds.yaml: %w", err)
	}
	return LoadProfile(profile.SourcePath, profile.Name)
}

// writeFileAtomic0600 writes data through a sibling temp file and renames it
// over path. clouds.yaml holds every cloud password the operator owns, so a
// partial write or a world-readable mode is not acceptable.
func writeFileAtomic0600(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".opencenter-clouds-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// yamlMapValue returns the value node stored under key in a mapping node.
func yamlMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	target := node
	if target.Kind == yaml.DocumentNode {
		if len(target.Content) == 0 {
			return nil
		}
		target = target.Content[0]
	}
	if target.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(target.Content); i += 2 {
		if target.Content[i].Value == key {
			return target.Content[i+1]
		}
	}
	return nil
}

// yamlMapSet replaces or appends key in a mapping node.
func yamlMapSet(node *yaml.Node, key string, value *yaml.Node) {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

// maskProfileSecrets removes the profile's own secrets from an error string so
// a Keystone failure can be surfaced without echoing credentials.
func maskProfileSecrets(message string, profile Profile) string {
	return security.MaskSecrets(message, profile.Password, profile.AppCredSecret)
}

// endpointOpts resolves the endpoint options for the profile's region and
// interface preference.
func (p Profile) endpointOpts() gophercloud.EndpointOpts {
	opts := gophercloud.EndpointOpts{Region: strings.TrimSpace(p.Region)}
	if strings.TrimSpace(p.Interface) != "" {
		opts.Availability = gophercloud.Availability(strings.TrimSpace(p.Interface))
	}
	return opts
}
