terraform {
  backend "s3" {
    bucket       = "{{ .OpenTofu.Backend.S3.Bucket | default "1270657-rackspace-gdo" }}"
    key          = "{{ .OpenTofu.Backend.S3.Key | default "gdo.prod.sjc3/tfstate/terraform.tfstate" }}"
    region       = "{{ .OpenTofu.Backend.S3.Region | default "us-west-2" }}"
    use_lockfile = true
    encrypt      = true
  }
}
