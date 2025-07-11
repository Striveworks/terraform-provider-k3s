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
  host        = var.host
  user        = var.user
  private_key = var.private_key
  config      = var.config
}
