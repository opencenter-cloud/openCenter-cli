package v2

func newValidV2TestConfig(provider string) *Config {
	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Meta: MetaConfig{
				Name:         "test-cluster",
				Organization: "test-org",
				Env:          "dev",
				Region:       "sjc3",
			},
			Cluster: ClusterConfig{
				ClusterName: "test-cluster",
				BaseDomain:  "example.com",
				ClusterFQDN: "test-cluster.example.com",
				AdminEmail:  "admin@example.com",
				Kubernetes: KubernetesConfig{
					Version:        "1.28.0",
					APIPort:        6443,
					SubnetPods:     "10.233.64.0/18",
					SubnetServices: "10.233.0.0/18",
					NetworkPlugin: NetworkPluginConfig{
						Calico: &CalicoConfig{
							Enabled:       true,
							Version:       "3.28.0",
							NetworkPolicy: true,
						},
					},
				},
			},
			Infrastructure: InfrastructureConfig{
				Provider:  provider,
				OSVersion: "24",
				SSH: SSHConfig{
					AuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCTestKey"},
				},
				Networking: NetworkingConfig{
					SubnetNodes:          "10.2.128.0/22",
					AllocationPoolStart:  "10.2.128.10",
					AllocationPoolEnd:    "10.2.131.254",
					VRRPEnabled:          true,
					VRRPIP:               "10.2.128.5",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          "cluster.local",
					DNSNameservers:       []string{"8.8.8.8", "8.8.4.4"},
					NTPServers:           []string{"time.google.com"},
				},
				Compute: ComputeConfig{
					FlavorMaster: "m1.medium",
					FlavorWorker: "m1.large",
					MasterCount:  3,
					WorkerCount:  2,
				},
				Storage: StorageConfig{
					DefaultStorageClass:         "standard",
					WorkerVolumeSize:            50,
					WorkerVolumeDestinationType: "volume",
					WorkerVolumeSourceType:      "image",
					WorkerVolumeType:            "ssd",
				},
			},
			GitOps: GitOpsConfig{
				Repository: GitOpsRepository{
					URL:    "ssh://git@github.com/example/repo.git",
					Branch: "main",
					Path:   "clusters/test-cluster",
				},
				Flux: GitOpsFluxConfig{
					Interval: "15m",
					Prune:    true,
				},
			},
		},
		Deployment: DeploymentConfig{
			AutoDeploy: false,
			Method:     "kubespray",
		},
		OpenTofu: OpenTofuConfig{
			Backend: BackendConfig{
				Type: "local",
				Local: &LocalBackendConfig{
					Path: "/tmp/terraform.tfstate",
				},
			},
		},
		Secrets: SecretsConfig{
			Global: GlobalSecrets{},
		},
	}

	switch provider {
	case "aws":
		cfg.OpenCenter.Meta.Region = "us-east-1"
		cfg.OpenCenter.Infrastructure.Cloud.AWS = &AWSCloudConfig{
			Region:            "us-east-1",
			VPCID:             "vpc-12345",
			SubnetIDs:         []string{"subnet-12345"},
			AMIID:             "ami-12345",
			AvailabilityZones: []string{"us-east-1a"},
		}
	case "gcp":
		cfg.OpenCenter.Meta.Region = "us-central1"
		cfg.OpenCenter.Infrastructure.Cloud.GCP = &GCPCloudConfig{
			Project:     "project-12345",
			Region:      "us-central1",
			Network:     "default",
			Subnetwork:  "default",
			ImageFamily: "ubuntu-2204-lts",
		}
	case "azure":
		cfg.OpenCenter.Meta.Region = "eastus"
		cfg.OpenCenter.Infrastructure.Cloud.Azure = &AzureCloudConfig{
			SubscriptionID: "sub-12345",
			ResourceGroup:  "rg-opencenter",
			Location:       "eastus",
			VNetName:       "opencenter-vnet",
			SubnetName:     "default",
			ImageReference: "Canonical:UbuntuServer:22_04-lts:latest",
		}
	case "baremetal":
		cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{}
	case "vmware", "vsphere":
		cfg.OpenCenter.Infrastructure.Cloud.VMware = &VMwareCloudConfig{
			VCenterServer: "vcenter.example.com",
			Datacenter:    "Datacenter1",
			Cluster:       "Cluster1",
			Datastore:     "datastore1",
			Network:       "VM Network",
			Template:      "ubuntu-2404-template",
			Folder:        "/vm/opencenter",
		}
	default:
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &OpenStackCloudConfig{
			AuthURL:           "https://identity.api.rackspacecloud.com/v3",
			Region:            "sjc3",
			ProjectID:         "project-12345",
			ProjectName:       "test-project",
			UserDomainName:    "default",
			ProjectDomainName: "default",
			ImageID:           "image-12345",
			NetworkID:         "network-12345",
			SubnetID:          "subnet-12345",
			AvailabilityZones: []string{"az1"},
		}
	}

	return cfg
}
