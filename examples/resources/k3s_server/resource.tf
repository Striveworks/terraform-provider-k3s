// Simple mode

variable "host" {
  type    = string
  default = ""
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
  count       = var.host != "" ? 1 : 0
  host        = var.host
  user        = var.user
  private_key = var.private_key
  config      = var.config
}

// HA Server mode

variable "hosts" {
  type    = list(string)
  default = []
}


resource "k3s_server" "init" {
  count       = length(var.hosts) > 0 ? 1 : 0
  host        = var.hosts[0]
  user        = var.user
  private_key = var.private_key
  config      = var.config
  highly_available = {
    cluster_init = true
  }
}

resource "k3s_server" "join" {
  count = length(var.hosts) > 0 ? length(var.hosts) - 1 : 0

  host        = var.hosts[count.index + 1]
  user        = var.user
  private_key = var.private_key
  config      = var.config
  highly_available = {
    token  = k3s_server.init[0].token
    server = k3s_server.init[0].server
  }
}
