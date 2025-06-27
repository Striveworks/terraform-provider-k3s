terraform {
  required_providers {
    k3s = {
      source = "striveworks/k3s"
    }
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~>3.0.0"
    }
  }
}

provider "k3s" {}

provider "openstack" {}

variable "openstack" {
  type = any
}

module "infra" {
  source = "../modules/openstack-backend"

  name              = "basic"
  user              = var.openstack.user
  network_id        = var.openstack.network_id
  flavor            = var.openstack.flavor
  availability_zone = var.openstack.availability_zone
  image_id          = var.openstack.image_id
  nodes             = 3
}

resource "k3s_server" "main" {
  host        = module.infra.nodes[0]
  user        = var.openstack.user
  private_key = module.infra.ssh_key
  config      = <<EOT
embedded-registry: true
EOT
}

resource "k3s_agent" "agent" {
  count       = 2
  host        = module.infra.nodes[count.index + 1]
  server      = module.infra.nodes[0]
  token       = k3s_server.main.token
  user        = var.openstack.user
  private_key = module.infra.ssh_key
}

resource "local_file" "ssh_key" {
  content         = module.infra.ssh_key
  filename        = "key.pem"
  file_permission = "0600"
}

output "nodes" {
  value = module.infra.nodes
}

resource "local_sensitive_file" "kubeconfig" {
  content         = k3s_server.main.kubeconfig
  filename        = "kubeconfig"
  file_permission = "0600"
}
