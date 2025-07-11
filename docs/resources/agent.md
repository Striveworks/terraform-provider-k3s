---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "k3s_agent Resource - k3s"
subcategory: ""
description: |-
  Creates a k3s agent resource. Only one of password or private_key can be passed. Requires a token and server address to a k3s_server resource
---

# k3s_agent (Resource)

Creates a k3s agent resource. Only one of `password` or `private_key` can be passed. Requires a token and server address to a k3s_server resource

## Example Usage

```terraform
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
  kubeconfig  = k3s_server.main.kubeconfig
  server      = k3s_server.main.server
  token       = k3s_server.main.token
  config      = var.config
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `host` (String) Hostname of the target server
- `kubeconfig` (String) KubeConfig for the cluster, needed so agent node can clean itself up
- `server` (String) Hostname for k3s api server
- `token` (String, Sensitive) Server token used for joining nodes to the cluster
- `user` (String) Username of the target server

### Optional

- `bin_dir` (String) Value of a path used to put the k3s binary
- `config` (String) K3s server config
- `password` (String, Sensitive) Username of the target server
- `port` (Number) Override default SSH port (22)
- `private_key` (String, Sensitive) Value of a privatekey used to auth

### Read-Only

- `active` (Boolean) The health of the server
- `id` (String) Id of the k3s server resource

## Import

Import is supported using the following syntax:

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
# Import with Password
tofu import k3s_agent.main "host=192.168.10.1,user=ubuntu,password=$PASS"

# Import with key
tofu import k3s_agent.main "host=192.168.10.1,user=ubuntu,private_key=$SSH_KEY"
```
