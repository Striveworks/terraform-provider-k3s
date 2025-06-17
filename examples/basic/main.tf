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

provider "k3s" {
  k3s_version = "v1.33.1+k3s1"
}
provider "openstack" {}

resource "tls_private_key" "ssh_keys" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "openstack_compute_keypair_v2" "keypair" {
  name       = "terraform-provider-k3s-basic-keypair"
  public_key = tls_private_key.ssh_keys.public_key_openssh
}

resource "k3s_server" "main" {
  host        = openstack_compute_instance_v2.k8s-controller.access_ip_v4
  user        = var.user
  private_key = tls_private_key.ssh_keys.private_key_openssh
}

resource "k3s_agent" "worker_node" {
  host        = openstack_compute_instance_v2.k8s_agent.access_ip_v4
  user        = "ubuntu"
  private_key = tls_private_key.ssh_keys.private_key_openssh
  server      = openstack_compute_instance_v2.k8s-controller.access_ip_v4
  token       = k3s_server.main.token
}

data "openstack_networking_network_v2" "float_ip_network" {
  name = var.network_id
}

data "openstack_networking_subnet_v2" "float_ip_subnet" {
  network_id = data.openstack_networking_network_v2.float_ip_network.id
  name       = var.network_id
}

resource "openstack_networking_port_v2" "k8s_controller_ports" {
  name                  = "terraform-provider-k3s-basic-server"
  network_id            = data.openstack_networking_network_v2.float_ip_network.id
  admin_state_up        = "true"
  port_security_enabled = false
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.float_ip_subnet.id
  }
}

resource "openstack_networking_port_v2" "k8s_agent_ports" {
  name                  = "terraform-provider-k3s-basic-agent"
  network_id            = data.openstack_networking_network_v2.float_ip_network.id
  admin_state_up        = "true"
  port_security_enabled = false
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.float_ip_subnet.id
  }
}

resource "openstack_compute_instance_v2" "k8s-controller" {
  name              = "terraform-provider-k3s-basic-controller"
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
    port = openstack_networking_port_v2.k8s_controller_ports.id
  }
}

resource "openstack_compute_instance_v2" "k8s_agent" {
  name              = "terraform-provider-k3s-basic-agent"
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
    port = openstack_networking_port_v2.k8s_agent_ports.id
  }
}

// Easy access just in case

resource "local_file" "ssh_key" {
  content         = tls_private_key.ssh_keys.private_key_openssh
  filename        = "key.pem"
  file_permission = "0600"
}

resource "local_file" "ssh_cmd" {
  content         = <<EOF
#!/bin/bash
ssh -i key.pem ${var.user}@${openstack_compute_instance_v2.k8s-controller.access_ip_v4}
EOF
  filename        = "connect.sh"
  file_permission = "0600"
}
