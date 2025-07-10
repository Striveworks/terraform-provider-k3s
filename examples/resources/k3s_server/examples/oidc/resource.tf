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

variable "oidc_config" {
  type = object({
    audience    = string
    pkcs8       = string
    signing_key = string
    issuer      = string
  })
  sensitive = true
}

resource "k3s_server" "init" {
  host        = var.host
  user        = var.user
  private_key = var.private_key
  config      = var.config
  oidc_config = var.oidc_config
}

output "jwks" {
  value     = k3s_server.init.oidc_config.jwks
  sensitive = true
}
