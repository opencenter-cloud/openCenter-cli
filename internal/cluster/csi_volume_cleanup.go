// Copyright 2025 Dinesh Gupta <dinesh.gupta@rackspace.com>
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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"

	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

const cinderCSIDriver = "cinder.csi.openstack.org"

// cinderVolumeService abstracts Cinder volume operations so that tests can
// inject fakes without a live OpenStack endpoint.
type cinderVolumeService interface {
	// GetStatus returns the volume status (e.g. "available", "in-use") and
	// its size in GB, or an error when the volume cannot be found.
	GetStatus(ctx context.Context, id string) (status string, sizeGB int, err error)
	// Delete removes the volume permanently.
	Delete(ctx context.Context, id string) error
}

// gophercloudCinderService is the production cinderVolumeService backed by
// the gophercloud block-storage v3 API.
type gophercloudCinderService struct {
	client *gophercloud.ServiceClient
}

func (s *gophercloudCinderService) GetStatus(_ context.Context, id string) (string, int, error) {
	vol, err := volumes.Get(s.client, id).Extract()
	if err != nil {
		return "", 0, err
	}
	return vol.Status, vol.Size, nil
}

func (s *gophercloudCinderService) Delete(_ context.Context, id string) error {
	return volumes.Delete(s.client, id, volumes.DeleteOpts{}).ExtractErr()
}

// newGophercloudCinderService authenticates with Keystone and returns a
// block-storage service client with a 30-second timeout.
func newGophercloudCinderService(creds *credentials.OpenStackCredentials) (cinderVolumeService, error) {
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

	// Set a 30-second timeout on the provider's HTTP client to prevent indefinite
	// hangs against unresponsive Cinder endpoints. gophercloud v1.14.1 does not
	// honor context deadlines, so an explicit timeout is needed.
	provider.HTTPClient = &http.Client{Timeout: 30 * time.Second}

	client, err := openstack.NewBlockStorageV3(provider, gophercloud.EndpointOpts{Region: creds.Region})
	if err != nil {
		return nil, fmt.Errorf("create block storage client: %w", err)
	}

	return &gophercloudCinderService{client: client}, nil
}

// pvListOutput mirrors just the fields we need from `kubectl get pv -o json`.
type pvListOutput struct {
	Items []struct {
		Spec struct {
			CSI *struct {
				Driver       string `json:"driver"`
				VolumeHandle string `json:"volumeHandle"`
			} `json:"csi"`
		} `json:"spec"`
	} `json:"items"`
}

// collectCSIVolumeHandles runs kubectl against the cluster to enumerate
// PersistentVolumes backed by the OpenStack Cinder CSI driver and returns
// their Cinder volume IDs (the volumeHandle field).
//
// Returns an error when the kubeconfig is missing or kubectl fails. Callers
// should treat this as non-fatal during a destroy (the cluster may already be
// unreachable when this is called, which is expected).
func collectCSIVolumeHandles(ctx context.Context, kubeconfigPath string, runner lifecycleCommandRunner) ([]string, error) {
	if kubeconfigPath == "" {
		return nil, fmt.Errorf("kubeconfig path is empty")
	}
	if _, err := os.Stat(kubeconfigPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("kubeconfig not found at %s", kubeconfigPath)
		}
		return nil, fmt.Errorf("stat kubeconfig %s: %w", kubeconfigPath, err)
	}

	out, err := runner.Run(ctx, "", nil, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "persistentvolumes",
		"-o", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("kubectl get persistentvolumes: %w", err)
	}

	var pvList pvListOutput
	if err := json.Unmarshal(out, &pvList); err != nil {
		return nil, fmt.Errorf("parse kubectl output: %w", err)
	}

	var handles []string
	for _, pv := range pvList.Items {
		if pv.Spec.CSI == nil {
			continue
		}
		if pv.Spec.CSI.Driver != cinderCSIDriver {
			continue
		}
		if h := strings.TrimSpace(pv.Spec.CSI.VolumeHandle); h != "" {
			handles = append(handles, h)
		}
	}

	return handles, nil
}

