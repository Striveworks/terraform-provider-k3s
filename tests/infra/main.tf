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
  cloud = "terraform-k3s-provider"
}

resource "tls_private_key" "ssh_keys" {
  algorithm = "RSA"
  rsa_bits  = 4096
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

module "server_tests" {
  source = "./modules/openstack-backend"

  name              = "server-test"
  user              = var.user
  network_id        = var.network_id
  flavor            = var.flavor
  availability_zone = var.availability_zone
  image_id          = var.image_id
  ssh_keys          = tls_private_key.ssh_keys
  nodes             = 8
}

module "agent_tests" {
  source = "./modules/openstack-backend"

  name              = "agent-test"
  user              = var.user
  network_id        = var.network_id
  flavor            = var.flavor
  availability_zone = var.availability_zone
  image_id          = var.image_id
  ssh_keys          = tls_private_key.ssh_keys
  nodes             = 8
}

// Outputs

output "ssh_key" {
  value     = tls_private_key.ssh_keys.private_key_openssh
  sensitive = true
}

output "nodes" {
  value = {
    server = module.server_tests.nodes
    agent  = module.agent_tests.nodes
  }
}

output "user" {
  value     = var.user
  sensitive = true
}

resource "local_sensitive_file" "kubeconfig" {
  content         = tls_private_key.ssh_keys.private_key_openssh
  filename        = "key.pem"
  file_permission = "0600"
}
