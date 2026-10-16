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

// openstackDefaults implements ProviderDefaults for OpenStack regions.
type openstackDefaults struct {
	imageIDs          map[string]string
	availabilityZones []string
	ntpServers        []string
	dnsNameservers    []string
	storageClass      string
	flavors           FlavorDefaults
}

func (d *openstackDefaults) GetImageID(osVersion string) string {
	if imageID, ok := d.imageIDs[osVersion]; ok {
		return imageID
	}
	// Return default Ubuntu 24.04 image if version not found
	return d.imageIDs["24"]
}

func (d *openstackDefaults) GetAvailabilityZones() []string {
	return d.availabilityZones
}

func (d *openstackDefaults) GetNTPServers() []string {
	return d.ntpServers
}

func (d *openstackDefaults) GetDNSNameservers() []string {
	return d.dnsNameservers
}

func (d *openstackDefaults) GetDefaultStorageClass() string {
	return d.storageClass
}

func (d *openstackDefaults) GetDefaultFlavors() FlavorDefaults {
	return d.flavors
}

// newOpenStackSJC3Defaults returns defaults for OpenStack SJC3 region.
func newOpenStackSJC3Defaults() ProviderDefaults {
	return &openstackDefaults{
		imageIDs:          map[string]string{}, // images are project-scoped; resolved via `cluster provider openstack plan`
		availabilityZones: []string{"az1", "az2", "az3"},
		ntpServers: []string{
			"time.sjc3.rackspace.com",
			"time2.sjc3.rackspace.com",
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "csi-cinder-sc-delete",
		flavors: FlavorDefaults{
			Bastion:       "gp.5.2.4",
			Master:        "gp.5.4.8",
			Worker:        "gp.5.4.16",
			WorkerWindows: "gp.5.4.16",
		},
	}
}

// newOpenStackDFW3Defaults returns defaults for OpenStack DFW3 region.
func newOpenStackDFW3Defaults() ProviderDefaults {
	return &openstackDefaults{
		imageIDs:          map[string]string{}, // images are project-scoped; resolved via `cluster provider openstack plan`
		availabilityZones: []string{"az1", "az2", "az3"},
		ntpServers: []string{
			"time.dfw3.rackspace.com",
			"time2.dfw3.rackspace.com",
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "csi-cinder-sc-delete",
		flavors: FlavorDefaults{
			Bastion:       "gp.5.2.4",
			Master:        "gp.5.4.8",
			Worker:        "gp.5.4.16",
			WorkerWindows: "gp.5.4.16",
		},
	}
}

// newOpenStackIAD3Defaults returns defaults for OpenStack IAD3 region.
func newOpenStackIAD3Defaults() ProviderDefaults {
	return &openstackDefaults{
		imageIDs:          map[string]string{}, // images are project-scoped; resolved via `cluster provider openstack plan`
		availabilityZones: []string{"az1", "az2", "az3"},
		ntpServers: []string{
			"time.iad3.rackspace.com",
			"time2.iad3.rackspace.com",
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "csi-cinder-sc-delete",
		flavors: FlavorDefaults{
			Bastion:       "gp.5.2.4",
			Master:        "gp.5.4.8",
			Worker:        "gp.5.4.16",
			WorkerWindows: "gp.5.4.16",
		},
	}
}

// newOpenStackORD1Defaults returns defaults for OpenStack ORD1 region.
func newOpenStackORD1Defaults() ProviderDefaults {
	return &openstackDefaults{
		imageIDs:          map[string]string{}, // images are project-scoped; resolved via `cluster provider openstack plan`
		availabilityZones: []string{"az1", "az2", "az3"},
		ntpServers: []string{
			"time.ord1.rackspace.com",
			"time2.ord1.rackspace.com",
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "csi-cinder-sc-delete",
		flavors: FlavorDefaults{
			Bastion:       "gp.5.2.4",
			Master:        "gp.5.4.8",
			Worker:        "gp.5.4.16",
			WorkerWindows: "gp.5.4.16",
		},
	}
}
