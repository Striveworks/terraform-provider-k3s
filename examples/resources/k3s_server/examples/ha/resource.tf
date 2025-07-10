variable "hosts" {
  type    = list(string)
  default = []

  validation {
    condition     = length(var.hosts) > 1 && length(var.hosts) % 2 != 0
    error_message = "Ensure more than 3 and odd nodes"
  }
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

resource "k3s_server" "init" {
  host        = var.hosts[0]
  user        = var.user
  private_key = var.private_key
  config      = var.config
  highly_available = {
    cluster_init = true
  }
}

resource "k3s_server" "join" {
  count = length(var.hosts) - 1

  host        = var.hosts[count.index + 1]
  user        = var.user
  private_key = var.private_key
  config      = var.config
  highly_available = {
    token      = k3s_server.init.token
    server     = k3s_server.init.server
    kubeconfig = k3s_server.init.kubeconfig
  }
}
