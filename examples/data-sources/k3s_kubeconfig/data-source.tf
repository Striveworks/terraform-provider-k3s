variable "host" {
  type = string
}

variable "user" {
  type = string
}

variable "private_key" {
  type      = string
  sensitive = true
  default   = null
}

variable "private_key_file" {
  type      = string
  sensitive = true
  default   = null
}

variable "ssh_port" {
  type    = number
  default = 22
}

variable "kubeconfig_hostname" {
  type    = string
  default = "mylb-dns-name"
}

variable "config" {
  type    = string
  default = null
}

resource "k3s_server" "main" {
  auth = {
    host                         = var.host
    user                         = var.user
    port                         = var.ssh_port
    private_key                  = var.private_key
    private_key_file             = var.private_key_file
    ignore_host_key_verification = true
  }
  config = var.config
}

data "k3s_kubeconfig" "kubeconfig" {
  auth = {
    host                         = var.host
    user                         = var.user
    port                         = var.ssh_port
    private_key                  = var.private_key
    private_key_file             = var.private_key_file
    ignore_host_key_verification = true
  }
  hostname = var.kubeconfig_hostname

  depends_on = [k3s_server.main]
}

output "kubeconfig" {
  value     = data.k3s_kubeconfig.kubeconfig.kubeconfig
  sensitive = true
}

output "k3s_url" {
  value = data.k3s_kubeconfig.kubeconfig.k3s_url
}

output "cluster_auth_server" {
  value = data.k3s_kubeconfig.kubeconfig.cluster_auth.server
}
