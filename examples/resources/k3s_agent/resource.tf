// Basic example with one agent node
variable "ssk_key" {
  type      = string
  sensitive = true
}

variable "password" {
  type      = string
  sensitive = true
}

resource "k3s_server" "main" {
  host        = "192.168.10.1"
  user        = "ubuntu"
  private_key = var.ssk_key
  config      = <<EOT
node-taint:
  - alice=john:NoExecute
EOT
}

resource "k3s_agent" "main" {
  host     = "192.168.10.2"
  user     = "ubuntu"
  password = var.password
  token    = k3s_server.token
  server   = k3s_server.host
  config   = <<EOT
node-taint:
  - alice=bob:NoExecute
EOT
}
