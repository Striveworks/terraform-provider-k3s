variable "host" {
  type = string
}

variable "user" {
  type = string
}

variable "private_key" {
  type      = string
  sensitive = true
}

variable "config" {
  type    = string
  default = null
}

resource "k3s_server" "main" {
  auth = {
    host        = var.host
    user        = var.user
    private_key = var.private_key
  }
  config = var.config
}

data "k3s_kubeconfig" "kubeconfig" {
  auth = {
    host        = var.host
    user        = var.user
    private_key = var.private_key
  }
  hostname = "mylb-dns-name"

  depends_on = [k3s_server.main]
}

output "kubeconfig" {
  value = data.k3s_kubeconfig.kubeconfig
}
