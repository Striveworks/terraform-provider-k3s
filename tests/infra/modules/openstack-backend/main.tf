// Terraform Control
terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~>3.0.0"
    }
  }
}

module "labels" {
  source  = "cloudposse/label/null"
  version = "0.25.0"

  namespace   = "tf"
  name        = "provider"
  environment = "k3s"
  stage       = var.name
}


// Resources


resource "openstack_compute_keypair_v2" "keypair" {
  name       = "${module.labels.id}-keypair"
  public_key = var.ssh_keys.public_key_openssh
}

data "openstack_networking_network_v2" "float_ip_network" {
  name = var.network_id
}

data "openstack_networking_subnet_v2" "float_ip_subnet" {
  network_id = data.openstack_networking_network_v2.float_ip_network.id
  name       = var.network_id
}

resource "openstack_networking_port_v2" "k8s_port" {
  count                 = var.nodes
  name                  = "${module.labels.id}-node-${count.index}"
  network_id            = data.openstack_networking_network_v2.float_ip_network.id
  admin_state_up        = "true"
  port_security_enabled = false
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.float_ip_subnet.id
  }
}

resource "openstack_compute_instance_v2" "k8s_node" {
  count             = var.nodes
  name              = "${module.labels.id}-node-${count.index}"
  key_pair          = openstack_compute_keypair_v2.keypair.name
  flavor_name       = var.flavor
  security_groups   = []
  availability_zone = var.availability_zone
  block_device {
    uuid                  = var.image_id
    source_type           = "image"
    volume_size           = 50
    boot_index            = 0
    destination_type      = "volume"
    delete_on_termination = true
  }

  network {
    port = openstack_networking_port_v2.k8s_port[count.index].id
  }
}
