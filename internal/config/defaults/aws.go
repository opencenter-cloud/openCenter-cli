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

// awsDefaults implements ProviderDefaults for AWS regions.
type awsDefaults struct {
	amiIDs            map[string]string
	availabilityZones []string
	ntpServers        []string
	dnsNameservers    []string
	storageClass      string
	flavors           FlavorDefaults
}

func (d *awsDefaults) GetImageID(osVersion string) string {
	if amiID, ok := d.amiIDs[osVersion]; ok {
		return amiID
	}
	// Return default Ubuntu 24.04 AMI if version not found
	return d.amiIDs["24"]
}

func (d *awsDefaults) GetAvailabilityZones() []string {
	return d.availabilityZones
}

func (d *awsDefaults) GetNTPServers() []string {
	return d.ntpServers
}

func (d *awsDefaults) GetDNSNameservers() []string {
	return d.dnsNameservers
}

func (d *awsDefaults) GetDefaultStorageClass() string {
	return d.storageClass
}

func (d *awsDefaults) GetDefaultFlavors() FlavorDefaults {
	return d.flavors
}

// newAWSUSEast1Defaults returns defaults for AWS us-east-1 region.
func newAWSUSEast1Defaults() ProviderDefaults {
	return &awsDefaults{
		amiIDs: map[string]string{
			"22": "ami-0c55b159cbfafe1f0", // Ubuntu 22.04 LTS
			"24": "ami-0e2c8caa4b6378d8c", // Ubuntu 24.04 LTS
		},
		availabilityZones: []string{"us-east-1a", "us-east-1b", "us-east-1c"},
		ntpServers: []string{
			"169.254.169.123", // AWS Time Sync Service
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "gp3",
		flavors: FlavorDefaults{
			Bastion:       "t3.small",
			Master:        "t3.medium",
			Worker:        "t3.large",
			WorkerWindows: "t3.xlarge",
		},
	}
}

// newAWSUSWest2Defaults returns defaults for AWS us-west-2 region.
func newAWSUSWest2Defaults() ProviderDefaults {
	return &awsDefaults{
		amiIDs: map[string]string{
			"22": "ami-0d70546e43a941d70", // Ubuntu 22.04 LTS
			"24": "ami-0cf2b4e024cdb6960", // Ubuntu 24.04 LTS
		},
		availabilityZones: []string{"us-west-2a", "us-west-2b", "us-west-2c"},
		ntpServers: []string{
			"169.254.169.123", // AWS Time Sync Service
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "gp3",
		flavors: FlavorDefaults{
			Bastion:       "t3.small",
			Master:        "t3.medium",
			Worker:        "t3.large",
			WorkerWindows: "t3.xlarge",
		},
	}
}

// newAWSEUWest1Defaults returns defaults for AWS eu-west-1 region.
func newAWSEUWest1Defaults() ProviderDefaults {
	return &awsDefaults{
		amiIDs: map[string]string{
			"22": "ami-0905a3c97561e0b69", // Ubuntu 22.04 LTS
			"24": "ami-0d64bb532e0502c46", // Ubuntu 24.04 LTS
		},
		availabilityZones: []string{"eu-west-1a", "eu-west-1b", "eu-west-1c"},
		ntpServers: []string{
			"169.254.169.123", // AWS Time Sync Service
		},
		dnsNameservers: []string{"8.8.8.8", "8.8.4.4"},
		storageClass:   "gp3",
		flavors: FlavorDefaults{
			Bastion:       "t3.small",
			Master:        "t3.medium",
			Worker:        "t3.large",
			WorkerWindows: "t3.xlarge",
		},
	}
}
