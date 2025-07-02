variable "host" {
  type    = string
  default = ""
}

variable "user" {
  type    = string
  default = ""
}

variable "private_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "config" {
  type    = string
  default = null
}

resource "k3s_server" "main" {
  host        = var.host
  user        = var.user
  private_key = var.private_key
  config      = var.config
}
