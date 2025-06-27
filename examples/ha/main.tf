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

provider "openstack" {
  tenant_name = "terraform-provider-k3s"
}

variable "openstack" {
  type = any
}


module "infra" {
  source = "../modules/openstack-backend"

  name              = "ha"
  user              = var.openstack.user
  network_id        = var.openstack.network_id
  flavor            = var.openstack.flavor
  availability_zone = var.openstack.availability_zone
  image_id          = var.openstack.image_id
  nodes             = 7
}

resource "k3s_ha_server" "main" {
  user        = var.openstack.user
  private_key = module.infra.ssh_key

  node {
    host = module.infra.nodes[0]
  }
  node {
    host = module.infra.nodes[1]
  }
  node {
    host = module.infra.nodes[2]
  }
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
  content         = k3s_ha_server.main.kubeconfig
  filename        = "kubeconfig"
  file_permission = "0600"
}
