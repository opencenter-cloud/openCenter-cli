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

package defaults

// gcpDefaults implements ProviderDefaults for GCP regions.
type gcpDefaults struct {
	imageIDs          map[string]string
	availabilityZones []string
	ntpServers        []string
	dnsNameservers    []string
	storageClass      string
	flavors           FlavorDefaults
}

func (d *gcpDefaults) GetImageID(osVersion string) string {
	if imageID, ok := d.imageIDs[osVersion]; ok {
		return imageID
	}
	// Return default Ubuntu 24.04 image if version not found
	return d.imageIDs["24"]
}

func (d *gcpDefaults) GetAvailabilityZones() []string {
	return d.availabilityZones
}

func (d *gcpDefaults) GetNTPServers() []string {
	return d.ntpServers
}

func (d *gcpDefaults) GetDNSNameservers() []string {
	return d.dnsNameservers
}

func (d *gcpDefaults) GetDefaultStorageClass() string {
	return d.storageClass
}

func (d *gcpDefaults) GetDefaultFlavors() FlavorDefaults {
	return d.flavors
}

// newGCPUSCentral1Defaults returns defaults for GCP us-central1 region.
func newGCPUSCentral1Defaults() ProviderDefaults {
	return &gcpDefaults{
		imageIDs: map[string]string{
			"22": "projects/ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20240319",
			"24": "projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-v20240523",
		},
		availabilityZones: []string{"us-central1-a", "us-central1-b", "us-central1-c"},
		ntpServers: []string{
			"metadata.google.internal", // GCP metadata server provides NTP
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "standard-rwo",
		flavors: FlavorDefaults{
			Bastion:       "e2-small",
			Master:        "e2-medium",
			Worker:        "e2-standard-4",
			WorkerWindows: "e2-standard-8",
		},
	}
}

// newGCPEuropeWest1Defaults returns defaults for GCP europe-west1 region.
func newGCPEuropeWest1Defaults() ProviderDefaults {
	return &gcpDefaults{
		imageIDs: map[string]string{
			"22": "projects/ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20240319",
			"24": "projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-v20240523",
		},
		availabilityZones: []string{"europe-west1-b", "europe-west1-c", "europe-west1-d"},
		ntpServers: []string{
			"metadata.google.internal", // GCP metadata server provides NTP
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "standard-rwo",
		flavors: FlavorDefaults{
			Bastion:       "e2-small",
			Master:        "e2-medium",
			Worker:        "e2-standard-4",
			WorkerWindows: "e2-standard-8",
		},
	}
}
