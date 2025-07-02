variable "server_host" {
  type    = string
  default = ""
}

variable "agent_host_1" {
  type    = string
  default = ""
}

variable "agent_host_2" {
  type    = string
  default = ""
}

variable "user" {
  type    = string
  default = "ubuntu"
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
  host        = var.agent_host_1
  user        = var.user
  private_key = var.private_key
  server      = var.server_host
  token       = k3s_server.main.token
  config      = var.config
}

resource "k3s_agent" "secondary" {
  count       = var.agent_host_2 == "" ? 0 : 1
  host        = var.agent_host_2
  user        = var.user
  private_key = var.private_key
  server      = var.server_host
  token       = k3s_server.main.token
  config      = var.config
}
