package config

import _ "embed"

// Default content for iac.main_tf seeded from defaults.
//
//go:embed defaults/main.tf
var defaultMainTF string

// Default structured IaC values (locals as main, and modules) parsed from Terraform
// and expressed as YAML. Used to seed iac.main and iac.modules during cluster init.
//
//go:embed defaults/openstack.yaml
var defaultIACYAML string
