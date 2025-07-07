variable "server_host" {
  type = string
}

variable "agent_hosts" {
  type = list(string)
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
  host        = var.server_host
  user        = var.user
  private_key = var.private_key
  config      = var.config
}

resource "k3s_agent" "main" {
  count = length(var.agent_hosts)

  host        = var.agent_hosts[count.index]
  user        = var.user
  private_key = var.private_key
  server      = k3s_server.main.host
  token       = k3s_server.main.token
  config      = var.config
}

