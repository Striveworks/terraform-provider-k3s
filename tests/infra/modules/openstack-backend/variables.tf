variable "user" {
  description = "User for target host"
  type        = string
}

variable "name" {
  type = string
}

variable "network_id" {
  description = "Network ID"
  type        = string
}

variable "flavor" {
  description = "Compute flavor"
  type        = string
}

variable "availability_zone" {
  type = string
}

variable "image_id" {
  type = string
}

variable "nodes" {
  type    = number
  default = 4
}
