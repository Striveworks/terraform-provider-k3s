// Terraform Control
terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~>3.0.0"
    }
  }
}

// Providers
provider "openstack" {
  tenant_name = "terraform-provider-k3s"
}

// Namings
module "labels" {
  source  = "cloudposse/label/null"
  version = "0.25.0"

  namespace   = "tf"
  name        = "provider"
  environment = "k3s"
  stage       = var.name
}

// Variables

variable "user" {
  description = "User for target host"
  type        = string
}

variable "name" {
  type = string
}

variable "network_id" {
  description = "Network ID"
  type        = string
}

variable "flavor" {
  description = "Compute flavor"
  type        = string
}

variable "availability_zone" {
  type = string
}

variable "image_id" {
  type = string
}

// Resources

module "infra" {
  source = "../../examples/modules/openstack-backend"

  name              = "ha"
  user              = var.user
  network_id        = var.network_id
  flavor            = var.flavor
  availability_zone = var.availability_zone
  image_id          = var.image_id
  nodes             = 7
}

// Outputs

output "ssh_key" {
  value     = module.infra.ssh_key
  sensitive = true
}

output "nodes" {
  value = module.infra.nodes
}

output "user" {
  value     = var.user
  sensitive = true
}

resource "local_sensitive_file" "kubeconfig" {
  content         = module.infra.ssh_key
  filename        = "key.pem"
  file_permission = "0600"
}