// cleanupCSIVolumes deletes or reports orphaned Cinder volumes identified by
// their IDs. It is always safe to call: volumes that are not in the
// "available" state are silently skipped.
//
// With deleteVolumes=true each available volume is deleted via the Cinder API.
// With deleteVolumes=false the operator is warned and shown a manual command.
func cleanupCSIVolumes(ctx context.Context, handles []string, svc cinderVolumeService, deleteVolumes bool, output io.Writer) error {
	type volumeEntry struct {
		id     string
		sizeGB int
	}

	var available []volumeEntry
	var skipped []string

	for _, id := range handles {
		status, sizeGB, err := svc.GetStatus(ctx, id)
		if err != nil {
			// Distinguish "definitely not found" from transient errors.
			// Only 404 means the volume is already gone and safe to skip.
			// Other errors (5xx, timeout, auth failure) indicate a check failure
			// and should be counted as retrieval failures, not skipped.
			if isNotFoundError(err) {
				fmt.Fprintf(output, "  %s: already deleted (not found)\n", id)
				continue
			}
			fmt.Fprintf(output, "  Warning: could not check volume %s: %v\n", id, err)
			skipped = append(skipped, fmt.Sprintf("%s (error: %v)", id, err))
			continue
		}

		if status == "available" {
			available = append(available, volumeEntry{id: id, sizeGB: sizeGB})
		} else {
			skipped = append(skipped, fmt.Sprintf("%s (status: %s)", id, status))
		}
	}

	if len(skipped) > 0 {
		fmt.Fprintf(output, "  Skipping %d volume(s) not in 'available' state (still in use by another cluster):\n", len(skipped))
		for _, s := range skipped {
			fmt.Fprintf(output, "    - %s\n", s)
		}
	}

	if len(available) == 0 {
		fmt.Fprintf(output, "No orphaned CSI-provisioned Cinder volumes to clean up.\n")
		return nil
	}

	totalGB := 0
	for _, v := range available {
		totalGB += v.sizeGB
	}

	if !deleteVolumes {
		fmt.Fprintf(output, "\nWarning: %d orphaned CSI-provisioned Cinder volume(s) detected (%d GB total).\n", len(available), totalGB)
		fmt.Fprintf(output, "These volumes are consuming quota but are no longer in use by any cluster.\n")
		fmt.Fprintf(output, "Re-run with --delete-volumes to remove them automatically, or delete manually:\n\n")
		ids := make([]string, len(available))
		for i, v := range available {
			ids[i] = v.id
		}
		fmt.Fprintf(output, "  openstack volume delete %s\n\n", strings.Join(ids, " "))
		fmt.Fprintf(output, "Orphaned volume details:\n")
		for _, v := range available {
			fmt.Fprintf(output, "  - %s (%d GB)\n", v.id, v.sizeGB)
		}
		return nil
	}

	// Delete mode.
	fmt.Fprintf(output, "Deleting %d orphaned CSI-provisioned Cinder volume(s) (%d GB total)...\n", len(available), totalGB)
	var deleteErrors []string
	deleted := 0
	for _, v := range available {
		if err := svc.Delete(ctx, v.id); err != nil {
			fmt.Fprintf(output, "  - %s (%d GB): ERROR: %v\n", v.id, v.sizeGB, err)
			deleteErrors = append(deleteErrors, fmt.Sprintf("%s: %v", v.id, err))
		} else {
			fmt.Fprintf(output, "  - %s (%d GB): deleted\n", v.id, v.sizeGB)
			deleted++
		}
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete %d of %d volume(s): %s",
			len(deleteErrors), len(available), strings.Join(deleteErrors, "; "))
	}

	fmt.Fprintf(output, "Successfully deleted %d CSI volume(s) (%d GB freed).\n", deleted, totalGB)
	return nil
}

// isNotFoundError returns true if err is a definitive "volume not found" error
// from gophercloud (404). Other errors (5xx, timeout, auth failure) return false
// so they can be treated as retrieval failures, not "already deleted".
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// gophercloud wraps 404 responses as ErrDefault404 or generic error messages
	// containing "404" or "not found". Check the error string for these signals.
	errStr := err.Error()
	return strings.Contains(errStr, "404") || strings.Contains(strings.ToLower(errStr), "not found")
}
