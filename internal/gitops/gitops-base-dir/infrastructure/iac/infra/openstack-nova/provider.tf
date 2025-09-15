provider "openstack" {
  auth_url     = var.openstack_auth_url
  cacert_file  = var.openstack_ca
  delayed_auth = true
  insecure     = var.openstack_insecure
  password     = var.openstack_password
  region       = var.openstack_region
  tenant_name  = var.openstack_tenant_name
  use_octavia  = true
  user_name    = var.openstack_user_name
  user_domain_name = var.openstack_user_domain_name
  project_domain_name = var.openstack_project_domain_name
  application_credential_id = var.application_credential_id
  application_credential_secret = var.application_credential_secret
}

