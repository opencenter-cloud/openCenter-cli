variable "os_application_credential_id" {
  type    = string
  default = ""
}

variable "os_application_credential_secret" {
  type    = string
  default = ""
}

variable "openstack_admin_name" {
  type    = string
  default = "admin"
}

variable "openstack_admin_password" {
  type    = string
  default = ""
}

variable "openstack_user_name" {
  type    = string
  default = ""
}

variable "openstack_user_password" {
  type    = string
  default = ""
}

variable "pf9_account_url" {
  type    = string
  default = ""
}

variable "pf9_username" {
  type    = string
  default = ""
}

variable "pf9_password" {
  type    = string
  default = ""
}

variable "worker_count" {
  type    = string
  default = "1"

}

variable "master_count" {
  type    = string
  default = "3"
}

variable "windows_admin_password" {
  type    = string
  default = ""
}