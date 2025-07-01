// Basic example with ssh key
variable "ssk_key" {
  type      = string
  sensitive = true
}

resource "k3s_server" "main" {
  host        = "192.168.10.1"
  user        = "ubuntu"
  private_key = var.ssk_key
}

// Basic example with password
variable "password" {
  type      = string
  sensitive = true
}

resource "k3s_server" "main" {
  host     = "192.168.10.1"
  user     = "ubuntu"
  password = var.password
}

// Pass k3s custom options with config.yaml
variable "password" {
  type      = string
  sensitive = true
}

resource "k3s_server" "main" {
  host     = "192.168.10.1"
  user     = "ubuntu"
  password = var.password
  config   = <<EOT
write-kubeconfig-mode: 600
node-taint:
  - alice=bob:NoExecute
EOT
}
