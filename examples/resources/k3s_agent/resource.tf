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

variable "config" {
  type    = string
  default = null
}

resource "k3s_server" "main" {
  auth = {
    host             = var.server_host
    user             = var.user
    port             = var.ssh_port
    private_key      = var.private_key
    private_key_file = var.private_key_file
  }
  config = var.config
}

resource "k3s_agent" "main" {
  count = length(var.agent_hosts)

  auth = {
    host             = var.agent_hosts[count.index]
    user             = var.user
    port             = var.ssh_port
    private_key      = var.private_key
    private_key_file = var.private_key_file
  }

  server = k3s_server.main.server
  token  = k3s_server.main.token
  config = var.config
}
