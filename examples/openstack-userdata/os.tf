# Copyright (c) HashiCorp, Inc.

data "openstack_networking_network_v2" "float_ip_network" {
  name = "cui_network"
}

data "openstack_networking_subnet_v2" "float_ip_subnet" {
  network_id = data.openstack_networking_network_v2.float_ip_network.id
  name       = "cui_network"
}

resource "openstack_networking_port_v2" "k8s_controller_ports" {
  name                  = "${module.labels.id}-server"
  network_id            = data.openstack_networking_network_v2.float_ip_network.id
  admin_state_up        = "true"
  port_security_enabled = false
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.float_ip_subnet.id
  }
}

resource "tls_private_key" "ssh_keys" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "openstack_compute_keypair_v2" "keypair" {
  name       = "${module.labels.id}-keypair"
  public_key = tls_private_key.ssh_keys.public_key_openssh
}

resource "openstack_compute_instance_v2" "k8s-controller" {
  name              = "${module.labels.id}-controller"
  key_pair          = openstack_compute_keypair_v2.keypair.name
  flavor_name       = "c4-m16-g0"
  security_groups   = []
  availability_zone = "nova"
  block_device {
    uuid                  = "0429c74d-e5bb-430f-b854-8d5fa98af8dd"
    source_type           = "image"
    volume_size           = 50
    boot_index            = 0
    destination_type      = "volume"
    delete_on_termination = true
  }

  network {
    port = openstack_networking_port_v2.k8s_controller_ports.id
  }
}
