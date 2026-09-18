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

package gitops

import (
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

type metallbMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type metallbIPAddressPoolManifest struct {
	APIVersion string                   `yaml:"apiVersion"`
	Kind       string                   `yaml:"kind"`
	Metadata   metallbMetadata          `yaml:"metadata"`
	Spec       metallbIPAddressPoolSpec `yaml:"spec"`
}

type metallbIPAddressPoolSpec struct {
	Addresses     []string `yaml:"addresses"`
	AutoAssign    *bool    `yaml:"autoAssign,omitempty"`
	AvoidBuggyIPs bool     `yaml:"avoidBuggyIPs,omitempty"`
}

type metallbL2AdvertisementManifest struct {
	APIVersion string                     `yaml:"apiVersion"`
	Kind       string                     `yaml:"kind"`
	Metadata   metallbMetadata            `yaml:"metadata"`
	Spec       metallbL2AdvertisementSpec `yaml:"spec"`
}

type metallbL2AdvertisementSpec struct {
	IPAddressPools []string `yaml:"ipAddressPools,omitempty"`
	Interfaces     []string `yaml:"interfaces,omitempty"`
}

func metallbOverlayFilesRenderer(cfg v2.Config) (map[string]string, error) {
	serviceValue, exists := cfg.OpenCenter.Services["metallb"]
	if !exists || serviceValue == nil {
		return map[string]string{}, nil
	}
	service, ok := serviceValue.(*services.MetalLBConfig)
	if !ok {
		return nil, fmt.Errorf("metallb service has unexpected configuration type %T", serviceValue)
	}
	if !service.Enabled {
		return map[string]string{}, nil
	}
	if err := v2.NewValidator().ValidateServices(&cfg); err != nil {
		return nil, err
	}

	namespace := service.Namespace
	if namespace == "" {
		namespace = "metallb-system"
	}
	files := make(map[string]string)
	if len(service.IPAddressPools) > 0 {
		documents := make([]any, 0, len(service.IPAddressPools))
		for _, pool := range service.IPAddressPools {
			autoAssign := pool.AutoAssign
			documents = append(documents, metallbIPAddressPoolManifest{
				APIVersion: "metallb.io/v1beta1",
				Kind:       "IPAddressPool",
				Metadata: metallbMetadata{
					Name:      pool.Name,
					Namespace: namespace,
				},
				Spec: metallbIPAddressPoolSpec{
					Addresses:     pool.Addresses,
					AutoAssign:    autoAssign,
					AvoidBuggyIPs: pool.AvoidBuggyIPs,
				},
			})
		}
		content, err := marshalMetalLBDocuments(documents)
		if err != nil {
			return nil, fmt.Errorf("marshal IPAddressPool resources: %w", err)
		}
		files["ipaddresspool.yaml"] = content
	}
	if len(service.L2Advertisements) > 0 {
		documents := make([]any, 0, len(service.L2Advertisements))
		for _, advertisement := range service.L2Advertisements {
			if advertisement.GetType() != services.L2AdvertisementType {
				continue
			}
			documents = append(documents, metallbL2AdvertisementManifest{
				APIVersion: "metallb.io/v1beta1",
				Kind:       "L2Advertisement",
				Metadata: metallbMetadata{
					Name:      advertisement.Name,
					Namespace: namespace,
				},
				Spec: metallbL2AdvertisementSpec{
					IPAddressPools: advertisement.IPAddressPools,
					Interfaces:     advertisement.Interfaces,
				},
			})
		}
		if len(documents) == 0 {
			return files, nil
		}
		content, err := marshalMetalLBDocuments(documents)
		if err != nil {
			return nil, fmt.Errorf("marshal L2Advertisement resources: %w", err)
		}
		files["l2advertisement.yaml"] = content
	}
	return files, nil
}

func marshalMetalLBDocuments(documents []any) (string, error) {
	var output strings.Builder
	for _, document := range documents {
		data, err := yaml.Marshal(document)
		if err != nil {
			return "", err
		}
		output.WriteString("---\n")
		output.Write(data)
	}
	return output.String(), nil
}
