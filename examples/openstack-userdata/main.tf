# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    k3s = {
      source = "striveworks.us/openstack/k3s"
    }
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~>3.0.0"
    }
  }
}

provider "k3s" {}

resource "k3s_server" "main" {
  host        = openstack_compute_instance_v2.k8s-controller.access_ip_v4
  user        = "ubuntu"
  private_key = tls_private_key.ssh_keys.private_key_openssh
}

provider "openstack" {
  tenant_name = "IT"
  auth_url    = "https://openstack.striveworks.us:5000"
  region      = "RegionOne"
}

module "labels" {
  source  = "cloudposse/label/null"
  version = "0.25.0"

  namespace   = "sw"
  name        = "main"
  environment = "os"
}
